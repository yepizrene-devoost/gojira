package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

// attachServer stands in for Jira: it serves the attachment settings and
// records every upload so the tests can assert what reached the wire.
type attachServer struct {
	mu      sync.Mutex
	srv     *httptest.Server
	reads   int
	posts   int
	files   map[string]string
	status  int
	enabled bool
}

func newAttachServer(t *testing.T, status int) *attachServer {
	t.Helper()
	s := &attachServer{files: map[string]string{}, status: status, enabled: true}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if r.URL.Path == "/rest/api/3/attachment/meta" {
			s.reads++
			_, _ = fmt.Fprintf(w, `{"enabled":%t,"uploadLimit":1000000}`, s.enabled)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/3/issue/A-1/attachments" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		s.posts++
		if r.Header.Get("X-Atlassian-Token") != "no-check" {
			t.Errorf("upload missing X-Atlassian-Token header")
		}
		if s.status != http.StatusOK {
			w.WriteHeader(s.status)
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("upload is not multipart: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f, header, err := r.FormFile("file")
		if err != nil {
			t.Errorf("multipart part \"file\" missing: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		body, _ := io.ReadAll(f)
		s.files[header.Filename] = string(body)
		_ = json.NewEncoder(w).Encode([]jira.Attachment{{ID: "1", Filename: header.Filename, Size: int64(len(body)), MimeType: "text/plain"}})
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *attachServer) counts() (reads, posts int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reads, s.posts
}

func attachDetailModel(t *testing.T, srv *attachServer) Model {
	t.Helper()
	issue := testIssue("A-1")
	return Model{
		client:         jira.NewClient(srv.srv.URL, "email", "token"),
		view:           viewDetail,
		detail:         &issue,
		detailIssueKey: issue.Key,
		width:          190,
		height:         50,
	}
}

func typeText(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		m, _ = updateModel(t, m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return m
}

// visibleText returns what the user reads: ANSI and the modal border removed
// and every run of whitespace (including wrapping line breaks) collapsed to
// one space. Wrapping can still split a long unbroken path, so assert only on
// phrases made of ordinary words.
func visibleText(m Model) string {
	plain := strings.ReplaceAll(ansi.Strip(m.View().Content), "│", " ")
	return strings.Join(strings.Fields(plain), " ")
}

func TestAttachPathPreservesSurroundingWhitespace(t *testing.T) {
	srv := newAttachServer(t, http.StatusOK)
	m := attachDetailModel(t, srv)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("decoy"), 0o600); err != nil {
		t.Fatal(err)
	}
	exact := filepath.Join(dir, " note.txt ")
	if err := os.WriteFile(exact, []byte("exact"), 0o600); err != nil {
		t.Fatal(err)
	}

	m, _ = updateModel(t, m, textKey("f"))
	m = typeText(t, m, exact)
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter did not start an upload")
	}
	m, _ = updateModel(t, m, cmd())

	if _, posts := srv.counts(); posts != 1 {
		t.Fatalf("uploads=%d, want 1 (err=%v)", posts, m.err)
	}
	if got := srv.files[" note.txt "]; got != "exact" {
		t.Fatalf("uploaded files=%v, want the file named with surrounding spaces", srv.files)
	}
}

func TestAttachEnterUploadsFileToSelectedIssue(t *testing.T) {
	srv := newAttachServer(t, http.StatusOK)
	m := attachDetailModel(t, srv)
	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("hello attachment"), 0o600); err != nil {
		t.Fatal(err)
	}

	m, _ = updateModel(t, m, textKey("f"))
	m = typeText(t, m, path)
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter did not start an upload")
	}
	m, _ = updateModel(t, m, cmd())

	if reads, posts := srv.counts(); reads != 1 || posts != 1 {
		t.Fatalf("settings reads=%d uploads=%d, want 1 and 1", reads, posts)
	}
	if srv.files["note.txt"] != "hello attachment" {
		t.Fatalf("uploaded files=%v, want note.txt with its content", srv.files)
	}
	if m.view != viewDetail || m.detail == nil || m.err != nil || !strings.Contains(m.toast, "Attached note.txt") {
		t.Fatalf("success did not return to detail with a status: view=%v err=%v toast=%q", m.view, m.err, m.toast)
	}
	if got := visibleText(m); !strings.Contains(got, "✓ Attached note.txt") {
		t.Fatalf("success feedback is not visible in the detail view:\n%s", got)
	}
}

func TestAttachEscCancelsWithoutJiraCall(t *testing.T) {
	srv := newAttachServer(t, http.StatusOK)
	m := attachDetailModel(t, srv)
	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("hello attachment"), 0o600); err != nil {
		t.Fatal(err)
	}

	m, _ = updateModel(t, m, textKey("f"))
	m = typeText(t, m, path)
	m, cmd := updateModel(t, m, specialKey(tea.KeyEsc))
	if cmd != nil {
		t.Fatal("esc returned a command; cancel must not start any request")
	}
	if m.view != viewDetail || m.detail == nil || m.err != nil {
		t.Fatalf("esc did not return to the ticket detail: view=%v err=%v", m.view, m.err)
	}
	if reads, posts := srv.counts(); reads != 0 || posts != 0 {
		t.Fatalf("cancelled prompt reached Jira: reads=%d uploads=%d", reads, posts)
	}
}

func TestAttachEmptyPathIsRefusedWithoutJiraCall(t *testing.T) {
	for _, value := range []string{"", "   "} {
		t.Run(fmt.Sprintf("%q", value), func(t *testing.T) {
			srv := newAttachServer(t, http.StatusOK)
			m := attachDetailModel(t, srv)
			m, _ = updateModel(t, m, textKey("f"))
			m = typeText(t, m, value)
			m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
			if cmd != nil {
				t.Fatal("empty path started a request")
			}
			if m.view != viewAttach || m.err == nil || !strings.Contains(m.View().Content, "file path is required") {
				t.Fatalf("empty path was not refused in the prompt: view=%v err=%v", m.view, m.err)
			}
			if reads, posts := srv.counts(); reads != 0 || posts != 0 {
				t.Fatalf("refused path reached Jira: reads=%d uploads=%d", reads, posts)
			}
		})
	}
}

func TestAttachUploadErrorShowsErrorAndStaysUsable(t *testing.T) {
	srv := newAttachServer(t, http.StatusForbidden)
	m := attachDetailModel(t, srv)
	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("hello attachment"), 0o600); err != nil {
		t.Fatal(err)
	}

	m, _ = updateModel(t, m, textKey("f"))
	m = typeText(t, m, path)
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter did not start an upload")
	}
	m, _ = updateModel(t, m, cmd())

	if reads, posts := srv.counts(); reads != 1 || posts != 1 {
		t.Fatalf("failed upload was retried or skipped: reads=%d uploads=%d, want 1 and 1", reads, posts)
	}
	if m.err == nil || m.view != viewDetail || !strings.Contains(m.View().Content, "API 403") {
		t.Fatalf("upload error was not shown in the detail: view=%v err=%v", m.view, m.err)
	}

	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	if m.view != viewKanban || m.err != nil {
		t.Fatalf("TUI not usable after upload error: view=%v err=%v", m.view, m.err)
	}
}

