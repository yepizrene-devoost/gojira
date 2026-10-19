package jira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// GetIssueSprint reads only Agile sprint evidence, without retries.
func (c *Client) GetIssueSprint(issueKey string) (*Sprint, error) {
	b, err := c.requestWithAttempts(http.MethodGet, issuePath(pathAgileIssue, issueKey)+"?fields=sprint", nil, 1)
	if err != nil {
		return nil, err
	}
	var issue Issue
	if err := json.Unmarshal(b, &issue); err != nil {
		return nil, err
	}
	return issue.Fields.Sprint, nil
}

func (c *Client) GetIssue(issueKey string) (*Issue, error) {
	b, err := c.get(pathIssueWithFields(issueKey, fieldsBasic+",created,updated,description"))
	if err != nil {
		return nil, err
	}
	var iss Issue
	if err := json.Unmarshal(b, &iss); err != nil {
		return nil, err
	}
	return &iss, nil
}

func (c *Client) GetIssueFull(issueKey string) (*Issue, error) {
	return c.getIssueFull(issueKey, "")
}

// GetIssueFullWithStoryPoints requests the normal full field set plus exactly
// one caller-selected custom field and strictly decodes its number/null value.
func (c *Client) GetIssueFullWithStoryPoints(issueKey, fieldID string) (*Issue, error) {
	if err := ValidateCustomFieldID(fieldID); err != nil {
		return nil, fmt.Errorf("invalid story points field ID: %w", err)
	}
	return c.getIssueFull(issueKey, fieldID)
}

func (c *Client) getIssueFull(issueKey, storyPointsField string) (*Issue, error) {
	fields := fieldsFull
	if storyPointsField != "" {
		fields += "," + storyPointsField
	}
	b, err := c.get(pathIssueWithFields(issueKey, fields))
	if err != nil {
		return nil, err
	}
	var iss Issue
	if err := json.Unmarshal(b, &iss); err != nil {
		return nil, err
	}
	if storyPointsField == "" {
		return &iss, nil
	}
	var envelope struct {
		Fields map[string]json.RawMessage `json:"fields"`
	}
	if err := json.Unmarshal(b, &envelope); err != nil {
		return nil, err
	}
	raw, present := envelope.Fields[storyPointsField]
	if !present {
		return nil, fmt.Errorf("selected story points field %q is missing from Jira response", storyPointsField)
	}
	if string(raw) == "null" {
		iss.StoryPoints = &NullableNumber{}
		return &iss, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("selected story points field %q is not a number or null: %w", storyPointsField, err)
	}
	number, ok := value.(json.Number)
	if !ok {
		return nil, fmt.Errorf("selected story points field %q is not a number or null", storyPointsField)
	}
	iss.StoryPoints = &NullableNumber{Value: &number}
	return &iss, nil
}

func (c *Client) GetIssueEditMeta(issueKey string) (*EditMeta, error) {
	b, err := c.get(issuePath(pathIssueEditMeta, issueKey))
	if err != nil {
		return nil, err
	}
	var meta EditMeta
	if err := json.Unmarshal(b, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func (c *Client) GetTransitions(issueKey string) ([]Transition, error) {
	b, err := c.get(issuePath(pathIssueTransitions, issueKey))
	if err != nil {
		return nil, err
	}
	var r struct {
		Transitions []Transition `json:"transitions"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	return r.Transitions, nil
}

// DeleteIssue permanently deletes an issue without cascading to subtasks.
func (c *Client) DeleteIssue(issueKey string) error {
	_, err := c.mutate(http.MethodDelete, issuePath(pathIssue, issueKey), nil)
	return err
}

func (c *Client) TransitionIssue(issueKey, transitionID string) error {
	payload, _ := json.Marshal(map[string]map[string]string{
		"transition": {"id": transitionID},
	})
	_, err := c.mutate(http.MethodPost, issuePath(pathIssueTransitions, issueKey), payload)
	return err
}

// CreateIssue creates a new issue and returns its key.
func (c *Client) CreateIssue(projectKey, issueType, summary string, description *ADFDoc) (string, error) {
	return c.createIssue(projectKey, issueType, summary, description, "")
}

// CreateIssueWithParent creates a child issue using the same write semantics as CreateIssue.
func (c *Client) CreateIssueWithParent(projectKey, issueType, summary string, description *ADFDoc, parentKey string) (string, error) {
	return c.createIssue(projectKey, issueType, summary, description, parentKey)
}

func (c *Client) createIssue(projectKey, issueType, summary string, description *ADFDoc, parentKey string) (string, error) {
	fields := map[string]interface{}{
		"project":   map[string]string{"key": projectKey},
		"issuetype": map[string]string{"name": issueType},
		"summary":   summary,
	}
	if description != nil && len(description.Content) > 0 {
		fields["description"] = description
	}
	if parentKey != "" {
		fields["parent"] = map[string]string{"key": parentKey}
	}
	payload, _ := json.Marshal(map[string]interface{}{"fields": fields})
	b, err := c.mutate(http.MethodPost, pathIssueCreate, payload)
	if err != nil {
		return "", err
	}
	var resp struct{ Key string }
	if err := json.Unmarshal(b, &resp); err != nil {
		return "", &MutationError{State: MutationApplied, Err: err}
	}
	if resp.Key == "" {
		return "", &MutationError{State: MutationApplied, Err: fmt.Errorf("create response did not include an issue key")}
	}
	return resp.Key, nil
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

// AssignIssue assigns an issue by account ID.
func (c *Client) AssignIssue(issueKey, accountID string) error {
	payload, _ := json.Marshal(map[string]string{"accountId": accountID})
	_, err := c.mutate(http.MethodPut, issuePath(pathIssueAssign, issueKey), payload)
	return err
}

// UpdateIssue updates one or more fields on an issue.
func (c *Client) UpdateIssue(issueKey string, fields map[string]interface{}) error {
	payload, _ := json.Marshal(map[string]interface{}{"fields": fields})
	_, err := c.mutate(http.MethodPut, issuePath(pathIssue, issueKey), payload)
	return err
}
