package jira

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateIssueWithParentKeepsLegacyPayloadSeparate(t *testing.T) {
	for _, tc := range []struct {
		name, parent string
	}{
		{"legacy create", ""},
		{"subtask create", "ARA-4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != http.MethodPost || r.URL.Path != pathIssueCreate {
					t.Errorf("request = %s %s", r.Method, r.URL.Path)
				}
				var body struct {
					Fields map[string]json.RawMessage `json:"fields"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode: %v", err)
				}
				parent, present := body.Fields["parent"]
				if present != (tc.parent != "") {
					t.Errorf("parent presence = %v", present)
				}
				if present && string(parent) != `{"key":"ARA-4"}` {
					t.Errorf("parent = %s", parent)
				}
				if len(body.Fields["description"]) == 0 {
					t.Error("description omitted")
				}
				_, _ = fmt.Fprint(w, `{"key":"ARA-5"}`)
			}))
			defer srv.Close()
			client := NewClient(srv.URL, "email", "token")
			description := TextToADF("context")
			var key string
			var err error
			if tc.parent == "" {
				key, err = client.CreateIssue("ARA", "Task", "summary", &description)
			} else {
				key, err = client.CreateIssueWithParent("ARA", "Sub-task", "summary", &description, tc.parent)
			}
			if key != "ARA-5" || err != nil || requests != 1 {
				t.Fatalf("key=%q err=%v requests=%d", key, err, requests)
			}
		})
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// GetBoardIssues must page through the Agile endpoint: a single request is
// capped server-side and silently truncates large sprints (the bug that hid
// a freshly created ticket beyond the first page).
func TestGetBoardIssuesPaginates(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("startAt") {
		case "0":
			if _, err := fmt.Fprint(w, `{"startAt":0,"maxResults":2,"total":3,"isLast":false,
				"issues":[{"id":"1","key":"A-1"},{"id":"2","key":"A-2"}]}`); err != nil {
				t.Errorf("write page 1: %v", err)
			}
		default:
			if _, err := fmt.Fprint(w, `{"startAt":2,"maxResults":2,"total":3,"isLast":true,
				"issues":[{"id":"3","key":"A-3"}]}`); err != nil {
				t.Errorf("write page 2: %v", err)
			}
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "e@x.com", "tok")
	issues, err := client.GetBoardIssues(1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 3 {
		t.Fatalf("got %d issues, want 3 (pagination stopped early?)", len(issues))
	}
	if issues[2].Key != "A-3" {
		t.Fatalf("last issue = %s, want A-3", issues[2].Key)
	}
	if hits != 2 {
		t.Fatalf("made %d requests, want 2", hits)
	}
}

type errorReadCloser struct {
	err error
}

func (b errorReadCloser) Read([]byte) (int, error) {
	return 0, b.err
}

func (b errorReadCloser) Close() error {
	return nil
}

func TestRequestReturnsResponseReadError(t *testing.T) {
	wantErr := errors.New("response body read failed")
	client := NewClient("https://jira.example", "e@x.com", "tok")
	client.http.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       errorReadCloser{err: wantErr},
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})

	body, err := client.request(http.MethodGet, "/rest/api/3/issue/A-1", nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want response body read error", err)
	}
	if body != nil {
		t.Fatalf("response body = %q, want nil when reading response fails", body)
	}
}

func TestRequestDoesNotRetryPostAfterTransportError(t *testing.T) {
	hits := 0
	client := NewClient("https://jira.example", "e@x.com", "tok")
	client.http.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		hits++
		return nil, fmt.Errorf("temporary transport failure")
	})

	if _, err := client.post("/rest/api/3/issue", []byte(`{"fields":{}}`)); err == nil {
		t.Fatal("POST transport error was swallowed")
	}
	if hits != 1 {
		t.Fatalf("made %d requests, want 1", hits)
	}
}

func TestWriteServerFailuresAfterDispatchAreUnknown(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusGatewayTimeout} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			writes := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writes++ // Simulate Jira applying the mutation before its proxy response fails.
				w.WriteHeader(status)
			}))
			defer srv.Close()

			client := NewClient(srv.URL, "e@x.com", "tok")
			err := client.UpdateIssue("A-1", map[string]interface{}{"summary": "new"})
			if state := MutationStateOf(err); state != MutationUnknown {
				t.Fatalf("state = %q, want %q after HTTP %d", state, MutationUnknown, status)
			}
			if writes != 1 {
				t.Fatalf("server observed %d writes, want exactly 1", writes)
			}
		})
	}
}

func TestWriteRedirectsAreNotFollowed(t *testing.T) {
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			redirectedWrites := 0
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				redirectedWrites++
				w.WriteHeader(http.StatusNoContent)
			}))
			defer target.Close()

			initialWrites := 0
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				initialWrites++
				w.Header().Set("Location", target.URL+r.URL.Path)
				w.WriteHeader(status)
			}))
			defer origin.Close()

			client := NewClient(origin.URL, "e@x.com", "tok")
			err := client.UpdateIssue("A-1", map[string]interface{}{"summary": "new"})
			if state := MutationStateOf(err); state != MutationUnknown {
				t.Fatalf("state = %q, want %q after HTTP %d", state, MutationUnknown, status)
			}
			if initialWrites != 1 || redirectedWrites != 0 {
				t.Fatalf("origin writes = %d, redirected writes = %d; want 1 and 0", initialWrites, redirectedWrites)
			}
		})
	}
}

func TestWriteRequestsAreNeverRetriedAndClassifyFailures(t *testing.T) {
	tests := []struct {
		name      string
		transport roundTripperFunc
		wantState MutationState
	}{
		{
			name: "transport failure is ambiguous",
			transport: func(req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("connection lost")
			},
			wantState: MutationUnknown,
		},
		{
			name: "HTTP rejection confirms not applied",
			transport: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader(`{"error":"invalid"}`)),
					Header:     make(http.Header),
					Request:    req,
				}, nil
			},
			wantState: MutationNotApplied,
		},
		{
			name: "server failure body read error is ambiguous",
			transport: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadGateway,
					Body:       errorReadCloser{err: errors.New("truncated proxy response")},
					Header:     make(http.Header),
					Request:    req,
				}, nil
			},
			wantState: MutationUnknown,
		},
		{
			name: "client rejection body read error remains not applied",
			transport: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       errorReadCloser{err: errors.New("truncated rejection response")},
					Header:     make(http.Header),
					Request:    req,
				}, nil
			},
			wantState: MutationNotApplied,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hits := 0
			client := NewClient("https://jira.example", "e@x.com", "tok")
			client.http.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				hits++
				return test.transport(req)
			})
			err := client.UpdateIssue("A-1", map[string]interface{}{"summary": "new"})
			if err == nil {
				t.Fatal("write error = nil")
			}
			if state := MutationStateOf(err); state != test.wantState {
				t.Fatalf("state = %q, want %q", state, test.wantState)
			}
			if hits != 1 {
				t.Fatalf("made %d write requests, want exactly 1", hits)
			}
		})
	}
}

func TestRequestDoesNotRetryPostAfterTransientResponse(t *testing.T) {
	hits := 0
	client := NewClient("https://jira.example", "e@x.com", "tok")
	client.http.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		hits++
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       io.NopCloser(strings.NewReader("temporarily unavailable")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})

	if _, err := client.post("/rest/api/3/issue", []byte(`{"fields":{}}`)); err == nil || !strings.Contains(err.Error(), "API 503") {
		t.Fatalf("error = %v, want immediate API 503 failure", err)
	}
	if hits != 1 {
		t.Fatalf("made %d requests, want 1", hits)
	}
}

func TestRequestRetriesTransportErrors(t *testing.T) {
	tests := []struct {
		name     string
		failures int
		wantHits int
	}{
		{name: "one transport error", failures: 1, wantHits: 2},
		{name: "multiple transport errors", failures: 2, wantHits: 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hits := 0
			client := NewClient("https://jira.example", "e@x.com", "tok")
			client.http.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				hits++
				if hits <= tc.failures {
					return nil, fmt.Errorf("temporary transport failure")
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"issues":[],"total":0,"maxResults":50}`)),
					Header:     make(http.Header),
					Request:    req,
				}, nil
			})

			if _, _, err := client.SearchJQL("project = A", 0); err != nil {
				t.Fatal(err)
			}
			if hits != tc.wantHits {
				t.Fatalf("made %d requests, want %d", hits, tc.wantHits)
			}
		})
	}
}

