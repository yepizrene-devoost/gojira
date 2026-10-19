package jira

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

func (c *Client) GetBoards() ([]Board, error) {
	b, err := c.get(pathBoardList)
	if err != nil {
		return nil, err
	}
	var r struct{ Values []Board }
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	return r.Values, nil
}

func (c *Client) GetBoardConfig(id int) (BoardConfig, error) {
	b, err := c.get(fmt.Sprintf(pathBoardConfig, id))
	if err != nil {
		return BoardConfig{}, err
	}
	var cfg BoardConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return BoardConfig{}, err
	}
	return cfg, nil
}

// GetSprintsByState lists the requested states without the legacy active fallback.
func (c *Client) GetSprintsByState(boardID int, states string) ([]Sprint, error) {
	b, err := c.get(fmt.Sprintf(pathSprintList, boardID) + "?state=" + url.QueryEscape(states) + "&maxResults=50")
	if err != nil {
		return nil, err
	}
	var r struct{ Values []Sprint }
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	if r.Values == nil {
		r.Values = []Sprint{}
	}
	return r.Values, nil
}

func (c *Client) GetSprints(boardID int) ([]Sprint, error) {
	b, err := c.get(fmt.Sprintf(pathSprintList+"?state=active&maxResults=50", boardID))
	if err != nil {
		return nil, err
	}
	var r struct{ Values []Sprint }
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	if len(r.Values) > 0 {
		return r.Values, nil
	}
	b, err = c.get(fmt.Sprintf(pathSprintList+"?maxResults=50", boardID))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	return r.Values, nil
}

// GetBoardIssues returns all issues on a board (or in one sprint), paging
// through the Agile endpoint until every result is collected. A single
// request is capped by the API at maxResults, which silently truncates
// large sprints.
func (c *Client) GetBoardIssues(boardID, sprintID int) ([]Issue, error) {
	path := fmt.Sprintf(pathBoardIssues, boardID)
	if sprintID > 0 {
		path = fmt.Sprintf(pathSprintIssues, boardID, sprintID)
	}
	return c.getAgileIssues(path)
}

// GetBoardBacklog returns ranked backlog issues from Jira's deprecated endpoint.
func (c *Client) GetBoardBacklog(boardID int) ([]Issue, error) {
	return c.getAgileIssues(fmt.Sprintf(pathBoardBacklog, boardID))
}

func (c *Client) getAgileIssues(endpoint string) ([]Issue, error) {
	const page = 100
	const maxIssues = 2000 // safety cap against servers that ignore startAt
	var all []Issue
	for startAt := 0; startAt < maxIssues; {
		path := fmt.Sprintf("%s?maxResults=%d&startAt=%d&fields=%s", endpoint, page, startAt, fieldsBasic)
		b, err := c.get(path)
		if err != nil {
			return nil, err
		}
		var r struct {
			Issues []Issue `json:"issues"`
			Total  int     `json:"total"`
			IsLast bool    `json:"isLast"`
		}
		if err := json.Unmarshal(b, &r); err != nil {
			return nil, err
		}
		if len(r.Issues) == 0 {
			break
		}
		all = append(all, r.Issues...)
		startAt += len(r.Issues)
		if r.IsLast || (r.Total > 0 && len(all) >= r.Total) {
			break
		}
	}
	return all, nil
}

// AddIssuesToSprint moves issues into a sprint (e.g. backlog → active sprint).
// POST /rest/agile/1.0/sprint/{id}/issue with {"issues": ["KEY", ...]}.
func (c *Client) AddIssuesToSprint(sprintID int, keys []string) error {
	payload, _ := json.Marshal(map[string][]string{"issues": keys})
	_, err := c.mutate(http.MethodPost, fmt.Sprintf(pathSprintAdd, sprintID), payload)
	return err
}

// MoveIssueToBacklog asks Jira to remove an issue from future and active sprints.
// A successful response does not independently verify its resulting membership.
func (c *Client) MoveIssueToBacklog(issueKey string) error {
	payload, _ := json.Marshal(map[string][]string{"issues": {issueKey}})
	_, err := c.mutate(http.MethodPost, pathBacklogIssue, payload)
	return err
}
