package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type JiraClient struct {
	baseURL, auth string
	http          *http.Client
}

func NewJiraClient(baseURL, email, token string) *JiraClient {
	return &JiraClient{
		baseURL: baseURL,
		auth:    email + ":" + token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (c *JiraClient) get(path string) ([]byte, error) {
	req, _ := http.NewRequest("GET", c.baseURL+path, nil)
	req.SetBasicAuth(c.auth, "")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API %d: %s", resp.StatusCode, string(body[:minInt(len(body), 200)]))
	}
	return body, nil
}

func (c *JiraClient) post(path string, payload []byte) ([]byte, error) {
	req, _ := http.NewRequest("POST", c.baseURL+path, strings.NewReader(string(payload)))
	req.SetBasicAuth(c.auth, "")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API %d: %s", resp.StatusCode, string(body[:minInt(len(body), 200)]))
	}
	return body, nil
}

// ─── Read ──────────────────────────────────────────────────────────────
func (c *JiraClient) TestConnection() (string, error) {
	b, err := c.get("/rest/api/3/serverInfo")
	if err != nil {
		return "", err
	}
	var info struct{ BaseURL, Version string }
	json.Unmarshal(b, &info)
	return fmt.Sprintf("%s (Jira %s)", info.BaseURL, info.Version), nil
}

func (c *JiraClient) GetBoards() ([]Board, error) {
	b, err := c.get("/rest/agile/1.0/board?maxResults=50")
	if err != nil {
		return nil, err
	}
	var r struct{ Values []Board }
	json.Unmarshal(b, &r)
	return r.Values, nil
}

func (c *JiraClient) GetBoardConfig(id int) (BoardConfig, error) {
	b, err := c.get(fmt.Sprintf("/rest/agile/1.0/board/%d/configuration", id))
	if err != nil {
		return BoardConfig{}, err
	}
	var cfg BoardConfig
	json.Unmarshal(b, &cfg)
	return cfg, nil
}

func (c *JiraClient) GetSprints(boardID int) ([]Sprint, error) {
	b, err := c.get(fmt.Sprintf("/rest/agile/1.0/board/%d/sprint?state=active&maxResults=50", boardID))
	if err != nil {
		return nil, err
	}
	var r struct{ Values []Sprint }
	json.Unmarshal(b, &r)
	if len(r.Values) > 0 {
		return r.Values, nil
	}
	b, err = c.get(fmt.Sprintf("/rest/agile/1.0/board/%d/sprint?maxResults=50", boardID))
	if err != nil {
		return nil, err
	}
	json.Unmarshal(b, &r)
	return r.Values, nil
}

func (c *JiraClient) GetBoardIssues(boardID, sprintID int) ([]Issue, error) {
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/issue?maxResults=100&fields=summary,status,priority,assignee,issuetype", boardID)
	if sprintID > 0 {
		path = fmt.Sprintf("/rest/agile/1.0/board/%d/sprint/%d/issue?maxResults=100&fields=summary,status,priority,assignee,issuetype", boardID, sprintID)
	}
	b, err := c.get(path)
	if err != nil {
		return nil, err
	}
	var r struct{ Issues []Issue }
	json.Unmarshal(b, &r)
	return r.Issues, nil
}

func (c *JiraClient) GetIssue(issueKey string) (*Issue, error) {
	b, err := c.get(fmt.Sprintf("/rest/api/3/issue/%s?fields=summary,status,priority,assignee,issuetype,created,updated,description", issueKey))
	if err != nil {
		return nil, err
	}
	var iss Issue
	json.Unmarshal(b, &iss)
	return &iss, nil
}

func (c *JiraClient) GetTransitions(issueKey string) ([]Transition, error) {
	b, err := c.get(fmt.Sprintf("/rest/api/3/issue/%s/transitions", issueKey))
	if err != nil {
		return nil, err
	}
	var r struct {
		Transitions []Transition `json:"transitions"`
	}
	json.Unmarshal(b, &r)
	return r.Transitions, nil
}

func (c *JiraClient) TransitionIssue(issueKey, transitionID string) error {
	payload, _ := json.Marshal(map[string]map[string]string{
		"transition": {"id": transitionID},
	})
	_, err := c.post(fmt.Sprintf("/rest/api/3/issue/%s/transitions", issueKey), payload)
	return err
}

func (c *JiraClient) GetWorklog(issueKey string) ([]Worklog, error) {
	b, err := c.get(fmt.Sprintf("/rest/api/3/issue/%s/worklog", issueKey))
	if err != nil {
		return nil, err
	}
	var r struct {
		Worklogs []Worklog `json:"worklogs"`
	}
	json.Unmarshal(b, &r)
	return r.Worklogs, nil
}

func (c *JiraClient) AddWorklog(issueKey, timeSpent, comment string) error {
	payload, _ := json.Marshal(map[string]string{
		"timeSpent": timeSpent,
		"comment":   comment,
	})
	_, err := c.post(fmt.Sprintf("/rest/api/3/issue/%s/worklog", issueKey), payload)
	return err
}
