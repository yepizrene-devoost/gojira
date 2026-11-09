package mcpserver

import (
	"reflect"
	"testing"
	"time"
)

// S6: update_worklog sends only the supplied fields on the worklog entry path.
func TestUpdateWorklogRequests(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		args       map[string]any
	}{
		{"time only", `{"timeSpent":"3h"}`, map[string]any{"key": "ABC-1", "worklogId": "7", "timeSpent": "3h"}},
		{"comment only", `{"comment":` + expectedWriteADF + `}`, map[string]any{"key": "ABC-1", "worklogId": "7", "comment": "Example"}},
		{"blank duration dropped", `{"comment":` + expectedWriteADF + `}`, map[string]any{"key": "ABC-1", "worklogId": "7", "timeSpent": "", "comment": "Example"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// S12: a supplied duration triggers one re-read after the PUT;
			// Jira reports 3h, so the result stays hint-free.
			a, store, log := writeTestAdapter(t, 200, `{"total":1,"worklogs":[{"id":"7","timeSpent":"3h"}]}`, time.Now)
			res := callWriteTest(t, a, store, "update_worklog", tc.args)
			if res["mutationState"] != "applied" {
				t.Fatalf("result=%v", res)
			}
			if _, hasHint := res["hint"]; hasHint {
				t.Fatalf("kept duration must not add a hint: %v", res)
			}
			want := []observedWriteRequest{{"PUT", "/rest/api/3/issue/ABC-1/worklog/7", decodedWriteJSON(t, tc.body)}}
			if timeSpent, _ := tc.args["timeSpent"].(string); timeSpent != "" {
				want = append(want, observedWriteRequest{"GET", "/rest/api/3/issue/ABC-1/worklog", nil})
			}
			if got := log.snapshot(); !reflect.DeepEqual(got, want) {
				t.Fatalf("requests=%+v want %+v", got, want)
			}
		})
	}
}

// S14d: clearComment issues exactly one request — clearing alone sends the
// null comment, and with a duration both fields land in a single PUT while the
// applied envelope keeps its hint-free shape.
func TestUpdateWorklogClearCommentRequests(t *testing.T) {
	for _, tc := range []struct {
		name    string
		getBody string
		args    map[string]any
		want    []observedWriteRequest
	}{
		{
			name: "clear alone",
			args: map[string]any{"key": "ABC-1", "worklogId": "7", "clearComment": true},
			want: []observedWriteRequest{{"PUT", "/rest/api/3/issue/ABC-1/worklog/7", decodedWriteJSON(t, `{"comment":null}`)}},
		},
		{
			name: "clear with blank comment",
			args: map[string]any{"key": "ABC-1", "worklogId": "7", "clearComment": true, "comment": " "},
			want: []observedWriteRequest{{"PUT", "/rest/api/3/issue/ABC-1/worklog/7", decodedWriteJSON(t, `{"comment":null}`)}},
		},
		{
			name:    "clear with duration",
			getBody: `{"total":1,"worklogs":[{"id":"7","timeSpent":"3h"}]}`,
			args:    map[string]any{"key": "ABC-1", "worklogId": "7", "clearComment": true, "timeSpent": "3h"},
			want: []observedWriteRequest{
				{"PUT", "/rest/api/3/issue/ABC-1/worklog/7", decodedWriteJSON(t, `{"timeSpent":"3h","comment":null}`)},
				{"GET", "/rest/api/3/issue/ABC-1/worklog", nil},
			},
		},
		{
			name:    "false keeps the plain update",
			getBody: `{"total":1,"worklogs":[{"id":"7","timeSpent":"3h"}]}`,
			args:    map[string]any{"key": "ABC-1", "worklogId": "7", "clearComment": false, "timeSpent": "3h"},
			want: []observedWriteRequest{
				{"PUT", "/rest/api/3/issue/ABC-1/worklog/7", decodedWriteJSON(t, `{"timeSpent":"3h"}`)},
				{"GET", "/rest/api/3/issue/ABC-1/worklog", nil},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, store, log := writeTestAdapter(t, 200, tc.getBody, time.Now)
			res := callWriteTest(t, a, store, "update_worklog", tc.args)
			if res["mutationState"] != "applied" {
				t.Fatalf("result=%v", res)
			}
			if _, hasHint := res["hint"]; hasHint {
				t.Fatalf("clearing must keep the hint-free applied envelope: %v", res)
			}
			if got := log.snapshot(); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("requests=%+v want %+v", got, tc.want)
			}
		})
	}
}

