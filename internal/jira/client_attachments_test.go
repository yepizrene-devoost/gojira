package jira

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
