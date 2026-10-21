package mcpserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

const (
	doneOnly         = `[{"id":"7","name":"Start","to":{"name":"Doing","statusCategory":{"key":"indeterminate"}}},{"id":"9","name":"Finish","to":{"name":"Done","statusCategory":{"key":"done"}}}]`
	newOnly          = `[{"id":"4","name":"Finish","to":{"name":"Done","statusCategory":{"key":"done"}}},{"id":"5","name":"Reopen","to":{"name":"To Do","statusCategory":{"key":"new"}}}]`
	closeAmbiguous   = `[{"id":"1","name":"A","to":{"name":"Done","statusCategory":{"key":"done"}}},{"id":"2","name":"B","to":{"statusCategory":{"key":"done"}}}]`
	reopenAmbiguous  = `[{"id":"3","name":"X","to":{"name":"Open","statusCategory":{"key":"new"}}},{"id":"4","name":"Y","to":{"name":"Doing","statusCategory":{"key":"indeterminate"}}}]`
	noDestinationCat = `[{"id":"1","name":"Done"},{"id":"2","name":"Other","to":{"name":"Other"}}]`
)

// lifecycleJira serves the transitions list and records every transition POST.
type lifecycleJira struct {
	transitions         string
	readStatus, postsAs int
	reads, posts        atomic.Int32
	body                atomic.Value
}

func (j *lifecycleJira) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/ABC-1/transitions" {
			t.Errorf("unexpected path %s %s", r.Method, r.URL.Path)
		}
		if r.Method == http.MethodPost {
			j.posts.Add(1)
			raw, _ := io.ReadAll(r.Body)
			j.body.Store(strings.TrimSpace(string(raw)))
			w.WriteHeader(j.postsAs)
			return
		}
		j.reads.Add(1)
		if j.readStatus != 0 {
			w.WriteHeader(j.readStatus)
			return
		}
		_, _ = fmt.Fprintf(w, `{"transitions":%s}`, j.transitions)
	}
}

func TestCloseAndReopenSendTheSingleTransitionOfTheRightCategory(t *testing.T) {
	for _, tc := range []struct {
		tool, transitions, wantID string
	}{
		{"gojira_close", doneOnly, "9"},
		{"gojira_reopen", newOnly, "5"},
	} {
		t.Run(tc.tool, func(t *testing.T) {
			jira := &lifecycleJira{transitions: tc.transitions, postsAs: http.StatusNoContent}
			cs := paritySession(t, true, jira.handler(t))
			isError, got := callWriteTool(t, cs, tc.tool, map[string]any{"key": " abc-1 "})
			body, _ := jira.body.Load().(string)
			if isError || got["action"] != strings.TrimPrefix(tc.tool, "gojira_") || got["issueKey"] != "ABC-1" || got["mutationState"] != "applied" || got["code"] != nil ||
				jira.reads.Load() != 1 || jira.posts.Load() != 1 || body != `{"transition":{"id":"`+tc.wantID+`"}}` {
				t.Fatalf("isError=%v result=%v reads=%d posts=%d body=%s", isError, got, jira.reads.Load(), jira.posts.Load(), body)
			}
		})
	}
}

func TestCloseAndReopenRejectZeroOrSeveralCandidatesWithoutMutating(t *testing.T) {
	for _, tc := range []struct {
		name, tool, transitions, message string
	}{
		{"close without candidates", "gojira_close", `[]`, "close requires exactly one available transition; candidates: none"},
		{"close ignores transitions without a destination category", "gojira_close", noDestinationCat, "close requires exactly one available transition; candidates: none"},
		{"close ambiguous", "gojira_close", closeAmbiguous, "close requires exactly one available transition; candidates: 1: A -> Done (done); 2: B ->  (done)"},
		{"reopen ambiguous", "gojira_reopen", reopenAmbiguous, "reopen requires exactly one available transition; candidates: 3: X -> Open (new); 4: Y -> Doing (indeterminate)"},
		{"reopen with only done transitions", "gojira_reopen", `[{"id":"4","name":"Finish","to":{"name":"Done","statusCategory":{"key":"done"}}}]`, "reopen requires exactly one available transition; candidates: none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jira := &lifecycleJira{transitions: tc.transitions, postsAs: http.StatusNoContent}
			cs := paritySession(t, true, jira.handler(t))
			isError, got := callWriteTool(t, cs, tc.tool, map[string]any{"key": "ABC-1"})
			if !isError || got["code"] != "validation_error" || got["message"] != tc.message || got["mutationState"] != "not_applied" || got["hint"] != "Correct arguments before retrying." || jira.posts.Load() != 0 {
				t.Fatalf("isError=%v result=%v posts=%d", isError, got, jira.posts.Load())
			}
		})
	}
}