// S6: validation rules fail before any Jira request.
func TestUpdateWorklogValidationWithoutRequests(t *testing.T) {
	for _, tc := range []struct {
		name, message string
		args          map[string]any
	}{
		{"missing optionals", "one of timeSpent, comment or clearComment is required", map[string]any{"key": "ABC-1", "worklogId": "7"}},
		{"blank optionals", "one of timeSpent, comment or clearComment is required", map[string]any{"key": "ABC-1", "worklogId": "7", "timeSpent": "", "comment": " "}},
		{"clearComment false counts as nothing", "one of timeSpent, comment or clearComment is required", map[string]any{"key": "ABC-1", "worklogId": "7", "clearComment": false}},
		{"clearComment with comment", "clearComment cannot be combined with comment", map[string]any{"key": "ABC-1", "worklogId": "7", "clearComment": true, "comment": "Example"}},
		{"clearComment with comment and duration", "clearComment cannot be combined with comment", map[string]any{"key": "ABC-1", "worklogId": "7", "clearComment": true, "comment": "Example", "timeSpent": "2h"}},
		{"clearComment as string", "clearComment must be a boolean", map[string]any{"key": "ABC-1", "worklogId": "7", "clearComment": "true"}},
		{"invalid duration", "invalid timeSpent; use positive Jira durations such as 2h 30m", map[string]any{"key": "ABC-1", "worklogId": "7", "timeSpent": "soon"}},
		{"padded worklogId", "worklogId must be trimmed", map[string]any{"key": "ABC-1", "worklogId": " 7 ", "timeSpent": "2h"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, store, log := writeTestAdapter(t, 200, "", time.Now)
			res := callWriteTest(t, a, store, "update_worklog", tc.args)
			if res["code"] != "validation_error" || res["message"] != tc.message || res["mutationState"] != "not_applied" || len(log.snapshot()) != 0 {
				t.Fatalf("result=%v requests=%v", res, log.snapshot())
			}
		})
	}
}

// S12: a duration Jira did not keep surfaces through the write result hint
// while the applied mutation keeps its applied state and the re-read is the
// only extra request.
func TestUpdateWorklogVerifyReportsMismatchInHint(t *testing.T) {
	a, store, log := writeTestAdapter(t, 200, `{"total":1,"worklogs":[{"id":"7","timeSpent":"15m"}]}`, time.Now)
	res := callWriteTest(t, a, store, "update_worklog", map[string]any{"key": "ABC-1", "worklogId": "7", "timeSpent": "3h"})
	if res["mutationState"] != "applied" {
		t.Fatalf("mutationState=%v, the update was applied", res["mutationState"])
	}
	if res["hint"] != "Jira reports 15m for worklog 7 (requested 3h)" {
		t.Fatalf("hint=%v, want the verification warning", res["hint"])
	}
	want := []observedWriteRequest{
		{"PUT", "/rest/api/3/issue/ABC-1/worklog/7", decodedWriteJSON(t, `{"timeSpent":"3h"}`)},
		{"GET", "/rest/api/3/issue/ABC-1/worklog", nil},
	}
	if got := log.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("requests=%+v want %+v", got, want)
	}
}

// S7: delete_worklog is a two-step, single-use, expiring confirmation flow.
func TestDeleteWorklogConfirmationFlow(t *testing.T) {
	now := time.Now()
	a, store, log := writeTestAdapter(t, 200, "", func() time.Time { return now })
	apply := func(token string) map[string]any {
		return callWriteTest(t, a, store, "delete_worklog", map[string]any{"key": "ABC-1", "worklogId": "7", "confirmationToken": token})
	}
	preview := callWriteTest(t, a, store, "delete_worklog", map[string]any{"key": " abc-1 ", "worklogId": "7"})
	if preview["mutationState"] != "not_applied" || len(log.snapshot()) != 0 {
		t.Fatalf("preview=%v requests=%v", preview, log.snapshot())
	}
	data := preview["data"].(map[string]any)
	if !reflect.DeepEqual(data["preview"], map[string]any{"action": "delete_worklog", "issueKey": "ABC-1", "worklogId": "7"}) || len(data) != 2 {
		t.Fatalf("preview data=%v", data)
	}
	token := data["confirmationToken"].(string)

	t.Run("unknown token", func(t *testing.T) {
		res := apply("unknown")
		if res["code"] != "invalid_confirmation" || res["mutationState"] != "not_applied" || len(log.snapshot()) != 0 {
			t.Fatalf("result=%v requests=%v", res, log.snapshot())
		}
	})
	t.Run("cross-tool token", func(t *testing.T) {
		issue := callWriteTest(t, a, store, "delete", map[string]any{"key": "ABC-1"})
		issueToken := issue["data"].(map[string]any)["confirmationToken"].(string)
		res := apply(issueToken)
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
		fresh := callWriteTest(t, a, store, "delete_worklog", map[string]any{"key": "ABC-1", "worklogId": "7"})
		freshToken := fresh["data"].(map[string]any)["confirmationToken"].(string)
		applied := apply(freshToken)
		if applied["mutationState"] != "applied" {
			t.Fatalf("applied=%v", applied)
		}
		want := []observedWriteRequest{{"DELETE", "/rest/api/3/issue/ABC-1/worklog/7", nil}}
		if got := log.snapshot(); !reflect.DeepEqual(got, want) {
			t.Fatalf("requests=%+v want %+v", got, want)
		}
		res := apply(freshToken)
		if res["code"] != "invalid_confirmation" || len(log.snapshot()) != 1 {
			t.Fatalf("result=%v requests=%v", res, log.snapshot())
		}
	})
}
