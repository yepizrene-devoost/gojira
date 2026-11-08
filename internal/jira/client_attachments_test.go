package jira

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetAttachmentSettingsReadsEnabledAndLimit(t *testing.T) {
	var request string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request = r.Method + " " + r.URL.Path
		_, _ = io.WriteString(w, `{"enabled":true,"uploadLimit":1048576}`)
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "me@example.com", "token").GetAttachmentSettings()
	if err != nil {
		t.Fatalf("GetAttachmentSettings: %v", err)
	}
	if request != "GET /rest/api/3/attachment/meta" {
		t.Fatalf("request = %q", request)
	}
	if want := (AttachmentSettings{Enabled: true, UploadLimit: 1048576}); got != want {
		t.Fatalf("settings = %+v, want %+v", got, want)
	}
}

func TestAddAttachmentSendsMultipartFilePart(t *testing.T) {
	var method, path, token, contentType, auth, filename, content string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		token = r.Header.Get("X-Atlassian-Token")
		contentType = r.Header.Get("Content-Type")
		auth = r.Header.Get("Authorization")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Errorf("FormFile(file): %v", err)
			return
		}
		defer func() { _ = file.Close() }()
		body, _ := io.ReadAll(file)
		filename, content = header.Filename, string(body)
		_, _ = io.WriteString(w, `[{"id":"10001","filename":"report.pdf","size":11,"mimeType":"application/pdf"}]`)
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "me@example.com", "token").
		AddAttachment("ARA-1", "report.pdf", strings.NewReader("hello world"))
	if err != nil {
		t.Fatalf("AddAttachment: %v", err)
	}
	if method != http.MethodPost || path != "/rest/api/3/issue/ARA-1/attachments" {
		t.Fatalf("request = %s %s", method, path)
	}
	if token != "no-check" {
		t.Fatalf("X-Atlassian-Token = %q, want no-check", token)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data; boundary=") {
		t.Fatalf("Content-Type = %q", contentType)
	}
	if auth == "" {
		t.Fatal("Authorization header missing")
	}
	if filename != "report.pdf" || content != "hello world" {
		t.Fatalf("file part = %q / %q", filename, content)
	}
	want := []Attachment{{ID: "10001", Filename: "report.pdf", Size: 11, MimeType: "application/pdf"}}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("attachments = %+v, want %+v", got, want)
	}
}

func TestAddAttachmentClassifiesOutcomeAndNeverRetries(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   MutationState
	}{
		{"rejected", http.StatusForbidden, MutationNotApplied},
		{"rate limited", http.StatusTooManyRequests, MutationNotApplied},
		{"server failure", http.StatusServiceUnavailable, MutationUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hits := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits++
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			_, err := NewClient(srv.URL, "me@example.com", "token").
				AddAttachment("ARA-1", "a.txt", strings.NewReader("x"))
			if err == nil || MutationStateOf(err) != tc.want || hits != 1 {
				t.Fatalf("err=%v state=%v hits=%d, want state %v and 1 hit", err, MutationStateOf(err), hits, tc.want)
			}
		})
	}
}

func TestAddAttachmentTreatsAnUnreadableSuccessBodyAsUploaded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "not json")
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "me@example.com", "token").
		AddAttachment("ARA-1", "a.txt", strings.NewReader("x"))
	if err != nil || len(got) != 0 {
		t.Fatalf("attachments=%+v err=%v, want no error: Jira already accepted the file", got, err)
	}
}

// slowReader blocks its first Read until Jira has already answered, then
// delivers its bytes. It records whether a Read is in flight or starts after
// AddAttachment returned.
type slowReader struct {
	answered <-chan struct{}
	reading  atomic.Bool
	returned atomic.Bool
	late     atomic.Int32
	sent     bool
}

func (r *slowReader) Read(p []byte) (int, error) {
	if r.returned.Load() {
		r.late.Add(1)
	}
	r.reading.Store(true)
	defer r.reading.Store(false)
	<-r.answered
	time.Sleep(50 * time.Millisecond)
	if r.sent {
		return 0, io.EOF
	}
	r.sent = true
	return copy(p, "payload"), nil
}

func TestAddAttachmentDoesNotReturnWhileItsBodyCopierStillReadsTheStream(t *testing.T) {
	answered := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Answer before reading the upload, as Jira does when it rejects early.
		rc := http.NewResponseController(w)
		_ = rc.EnableFullDuplex()
		w.WriteHeader(http.StatusForbidden)
		_ = rc.Flush()
		close(answered)
	}))
	defer srv.Close()
	body := &slowReader{answered: answered}

	_, err := NewClient(srv.URL, "me@example.com", "token").AddAttachment("ARA-1", "a.txt", body)
	body.returned.Store(true)
	if err == nil || MutationStateOf(err) != MutationNotApplied {
		t.Fatalf("err=%v state=%v, want a rejected (not applied) upload", err, MutationStateOf(err))
	}
	if body.reading.Load() {
		t.Fatal("AddAttachment returned while the body copier was still reading the caller's stream")
	}
	time.Sleep(150 * time.Millisecond)
	if body.late.Load() != 0 {
		t.Fatal("the body copier read the caller's stream after AddAttachment returned")
	}
}

// blockedReader never returns from its first Read until released, like a
// stalled pipe handed to AddAttachment.
type blockedReader struct {
	release chan struct{}
	reads   atomic.Int32
}

func (r *blockedReader) Read(p []byte) (int, error) {
	r.reads.Add(1)
	<-r.release
	return 0, io.EOF
}

func TestAddAttachmentReturnsBoundedWhenTheCallerStreamNeverUnblocks(t *testing.T) {
	old := bodyCopierGrace
	bodyCopierGrace = 100 * time.Millisecond
	t.Cleanup(func() { bodyCopierGrace = old })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)
		_ = rc.EnableFullDuplex()
		w.WriteHeader(http.StatusForbidden)
		_ = rc.Flush()
	}))
	defer srv.Close()
	body := &blockedReader{release: make(chan struct{})}
	t.Cleanup(func() { close(body.release) })

	done := make(chan error, 1)
	go func() {
		_, err := NewClient(srv.URL, "me@example.com", "token").AddAttachment("ARA-1", "a.txt", body)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || MutationStateOf(err) != MutationNotApplied {
			t.Fatalf("err=%v state=%v, want the rejected upload classified", err, MutationStateOf(err))
		}
	case <-time.After(3 * time.Second):
		t.Fatal("AddAttachment hung on a caller stream that never returns")
	}
	if got := body.reads.Load(); got != 1 {
		t.Fatalf("reads = %d, want only the abandoned in-flight read", got)
	}
}