func TestRequestRetriesTransientResponses(t *testing.T) {
	tests := []struct {
		name       string
		statuses   []int
		retryAfter string
		wantHits   int
	}{
		{name: "rate limited", statuses: []int{http.StatusTooManyRequests, http.StatusOK}, retryAfter: "0", wantHits: 2},
		{name: "server error", statuses: []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusOK}, wantHits: 3},
		{name: "client error is immediate", statuses: []int{http.StatusBadRequest}, wantHits: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hits := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				status := tc.statuses[hits]
				hits++
				if tc.retryAfter != "" && status != http.StatusOK {
					w.Header().Set("Retry-After", tc.retryAfter)
				}
				w.WriteHeader(status)
				if status == http.StatusOK {
					_, _ = fmt.Fprint(w, `{"issues":[],"total":0,"maxResults":50}`)
				}
			}))
			defer srv.Close()

			client := NewClient(srv.URL, "e@x.com", "tok")
			_, _, err := client.SearchJQL("project = A", 0)
			if tc.name == "client error is immediate" {
				if err == nil || !strings.Contains(err.Error(), "API 400") {
					t.Fatalf("error = %v, want immediate API 400 failure", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if hits != tc.wantHits {
				t.Fatalf("made %d requests, want %d", hits, tc.wantHits)
			}
		})
	}
}

