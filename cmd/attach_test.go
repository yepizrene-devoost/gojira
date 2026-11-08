package cmd

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

// attachServer fakes the two Jira endpoints the attach command uses. Each POST
// is one upload; failOn names the file whose upload is rejected with failStatus.
type attachServer struct {
	t          *testing.T
	enabled    bool
	limit      int64
	failOn     string
	failStatus int
	// noDetails makes Jira accept an upload but answer with an empty list.
	noDetails bool
	posts     int
	uploads   []string
}

func (s *attachServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/rest/api/3/attachment/meta" {
		_, _ = fmt.Fprintf(w, `{"enabled":%t,"uploadLimit":%d}`, s.enabled, s.limit)
		return
	}
	if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/issue/A-1/attachments") {
		s.posts++
		file, header, err := r.FormFile("file")
		if err != nil {
			s.t.Errorf("multipart file part: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer func() { _ = file.Close() }()
		size, _ := io.Copy(io.Discard, file)
		if header.Filename == s.failOn {
			w.WriteHeader(s.failStatus)
			_, _ = fmt.Fprint(w, `{"errorMessages":["rejected"]}`)
			return
		}
		s.uploads = append(s.uploads, header.Filename)
		if s.noDetails {
			_, _ = fmt.Fprint(w, `[]`)
			return
		}
		_, _ = fmt.Fprintf(w, `[{"id":"%d","filename":%q,"size":%d}]`, 10000+len(s.uploads), header.Filename, size)
		return
	}
	s.t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
	w.WriteHeader(http.StatusNotFound)
}

func newAttachServer(t *testing.T, enabled bool, limit int64) (*attachServer, *httptest.Server) {
	t.Helper()
	fake := &attachServer{t: t, enabled: enabled, limit: limit}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	return fake, srv
}

func writeAttachFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAttachUploadsEachFileAndPrintsALinePerFile(t *testing.T) {
	dir := t.TempDir()
	first := writeAttachFile(t, dir, "notes.txt", "alpha")
	second := writeAttachFile(t, dir, "shot.png", "beta!")
	fake, srv := newAttachServer(t, true, 1000)

	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "attach", "A-1", first, second)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if strings.Join(fake.uploads, ",") != "notes.txt,shot.png" {
		t.Fatalf("uploads = %v, want notes.txt then shot.png", fake.uploads)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "notes.txt") || !strings.Contains(lines[1], "shot.png") {
		t.Fatalf("stdout = %q, want one line per file naming it", stdout)
	}
}

func TestAttachJSONWritesOneDocumentWithUploadedAttachments(t *testing.T) {
	dir := t.TempDir()
	first := writeAttachFile(t, dir, "notes.txt", "alpha")
	second := writeAttachFile(t, dir, "shot.png", "beta!")
	_, srv := newAttachServer(t, true, 1000)

	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "attach", "A-1", first, second, "--json")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty on success", stderr)
	}
	var result struct {
		IssueKey    string `json:"issueKey"`
		Attachments []struct {
			ID       string `json:"id"`
			Filename string `json:"filename"`
			Size     int64  `json:"size"`
		} `json:"attachments"`
	}
	assertSingleJSONDocument(t, []byte(stdout), &result)
	if result.IssueKey != "A-1" || len(result.Attachments) != 2 {
		t.Fatalf("result = %+v, want issue A-1 with 2 attachments", result)
	}
	got := result.Attachments[1]
	if got.Filename != "shot.png" || got.Size != int64(len("beta!")) || got.ID == "" {
		t.Fatalf("second attachment = %+v, want shot.png with its size and an id", got)
	}
}

func TestAttachPartialFailureReportsWhatWasUploadedAndStops(t *testing.T) {
	dir := t.TempDir()
	first := writeAttachFile(t, dir, "notes.txt", "alpha")
	second := writeAttachFile(t, dir, "shot.png", "beta")
	third := writeAttachFile(t, dir, "report.pdf", "gamma")
	fake, srv := newAttachServer(t, true, 1000)
	fake.failOn = "shot.png"
	fake.failStatus = http.StatusInternalServerError

	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "attach", "A-1", first, second, third, "--json")
	if code != 1 || stdout != "" {
		t.Fatalf("exit = %d, stdout = %q, want exit 1 and no stdout", code, stdout)
	}
	envelope := decodeMutationError(t, stderr)
	if envelope.Code != "partial_failure" || envelope.IssueKey != "A-1" || envelope.MutationState != jira.MutationUnknown {
		t.Fatalf("envelope = %+v, want partial_failure for A-1 with state unknown", envelope)
	}
	if !strings.Contains(envelope.Message, "notes.txt") {
		t.Fatalf("message = %q, want it to name the file uploaded before the failure", envelope.Message)
	}
	if strings.Join(fake.uploads, ",") != "notes.txt" || fake.posts != 2 {
		t.Fatalf("uploads = %v posts = %d, want notes.txt and the failed attempt only", fake.uploads, fake.posts)
	}
}

