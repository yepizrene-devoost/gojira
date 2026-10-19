package jira

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestProjectPagination(t *testing.T) {
	for _, mode := range []string{"last", "empty", "repeat", "cap", "error", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/rest/api/3/project/search" || r.URL.Query().Get("maxResults") != "50" {
					t.Errorf("unexpected request %s", r.URL)
				}
				if r.URL.Query().Get("startAt") != fmt.Sprint((calls-1)*50) {
					t.Errorf("startAt=%s", r.URL.Query().Get("startAt"))
				}
				if mode == "error" {
					w.WriteHeader(403)
					return
				}
				if mode == "invalid" {
					_, _ = fmt.Fprint(w, `{`)
					return
				}
				values := []Project{{Key: fmt.Sprintf("P%d", calls), Name: "Project"}}
				if mode == "repeat" {
					values[0].Key = "SAME"
				}
				if mode == "empty" && calls == 2 {
					values = nil
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"startAt": (calls - 1) * 50, "maxResults": 50, "isLast": mode == "last" && calls == 2, "values": values})
			}))
			defer server.Close()
			projects, err := NewClient(server.URL, "email", "token").GetProjects()
			if mode == "last" {
				if err != nil || len(projects) != 2 || calls != 2 {
					t.Fatalf("len=%d calls=%d err=%v", len(projects), calls, err)
				}
			} else if err == nil {
				// An empty page that is not marked last must not be reported
				// as a complete list.
				t.Fatal("expected error")
			}
			if mode == "repeat" && calls != 2 {
				t.Fatalf("repeat took %d requests", calls)
			}
			if mode == "cap" && calls != 1000 {
				t.Fatalf("cap took %d requests", calls)
			}
		})
	}
}

func TestProjectsPreserveLegacyOutput(t *testing.T) {
	for _, size := range []int{0, 1, 50, 100} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			legacy := make([]Project, size)
			for i := range legacy {
				legacy[i] = Project{Key: fmt.Sprintf("P%d", i), Name: fmt.Sprintf("Project %d", i)}
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				start := calls * 50
				calls++
				end := start + 50
				if end > size {
					end = size
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"startAt": start, "maxResults": 50, "isLast": end == size, "values": legacy[start:end]})
			}))
			defer server.Close()
			got, err := NewClient(server.URL, "email", "token").GetProjects()
			if err != nil || !reflect.DeepEqual(got, legacy) {
				t.Fatalf("got=%v err=%v", got, err)
			}
			oldJSON, _ := json.Marshal(legacy)
			newJSON, _ := json.Marshal(got)
			if string(oldJSON) != string(newJSON) {
				t.Fatal("legacy JSON changed")
			}
		})
	}
}

func TestProjectAndMyself(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/3/project/ABC":
			_, _ = fmt.Fprint(w, `{"key":"ABC","name":"Example"}`)
		case "/rest/api/3/myself":
			_, _ = fmt.Fprint(w, `{"accountId":"id","displayName":"Example","active":true}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, "email", "token")
	p, err := client.GetProject("ABC")
	if err != nil || p.Key != "ABC" {
		t.Fatalf("project=%v err=%v", p, err)
	}
	u, err := client.GetMyself()
	if err != nil || u.AccountID != "id" || !u.Active {
		t.Fatalf("user=%v err=%v", u, err)
	}
}

// The legacy /project endpoint listed projects by ascending numeric ID and no
// orderBy of the paginated endpoint reproduces it, so GetProjects restores it.
func TestProjectsKeepLegacyIDOrder(t *testing.T) {
	pages := []string{
		`{"startAt":0,"maxResults":2,"isLast":false,"values":[{"id":"10","key":"TEN","name":"Ten"},{"id":"2","key":"TWO","name":"Two"}]}`,
		`{"startAt":2,"maxResults":2,"isLast":true,"values":[{"id":"9","key":"NINE","name":"Nine"},{"id":"100","key":"HUNDRED","name":"Hundred"},{"id":"oops","key":"ODD","name":"Odd"}]}`,
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, pages[calls])
		calls++
	}))
	defer server.Close()
	projects, err := NewClient(server.URL, "email", "token").GetProjects()
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, p := range projects {
		keys = append(keys, p.Key)
	}
	// Non-numeric IDs sort last, keeping their API order.
	if want := []string{"TWO", "NINE", "TEN", "HUNDRED", "ODD"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys=%v want %v", keys, want)
	}
	encoded, _ := json.Marshal(projects[0])
	if string(encoded) != `{"key":"TWO","name":"Two"}` {
		t.Fatalf("JSON shape changed: %s", encoded)
	}
}
