package mcpserver

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yepizrene-devoost/gojira/internal/reporting"
)

// ReportingReader is optional: existing consumers of Reader need not implement
// these context-aware, paginated reads. The concrete Jira client implements both.
type ReportingReader interface {
	reporting.UserReader
	reporting.TimeReader
}

type reportingInput struct {
	Query     string `json:"query"`
	AccountID string `json:"accountId"`
	Project   string `json:"project"`
	From      string `json:"from"`
	To        string `json:"to"`
	Timezone  string `json:"timezone"`
}

func (a adapter) registerReporting(server *mcp.Server) {
	destructive := false
	annotations := &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: &destructive, IdempotentHint: true}
	for _, name := range []string{"gojira_users_search", "gojira_report_time"} {
		properties := map[string]any{}
		required := []string{"query"}
		description := "Find visible active user candidates; never silently select an account. Permission-limited empty results do not prove no user exists."
		if name == "gojira_report_time" {
			required = []string{"accountId", "project", "from", "to", "timezone"}
			description = "Sum visible worklogs by explicit account, project, inclusive dates and IANA timezone. Inspect complete and warnings; partial totals are not all time. Bounded to 1000 issues and 2000 page requests."
		}
		for _, key := range required {
			properties[key] = map[string]any{"type": "string"}
		}
		schema := objectSchema(properties, required...)
		server.AddTool(&mcp.Tool{Name: name, Description: description, InputSchema: schema, Annotations: annotations}, a.reportingCall(name, properties))
	}
}

func (a adapter) reportingCall(name string, properties map[string]any) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var fields map[string]json.RawMessage
		raw := req.Params.Arguments
		if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
			return a.validation("arguments must be an object")
		}
		for key, value := range fields {
			if _, ok := properties[key]; !ok {
				return a.validation("unexpected argument")
			}
			if string(value) == "null" {
				return a.validation("arguments must not be null")
			}
		}
		var in reportingInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return a.validation("invalid argument type")
		}
		filters := reporting.TimeFilters{AccountID: in.AccountID, Project: in.Project, From: in.From, To: in.To, Timezone: in.Timezone}
		var err error
		if name == "gojira_users_search" {
			err = reporting.ValidateUserQuery(in.Query)
		} else {
			err = reporting.ValidateTimeFilters(filters)
		}
		if err != nil {
			return a.validation(err.Error())
		}
		if err = ctx.Err(); err != nil {
			return a.failure(err)
		}
		reader, ok := a.Client.(ReportingReader)
		if !ok || reader == nil {
			return a.failure(errors.New("reporting unavailable"))
		}
		var value any
		if name == "gojira_users_search" {
			value, err = reporting.SearchUsers(ctx, reader, in.Query)
		} else {
			value, err = reporting.TimeReport(ctx, reader, filters)
		}
		if err != nil {
			return a.failure(err)
		}
		return result(value), nil
	}
}