func TestIssueKeyPathEscapingAcrossEndpoints(t *testing.T) {
	const issueKey = "PROJ/13?part=1"
	wantPath := "/rest/api/3/issue/PROJ%2F13%3Fpart=1"
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.URL.EscapedPath())
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "e@x.com", "tok")
	if err := client.AssignIssue(issueKey, "account"); err != nil {
		t.Fatal(err)
	}
	if err := client.UpdateIssue(issueKey, map[string]interface{}{"summary": "updated"}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d requests, want 2", len(got))
	}
	for _, path := range got {
		if path != wantPath && path != wantPath+"/assignee" {
			t.Errorf("path = %q, want issue key escaped in %q or assignee variant", path, wantPath)
		}
	}
}

func TestGetIssueEditMetaUsesCentralizedEscapedPathAndPreservesAllowedValuesPresence(t *testing.T) {
	const issueKey = "PROJ/13?part=1"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/rest/api/3/issue/PROJ%2F13%3Fpart=1/editmeta"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		_, _ = fmt.Fprint(w, `{"fields":{"components":{"allowedValues":[{"name":"API"}]},"fixVersions":{}}}`)
	}))
	defer srv.Close()

	meta, err := NewClient(srv.URL, "e@x.com", "tok").GetIssueEditMeta(issueKey)
	if err != nil {
		t.Fatal(err)
	}
	if values := meta.Fields["components"].AllowedValues; values == nil || len(*values) != 1 || (*values)[0].Name != "API" {
		t.Fatalf("component allowedValues = %#v", values)
	}
	if meta.Fields["fixVersions"].AllowedValues != nil {
		t.Fatal("absent allowedValues must remain distinguishable from an empty list")
	}
}

func TestGetIssueFullWithStoryPointsRejectsInvalidSelectorBeforeRequest(t *testing.T) {
	invalid := []string{
		"",
		"customfield_0",
		"customfield_01",
		"customfield_-1",
		"customfield_10016,summary",
		"customfield_10016&fields=summary",
		"customfield_10016 ",
		" customfield_10016",
		"customfield_10016/extra",
		"CUSTOMFIELD_10016",
	}
	for _, fieldID := range invalid {
		t.Run(fmt.Sprintf("selector_%q", fieldID), func(t *testing.T) {
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer srv.Close()

			_, err := NewClient(srv.URL, "e@x.com", "tok").GetIssueFullWithStoryPoints("A-1", fieldID)
			if err == nil {
				t.Fatal("invalid selector was accepted")
			}
			if requests != 0 {
				t.Fatalf("invalid selector made %d requests, want zero", requests)
			}
		})
	}
}

