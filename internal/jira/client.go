package jira

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL, auth string
	http          *http.Client
}

func NewClient(baseURL, email, token string) *Client {
	return &Client{
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

func (c *Client) get(path string) ([]byte, error) {
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

func (c *Client) post(path string, payload []byte) ([]byte, error) {
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

func (c *Client) TestConnection() (string, error) {
	b, err := c.get(pathServerInfo)
	if err != nil {
		return "", err
	}
	var info struct{ BaseURL, Version string }
	json.Unmarshal(b, &info)
	return fmt.Sprintf("%s (Jira %s)", info.BaseURL, info.Version), nil
}

func (c *Client) GetBoards() ([]Board, error) {
	b, err := c.get(pathBoardList)
	if err != nil {
		return nil, err
	}
	var r struct{ Values []Board }
	json.Unmarshal(b, &r)
	return r.Values, nil
}

func (c *Client) GetBoardConfig(id int) (BoardConfig, error) {
	b, err := c.get(fmt.Sprintf(pathBoardConfig, id))
	if err != nil {
		return BoardConfig{}, err
	}
	var cfg BoardConfig
	json.Unmarshal(b, &cfg)
	return cfg, nil
}

func (c *Client) GetSprints(boardID int) ([]Sprint, error) {
	b, err := c.get(fmt.Sprintf(pathSprintList+"?state=active&maxResults=50", boardID))
	if err != nil {
		return nil, err
	}
	var r struct{ Values []Sprint }
	json.Unmarshal(b, &r)
	if len(r.Values) > 0 {
		return r.Values, nil
	}
	b, err = c.get(fmt.Sprintf(pathSprintList+"?maxResults=50", boardID))
	if err != nil {
		return nil, err
	}
	json.Unmarshal(b, &r)
	return r.Values, nil
}

func (c *Client) GetBoardIssues(boardID, sprintID int) ([]Issue, error) {
	var path string
	if sprintID > 0 {
		path = fmt.Sprintf(pathSprintIssues+"?maxResults=100&fields=%s", boardID, sprintID, fieldsBasic)
	} else {
		path = fmt.Sprintf(pathBoardIssues+"?maxResults=100&fields=%s", boardID, fieldsBasic)
	}
	b, err := c.get(path)
	if err != nil {
		return nil, err
	}
	var r struct{ Issues []Issue }
	json.Unmarshal(b, &r)
	return r.Issues, nil
}

func (c *Client) GetIssue(issueKey string) (*Issue, error) {
	b, err := c.get(pathIssueWithFields(issueKey, fieldsBasic+",created,updated,description"))
	if err != nil {
		return nil, err
	}
	var iss Issue
	json.Unmarshal(b, &iss)
	return &iss, nil
}

func (c *Client) GetIssueFull(issueKey string) (*Issue, error) {
	b, err := c.get(pathIssueWithFields(issueKey, fieldsFull))
	if err != nil {
		return nil, err
	}
	var iss Issue
	json.Unmarshal(b, &iss)
	return &iss, nil
}

func (c *Client) GetProjects() ([]Project, error) {
	b, err := c.get(pathProjects)
	if err != nil {
		return nil, err
	}
	var projects []Project
	json.Unmarshal(b, &projects)
	return projects, nil
}

func (c *Client) SearchJQL(jql string, maxResults int) ([]Issue, int, error) {
	if maxResults <= 0 {
		maxResults = 50
	}
	encoded := strings.ReplaceAll(jql, "'", "\\'")
	path := fmt.Sprintf("%s?jql=%s&maxResults=%d&fields=%s", pathSearchJQL, encoded, maxResults, fieldsSearch)
	b, err := c.get(path)
	if err != nil {
		return nil, 0, err
	}
	var r struct {
		Issues     []Issue `json:"issues"`
		Total      int     `json:"total"`
		MaxResults int     `json:"maxResults"`
	}
	json.Unmarshal(b, &r)
	return r.Issues, r.Total, nil
}

func (c *Client) GetTransitions(issueKey string) ([]Transition, error) {
	b, err := c.get(fmt.Sprintf(pathIssueTransitions, issueKey))
	if err != nil {
		return nil, err
	}
	var r struct {
		Transitions []Transition `json:"transitions"`
	}
	json.Unmarshal(b, &r)
	return r.Transitions, nil
}

func (c *Client) TransitionIssue(issueKey, transitionID string) error {
	payload, _ := json.Marshal(map[string]map[string]string{
		"transition": {"id": transitionID},
	})
	_, err := c.post(fmt.Sprintf(pathIssueTransitions, issueKey), payload)
	return err
}

func (c *Client) GetWorklog(issueKey string) ([]Worklog, error) {
	b, err := c.get(fmt.Sprintf(pathIssueWorklog, issueKey))
	if err != nil {
		return nil, err
	}
	var r struct {
		Worklogs []Worklog `json:"worklogs"`
	}
	json.Unmarshal(b, &r)
	return r.Worklogs, nil
}

func (c *Client) AddWorklog(issueKey, timeSpent, comment string) error {
	payload, _ := json.Marshal(map[string]string{
		"timeSpent": timeSpent,
		"comment":   comment,
	})
	_, err := c.post(fmt.Sprintf(pathIssueWorklog, issueKey), payload)
	return err
}

// ─── Write (Phase 8) ──────────────────────────────────────────────────

// CreateIssue creates a new issue and returns its key.
func (c *Client) CreateIssue(projectKey, issueType, summary string, description *ADFDoc) (string, error) {
	fields := map[string]interface{}{
		"project":  map[string]string{"key": projectKey},
		"issuetype": map[string]string{"name": issueType},
		"summary":  summary,
	}
	if description != nil && len(description.Content) > 0 {
		fields["description"] = description
	}
	payload, _ := json.Marshal(map[string]interface{}{"fields": fields})
	b, err := c.post(pathIssueCreate, payload)
	if err != nil {
		return "", err
	}
	var resp struct{ Key string }
	json.Unmarshal(b, &resp)
	return resp.Key, nil
}

// AddIssuesToSprint moves issues into a sprint (e.g. backlog → active sprint).
// POST /rest/agile/1.0/sprint/{id}/issue with {"issues": ["KEY", ...]}.
func (c *Client) AddIssuesToSprint(sprintID int, keys []string) error {
	payload, _ := json.Marshal(map[string][]string{"issues": keys})
	_, err := c.post(fmt.Sprintf(pathSprintAdd, sprintID), payload)
	return err
}

// GetIssueTypes returns the creatable issue type names for a project
// via the Platform v3 createmeta endpoint.
func (c *Client) GetIssueTypes(projectKey string) ([]string, error) {
	b, err := c.get(fmt.Sprintf(pathCreateMeta, projectKey))
	if err != nil {
		return nil, err
	}
	var r struct {
		Projects []struct {
			IssueTypes []struct {
				Name string `json:"name"`
			} `json:"issuetypes"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	var names []string
	for _, p := range r.Projects {
		for _, t := range p.IssueTypes {
			if t.Name != "" {
				names = append(names, t.Name)
			}
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no creatable issue types for project %s", projectKey)
	}
	return names, nil
}

// AddComment adds a comment (ADF body) to an issue.
func (c *Client) AddComment(issueKey string, body ADFDoc) error {
	payload, _ := json.Marshal(map[string]interface{}{"body": body})
	_, err := c.post(fmt.Sprintf(pathIssueComment, issueKey), payload)
	return err
}

// AssignIssue assigns an issue by account ID.
func (c *Client) AssignIssue(issueKey, accountID string) error {
	payload, _ := json.Marshal(map[string]string{"accountId": accountID})
	req, err := http.NewRequest("PUT",
		c.baseURL+fmt.Sprintf(pathIssueAssign, issueKey),
		strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.auth, "")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API %d: %s", resp.StatusCode, string(body[:minInt(len(body), 200)]))
	}
	return nil
}

// UpdateIssue updates one or more fields on an issue.
func (c *Client) UpdateIssue(issueKey string, fields map[string]interface{}) error {
	payload, _ := json.Marshal(map[string]interface{}{"fields": fields})
	req, err := http.NewRequest("PUT",
		c.baseURL+fmt.Sprintf(pathIssue, issueKey),
		strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.auth, "")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API %d: %s", resp.StatusCode, string(body[:minInt(len(body), 200)]))
	}
	return nil
}

// ResolveAccountID looks up a Jira user by email and returns their accountId.
func (c *Client) ResolveAccountID(email string) (accountID, displayName string, err error) {
	path := "/rest/api/3/user/username?username=" + email
	b, err := c.get(path)
	if err != nil {
		return "", "", fmt.Errorf("user lookup failed: %w", err)
	}
	var user struct {
		AccountID   string `json:"accountId"`
		DisplayName string `json:"displayName"`
	}
	json.Unmarshal(b, &user)
	if user.AccountID == "" {
		return "", "", fmt.Errorf("no user found for %s", email)
	}
	return user.AccountID, user.DisplayName, nil
}
