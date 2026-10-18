package jira

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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

const (
	maxRequestAttempts = 4
	initialBackoff     = 10 * time.Millisecond
)

func (c *Client) request(method, path string, payload []byte) ([]byte, error) {
	return c.requestWithAttempts(method, path, payload, maxRequestAttempts)
}

func (c *Client) requestWithAttempts(method, path string, payload []byte, attempts int) ([]byte, error) {
	idempotent := method == http.MethodGet
	for attempt := 0; attempt < attempts; attempt++ {
		var body io.Reader
		if payload != nil {
			body = bytes.NewReader(payload)
		}
		req, err := http.NewRequest(method, c.baseURL+path, body)
		if err != nil {
			return nil, err
		}
		req.SetBasicAuth(c.auth, "")
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.http.Do(req)
		if err != nil {
			if !idempotent || attempt+1 == attempts {
				return nil, err
			}
			time.Sleep(initialBackoff * time.Duration(1<<attempt))
			continue
		}
		responseBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode < 400 {
			return responseBody, nil
		}
		if !idempotent || !isRetryableStatus(resp.StatusCode) || attempt+1 == attempts {
			return nil, &APIError{Status: resp.StatusCode, Body: string(responseBody[:minInt(len(responseBody), 200)])}
		}
		time.Sleep(retryDelay(resp, attempt))
	}
	return nil, fmt.Errorf("request failed after %d attempts", maxRequestAttempts)
}

func isRetryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func retryDelay(resp *http.Response, attempt int) time.Duration {
	if value := resp.Header.Get("Retry-After"); value != "" {
		if seconds, err := time.ParseDuration(value + "s"); err == nil {
			return seconds
		}
		if when, err := http.ParseTime(value); err == nil {
			if delay := time.Until(when); delay > 0 {
				return delay
			}
			return 0
		}
	}
	return initialBackoff * time.Duration(1<<attempt)
}

func (c *Client) get(path string) ([]byte, error) {
	return c.request(http.MethodGet, path, nil)
}

func (c *Client) post(path string, payload []byte) ([]byte, error) {
	return c.request(http.MethodPost, path, payload)
}

// MutationState describes what is known about a failed write request.
type MutationState string

const (
	MutationApplied    MutationState = "applied"
	MutationNotApplied MutationState = "not_applied"
	MutationUnknown    MutationState = "unknown"
)

// APIError preserves the HTTP status and bounded response body from Jira.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string { return fmt.Sprintf("API %d: %s", e.Status, e.Body) }

// ClassifyAPIError identifies authentication and visibility failures through wrapping.
func ClassifyAPIError(err error) (string, int) {
	var api *APIError
	if !errors.As(err, &api) {
		return "", 0
	}
	switch api.Status {
	case 401:
		return "authentication_failed", api.Status
	case 403:
		return "permission_denied", api.Status
	case 404:
		return "not_found_or_no_permission", api.Status
	default:
		return "", 0
	}
}

// MutationError preserves whether Jira confirmed or rejected a write. A
// transport error after dispatch is always unknown because the server may have
// applied the request before the connection failed.
type MutationError struct {
	State MutationState
	Err   error
}

func (e *MutationError) Error() string { return e.Err.Error() }
func (e *MutationError) Unwrap() error { return e.Err }

// MutationStateOf returns the state carried by a write error. Unknown is the
// safe fallback for errors whose origin cannot be classified.
func MutationStateOf(err error) MutationState {
	var mutationErr *MutationError
	if errors.As(err, &mutationErr) {
		return mutationErr.State
	}
	return MutationUnknown
}

// mutate dispatches a write exactly once. Writes are never retried because a
// repeated request could duplicate a mutation whose first result was lost.
func (c *Client) mutate(method, path string, payload []byte) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, &MutationError{State: MutationNotApplied, Err: err}
	}
	req.SetBasicAuth(c.auth, "")
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	writeClient := *c.http
	writeClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := writeClient.Do(req)
	if err != nil {
		return nil, &MutationError{State: MutationUnknown, Err: err}
	}
	responseBody, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return nil, &MutationError{State: mutationResponseState(resp.StatusCode), Err: readErr}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &MutationError{
			State: mutationResponseState(resp.StatusCode),
			Err:   &APIError{Status: resp.StatusCode, Body: string(responseBody[:minInt(len(responseBody), 200)])},
		}
	}
	return responseBody, nil
}

func mutationResponseState(status int) MutationState {
	switch {
	case status >= http.StatusOK && status < http.StatusMultipleChoices:
		return MutationApplied
	case status >= http.StatusBadRequest && status < http.StatusInternalServerError:
		return MutationNotApplied
	default:
		return MutationUnknown
	}
}

// ─── Read ──────────────────────────────────────────────────────────────

func (c *Client) TestConnection() (string, error) {
	b, err := c.get(pathServerInfo)
	if err != nil {
		return "", err
	}
	var info struct{ BaseURL, Version string }
	if err := json.Unmarshal(b, &info); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s (Jira %s)", info.BaseURL, info.Version), nil
}