func TestGetIssueFullWithStoryPointsRequestsOnlySelectedCustomFieldAndDecodesTriState(t *testing.T) {
	tests := []struct {
		name      string
		fields    string
		wantValue string
		wantNull  bool
		wantErr   string
	}{
		{name: "number", fields: `{"summary":"Issue","customfield_10016":9007199254740993.125}`, wantValue: "9007199254740993.125"},
		{name: "null", fields: `{"summary":"Issue","customfield_10016":null}`, wantNull: true},
		{name: "missing", fields: `{"summary":"Issue"}`, wantErr: "missing"},
		{name: "string", fields: `{"summary":"Issue","customfield_10016":"5"}`, wantErr: "not a number or null"},
		{name: "boolean", fields: `{"summary":"Issue","customfield_10016":true}`, wantErr: "not a number or null"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requested := strings.Split(r.URL.Query().Get("fields"), ",")
				customFields := 0
				for _, field := range requested {
					if strings.HasPrefix(field, "customfield_") {
						customFields++
						if field != "customfield_10016" {
							t.Errorf("unexpected custom field %q", field)
						}
					}
				}
				if customFields != 1 {
					t.Errorf("custom fields requested = %d, want exactly 1 in %q", customFields, r.URL.Query().Get("fields"))
				}
				_, _ = fmt.Fprintf(w, `{"key":"A-1","fields":%s}`, tc.fields)
			}))
			defer srv.Close()

			issue, err := NewClient(srv.URL, "e@x.com", "tok").GetIssueFullWithStoryPoints("A-1", "customfield_10016")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if issue.StoryPoints == nil {
				t.Fatal("selected story points state is nil")
			}
			if tc.wantNull {
				if issue.StoryPoints.Value != nil {
					t.Fatalf("value = %v, want null", issue.StoryPoints.Value)
				}
			} else if issue.StoryPoints.Value == nil || issue.StoryPoints.Value.String() != tc.wantValue {
				t.Fatalf("value = %v, want %s", issue.StoryPoints.Value, tc.wantValue)
			}
		})
	}
}

func TestGetIssueFullHierarchyFromSingleResponse(t *testing.T) {
	for _, selected := range []bool{false, true} {
		t.Run(fmt.Sprintf("story points selected=%t", selected), func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodGet || r.URL.Path != "/rest/api/3/issue/A-1" {
					t.Errorf("request = %s %s", r.Method, r.URL.Path)
				}
				fields := "," + r.URL.Query().Get("fields") + ","
				for _, name := range []string{"parent", "subtasks"} {
					if !strings.Contains(fields, ","+name+",") {
						t.Errorf("missing %s in %s", name, fields)
					}
				}
				if selected != strings.Contains(fields, ",customfield_10016,") {
					t.Errorf("story-points selector mismatch: %s", fields)
				}
				points := ""
				if selected {
					points = `,"customfield_10016":5.5`
				}
				_, _ = fmt.Fprintf(w, `{"key":"A-1","fields":{"parent":{"key":"A-0","fields":{"summary":"Parent","status":{"name":"Open","statusCategory":{"key":"new"}}}},"subtasks":[{"key":"A-2","fields":{"summary":"Child","status":{"name":"Complete","statusCategory":{"key":"done"}}}}]%s}}`, points)
			}))
			defer srv.Close()
			client := NewClient(srv.URL, "email", "token")
			var issue *Issue
			var err error
			if selected {
				issue, err = client.GetIssueFullWithStoryPoints("A-1", "customfield_10016")
			} else {
				issue, err = client.GetIssueFull("A-1")
			}
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 || issue.Fields.Parent == nil || issue.Fields.Parent.Key != "A-0" || issue.Fields.Subtasks == nil || len(*issue.Fields.Subtasks) != 1 {
				t.Fatalf("calls=%d hierarchy=%+v", calls, issue.Fields)
			}
		})
	}
}

