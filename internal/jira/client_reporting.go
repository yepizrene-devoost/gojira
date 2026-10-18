package jira

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/yepizrene-devoost/gojira/internal/reporting"
)

// reportingHTTPError retains API classification and exposes only the HTTP status
// to reporting policy without coupling that package to the Jira adapter.
type reportingHTTPError struct{ *APIError }

func (e *reportingHTTPError) HTTPStatus() int { return e.Status }
func (e *reportingHTTPError) Unwrap() error   { return e.APIError }

// reportingGet uses the existing client's HTTP configuration with cancellable,
// single-attempt reads. Reporting owns its strict page budget; no hidden retries.
func (c *Client) reportingGet(ctx context.Context, path string, query url.Values, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path+"?"+query.Encode(), nil)
	if err != nil {
		return errors.New("invalid reporting request")
	}
	req.SetBasicAuth(c.auth, "")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("reporting request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &reportingHTTPError{&APIError{Status: resp.StatusCode}}
	}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(target); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("invalid reporting response")
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("invalid reporting response trailer")
	}
	return nil
}
func (c *Client) ReportingUsers(ctx context.Context, query string, offset int) ([]reporting.User, error) {
	var users []reporting.User
	err := c.reportingGet(ctx, pathReportingUsers, url.Values{"query": {query}, "startAt": {strconv.Itoa(offset)}, "maxResults": {"100"}}, &users)
	if err == nil && users == nil {
		err = errors.New("invalid null user response")
	}
	return users, err
}
func (c *Client) ReportingIssues(ctx context.Context, jql, token string) (reporting.IssuePage, error) {
	var page reporting.IssuePage
	q := url.Values{"jql": {jql}, "maxResults": {"100"}, "fields": {"id,key"}}
	if token != "" {
		q.Set("nextPageToken", token)
	}
	err := c.reportingGet(ctx, pathSearchJQL, q, &page)
	return page, err
}
func (c *Client) ReportingWorklogs(ctx context.Context, id string, offset int) (reporting.WorklogPage, error) {
	var page reporting.WorklogPage
	if strings.TrimSpace(id) == "" {
		return page, errors.New("issue ID is required")
	}
	err := c.reportingGet(ctx, issuePath(pathIssueWorklog, id), url.Values{"startAt": {strconv.Itoa(offset)}, "maxResults": {"100"}}, &page)
	return page, err
}
