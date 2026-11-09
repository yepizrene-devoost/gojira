package upgrade

import (
	"context"
	"errors"
	"github.com/yepizrene-devoost/gojira/internal/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// countingTransport counts every request that leaves the process. It always
// fails, so a test that expects zero requests can prove none was attempted.
type countingTransport struct{ calls atomic.Int32 }

func (t *countingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.calls.Add(1)
	return nil, errors.New("unexpected HTTP request")
}

func countingClient(baseURL string, transport *countingTransport) *Client {
	return NewClient(baseURL, &http.Client{Transport: transport})
}

const releaseBody = `{"tag_name":"v0.7.0","draft":false,"prerelease":false,"assets":[]}`

// Rule S2, fresh check: without a stored ETag no If-None-Match is sent, and a
// 200 yields both the tag and the response ETag to persist, plus the headers
// the GitHub API requires.
func TestCheckLatestWithoutETagSendsNoConditionalHeaderAndReturnsTagPlusETag(t *testing.T) {
	var gotMethod, gotPath, gotAccept, gotUA, gotIfNoneMatch string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAccept = r.Header.Get("Accept")
		gotUA = r.Header.Get("User-Agent")
		gotIfNoneMatch = r.Header.Get("If-None-Match")
		w.Header().Set("ETag", `"etag-v0.7.0"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(releaseBody))
	}))
	defer server.Close()

	result := NewClient(server.URL, server.Client()).CheckLatest(context.Background(), "", "")

	if result.State != Completed {
		t.Fatalf("State = %q (%v), want Completed: %v", result.State, result.Reason, result.Err)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v, want nil", result.Err)
	}
	if result.NotModified {
		t.Fatal("NotModified = true for a 200, want false")
	}
	if result.LatestVersion != "v0.7.0" {
		t.Fatalf("LatestVersion = %q, want v0.7.0", result.LatestVersion)
	}
	if result.ETag != `"etag-v0.7.0"` {
		t.Fatalf("ETag = %q, want the response ETag", result.ETag)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("method = %q, want GET", gotMethod)
	}
	if gotPath != latestReleasePath {
		t.Fatalf("path = %q, want the pinned latest-release path", gotPath)
	}
	if gotIfNoneMatch != "" {
		t.Fatalf("If-None-Match = %q, want no conditional header without a stored ETag", gotIfNoneMatch)
	}
	if gotAccept != "application/vnd.github+json" {
		t.Fatalf("Accept = %q, want application/vnd.github+json", gotAccept)
	}
	if strings.TrimSpace(gotUA) == "" {
		t.Fatal("User-Agent is empty, want a non-empty User-Agent")
	}
}

// Rule S2, second check: the stored ETag travels as If-None-Match, and a 304
// reports no new release — the stored version and ETag come back unchanged
// with no error.
func TestCheckLatestSendsStoredETagAnd304KeepsStoredVersion(t *testing.T) {
	var gotIfNoneMatch string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIfNoneMatch = r.Header.Get("If-None-Match")
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	result := NewClient(server.URL, server.Client()).CheckLatest(context.Background(), `"etag-v0.6.0"`, "v0.6.0")

	if gotIfNoneMatch != `"etag-v0.6.0"` {
		t.Fatalf("If-None-Match = %q, want the stored ETag", gotIfNoneMatch)
	}
	if result.State != Completed {
		t.Fatalf("State = %q, want Completed: %v", result.State, result.Err)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v, want nil for a 304", result.Err)
	}
	if !result.NotModified {
		t.Fatal("NotModified = false after a 304, want true")
	}
	if result.LatestVersion != "v0.6.0" {
		t.Fatalf("LatestVersion = %q, want the stored v0.6.0", result.LatestVersion)
	}
	if result.ETag != `"etag-v0.6.0"` {
		t.Fatalf("ETag = %q, want the stored ETag unchanged", result.ETag)
	}
}

// Rule S2, refresh: a 200 while an old ETag is stored replaces both the tag
// and the ETag.
func TestCheckLatest200RefreshesTagAndETag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("If-None-Match"); got != `"etag-old"` {
			t.Errorf("If-None-Match = %q, want the stored old ETag", got)
		}
		w.Header().Set("ETag", `"etag-new"`)
		_, _ = w.Write([]byte(releaseBody))
	}))
	defer server.Close()

	result := NewClient(server.URL, server.Client()).CheckLatest(context.Background(), `"etag-old"`, "v0.6.0")

	if result.State != Completed {
		t.Fatalf("State = %q, want Completed: %v", result.State, result.Err)
	}
	if result.LatestVersion != "v0.7.0" {
		t.Fatalf("LatestVersion = %q, want the new tag v0.7.0", result.LatestVersion)
	}
	if result.ETag != `"etag-new"` {
		t.Fatalf("ETag = %q, want the new ETag", result.ETag)
	}
	if result.NotModified {
		t.Fatal("NotModified = true for a 200, want false")
	}
}

// Rule S3: HTTP failures resolve to Unsuccessful, never Completed.
func TestCheckLatestHTTPFailuresResolveToUnsuccessful(t *testing.T) {
	cases := []struct {
		name string
		code int
	}{
		{name: "404", code: http.StatusNotFound},
		{name: "500", code: http.StatusInternalServerError},
		{name: "503", code: http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(`{"message":"secret-token-value"}`))
			}))
			defer server.Close()

			result := NewClient(server.URL, server.Client()).CheckLatest(context.Background(), `"etag"`, "v0.6.0")
			if result.State != Unsuccessful {
				t.Fatalf("State = %q, want Unsuccessful", result.State)
			}
			if result.State == Completed {
				t.Fatal("a failing HTTP status resolved to Completed")
			}
			if result.Err == nil {
				t.Fatal("Err = nil, want the HTTP failure")
			}
			if strings.Contains(result.Err.Error(), "secret-token-value") {
				t.Fatalf("Err %q leaked the response body", result.Err)
			}
			if result.LatestVersion != "v0.6.0" {
				t.Fatalf("LatestVersion = %q, want the stored value preserved", result.LatestVersion)
			}
		})
	}
}

// Rule S3: a network error resolves to Unsuccessful, never Completed.
func TestCheckLatestNetworkErrorResolvesToUnsuccessful(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	client := server.Client()
	server.Close()

	result := NewClient(url, client).CheckLatest(context.Background(), "", "")
	if result.State != Unsuccessful {
		t.Fatalf("State = %q, want Unsuccessful", result.State)
	}
	if result.Err == nil {
		t.Fatal("Err = nil, want the network failure")
	}
}

// Rule S4: a dev build never performs the request — zero HTTP calls.
func TestCheckSkipsDevBuildWithoutRequest(t *testing.T) {
	transport := &countingTransport{}
	client := countingClient("https://example.test", transport)

	result := client.Check(context.Background(), CheckRequest{
		CurrentVersion: "dev",
		Enabled:        true,
		Now:            time.Now(),
		Interval:       0,
	})

	if result.State != Skipped {
		t.Fatalf("State = %q (%v), want Skipped: %v", result.State, result.Reason, result.Err)
	}
	if result.Reason != SkipDevBuild {
		t.Fatalf("Reason = %q, want %q", result.Reason, SkipDevBuild)
	}
	if got := transport.calls.Load(); got != 0 {
		t.Fatalf("HTTP calls = %d, want 0 for a dev build", got)
	}
}

// Rule S1/S3: update_check false is the master switch — skipped with a reason
// and zero HTTP calls.
func TestCheckSkipsWhenUpdateCheckDisabled(t *testing.T) {
	transport := &countingTransport{}
	client := countingClient("https://example.test", transport)

	result := client.Check(context.Background(), CheckRequest{
		CurrentVersion: "v0.5.0",
		Enabled:        false,
		Now:            time.Now(),
		Interval:       0,
	})

	if result.State != Skipped {
		t.Fatalf("State = %q, want Skipped: %v", result.State, result.Err)
	}
	if result.Reason != SkipUpdateCheckDisabled {
		t.Fatalf("Reason = %q, want %q", result.Reason, SkipUpdateCheckDisabled)
	}
	if got := transport.calls.Load(); got != 0 {
		t.Fatalf("HTTP calls = %d, want 0 when update_check is disabled", got)
	}
}

// Rule S1: interval 0 checks on every start even when a check was recorded a
// minute ago; a positive interval skips inside the window and proceeds once it
// has elapsed; an unparseable stored timestamp stays fail-safe (due).
func TestCheckIntervalGate(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		interval  time.Duration
		lastCheck string
		wantState CheckState
		wantErr   error
	}{
		{
			name:      "interval zero checks every start",
			interval:  0,
			lastCheck: now.Add(-time.Minute).Format(time.RFC3339),
			wantState: Completed,
		},
		{
			name:      "positive interval skips inside the window",
			interval:  6 * time.Hour,
			lastCheck: now.Add(-time.Hour).Format(time.RFC3339),
			wantState: Skipped,
		},
		{
			name:      "positive interval proceeds after the window",
			interval:  6 * time.Hour,
			lastCheck: now.Add(-7 * time.Hour).Format(time.RFC3339),
			wantState: Completed,
		},
		{
			name:      "unparseable stored timestamp proceeds",
			interval:  6 * time.Hour,
			lastCheck: "yesterday",
			wantState: Completed,
		},
		{
			name:      "never checked proceeds",
			interval:  6 * time.Hour,
			lastCheck: "",
			wantState: Completed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requests := atomic.Int32{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.Header().Set("ETag", `"etag-v0.7.0"`)
				_, _ = w.Write([]byte(releaseBody))
			}))
			defer server.Close()

			client := NewClient(server.URL, server.Client())
			result := client.Check(context.Background(), CheckRequest{
				CurrentVersion: "v0.6.0",
				Enabled:        true,
				Now:            now,
				Interval:       tc.interval,
				LastCheck:      tc.lastCheck,
				ETag:           `"etag-v0.6.0"`,
				LatestVersion:  "v0.6.0",
			})

			if result.State != tc.wantState {
				t.Fatalf("State = %q (%v), want %q: %v", result.State, result.Reason, tc.wantState, result.Err)
			}
			got := requests.Load()
			switch tc.wantState {
			case Skipped:
				if result.Reason != SkipInsideInterval {
					t.Fatalf("Reason = %q, want %q", result.Reason, SkipInsideInterval)
				}
				if got != 0 {
					t.Fatalf("HTTP calls = %d, want 0 inside the interval", got)
				}
			case Completed:
				if got != 1 {
					t.Fatalf("HTTP calls = %d, want 1", got)
				}
				if result.LatestVersion != "v0.7.0" {
					t.Fatalf("LatestVersion = %q, want v0.7.0", result.LatestVersion)
				}
			}
		})
	}
}

// Rules S3+S4 triangulation: an unpublished tag on 200 never resolves to
// Completed, and any current version that cannot be compared with a release
// (not just the literal "dev") skips without a request.
func TestCheckLatestRefusesUnpublishedTagsAsUnsuccessful(t *testing.T) {
	bodies := map[string]string{
		"draft":        `{"tag_name":"v0.7.0","draft":true,"prerelease":false,"assets":[]}`,
		"prerelease":   `{"tag_name":"v0.7.0","draft":false,"prerelease":true,"assets":[]}`,
		"uncomparable": `{"tag_name":"nightly","draft":false,"prerelease":false,"assets":[]}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()

			result := NewClient(server.URL, server.Client()).CheckLatest(context.Background(), "", "")
			if result.State != Unsuccessful {
				t.Fatalf("State = %q, want Unsuccessful", result.State)
			}
			if result.Err == nil {
				t.Fatal("Err = nil, want the refusal")
			}
		})
	}
}

