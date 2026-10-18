package mcpserver

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yepizrene-devoost/gojira/internal/jira"
	"github.com/yepizrene-devoost/gojira/internal/projectctx"
)

type tool struct {
	name        string
	description string
	schema      map[string]any
	annotations *mcp.ToolAnnotations
	validate    func(*input, map[string]json.RawMessage) string
	handle      func(input) (any, error)
}

type validationError struct{ message string }

func (e *validationError) Error() string { return e.message }

// tools is the single registration point: metadata, validation and handling live
// together in each entry. Appending an entry is sufficient to expose a tool.
func (a adapter) tools() []tool {
	destructive := false
	annotations := &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: &destructive, IdempotentHint: true}
	return []tool{
		{
			name:        "gojira_context",
			description: "No arguments. Resolve project from the server process's working directory and environment and read authenticated identity. If needsProject=true, ask the human.",
			schema:      objectSchema(map[string]any{}), annotations: annotations,
			handle: func(input) (any, error) {
				value, err := a.Context(a.Client)
				var invalid *projectctx.ProjectValidationError
				if errors.As(err, &invalid) {
					return nil, &validationError{invalid.Error()}
				}
				return value, err
			},
		},
		{
			name:        "gojira_projects",
			description: "No arguments. Read all visible projects with pagination.",
			schema:      objectSchema(map[string]any{}), annotations: annotations,
			handle: func(input) (any, error) {
				data, err := a.Client.GetProjects()
				return output{SchemaVersion: a.SchemaVersion, Data: data, Complete: true, Completeness: "complete"}, err
			},
		},
		{
			name:        "gojira_boards",
			description: "No arguments required. Single page, completeness unknown.",
			schema:      objectSchema(map[string]any{}), annotations: annotations,
			handle: func(input) (any, error) {
				data, err := a.Client.GetBoards()
				return a.singlePage(data), err
			},
		},
		{
			name:        "gojira_sprints",
			description: "Requires positive boardId. Single page, completeness unknown.",
			schema:      objectSchema(map[string]any{"boardId": map[string]any{"type": "integer", "minimum": 1}}, "boardId"), annotations: annotations,
			validate: func(in *input, _ map[string]json.RawMessage) string {
				if in.BoardID <= 0 {
					return "boardId must be positive"
				}
				return ""
			},
			handle: func(in input) (any, error) {
				data, err := a.Client.GetSprintsByState(in.BoardID, "active,future,closed")
				return a.singlePage(data), err
			},
		},
		{
			name:        "gojira_get",
			description: "Requires key. Embedded history may be partial.",
			schema:      objectSchema(map[string]any{"key": map[string]any{"type": "string", "minLength": 1}}, "key"), annotations: annotations,
			validate: func(in *input, _ map[string]json.RawMessage) string {
				if strings.TrimSpace(in.Key) == "" {
					return "key is required"
				}
				return ""
			},
			handle: func(in input) (any, error) {
				issue, err := a.Client.GetIssueFull(in.Key)
				if err != nil {
					return nil, err
				}
				out := a.singlePage(jira.IssueToTicketJSON(*issue, a.Domain))
				out.Warning = "Embedded comments/worklogs and sprint evidence may be incomplete; this is not a complete history."
				return out, nil
			},
		},
		{
			name:        "gojira_search",
			description: "Requires jql; optional limit (default 50, maximum 1000). Observe complete.",
			schema:      objectSchema(map[string]any{"jql": map[string]any{"type": "string", "minLength": 1}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 1000, "default": 50}}, "jql"), annotations: annotations,
			validate: func(in *input, fields map[string]json.RawMessage) string {
				if strings.TrimSpace(in.JQL) == "" {
					return "jql is required"
				}
				if _, provided := fields["limit"]; !provided {
					in.Limit = 50
				}
				if in.Limit < 1 || in.Limit > 1000 {
					return "limit must be between 1 and 1000"
				}
				return ""
			},
			handle: func(in input) (any, error) {
				found, err := a.Client.SearchIssues(in.JQL, in.Limit)
				if err != nil {
					return nil, err
				}
				out := output{SchemaVersion: a.SchemaVersion, Data: jira.IssuesToTicketJSON(found.Issues, a.Domain), Complete: !found.Truncated, Completeness: "complete"}
				if found.Truncated {
					out.Completeness = "partial"
					out.Warning = "results_truncated: refine JQL or increase limit; results are partial."
				}
				return out, nil
			},
		},
	}
}

func (a adapter) singlePage(data any) output {
	return output{SchemaVersion: a.SchemaVersion, Data: data, Completeness: "unknown", Warning: "Single page only; additional results may exist."}
}