func TestAttachRefusedInputUploadsNothingAndReportsNotApplied(t *testing.T) {
	dir := t.TempDir()
	good := writeAttachFile(t, dir, "notes.txt", "0123456789")
	missing := filepath.Join(dir, "missing.txt")
	cases := []struct {
		name    string
		enabled bool
		limit   int64
		files   []string
	}{
		{name: "missing path after a valid one", enabled: true, limit: 1000, files: []string{good, missing}},
		{name: "file over the instance limit", enabled: true, limit: 5, files: []string{good}},
		{name: "attachments disabled", enabled: false, limit: 1000, files: []string{good}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fake, srv := newAttachServer(t, test.enabled, test.limit)
			args := append([]string{"attach", "A-1"}, test.files...)
			code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), append(args, "--json")...)
			if code != 1 || stdout != "" {
				t.Fatalf("exit = %d, stdout = %q, want exit 1 and no stdout", code, stdout)
			}
			if fake.posts != 0 {
				t.Fatalf("posts = %d, want no upload request", fake.posts)
			}
			envelope := decodeMutationError(t, stderr)
			if envelope.MutationState != jira.MutationNotApplied || envelope.IssueKey != "A-1" {
				t.Fatalf("envelope = %+v, want mutationState not_applied for A-1", envelope)
			}
		})
	}
}

func TestAttachRequiresAtLeastOneFileArgument(t *testing.T) {
	fake, srv := newAttachServer(t, true, 1000)
	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "attach", "A-1", "--json")
	if code != 1 || stdout != "" {
		t.Fatalf("exit = %d, stdout = %q, want exit 1 and no stdout", code, stdout)
	}
	if fake.posts != 0 {
		t.Fatalf("posts = %d, want no upload request", fake.posts)
	}
	envelope := decodeMutationError(t, stderr)
	if envelope.Code != "validation_error" || envelope.MutationState != jira.MutationNotApplied {
		t.Fatalf("envelope = %+v, want validation_error with mutationState not_applied", envelope)
	}
}

func TestAttachPartialFailureAfterRejectedUploadReportsApplied(t *testing.T) {
	dir := t.TempDir()
	first := writeAttachFile(t, dir, "notes.txt", "alpha")
	second := writeAttachFile(t, dir, "shot.png", "beta")
	fake, srv := newAttachServer(t, true, 1000)
	fake.failOn = "shot.png"
	fake.failStatus = http.StatusForbidden

	code, _, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "attach", "A-1", first, second, "--json")
	if code != 1 {
		t.Fatalf("exit = %d", code)
	}
	envelope := decodeMutationError(t, stderr)
	if envelope.Code != "partial_failure" || envelope.MutationState != jira.MutationApplied {
		t.Fatalf("envelope = %+v, want partial_failure with state applied because notes.txt was uploaded", envelope)
	}
}

func TestAttachPartialFailureNamesUploadedFilesEvenWithoutJiraDetails(t *testing.T) {
	dir := t.TempDir()
	first := writeAttachFile(t, dir, "notes.txt", "alpha")
	second := writeAttachFile(t, dir, "shot.png", "beta")
	fake, srv := newAttachServer(t, true, 1000)
	fake.noDetails = true
	fake.failOn = "shot.png"
	fake.failStatus = http.StatusInternalServerError

	_, _, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "attach", "A-1", first, second, "--json")
	envelope := decodeMutationError(t, stderr)
	if !strings.Contains(envelope.Message, "notes.txt") {
		t.Fatalf("message = %q, want it to name notes.txt even though Jira returned no details", envelope.Message)
	}
}

func TestAttachPrintsALinePerFileEvenWithoutJiraDetails(t *testing.T) {
	dir := t.TempDir()
	first := writeAttachFile(t, dir, "notes.txt", "alpha")
	fake, srv := newAttachServer(t, true, 1000)
	fake.noDetails = true

	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "attach", "A-1", first)
	if code != 0 || !strings.Contains(stdout, "notes.txt") {
		t.Fatalf("exit = %d stdout = %q stderr = %q, want a line naming notes.txt", code, stdout, stderr)
	}
}

func TestAttachPartialFailureNamesTheFileJiraAcceptedWhenItsResponseIsCut(t *testing.T) {
	dir := t.TempDir()
	first := writeAttachFile(t, dir, "notes.txt", "alpha")
	second := writeAttachFile(t, dir, "shot.png", "beta")
	third := writeAttachFile(t, dir, "report.pdf", "gamma")
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/attachment/meta" {
			_, _ = fmt.Fprint(w, `{"enabled":true,"uploadLimit":1000}`)
			return
		}
		posts++
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Errorf("multipart file part: %v", err)
			return
		}
		_, _ = io.Copy(io.Discard, file)
		if header.Filename == "shot.png" {
			// Accepted, but the response is cut short: applied with no details.
			w.Header().Set("Content-Length", "100")
			_, _ = fmt.Fprint(w, "[")
			return
		}
		_, _ = fmt.Fprintf(w, `[{"id":"1","filename":%q,"size":1}]`, header.Filename)
	}))
	t.Cleanup(srv.Close)

	code, _, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "attach", "A-1", first, second, third, "--json")
	if code != 1 || posts != 2 {
		t.Fatalf("exit=%d posts=%d, want failure after the second upload", code, posts)
	}
	envelope := decodeMutationError(t, stderr)
	if envelope.Code != "partial_failure" || envelope.MutationState != jira.MutationApplied || !strings.Contains(envelope.Message, "shot.png") {
		t.Fatalf("envelope = %+v, want partial_failure, applied, naming shot.png", envelope)
	}
}