func TestCheckTreatsUncomparableCurrentVersionAsDevBuild(t *testing.T) {
	for _, current := range []string{"", "dev", "nightly"} {
		t.Run("current "+current, func(t *testing.T) {
			transport := &countingTransport{}
			client := countingClient("https://example.test", transport)

			result := client.Check(context.Background(), CheckRequest{
				CurrentVersion: current,
				Enabled:        true,
				Now:            time.Now(),
			})
			if result.State != Skipped || result.Reason != SkipDevBuild {
				t.Fatalf("State = %q Reason = %q, want Skipped/SkipDevBuild", result.State, result.Reason)
			}
			if got := transport.calls.Load(); got != 0 {
				t.Fatalf("HTTP calls = %d, want 0", got)
			}
		})
	}
}

// Rules S1+S4 tie together: an invalid update_check_interval parses to 0 in
// the config layer, so the check runs on every start.
func TestCheckUsesConfiguredIntervalFromConfig(t *testing.T) {
	requests := atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(releaseBody))
	}))
	defer server.Close()

	cfg := &config.Config{UpdateCheckInterval: "daily"}
	interval := cfg.UpdateCheckIntervalDuration()
	if interval != 0 {
		t.Fatalf("UpdateCheckIntervalDuration() = %v, want 0 for an invalid value", interval)
	}

	result := NewClient(server.URL, server.Client()).Check(context.Background(), CheckRequest{
		CurrentVersion: "v0.6.0",
		Enabled:        true,
		Now:            time.Now(),
		Interval:       interval,
		LastCheck:      time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
	})

	if result.State != Completed {
		t.Fatalf("State = %q (%v), want Completed: %v", result.State, result.Reason, result.Err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("HTTP calls = %d, want 1 with an invalid configured interval", got)
	}
}
