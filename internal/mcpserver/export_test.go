package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestExportPublicIssueSafetyCap(t *testing.T) {
	requests := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/rest/agile/1.0/board/7/sprint/9/issue" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		start, err := strconv.Atoi(r.URL.Query().Get("startAt"))
		if err != nil || start != requests*100 || r.URL.Query().Get("maxResults") != "100" || requests >= 20 {
			t.Errorf("unexpected page request: %s", r.URL)
			http.Error(w, "unexpected page", http.StatusBadRequest)
			return
		}
		requests++
		issues := make([]map[string]any, 100)
		for i := range issues {
			issues[i] = map[string]any{"key": fmt.Sprintf("CAP-%d", start+i+1)}
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"issues": issues, "total": 2100, "isLast": false}); err != nil {
			t.Error(err)
		}
	}))
	defer api.Close()

	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := New(Options{Client: jira.NewClient(api.URL, "test", "test"), Domain: "example.invalid", SchemaVersion: "v1", MapError: func(error) (string, int, string) { return "", 0, "" }}).Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ss.Close() }()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cs.Close() }()
	got, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "gojira_export", Arguments: map[string]any{"boardId": 7, "sprintId": 9}})
	if err != nil || got.IsError {
		t.Fatalf("export: %v %v", got, err)
	}
	encoded, err := json.Marshal(got.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		SchemaVersion string `json:"schemaVersion"`
		Data          []struct {
			Key string `json:"key"`
		} `json:"data"`
		Complete     bool   `json:"complete"`
		Completeness string `json:"completeness"`
		Warning      string `json:"warning"`
	}
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatal(err)
	}
	if requests != 20 || len(envelope.Data) != 2000 || envelope.Data[0].Key != "CAP-1" || envelope.Data[1999].Key != "CAP-2000" {
		t.Fatalf("requests=%d tickets=%d", requests, len(envelope.Data))
	}
	if envelope.SchemaVersion != "v1" || envelope.Complete || envelope.Completeness != "unknown" || !strings.Contains(envelope.Warning, "2000-issue safety cap") || !strings.Contains(envelope.Warning, "additional results may exist") {
		t.Fatalf("unsafe completeness envelope: %s", encoded)
	}
	if len(got.Content) != 1 {
		t.Fatalf("content count=%d", len(got.Content))
	}
	text, ok := got.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("expected text content")
	}
	var textValue, structuredValue any
	if err := json.Unmarshal([]byte(text.Text), &textValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &structuredValue); err != nil {
		t.Fatal(err)
	}
	textJSON, _ := json.Marshal(textValue)
	structuredJSON, _ := json.Marshal(structuredValue)
	if string(textJSON) != string(structuredJSON) {
		t.Fatal("text and structured content differ")
	}
}

func TestExportPublic(t *testing.T) {
	for _, tc := range []struct {
		name, args string
		responses  []string
		paths      []string
		errorCode  string
	}{
		{"explicit", `{"boardId":7,"sprintId":9}`, []string{`{"issues":[{"key":"ABC-1","fields":{"summary":"Example"}}],"total":1}`}, []string{"/board/7/sprint/9/issue"}, ""},
		{"conversion", `{"boardId":7,"sprintId":9}`, []string{`{"issues":[{"key":"ABC-1","fields":{"summary":"Example"}}],"total":1}`}, []string{"/board/7/sprint/9/issue"}, ""},
		{"completeness", `{"boardId":7,"sprintId":9}`, []string{`{"issues":[],"total":0}`}, []string{"/board/7/sprint/9/issue"}, ""},
		{"read_only", `{"boardId":7,"sprintId":9}`, []string{`{"issues":[],"total":0}`}, []string{"/board/7/sprint/9/issue"}, ""},
		{"upstream_error", `{"boardId":7,"sprintId":9}`, []string{`not json`}, []string{"/board/7/sprint/9/issue"}, "read_failed"},
		{"defaults", `{}`, []string{`{"values":[{"id":7},{"id":8}]}`, `{"values":[{"id":9,"state":"active"}]}`, `{"issues":[],"total":0}`}, []string{"/board", "/board/7/sprint", "/board/7/sprint/9/issue"}, ""},
		{"no_active", `{"boardId":7}`, []string{`{"values":[]}`, `{"values":[{"id":8,"state":"future"}]}`, `{"issues":[],"total":0}`}, []string{"/board/7/sprint", "/board/7/sprint", "/board/7/issue"}, ""},
		{"no_boards", `{}`, []string{`{"values":[]}`}, []string{"/board"}, "validation_error"},
		{"invalid", `{"boardId":-1}`, nil, nil, "validation_error"},
		{"negative_sprint", `{"boardId":7,"sprintId":-1}`, nil, nil, "validation_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || calls >= len(tc.responses) {
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
					http.Error(w, "unexpected", 500)
					return
				}
				if !strings.HasSuffix(r.URL.Path, tc.paths[calls]) {
					t.Errorf("path %s want suffix %s", r.URL.Path, tc.paths[calls])
				}
				_, _ = fmt.Fprint(w, tc.responses[calls])
				calls++
			}))
			defer api.Close()
			ctx := context.Background()
			st, ct := mcp.NewInMemoryTransports()
			ss, err := New(Options{Client: jira.NewClient(api.URL, "test", "test"), Domain: "example.invalid", SchemaVersion: "v1", MapError: func(error) (string, int, string) { return "", 0, "" }}).Connect(ctx, st, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = ss.Close() }()
			cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = cs.Close() }()
			if tc.name == "read_only" {
				list, err := cs.ListTools(ctx, &mcp.ListToolsParams{})
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, tool := range list.Tools {
					if tool.Name == "gojira_export" {
						found = true
						if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
							t.Fatal("missing read-only hint")
						}
					}
				}
				if !found {
					t.Fatal("export tool missing")
				}
			}
			got, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "gojira_export", Arguments: json.RawMessage(tc.args)})
			if err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(got.StructuredContent)
			if calls != len(tc.responses) {
				t.Errorf("calls=%d want=%d", calls, len(tc.responses))
			}
			if tc.errorCode != "" {
				if !got.IsError || !strings.Contains(string(data), tc.errorCode) {
					t.Fatalf("result %s", data)
				}
				return
			}
			if got.IsError || !strings.Contains(string(data), `"complete":false`) || !strings.Contains(string(data), `"completeness":"unknown"`) {
				t.Fatalf("result %s", data)
			}
			if tc.name == "no_active" && !strings.Contains(string(data), "each sprint query reads one page") {
				t.Fatalf("warning must distinguish the active and fallback sprint queries: %s", data)
			}
			if (tc.name == "explicit" || tc.name == "conversion") && (!strings.Contains(string(data), `"key":"ABC-1"`) || !strings.Contains(string(data), `"schemaVersion":"v1"`)) {
				t.Fatalf("conversion %s", data)
			}
		})
	}
}
