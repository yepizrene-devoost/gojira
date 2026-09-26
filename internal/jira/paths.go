package jira

import (
	"fmt"
	"net/url"
)

// API path constants — all endpoints live here.
// Platform REST API v3 (Cloud)  — https://developer.atlassian.com/cloud/jira/platform/rest/v3/
// Agile API 1.0                — https://developer.atlassian.com/cloud/jira/software/rest/v1/
//
// If Atlassian deprecates an endpoint or you switch to Server/DC (v2),
// change these paths here — no other file should contain raw paths.

const (
	// ─── Platform API v3 ──────────────────────────────────────────
	pathServerInfo       = "/rest/api/3/serverInfo"
	pathProjects         = "/rest/api/3/project?maxResults=100"
	pathIssue            = "/rest/api/3/issue/%s"
	pathIssueCreate      = "/rest/api/3/issue"
	pathIssueTransitions = "/rest/api/3/issue/%s/transitions"
	pathIssueWorklog     = "/rest/api/3/issue/%s/worklog"
	pathIssueComment     = "/rest/api/3/issue/%s/comment"
	pathIssueAssign      = "/rest/api/3/issue/%s/assignee"
	pathSearchJQL        = "/rest/api/3/search/jql"
	pathUserByAccountID  = "/rest/api/3/user?accountId=%s"
	pathCreateMeta       = "/rest/api/3/issue/createmeta?projectKeys=%s"
	// Email lookup: /rest/api/3/user/username was removed by Atlassian and
	// users/search no longer exposes emails. The v2 picker still matches email
	// queries server-side; pathUserByAccountID verifies the exact match.
	pathUserPicker = "/rest/api/2/user/picker?query=%s"

	// ─── Agile API 1.0 ────────────────────────────────────────────
	pathBoardList    = "/rest/agile/1.0/board?maxResults=50"
	pathBoardConfig  = "/rest/agile/1.0/board/%d/configuration"
	pathSprintList   = "/rest/agile/1.0/board/%d/sprint"
	pathBoardIssues  = "/rest/agile/1.0/board/%d/issue"
	pathSprintIssues = "/rest/agile/1.0/board/%d/sprint/%d/issue"
	pathSprintAdd    = "/rest/agile/1.0/sprint/%d/issue"
)

// Issue fields for different contexts.
const (
	fieldsBasic  = "summary,status,priority,assignee,issuetype"
	fieldsFull   = "summary,status,priority,assignee,reporter,issuetype,project,labels,components,created,updated,description,comment,worklog"
	fieldsSearch = "summary,status,priority,assignee,issuetype,project,labels,created,updated"
	fieldsExport = "summary,status,priority,assignee,issuetype"
)

func issuePath(path, key string) string {
	return fmt.Sprintf(path, url.PathEscape(key))
}

func pathIssueWithFields(key, fields string) string {
	return fmt.Sprintf("%s?fields=%s", issuePath(pathIssue, key), fields)
}
