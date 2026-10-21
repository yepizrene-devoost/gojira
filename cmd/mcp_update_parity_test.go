package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

// Fixed, input-free messages: an agent learns which field to fix without any ticket text echoed back.
const (
	msgLabels           = "invalid labels; use a non-empty array of non-empty, trimmed names"
	msgDueDate          = "invalid dueDate; use a real calendar date in YYYY-MM-DD format, or an empty string to clear"
	msgComponents       = "invalid components; use an array of non-empty, unique names, or [] to clear"
	msgFixVersions      = "invalid fixVersions; use an array of non-empty, unique names, or [] to clear"
	msgStoryPoints      = "invalid storyPoints; use a finite decimal number, or null to clear, together with storyPointsField"
	msgStoryPointsField = "invalid storyPointsField; use a customfield_NNN identifier"
)

func TestMCPUpdateParity(t *testing.T) {
	for _, tc := range []struct {
		name, fields, payload string
		message               string
		invalid               bool
		meta                  string
		status                int
	}{
		{name: "description", fields: `{"description":"Context"}`, payload: `{"description":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Context"}]}]}}`},
		{name: "description_clear", fields: `{"description":""}`, payload: `{"description":null}`},
		{name: "labels", fields: `{"labels":["one","two"]}`, payload: `{"labels":["one","two"]}`},
		{name: "labels_empty", fields: `{"labels":[]}`, invalid: true, message: msgLabels},
		{name: "labels_blank", fields: `{"labels":[""]}`, invalid: true, message: msgLabels},
		{name: "labels_untrimmed", fields: `{"labels":[" one"]}`, invalid: true, message: msgLabels},
		{name: "date", fields: `{"dueDate":"2028-02-29"}`, payload: `{"duedate":"2028-02-29"}`},
		{name: "date_clear", fields: `{"dueDate":""}`, payload: `{"duedate":null}`},
		{name: "date_invalid", fields: `{"dueDate":"2027-02-29"}`, invalid: true, message: msgDueDate},
		{name: "components", fields: `{"components":["API","Web"]}`, payload: `{"components":[{"name":"API"},{"name":"Web"}]}`},
		{name: "components_clear", fields: `{"components":[]}`, payload: `{"components":[]}`},
		{name: "components_duplicate", fields: `{"components":["API","api"]}`, invalid: true, message: msgComponents},
		{name: "components_empty", fields: `{"components":[""]}`, invalid: true, message: msgComponents},
		{name: "versions", fields: `{"fixVersions":["v1"]}`, payload: `{"fixVersions":[{"name":"v1"}]}`},
		{name: "versions_clear", fields: `{"fixVersions":[]}`, payload: `{"fixVersions":[]}`},
		{name: "versions_duplicate", fields: `{"fixVersions":["v1","V1"]}`, invalid: true, message: msgFixVersions},
		{name: "points", fields: `{"storyPoints":9007199254740993.125,"storyPointsField":"customfield_10016"}`, payload: `{"customfield_10016":9007199254740993.125}`},
		{name: "points_clear", fields: `{"storyPoints":null,"storyPointsField":"customfield_10016"}`, payload: `{"customfield_10016":null}`},
		{name: "points_invalid", fields: `{"storyPoints":"NaN","storyPointsField":"customfield_10016"}`, invalid: true, message: msgStoryPoints},
		{name: "points_exponent", fields: `{"storyPoints":1e3,"storyPointsField":"customfield_10016"}`, invalid: true, message: msgStoryPoints},
		{name: "points_unpaired", fields: `{"storyPoints":5}`, invalid: true, message: msgStoryPoints},
		{name: "field_unpaired", fields: `{"storyPointsField":"customfield_10016"}`, invalid: true, message: msgStoryPoints},
		{name: "field_invalid", fields: `{"storyPoints":5,"storyPointsField":"points"}`, invalid: true, message: msgStoryPointsField},
		{name: "metadata_rejection", fields: `{"components":["API"]}`, invalid: true, meta: `{"fields":{}}`},
		{name: "metadata_allowed_values", fields: `{"components":["API"]}`, invalid: true, meta: `{"fields":{"components":{"allowedValues":[{"name":"Web"}]}}}`},
		{name: "metadata_points_type", fields: `{"storyPoints":5,"storyPointsField":"customfield_10016"}`, invalid: true, meta: `{"fields":{"customfield_10016":{"schema":{"type":"string"},"operations":["set"]}}}`},
		{name: "metadata_points_set", fields: `{"storyPoints":5,"storyPointsField":"customfield_10016"}`, invalid: true, meta: `{"fields":{"customfield_10016":{"schema":{"type":"number"},"operations":["add"]}}}`},
		{name: "metadata_failure", fields: `{"dueDate":""}`, status: 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mutations, gets atomic.Int32
			var body string
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					gets.Add(1)
					if tc.status != 0 {
						w.WriteHeader(tc.status)
						return
					}
					meta := tc.meta
					if meta == "" {
						meta = `{"fields":{"duedate":{},"components":{},"fixVersions":{},"customfield_10016":{"schema":{"type":"number"},"operations":["set"]}}}`
					}
					_, _ = fmt.Fprint(w, meta)
					return
				}
				mutations.Add(1)
				raw, _ := io.ReadAll(r.Body)
				body = strings.TrimSpace(string(raw))
				w.WriteHeader(http.StatusNoContent)
			}))
			defer api.Close()
			ctx := context.Background()
			st, ct := mcp.NewInMemoryTransports()
			ss, err := newMCPServer(jira.NewClient(api.URL, "user", "test-token"), "example.invalid", true).Connect(ctx, st, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = ss.Close() }()
			cs, err := mcp.NewClient(&mcp.Implementation{Name: "parity-test", Version: "1"}, nil).Connect(ctx, ct, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = cs.Close() }()
			res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "gojira_update", Arguments: json.RawMessage(`{"key":"A-1","fields":` + tc.fields + `}`)})
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(res.StructuredContent)
			var result map[string]any
			if err := json.Unmarshal(raw, &result); err != nil {
				t.Fatal(err)
			}
			if tc.invalid || tc.status != 0 {
				wantCode, wantMessage := "validation_error", tc.message
				if tc.meta != "" {
					wantMessage = "requested field update is not allowed"
				}
				if tc.status != 0 {
					wantCode, wantMessage = "permission_denied", "Jira operation failed"
				}
				if !res.IsError || result["mutationState"] != "not_applied" || result["code"] != wantCode || result["message"] != wantMessage || result["hint"] == "" || mutations.Load() != 0 {
					t.Fatalf("result=%s mutations=%d", raw, mutations.Load())
				}
				return
			}
			if res.IsError || result["mutationState"] != "applied" || result["code"] != nil || result["message"] != nil || mutations.Load() != 1 {
				t.Fatalf("result=%s mutations=%d", raw, mutations.Load())
			}
			var got, want any
			decoder := json.NewDecoder(strings.NewReader(body))
			decoder.UseNumber()
			if err := decoder.Decode(&got); err != nil {
				t.Fatal(err)
			}
			decoder = json.NewDecoder(strings.NewReader(`{"fields":` + tc.payload + `}`))
			decoder.UseNumber()
			if err := decoder.Decode(&want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("payload=%s, want fields %s", body, tc.payload)
			}
			wantGets := int32(0)
			if strings.Contains(tc.fields, "dueDate") || strings.Contains(tc.fields, "components") || strings.Contains(tc.fields, "fixVersions") || strings.Contains(tc.fields, "storyPoints") {
				wantGets = 1
			}
			if gets.Load() != wantGets {
				t.Fatalf("metadata gets=%d, want %d", gets.Load(), wantGets)
			}
		})
	}
}
