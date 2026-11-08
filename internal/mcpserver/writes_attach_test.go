package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func listedToolNames(t *testing.T, allowWrites bool) []string {
	t.Helper()
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := New(Options{AllowWrites: allowWrites}).Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ss.Close() }()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "attach-surface", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cs.Close() }()
	list, err := cs.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(list.Tools))
	for _, tool := range list.Tools {
		names = append(names, tool.Name)
	}
	return names
}

func TestAttachIsListedOnlyWhenWritesAreAllowed(t *testing.T) {
	if !slices.Contains(listedToolNames(t, true), "gojira_attach") {
		t.Fatal("gojira_attach not listed with writes enabled")
	}
	if slices.Contains(listedToolNames(t, false), "gojira_attach") {
		t.Fatal("gojira_attach listed without writes enabled")
	}
}

// attachJira serves the attachment settings and upload endpoints for one issue
// and records every upload it receives.
type attachJira struct {
	t          *testing.T
	settings   string
	postStatus int
	postBody   string
	// cutResponse accepts the upload but truncates the response, so the client
	// reports an applied mutation without details.
	cutResponse bool
	metaReads   atomic.Int32
	posts       atomic.Int32
	mu          sync.Mutex
	uploads     []string
}

func (j *attachJira) handler(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/attachment/meta":
		j.metaReads.Add(1)
		_, _ = w.Write([]byte(j.settings))
	case r.Method == http.MethodPost && r.URL.Path == "/rest/api/3/issue/ABC-1/attachments":
		j.posts.Add(1)
		if r.Header.Get("X-Atlassian-Token") != "no-check" {
			j.t.Errorf("missing X-Atlassian-Token header")
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			j.t.Errorf("not multipart: %v", err)
		}
		if files := r.MultipartForm.File["file"]; len(files) == 1 {
			j.mu.Lock()
			j.uploads = append(j.uploads, files[0].Filename)
			j.mu.Unlock()
		}
		if j.cutResponse {
			w.Header().Set("Content-Length", "100")
		}
		w.WriteHeader(j.postStatus)
		_, _ = w.Write([]byte(j.postBody))
	default:
		j.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func writeAttachFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAttachUploadsTheFileAndReturnsTheAttachment(t *testing.T) {
	jira := &attachJira{t: t, settings: `{"enabled":true,"uploadLimit":1024}`, postStatus: http.StatusOK, postBody: `[{"id":"10001","filename":"note.txt","size":5,"mimeType":"text/plain"}]`}
	cs := paritySession(t, true, jira.handler)
	path := writeAttachFile(t, "note.txt", "hello")
	isError, got := callWriteTool(t, cs, "gojira_attach", map[string]any{"key": "abc-1", "path": path})
	data, _ := got["data"].(map[string]any)
	attachments, _ := data["attachments"].([]any)
	if isError || got["action"] != "attach" || got["issueKey"] != "ABC-1" || got["mutationState"] != "applied" || got["code"] != nil ||
		jira.posts.Load() != 1 || !slices.Equal(jira.uploads, []string{"note.txt"}) ||
		!reflect.DeepEqual(attachments, []any{map[string]any{"id": "10001", "filename": "note.txt", "size": float64(5)}}) {
		t.Fatalf("isError=%v result=%v posts=%d uploads=%v", isError, got, jira.posts.Load(), jira.uploads)
	}
}

func TestAttachRejectsMissingKeyOrPathBeforeAnyJiraCall(t *testing.T) {
	path := writeAttachFile(t, "note.txt", "hello")
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"empty key", map[string]any{"key": "", "path": path}},
		{"blank key", map[string]any{"key": "   ", "path": path}},
		{"missing key", map[string]any{"path": path}},
		{"malformed key", map[string]any{"key": "abc", "path": path}},
		{"empty path", map[string]any{"key": "ABC-1", "path": ""}},
		{"blank path", map[string]any{"key": "ABC-1", "path": "   "}},
		{"missing path", map[string]any{"key": "ABC-1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jira := &attachJira{t: t, settings: `{"enabled":true,"uploadLimit":1024}`, postStatus: http.StatusOK, postBody: `[]`}
			cs := paritySession(t, true, jira.handler)
			isError, got := callWriteTool(t, cs, "gojira_attach", tc.args)
			if !isError || got["code"] != "validation_error" || got["mutationState"] != "not_applied" || jira.metaReads.Load() != 0 || jira.posts.Load() != 0 {
				t.Fatalf("isError=%v result=%v metaReads=%d posts=%d", isError, got, jira.metaReads.Load(), jira.posts.Load())
			}
		})
	}
}