func TestGetIssueFullRequestsAndDecodesUpdateFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, field := range []string{"duedate", "components", "fixVersions"} {
			if !strings.Contains(","+r.URL.Query().Get("fields")+",", ","+field+",") {
				t.Errorf("fields query omitted %q: %q", field, r.URL.Query().Get("fields"))
			}
		}
		_, _ = fmt.Fprint(w, `{"key":"A-1","fields":{"duedate":"2026-10-31","components":[{"name":"API"}],"fixVersions":[{"name":"v1.0"}]}}`)
	}))
	defer srv.Close()

	issue, err := NewClient(srv.URL, "e@x.com", "tok").GetIssueFull("A-1")
	if err != nil {
		t.Fatal(err)
	}
	if issue.Fields.DueDate == nil || *issue.Fields.DueDate != "2026-10-31" || len(issue.Fields.FixVersions) != 1 {
		t.Fatalf("decoded fields = %#v", issue.Fields)
	}
}

func TestSearchTextEscapesJQLLiteralBeforeURLEncoding(t *testing.T) {
	tests := []struct {
		name  string
		query string
		jql   string
	}{
		{name: "plain text", query: "login failure", jql: `text ~ "login failure"`},
		{name: "backslash and quote", query: `path\segment "quoted"`, jql: `text ~ "path\\segment \"quoted\""`},
		{name: "trailing backslash", query: `folder\`, jql: `text ~ "folder\\"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("jql"); got != tt.jql {
					t.Errorf("JQL query = %q, want %q", got, tt.jql)
				}
				if got := r.URL.Query().Get("maxResults"); got != "50" {
					t.Errorf("maxResults = %q, want 50", got)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `{"issues":[],"total":0,"maxResults":50}`)
			}))
			defer srv.Close()

			client := NewClient(srv.URL, "e@x.com", "tok")
			if _, _, err := client.SearchText(tt.query, 50); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSearchJQLPreservesQueryAtServerBoundary(t *testing.T) {
	const wantJQL = `project = "A&B" AND summary ~ "50% + #tag"`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("jql"); got != wantJQL {
			t.Errorf("JQL query = %q, want %q", got, wantJQL)
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprint(w, `{"issues":[],"total":0,"maxResults":50}`); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "e@x.com", "tok")
	if _, _, err := client.SearchJQL(wantJQL, 0); err != nil {
		t.Fatal(err)
	}
}

// A 2xx response whose body does not decode must surface as an error: every
// decode path returns the json.Unmarshal failure instead of handing back zero
// values that read like an empty result (the contract fixed in 4f58abc).
func TestMalformedSuccessBodySurfacesDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Truncated JSON: syntactically invalid for every target type.
		if _, err := fmt.Fprint(w, `{"issues": [}`); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "e@x.com", "tok")

	tests := []struct {
		name string
		call func() error
	}{
		{"TestConnection", func() error { _, err := client.TestConnection(); return err }},
		{"GetBoards", func() error { _, err := client.GetBoards(); return err }},
		{"GetBoardConfig", func() error { _, err := client.GetBoardConfig(1); return err }},
		{"GetSprints", func() error { _, err := client.GetSprints(1); return err }},
		{"GetBoardIssues", func() error { _, err := client.GetBoardIssues(1, 0); return err }},
		{"GetIssue", func() error { _, err := client.GetIssue("A-1"); return err }},
		{"GetIssueFull", func() error { _, err := client.GetIssueFull("A-1"); return err }},
		{"GetIssueEditMeta", func() error { _, err := client.GetIssueEditMeta("A-1"); return err }},
		{"GetProjects", func() error { _, err := client.GetProjects(); return err }},
		{"SearchJQL", func() error { _, _, err := client.SearchJQL("project = A", 10); return err }},
		{"GetTransitions", func() error { _, err := client.GetTransitions("A-1"); return err }},
		{"GetWorklog", func() error { _, err := client.GetWorklog("A-1"); return err }},
		{"CreateIssue", func() error {
			_, err := client.CreateIssue("A", "Task", "summary", nil)
			return err
		}},
		{"GetIssueTypes", func() error { _, err := client.GetIssueTypes("A"); return err }},
		{"userPicker", func() error { _, err := client.userPicker("rene@devoost.com"); return err }},
		{"getUser", func() error { _, err := client.getUser("712020:7e4261c7"); return err }},
		// Both fallbacks fail here, so the resolver must say so rather than
		// report a plain "no user found".
		{"ResolveAccountID", func() error {
			_, _, err := client.ResolveAccountID("rene@devoost.com")
			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Fatal("malformed 2xx body was accepted; want a decode error")
			}
		})
	}
}

