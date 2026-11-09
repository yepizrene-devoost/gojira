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

// AddWorklog records time spent. Jira rejects a worklog whose comment is not a
// valid ADF document, so a blank comment is omitted and a supplied one is sent
// as ADF, matching AddComment's body convention.
func (c *Client) AddWorklog(issueKey, timeSpent, comment string) error {
	body := map[string]interface{}{"timeSpent": timeSpent}
	if comment != "" {
		body["comment"] = TextToADF(comment)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	_, err = c.mutate(http.MethodPost, issuePath(pathIssueWorklog, issueKey), payload)
	return err
}

// ─── Write (Phase 8) ──────────────────────────────────────────────────

// AddComment adds a comment (ADF body) to an issue.
func (c *Client) AddComment(issueKey string, body ADFDoc) error {
	payload, _ := json.Marshal(map[string]interface{}{"body": body})
	_, err := c.mutate(http.MethodPost, issuePath(pathIssueComment, issueKey), payload)
	return err
}