func TestAttachRejectsUnusableFilesWithoutUploading(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.txt")
	for _, tc := range []struct {
		name, path, limit, message string
	}{
		{"missing file", missing, "1024", "no such file"},
		{"directory", t.TempDir(), "1024", "is not a regular file"},
		{"over the Jira limit", writeAttachFile(t, "big.txt", "hello"), "4", "over the Jira upload limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jira := &attachJira{t: t, settings: `{"enabled":true,"uploadLimit":` + tc.limit + `}`, postStatus: http.StatusOK, postBody: `[]`}
			cs := paritySession(t, true, jira.handler)
			isError, got := callWriteTool(t, cs, "gojira_attach", map[string]any{"key": "ABC-1", "path": tc.path})
			message, _ := got["message"].(string)
			if !isError || got["mutationState"] != "not_applied" || got["code"] != "validation_error" || !strings.Contains(message, tc.message) ||
				jira.posts.Load() != 0 || len(jira.uploads) != 0 {
				t.Fatalf("isError=%v result=%v posts=%d uploads=%v", isError, got, jira.posts.Load(), jira.uploads)
			}
		})
	}
}

func TestAttachReportsJiraFailureWithItsMutationState(t *testing.T) {
	for _, tc := range []struct {
		name, state, hint string
		status            int
	}{
		{"client error", "not_applied", "Check Jira access before retrying.", http.StatusBadRequest},
		{"server error", "unknown", "Verify the attachment in Jira before retrying", http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jira := &attachJira{t: t, settings: `{"enabled":true,"uploadLimit":1024}`, postStatus: tc.status, postBody: `SECRET_MARKER https://private.invalid/private`}
			cs := paritySession(t, true, jira.handler)
			path := writeAttachFile(t, "note.txt", "hello")
			isError, got := callWriteTool(t, cs, "gojira_attach", map[string]any{"key": "ABC-1", "path": path})
			hint, _ := got["hint"].(string)
			raw, _ := json.Marshal(got)
			if !isError || got["mutationState"] != tc.state || got["code"] != "mutation_failed" || got["issueKey"] != "ABC-1" || !strings.Contains(hint, tc.hint) ||
				jira.posts.Load() != 1 || strings.Contains(string(raw), "SECRET_MARKER") || strings.Contains(string(raw), "private.invalid") {
				t.Fatalf("isError=%v result=%v posts=%d", isError, got, jira.posts.Load())
			}
		})
	}
}

func TestAttachReportsAnAppliedUploadExplicitlyAndNeverAsksToRetry(t *testing.T) {
	jira := &attachJira{t: t, settings: `{"enabled":true,"uploadLimit":1024}`, postStatus: http.StatusOK, postBody: `[`, cutResponse: true}
	cs := paritySession(t, true, jira.handler)
	path := writeAttachFile(t, "note.txt", "hello")

	isError, got := callWriteTool(t, cs, "gojira_attach", map[string]any{"key": "ABC-1", "path": path})
	hint, _ := got["hint"].(string)
	data, _ := got["data"].(map[string]any)
	files, _ := data["files"].([]any)
	lower := strings.ToLower(hint)
	if !isError || got["mutationState"] != "applied" || got["issueKey"] != "ABC-1" || jira.posts.Load() != 1 {
		t.Fatalf("isError=%v result=%v posts=%d", isError, got, jira.posts.Load())
	}
	if !strings.Contains(lower, "already in jira") || !strings.Contains(lower, "do not upload it again") || strings.Contains(lower, "retrying") || strings.Contains(lower, "access") {
		t.Fatalf("hint = %q, want an explicit applied notice and no retry advice", hint)
	}
	if !reflect.DeepEqual(files, []any{"note.txt"}) {
		t.Fatalf("data = %v, want the confirmed file retained", data)
	}
}