// A well-formed body of the wrong shape is a decode error too, so the contract
// does not depend on the body being syntactically broken.
func TestWrongShapeSuccessBodySurfacesDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Valid JSON, but an array where the endpoint promises an object.
		if _, err := fmt.Fprint(w, `[]`); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "e@x.com", "tok")
	if _, err := client.GetIssue("A-1"); err == nil {
		t.Fatal("wrong-shape 2xx body was accepted; want a decode error")
	}
}

// ResolveAccountID must go picker → exact email verification → JQL fallback,
// because /user/username is gone and users/search no longer exposes emails.
func TestResolveAccountID(t *testing.T) {
	const email = "rene@devoost.com"
	const reneID = "712020:7e4261c7"
	const otherID = "557058:other"

	tests := []struct {
		name string
		// picker: JSON for the picker response; "" → picker 404s
		picker string
		// emails per accountId for the /user?accountId verification step
		emails map[string]string
		// jql: JSON for the search/jql response; "" → endpoint 404s
		jql     string
		wantID  string
		wantErr bool
	}{
		{
			name:   "picker sole candidate verified by email",
			picker: `{"users":[{"accountId":"` + reneID + `","displayName":"René"}],"total":1}`,
			emails: map[string]string{reneID: email},
			wantID: reneID,
		},
		{
			name:   "picker fuzzy match rejected, second candidate wins",
			picker: `{"users":[{"accountId":"` + otherID + `","displayName":"Other"},{"accountId":"` + reneID + `","displayName":"René"}],"total":2}`,
			emails: map[string]string{otherID: "other@x.com", reneID: email},
			wantID: reneID,
		},
		{
			name:   "emails hidden from API: sole picker match accepted",
			picker: `{"users":[{"accountId":"` + reneID + `","displayName":"René"}],"total":1}`,
			emails: map[string]string{reneID: ""},
			wantID: reneID,
		},
		{
			name:   "picker empty falls back to JQL",
			picker: `{"users":[],"total":0}`,
			emails: map[string]string{},
			jql:    `{"issues":[{"fields":{"creator":{"accountId":"` + reneID + `","displayName":"René","emailAddress":"RENE@devoost.com"}}}]}`,
			wantID: reneID,
		},
		{
			name:   "picker down falls back to JQL",
			picker: "",
			jql:    `{"issues":[{"fields":{"assignee":{"accountId":"` + reneID + `","displayName":"René","emailAddress":"` + email + `"}}}]}`,
			wantID: reneID,
		},
		{
			name:    "no path resolves the email",
			picker:  `{"users":[],"total":0}`,
			jql:     `{"issues":[]}`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/rest/api/2/user/picker":
					if tc.picker == "" {
						w.WriteHeader(404)
						return
					}
					if _, err := fmt.Fprint(w, tc.picker); err != nil {
						t.Errorf("write picker response: %v", err)
					}
				case "/rest/api/3/user":
					e, known := tc.emails[r.URL.Query().Get("accountId")]
					if !known {
						w.WriteHeader(404)
						return
					}
					if _, err := fmt.Fprintf(w, `{"accountId":%q,"displayName":"René","emailAddress":%q}`,
						r.URL.Query().Get("accountId"), e); err != nil {
						t.Errorf("write user response: %v", err)
					}
				case "/rest/api/3/search/jql":
					if tc.jql == "" {
						w.WriteHeader(404)
						return
					}
					if _, err := fmt.Fprint(w, tc.jql); err != nil {
						t.Errorf("write jql response: %v", err)
					}
				default:
					w.WriteHeader(404)
				}
			}))
			defer srv.Close()

			client := NewClient(srv.URL, "e@x.com", "tok")
			id, _, err := client.ResolveAccountID(email)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got id %q", id)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if id != tc.wantID {
				t.Fatalf("got %q, want %q", id, tc.wantID)
			}
		})
	}
}
