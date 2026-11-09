package jira

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// commentPageMaxResults is the page size used when walking the full comment
// collection; Jira caps any single page far above this value.
const commentPageMaxResults = 50

// listCommentsPageCap bounds the pagination walk so a misbehaving server can
// never loop forever.
const listCommentsPageCap = 1000

// ListComments reads the complete comment collection of an issue by walking
// the paginated endpoint until every page is collected. It returns the
// comments in Jira order and Jira's reported total. A page that does not echo
// the requested startAt aborts the walk instead of looping.
func (c *Client) ListComments(issueKey string) ([]Comment, int, error) {
	base := issuePath(pathIssueComment, issueKey)
	var comments []Comment
	startAt := 0
	for page := 0; page < listCommentsPageCap; page++ {
		path := fmt.Sprintf("%s?startAt=%d&maxResults=%d", base, startAt, commentPageMaxResults)
		b, err := c.get(path)
		if err != nil {
			return nil, 0, err
		}
		var result struct {
			StartAt    int       `json:"startAt"`
			MaxResults int       `json:"maxResults"`
			Total      int       `json:"total"`
			IsLast     bool      `json:"isLast"`
			Comments   []Comment `json:"comments"`
		}
		if err := json.Unmarshal(b, &result); err != nil {
			return nil, 0, err
		}
		if result.StartAt != startAt {
			return nil, 0, fmt.Errorf("comment page reported startAt %d, requested %d", result.StartAt, startAt)
		}
		comments = append(comments, result.Comments...)
		startAt += len(result.Comments)
		if result.IsLast || len(result.Comments) == 0 || startAt >= result.Total {
			return comments, result.Total, nil
		}
	}
	return nil, 0, errors.New("comment pagination exceeded the page cap")
}

// GetComment reads one comment by its stable id.
func (c *Client) GetComment(issueKey, commentID string) (*Comment, error) {
	if commentID == "" {
		return nil, errors.New("comment id is required")
	}
	b, err := c.get(commentEntryPath(issueKey, commentID))
	if err != nil {
		return nil, err
	}
	var comment Comment
	if err := json.Unmarshal(b, &comment); err != nil {
		return nil, err
	}
	return &comment, nil
}

// UpdateComment replaces a comment body with a new ADF document built from
// plain text. Jira replaces the whole body: rich formatting, mentions and
// visibility restrictions of the previous body are not preserved, so callers
// must warn before editing.
func (c *Client) UpdateComment(issueKey, commentID, text string) error {
	if commentID == "" {
		return errors.New("comment id is required")
	}
	if text == "" {
		return errors.New("update requires comment text")
	}
	payload, err := json.Marshal(map[string]interface{}{"body": TextToADF(text)})
	if err != nil {
		return err
	}
	_, err = c.mutate(http.MethodPut, commentEntryPath(issueKey, commentID), payload)
	return err
}

// DeleteComment removes one comment. The request carries no body.
func (c *Client) DeleteComment(issueKey, commentID string) error {
	if commentID == "" {
		return errors.New("comment id is required")
	}
	_, err := c.mutate(http.MethodDelete, commentEntryPath(issueKey, commentID), nil)
	return err
}
