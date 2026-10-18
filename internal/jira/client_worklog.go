package jira

import (
	"encoding/json"
	"net/http"
)

func (c *Client) GetWorklog(issueKey string) ([]Worklog, error) {
	b, err := c.get(issuePath(pathIssueWorklog, issueKey))
	if err != nil {
		return nil, err
	}
	var r struct {
		Worklogs []Worklog `json:"worklogs"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	return r.Worklogs, nil
}

func (c *Client) AddWorklog(issueKey, timeSpent, comment string) error {
	payload, _ := json.Marshal(map[string]string{
		"timeSpent": timeSpent,
		"comment":   comment,
	})
	_, err := c.mutate(http.MethodPost, issuePath(pathIssueWorklog, issueKey), payload)
	return err
}

// ─── Write (Phase 8) ──────────────────────────────────────────────────

// AddComment adds a comment (ADF body) to an issue.
func (c *Client) AddComment(issueKey string, body ADFDoc) error {
	payload, _ := json.Marshal(map[string]interface{}{"body": body})
	_, err := c.mutate(http.MethodPost, issuePath(pathIssueComment, issueKey), payload)
	return err
}