func TestAttachKeyIsListedInDetailFooterAndHelp(t *testing.T) {
	srv := newAttachServer(t, http.StatusOK)
	m := attachDetailModel(t, srv)
	if content := m.View().Content; !strings.Contains(content, "[f] Attach") {
		t.Fatalf("ticket detail footer does not list f:\n%s", content)
	}
	if help, _ := m.helpContent(); !strings.Contains(help, "f (detail)") {
		t.Fatalf("help does not list f:\n%s", help)
	}
}

func TestAttachKeyOpensPathPrompt(t *testing.T) {
	srv := newAttachServer(t, http.StatusOK)
	m := attachDetailModel(t, srv)

	m, _ = updateModel(t, m, textKey("f"))
	if m.view == viewDetail || !strings.Contains(m.View().Content, "Path") {
		t.Fatalf("f did not open the attachment path prompt: view=%v", m.view)
	}
}

func startAttachUpload(t *testing.T, srv *attachServer) (Model, tea.Cmd) {
	t.Helper()
	m := attachDetailModel(t, srv)
	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("hello attachment"), 0o600); err != nil {
		t.Fatal(err)
	}
	m, _ = updateModel(t, m, textKey("f"))
	m = typeText(t, m, path)
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter did not start an upload")
	}
	return m, cmd
}

func TestAttachEscWhileInFlightKeepsPromptAndPreservesResult(t *testing.T) {
	srv := newAttachServer(t, http.StatusOK)
	m, cmd := startAttachUpload(t, srv)

	m, escCmd := updateModel(t, m, specialKey(tea.KeyEsc))
	if escCmd != nil {
		t.Fatal("esc during an upload returned a command")
	}
	if m.view != viewAttach || m.activeAttachRequest == 0 {
		t.Fatalf("esc cancelled an in-flight upload: view=%v active=%d", m.view, m.activeAttachRequest)
	}
	if !strings.Contains(m.View().Content, "Uploading") {
		t.Fatalf("in-flight prompt does not tell the user the upload is running:\n%s", m.View().Content)
	}

	m, _ = updateModel(t, m, cmd())
	if m.view != viewDetail || m.err != nil || !strings.Contains(m.toast, "Attached note.txt") {
		t.Fatalf("result was dropped after esc: view=%v err=%v toast=%q", m.view, m.err, m.toast)
	}
}

func TestAttachUnknownMutationStateWarnsToCheckJira(t *testing.T) {
	srv := newAttachServer(t, http.StatusInternalServerError)
	m, cmd := startAttachUpload(t, srv)
	m, _ = updateModel(t, m, cmd())

	if m.err == nil {
		t.Fatal("unknown mutation state produced no error")
	}
	for _, want := range []string{"may have been applied", "check Jira", "before retrying"} {
		if !strings.Contains(m.err.Error(), want) {
			t.Fatalf("unknown-state warning missing %q: %v", want, m.err)
		}
	}
	got := visibleText(m)
	for _, want := range []string{"Error:", "may have been applied", "check Jira before retrying"} {
		if !strings.Contains(got, want) {
			t.Fatalf("visible detail view missing %q:\n%s", want, got)
		}
	}
	if m.view != viewDetail {
		t.Fatalf("warning is not shown in the detail view: view=%v", m.view)
	}
	if _, posts := srv.counts(); posts != 1 {
		t.Fatalf("uploads=%d, want exactly 1 (no retry)", posts)
	}
}

func TestAttachKnownAndValidationErrorsDoNotWarnUnknownState(t *testing.T) {
	srv := newAttachServer(t, http.StatusForbidden)
	m, cmd := startAttachUpload(t, srv)
	m, _ = updateModel(t, m, cmd())
	if strings.Contains(m.View().Content, "may have been applied") {
		t.Fatalf("rejected upload got the unknown-state warning:\n%s", m.View().Content)
	}

	m = attachDetailModel(t, srv)
	m, _ = updateModel(t, m, textKey("f"))
	m = typeText(t, m, filepath.Join(t.TempDir(), "missing.txt"))
	m, cmd = updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter did not start an upload")
	}
	m, _ = updateModel(t, m, cmd())
	if m.err == nil || strings.Contains(m.View().Content, "may have been applied") {
		t.Fatalf("validation error was misreported: err=%v", m.err)
	}
}
