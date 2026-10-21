package jira

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// SearchText searches Jira's text index while treating query as a JQL string
// literal rather than executable JQL.
func (c *Client) SearchText(query string, maxResults int) (SearchResult, error) {
	escaped := strings.ReplaceAll(query, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return c.SearchJQL(fmt.Sprintf(`text ~ "%s"`, escaped), maxResults)
}

// SearchResult describes completeness independently of Jira's optional estimate.
type SearchResult struct {
	Issues         []Issue
	Truncated      bool
	EstimatedTotal *int
}

// SearchIssues follows tokens with at most 1,000 pages to bound malformed servers.
// Repeated tokens fail rather than presenting a partial list as complete.
func (c *Client) SearchIssues(jql string, limit int) (SearchResult, error) {
	if limit <= 0 {
		limit = 50
	}
	result := SearchResult{Issues: []Issue{}}
	token := ""
	seen := make(map[string]bool)
	for page := 0; page < 1000; page++ {
		remaining := limit - len(result.Issues)
		path := fmt.Sprintf("%s?jql=%s&maxResults=%d&fields=%s", pathSearchJQL, url.QueryEscape(jql), min(100, remaining), fieldsSearch)
		if token != "" {
			path += "&nextPageToken=" + url.QueryEscape(token)
		}
		b, err := c.get(path)
		if err != nil {
			return SearchResult{}, err
		}
		var response struct {
			Issues        []Issue `json:"issues"`
			NextPageToken string  `json:"nextPageToken"`
			IsLast        bool    `json:"isLast"`
		}
		if err := json.Unmarshal(b, &response); err != nil {
			return SearchResult{}, err
		}
		more := !response.IsLast && response.NextPageToken != ""
		if len(response.Issues) > remaining {
			more = true
			response.Issues = response.Issues[:remaining]
		}
		result.Issues = append(result.Issues, response.Issues...)
		if len(result.Issues) == limit {
			result.Truncated = more
			if more {
				result.EstimatedTotal = c.searchEstimate(jql)
			}
			return result, nil
		}
		if !more {
			return result, nil
		}
		token = response.NextPageToken
		if seen[token] {
			return SearchResult{}, fmt.Errorf("search pagination repeated a token")
		}
		seen[token] = true
	}
	return SearchResult{}, fmt.Errorf("search exceeded the 1000-page safety limit")
}

func (c *Client) searchEstimate(jql string) *int {
	payload, _ := json.Marshal(map[string]string{"jql": jql})
	// Counting is best-effort and deliberately attempted only once, without retries.
	b, err := c.post(pathSearchApproximateCount, payload)
	if err != nil {
		return nil
	}
	var response struct {
		Count *int `json:"count"`
	}
	if json.Unmarshal(b, &response) != nil || response.Count == nil || *response.Count < 0 {
		return nil
	}
	return response.Count
}

func (c *Client) SearchJQL(jql string, maxResults int) (SearchResult, error) {
	return c.SearchIssues(jql, maxResults)
}
