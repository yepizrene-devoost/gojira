package mcpserver

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// S1/S4: update_comment replaces the body with plain text as ADF.
func TestUpdateCommentWriteSendsADF(t *testing.T) {
	now := time.Now()
	a, _, log := writeTestAdapter(t, 200, "", func() time.Time { return now })
	res := callWriteTest(t, a, nil, "update_comment", map[string]any{"key": "ABC-1", "commentId": "10001", "text": "edited text"})
	if res["mutationState"] != "applied" {
		t.Fatalf("res=%v", res)
	}
	want := []observedWriteRequest{{"PUT", "/rest/api/3/issue/ABC-1/comment/10001", map[string]any{
		"body": map[string]any{
			"type": "doc", "version": float64(1),
			"content": []any{map[string]any{
				"type":    "paragraph",
				"content": []any{map[string]any{"type": "text", "text": "edited text"}},
			}},
		},
	}}}
	if got := log.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("requests=%+v want %+v", got, want)
	}
}

// S4: editing must state the ADF replacement semantics in the tool description.
func TestUpdateCommentDescriptionWarnsAboutADFLoss(t *testing.T) {
	for _, entry := range writeTools() {
		if entry.action == "update_comment" {
			if !strings.Contains(entry.description, "replaces the whole comment body") {
				t.Fatalf("description=%q", entry.description)
			}
			return
		}
	}
	t.Fatal("update_comment not registered")
}

// S4: validation errors fire before any request.
func TestUpdateCommentValidation(t *testing.T) {
	now := time.Now()
	a, _, log := writeTestAdapter(t, 200, "", func() time.Time { return now })
	for name, args := range map[string]map[string]any{
		"blank text":    {"key": "ABC-1", "commentId": "10001", "text": " "},
		"missing text":  {"key": "ABC-1", "commentId": "10001"},
		"missing id":    {"key": "ABC-1", "text": "x"},
		"untrimmed id":  {"key": "ABC-1", "commentId": " 10001", "text": "x"},
		"missing key":   {"commentId": "10001", "text": "x"},
		"forbidden arg": {"key": "ABC-1", "commentId": "10001", "text": "x", "clearComment": "true"},
	} {
		res := callWriteTest(t, a, nil, "update_comment", args)
		if res["code"] != "validation_error" || res["mutationState"] != "not_applied" {
			t.Fatalf("%s: res=%v", name, res)
		}
	}
	if len(log.snapshot()) != 0 {
		t.Fatalf("requests=%v", log.snapshot())
	}
}

// S4: delete_comment is a two-step, single-use, expiring confirmation flow
// whose token binds action, key and commentId.
func TestDeleteCommentConfirmationFlow(t *testing.T) {
	now := time.Now()
	a, store, log := writeTestAdapter(t, 200, "", func() time.Time { return now })
	apply := func(token string) map[string]any {
		return callWriteTest(t, a, store, "delete_comment", map[string]any{"key": "ABC-1", "commentId": "10001", "confirmationToken": token})
	}
	preview := callWriteTest(t, a, store, "delete_comment", map[string]any{"key": " abc-1 ", "commentId": "10001"})
	if preview["mutationState"] != "not_applied" || len(log.snapshot()) != 0 {
		t.Fatalf("preview=%v requests=%v", preview, log.snapshot())
	}
	data := preview["data"].(map[string]any)
	if !reflect.DeepEqual(data["preview"], map[string]any{"action": "delete_comment", "issueKey": "ABC-1", "commentId": "10001"}) || len(data) != 2 {
		t.Fatalf("preview data=%v", data)
	}
	token := data["confirmationToken"].(string)

	t.Run("unknown token", func(t *testing.T) {
		res := apply("unknown")
		if res["code"] != "invalid_confirmation" || res["mutationState"] != "not_applied" || len(log.snapshot()) != 0 {
			t.Fatalf("result=%v requests=%v", res, log.snapshot())
		}
	})
	t.Run("worklog token cannot delete comment", func(t *testing.T) {
		worklog := callWriteTest(t, a, store, "delete_worklog", map[string]any{"key": "ABC-1", "worklogId": "7"})
		worklogToken := worklog["data"].(map[string]any)["confirmationToken"].(string)
		res := apply(worklogToken)
		if res["code"] != "invalid_confirmation" || len(log.snapshot()) != 0 {
			t.Fatalf("result=%v requests=%v", res, log.snapshot())
		}
	})
	t.Run("expired token", func(t *testing.T) {
		now = now.Add(2 * time.Minute)
		res := apply(token)
		if res["code"] != "invalid_confirmation" || len(log.snapshot()) != 0 {
			t.Fatalf("result=%v requests=%v", res, log.snapshot())
		}
	})
	t.Run("valid apply then reuse", func(t *testing.T) {
		fresh := callWriteTest(t, a, store, "delete_comment", map[string]any{"key": "ABC-1", "commentId": "10001"})
		freshToken := fresh["data"].(map[string]any)["confirmationToken"].(string)
		applied := apply(freshToken)
		if applied["mutationState"] != "applied" {
			t.Fatalf("applied=%v", applied)
		}
		want := []observedWriteRequest{{"DELETE", "/rest/api/3/issue/ABC-1/comment/10001", nil}}
		if got := log.snapshot(); !reflect.DeepEqual(got, want) {
			t.Fatalf("requests=%+v want %+v", got, want)
		}
		res := apply(freshToken)
		if res["code"] != "invalid_confirmation" || len(log.snapshot()) != 1 {
			t.Fatalf("result=%v requests=%v", res, log.snapshot())
		}
	})
}

// S4: a mutation failure reports the mutation state (unknown on network
// ambiguity) and never retries.
func TestCommentWriteFailureReportsState(t *testing.T) {
	now := time.Now()
	a, _, _ := writeTestAdapter(t, 500, "", func() time.Time { return now })
	res := callWriteTest(t, a, nil, "update_comment", map[string]any{"key": "ABC-1", "commentId": "10001", "text": "x"})
	if res["code"] != "mutation_failed" {
		t.Fatalf("res=%v", res)
	}
	state, _ := res["mutationState"].(string)
	if state != "unknown" && state != "not_applied" {
		t.Fatalf("mutationState=%q", state)
	}
}
