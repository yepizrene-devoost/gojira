package mcpserver

import (
	"reflect"
	"testing"
	"time"
)

func TestWriteDeleteGuardsWithoutRequests(t *testing.T) {
	for _, scenario := range []string{"preview", "wrong_key", "expired", "reused", "invalid"} {
		t.Run(scenario, func(t *testing.T) {
			now := time.Now()
			a, store, log := writeTestAdapter(t, 200, "", func() time.Time { return now })
			preview := callWriteTest(t, a, store, "delete", map[string]any{"key": " abc-1 "})
			if preview["mutationState"] != "not_applied" || len(log.snapshot()) != 0 {
				t.Fatalf("preview=%v requests=%v", preview, log.snapshot())
			}
			data := preview["data"].(map[string]any)
			if !reflect.DeepEqual(data["preview"], map[string]any{"action": "delete", "issueKey": "ABC-1"}) || len(data) != 2 {
				t.Fatalf("preview data=%v", data)
			}
			if scenario == "preview" {
				return
			}
			token := data["confirmationToken"].(string)
			args := map[string]any{"key": "ABC-1", "confirmationToken": token}
			switch scenario {
			case "wrong_key":
				args["key"] = "ABC-2"
			case "expired":
				now = now.Add(2 * time.Minute)
			case "invalid":
				args["confirmationToken"] = "invalid"
			case "reused":
				applied := callWriteTest(t, a, store, "delete", args)
				if applied["mutationState"] != "applied" || !reflect.DeepEqual(log.snapshot(), []observedWriteRequest{{"DELETE", "/rest/api/3/issue/ABC-1", nil}}) {
					t.Fatalf("delete=%v requests=%v", applied, log.snapshot())
				}
			}
			before := len(log.snapshot())
			res := callWriteTest(t, a, store, "delete", args)
			if res["mutationState"] != "not_applied" || res["code"] != "invalid_confirmation" || len(log.snapshot()) != before {
				t.Fatalf("guard=%v requests=%v", res, log.snapshot())
			}
		})
	}
}