func TestCloseAndReopenValidateArgumentsBeforeAnyRequest(t *testing.T) {
	for _, tc := range []struct {
		name, tool, args, message string
	}{
		{"invalid key", "gojira_close", `{"key":"not a key"}`, "invalid issue key"},
		{"missing key", "gojira_reopen", `{}`, "required argument missing"},
		{"unexpected status argument", "gojira_close", `{"key":"ABC-1","status":"Done"}`, "unexpected argument"},
		{"unexpected sprint argument", "gojira_reopen", `{"key":"ABC-1","sprintId":5}`, "unexpected argument"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			cs := paritySession(t, true, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			})
			isError, got := callWriteTool(t, cs, tc.tool, json.RawMessage(tc.args))
			if !isError || got["code"] != "validation_error" || got["message"] != tc.message || got["mutationState"] != "not_applied" || calls.Load() != 0 {
				t.Fatalf("isError=%v result=%v calls=%d", isError, got, calls.Load())
			}
		})
	}
}

func TestCloseAndReopenReportTheMutationStateAndNeverRetry(t *testing.T) {
	for _, tc := range []struct {
		name, tool, transitions string
		readStatus, postStatus  int
		state, hintHas          string
		wantReads, wantPosts    int32
	}{
		{"transitions cannot be read", "gojira_close", doneOnly, http.StatusForbidden, 0, "not_applied", "Check Jira access before retrying.", 1, 0},
		{"post fails with a server error", "gojira_close", doneOnly, 0, http.StatusInternalServerError, "unknown", "Verify the issue in Jira before retrying", 1, 1},
		{"post is rejected", "gojira_reopen", newOnly, 0, http.StatusBadRequest, "not_applied", "Check Jira access before retrying.", 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jira := &lifecycleJira{transitions: tc.transitions, readStatus: tc.readStatus, postsAs: tc.postStatus}
			cs := paritySession(t, true, jira.handler(t))
			isError, got := callWriteTool(t, cs, tc.tool, map[string]any{"key": "ABC-1"})
			hint, _ := got["hint"].(string)
			if !isError || got["code"] != "mutation_failed" || got["mutationState"] != tc.state || !strings.Contains(hint, tc.hintHas) || jira.reads.Load() != tc.wantReads || jira.posts.Load() != tc.wantPosts {
				t.Fatalf("isError=%v result=%v reads=%d posts=%d", isError, got, jira.reads.Load(), jira.posts.Load())
			}
		})
	}
}

// The existing status-name transition keeps its behavior next to the new tools.
func TestMoveByStatusNameStillWorksAlongsideTheLifecycleTools(t *testing.T) {
	jira := &lifecycleJira{transitions: doneOnly, postsAs: http.StatusNoContent}
	cs := paritySession(t, true, jira.handler(t))
	isError, got := callWriteTool(t, cs, "gojira_move", map[string]any{"key": "ABC-1", "status": "Finish"})
	body, _ := jira.body.Load().(string)
	if isError || got["mutationState"] != "applied" || jira.posts.Load() != 1 || body != `{"transition":{"id":"9"}}` {
		t.Fatalf("isError=%v result=%v posts=%d body=%s", isError, got, jira.posts.Load(), body)
	}
	isError, got = callWriteTool(t, cs, "gojira_move", map[string]any{"key": "ABC-1", "status": "Nope"})
	if !isError || got["message"] != "status must match exactly one available transition" || jira.posts.Load() != 1 {
		t.Fatalf("isError=%v result=%v posts=%d", isError, got, jira.posts.Load())
	}
}
