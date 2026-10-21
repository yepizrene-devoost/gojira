package lifecycle

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func transitions(t *testing.T, raw string) []jira.Transition {
	t.Helper()
	var out []jira.Transition
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSelectPicksTheOnlyTransitionOfTheRightCategory(t *testing.T) {
	for _, tc := range []struct {
		name   string
		action Action
		raw    string
		wantID string
	}{
		{"close picks done", Close, `[{"id":"7","name":"Start","to":{"name":"Doing","statusCategory":{"key":"indeterminate"}}},{"id":"9","name":"Finish","to":{"name":"Done","statusCategory":{"key":"done"}}}]`, "9"},
		{"reopen picks new", Reopen, `[{"id":"4","name":"Finish","to":{"name":"Done","statusCategory":{"key":"done"}}},{"id":"5","name":"Reopen","to":{"name":"To Do","statusCategory":{"key":"new"}}}]`, "5"},
		{"reopen picks indeterminate", Reopen, `[{"id":"3","name":"Resume","to":{"statusCategory":{"key":"indeterminate"}}}]`, "3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Select(tc.action, transitions(t, tc.raw))
			if err != nil || got.ID != tc.wantID {
				t.Fatalf("transition=%+v err=%v, want id %s", got, err, tc.wantID)
			}
		})
	}
}

func TestSelectRejectsZeroOrSeveralCandidatesWithTheCandidateList(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		action     Action
		raw        string
	}{
		{"none", "close requires exactly one available transition; candidates: none", Close, `[]`},
		{"without destination category", "close requires exactly one available transition; candidates: none", Close, `[{"id":"1","name":"Done"},{"id":"2","name":"Other","to":{"name":"Other"}}]`},
		{"close ambiguous", "close requires exactly one available transition; candidates: 1: A -> Done (done); 2: B ->  (done)", Close, `[{"id":"1","name":"A","to":{"name":"Done","statusCategory":{"key":"done"}}},{"id":"2","name":"B","to":{"statusCategory":{"key":"done"}}}]`},
		{"reopen ambiguous", "reopen requires exactly one available transition; candidates: 3: X -> Open (new); 4: Y -> Doing (indeterminate)", Reopen, `[{"id":"3","name":"X","to":{"name":"Open","statusCategory":{"key":"new"}}},{"id":"4","name":"Y","to":{"name":"Doing","statusCategory":{"key":"indeterminate"}}}]`},
		{"reopen with only done", "reopen requires exactly one available transition; candidates: none", Reopen, `[{"id":"4","name":"Finish","to":{"name":"Done","statusCategory":{"key":"done"}}}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Select(tc.action, transitions(t, tc.raw))
			var selection *SelectionError
			if !errors.As(err, &selection) || err.Error() != tc.want || got.ID != "" {
				t.Fatalf("transition=%+v err=%v, want a SelectionError %q", got, err, tc.want)
			}
		})
	}
}

func TestSelectRejectsAnUnsupportedAction(t *testing.T) {
	got, err := Select(Action("delete"), transitions(t, `[{"id":"1","name":"A","to":{"name":"Done","statusCategory":{"key":"done"}}}]`))
	var selection *SelectionError
	if err == nil || errors.As(err, &selection) || got.ID != "" {
		t.Fatalf("transition=%+v err=%v, want a plain unsupported-action error that is not a SelectionError", got, err)
	}
}
