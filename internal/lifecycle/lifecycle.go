// Package lifecycle holds the rule that chooses which workflow transition closes or
// reopens an issue, so every surface applies the same one.
package lifecycle

import (
	"fmt"
	"strings"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

// Action is a lifecycle request.
type Action string

const (
	Close  Action = "close"
	Reopen Action = "reopen"
)

// SelectionError means the available transitions do not identify exactly one. It is a
// validation failure: nothing has been sent to Jira when it is returned.
type SelectionError struct {
	Action     Action
	Candidates []string
}

func (e *SelectionError) Error() string {
	listing := "none"
	if len(e.Candidates) > 0 {
		listing = strings.Join(e.Candidates, "; ")
	}
	return fmt.Sprintf("%s requires exactly one available transition; candidates: %s", e.Action, listing)
}

// Select returns the only available transition whose destination status category fits
// the action: done for Close, new or indeterminate for Reopen. Transitions without a
// destination category are ignored.
func Select(action Action, transitions []jira.Transition) (jira.Transition, error) {
	if action != Close && action != Reopen {
		return jira.Transition{}, fmt.Errorf("unsupported lifecycle action %q", action)
	}
	var candidates []jira.Transition
	var descriptions []string
	for _, transition := range transitions {
		if transition.To == nil || transition.To.Category == nil {
			continue
		}
		category := transition.To.Category.Key
		if !fits(action, category) {
			continue
		}
		candidates = append(candidates, transition)
		descriptions = append(descriptions, fmt.Sprintf("%s: %s -> %s (%s)", transition.ID, transition.Name, transition.To.Name, category))
	}
	if len(candidates) != 1 {
		return jira.Transition{}, &SelectionError{Action: action, Candidates: descriptions}
	}
	return candidates[0], nil
}

func fits(action Action, category string) bool {
	if action == Close {
		return category == "done"
	}
	return category == "new" || category == "indeterminate"
}
