// Package reporting implements bounded, read-only reporting over visible Jira data.
package reporting

import "context"

type User struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Active      bool   `json:"active"`
}
type UserReader interface {
	ReportingUsers(context.Context, string, int) ([]User, error)
}
type UserSearchResult struct {
	SchemaVersion string   `json:"schemaVersion"`
	Query         string   `json:"query"`
	Candidates    []User   `json:"candidates"`
	Completeness  string   `json:"completeness"`
	Warnings      []string `json:"warnings"`
}
type Issue struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}
type IssuePage struct {
	Issues        []Issue `json:"issues"`
	NextPageToken string  `json:"nextPageToken"`
	IsLast        *bool   `json:"isLast"`
}
type Worklog struct {
	ID     string `json:"id"`
	Author struct {
		AccountID string `json:"accountId"`
	} `json:"author"`
	Started string `json:"started"`
	Seconds *int64 `json:"timeSpentSeconds"`
}
type WorklogPage struct {
	StartAt    *int      `json:"startAt"`
	MaxResults *int      `json:"maxResults"`
	Total      *int      `json:"total"`
	Worklogs   []Worklog `json:"worklogs"`
}
type TimeReader interface {
	ReportingIssues(context.Context, string, string) (IssuePage, error)
	ReportingWorklogs(context.Context, string, int) (WorklogPage, error)
}
type TimeFilters struct {
	AccountID string `json:"accountId"`
	Project   string `json:"project"`
	From      string `json:"from"`
	To        string `json:"to"`
	Timezone  string `json:"timezone"`
}
type TimeRow struct {
	Key     string `json:"key"`
	Seconds int64  `json:"seconds"`
}
type TimeReportResult struct {
	SchemaVersion     string      `json:"schemaVersion"`
	Filters           TimeFilters `json:"filters"`
	Rows              []TimeRow   `json:"rows"`
	TotalSeconds      int64       `json:"totalSeconds"`
	Complete          bool        `json:"complete"`
	CompletenessScope string      `json:"completenessScope"`
	IssuesScanned     int         `json:"issuesScanned"`
	WorklogsScanned   int         `json:"worklogsScanned"`
	PageRequests      int         `json:"pageRequests"`
	Warnings          []string    `json:"warnings"`
}

const MaxIssues = 1000
const MaxPageRequests = 2000
