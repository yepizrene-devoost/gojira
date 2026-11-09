package jira

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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

// GetAllWorklogs reads the complete worklog collection of an issue. The
// embedded fields.worklog page in a ticket read is smaller than Jira's full
// list, so management flows read the dedicated endpoint instead.
func (c *Client) GetAllWorklogs(issueKey string) ([]Worklog, error) {
	b, err := c.get(fmt.Sprintf(pathIssueWorklogAll, url.PathEscape(issueKey)))
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

// ClearWorklogComment removes the comment of one worklog. A JSON null is sent
// because an empty string is not a valid worklog body for Jira and would be
// read as "leave the comment untouched".
func (c *Client) ClearWorklogComment(issueKey, worklogID string) error {
	return c.putWorklog(issueKey, worklogID, map[string]interface{}{"comment": nil})
}

// UpdateWorklogAndClearComment changes the duration and removes the comment in a
// single request, so a failure cannot leave the worklog half-edited.
func (c *Client) UpdateWorklogAndClearComment(issueKey, worklogID, timeSpent string) error {
	if timeSpent == "" {
		return errors.New("update requires timeSpent")
	}
	return c.putWorklog(issueKey, worklogID, map[string]interface{}{
		"timeSpent": timeSpent,
		"comment":   nil,
	})
}

// putWorklog applies one worklog change in a single request.
func (c *Client) putWorklog(issueKey, worklogID string, body map[string]interface{}) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	_, err = c.mutate(http.MethodPut, worklogEntryPath(issueKey, worklogID), payload)
	return err
}

// UpdateWorklog changes one existing worklog. Only the fields the caller
// supplies are sent, so an omitted field keeps its Jira value; a comment is
// sent as ADF because Jira rejects any other body shape.
func (c *Client) UpdateWorklog(issueKey, worklogID, timeSpent, comment string) error {
	if timeSpent == "" && comment == "" {
		return errors.New("update requires timeSpent or comment")
	}
	body := map[string]interface{}{}
	if timeSpent != "" {
		body["timeSpent"] = timeSpent
	}
	if comment != "" {
		body["comment"] = TextToADF(comment)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	_, err = c.mutate(http.MethodPut, worklogEntryPath(issueKey, worklogID), payload)
	return err
}

// DeleteWorklog removes one worklog entry. The request carries no body.
func (c *Client) DeleteWorklog(issueKey, worklogID string) error {
	_, err := c.mutate(http.MethodDelete, worklogEntryPath(issueKey, worklogID), nil)
	return err
}

// VerifyWorklogUpdate re-reads one worklog and reports a duration Jira did not
// keep, which is how an edit silently ignored for an app-owned entry surfaces.
// It never mutates anything, and a blank duration needs no read at all.
func (c *Client) VerifyWorklogUpdate(issueKey, worklogID, wantTimeSpent string) error {
	if wantTimeSpent == "" {
		return nil
	}
	worklogs, err := c.GetWorklog(issueKey)
	if err != nil {
		return err
	}
	for _, worklog := range worklogs {
		if worklog.ID != worklogID {
			continue
		}
		if worklog.TimeSpent == wantTimeSpent {
			return nil
		}
		return fmt.Errorf("jira reports %s for worklog %s (requested %s)", worklog.TimeSpent, worklogID, wantTimeSpent)
	}
	return fmt.Errorf("worklog %s is not reported by Jira after the update", worklogID)
}

// AddComment adds a comment (ADF body) to an issue.
func (c *Client) AddComment(issueKey string, body ADFDoc) error {
	payload, _ := json.Marshal(map[string]interface{}{"body": body})
	_, err := c.mutate(http.MethodPost, issuePath(pathIssueComment, issueKey), payload)
	return err
}
