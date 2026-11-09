package upgrade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// userAgent identifies this conditional release check to the GitHub API. The
// check never sends credentials.
const userAgent = "gojira update-check"

// CheckState is the provenance of one start-up release check. Only Completed
// ran and produced a version; Skipped and Unsuccessful must never claim
// "up to date".
type CheckState string

const (
	// Completed means the check ran: either a 200 delivered a fresh tag or a
	// 304 confirmed the stored version.
	Completed CheckState = "completed"
	// Skipped means no request was made; Reason says why.
	Skipped CheckState = "skipped"
	// Unsuccessful means the request failed; Err says how.
	Unsuccessful CheckState = "unsuccessful"
)

// SkipReason explains why a check was skipped without performing a request.
type SkipReason string

const (
	// SkipUpdateCheckDisabled: update_check is explicitly false.
	SkipUpdateCheckDisabled SkipReason = "update_check disabled"
	// SkipDevBuild: the running version cannot be compared with a release.
	SkipDevBuild SkipReason = "dev build"
	// SkipInsideInterval: the last check is younger than the configured interval.
	SkipInsideInterval SkipReason = "inside configured interval"
)

// CheckRequest carries everything one start-up release check needs: the
// configuration gate (Enabled, Interval, LastCheck), the conditional-request
// state (ETag, LatestVersion) and the running build (CurrentVersion). The
// zero value is a check that runs on every start.
type CheckRequest struct {
	CurrentVersion string
	Enabled        bool
	Now            time.Time
	Interval       time.Duration
	LastCheck      string // RFC3339 timestamp, "" when never checked
	ETag           string // stored response ETag, "" when unknown
	LatestVersion  string // stored latest release tag, "" when unknown
}

// CheckResult is the outcome of one release check with its provenance.
// LatestVersion and ETag always carry the pair the caller should keep
// persisted: the new tag and response ETag after a 200, and the unchanged
// stored pair in every other state. Err is set only when State is
// Unsuccessful, Reason only when State is Skipped, NotModified only for a 304.
type CheckResult struct {
	State         CheckState
	Reason        SkipReason
	Err           error
	LatestVersion string
	ETag          string
	NotModified   bool
}

// Check performs one start-up release check: it resolves the S3 provenance
// states first and only performs a request when the check is due. A disabled
// update_check, a dev build or a check inside the configured interval skip
// without any HTTP traffic; otherwise CheckLatest runs the conditional
// request.
func (c *Client) Check(ctx context.Context, req CheckRequest) CheckResult {
	if !req.Enabled {
		return skippedResult(SkipUpdateCheckDisabled, req)
	}
	if _, err := ParseVersion(req.CurrentVersion); err != nil {
		return skippedResult(SkipDevBuild, req)
	}
	if req.Interval > 0 && !checkDue(req.Now, req.LastCheck, req.Interval) {
		return skippedResult(SkipInsideInterval, req)
	}
	return c.CheckLatest(ctx, req.ETag, req.LatestVersion)
}

// CheckLatest performs the conditional GET against the pinned latest-release
// endpoint with the Accept and User-Agent headers the GitHub API requires,
// sending If-None-Match only when a stored ETag is known. A 304 reports "no
// new release" as Completed with the stored version and ETag unchanged; a 200
// returns the fresh tag and response ETag; a 404, any 5xx and any network
// error resolve to Unsuccessful, never Completed, and the response body is
// never echoed in an error.
func (c *Client) CheckLatest(ctx context.Context, etag, storedLatest string) CheckResult {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return failureResult(fmt.Errorf("build release check request: %w", err), etag, storedLatest)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", userAgent)
	if etag != "" {
		request.Header.Set("If-None-Match", etag)
	}

	response, err := c.http.Do(request)
	if err != nil {
		return failureResult(fmt.Errorf("check latest release: %w", err), etag, storedLatest)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == http.StatusNotModified {
		return CheckResult{
			State:         Completed,
			NotModified:   true,
			LatestVersion: storedLatest,
			ETag:          etag,
		}
	}
	if response.StatusCode != http.StatusOK {
		return failureResult(fmt.Errorf("check latest release: unexpected HTTP status %d", response.StatusCode), etag, storedLatest)
	}

	var payload releasePayload
	if err := json.NewDecoder(io.LimitReader(response.Body, maxMetadataBytes)).Decode(&payload); err != nil {
		return failureResult(fmt.Errorf("decode latest release: %w", err), etag, storedLatest)
	}
	if payload.Draft || payload.PreRelease {
		return failureResult(errors.New("latest release is not published"), etag, storedLatest)
	}
	if _, err := ParseVersion(payload.TagName); err != nil {
		return failureResult(fmt.Errorf("latest release tag %q: %w", payload.TagName, err), etag, storedLatest)
	}
	return CheckResult{
		State:         Completed,
		LatestVersion: payload.TagName,
		ETag:          response.Header.Get("ETag"),
	}
}

// skippedResult builds a Skipped outcome that keeps the stored pair intact.
func skippedResult(reason SkipReason, req CheckRequest) CheckResult {
	return CheckResult{
		State:         Skipped,
		Reason:        reason,
		LatestVersion: req.LatestVersion,
		ETag:          req.ETag,
	}
}

// failureResult builds an Unsuccessful outcome that keeps the stored pair
// intact.
func failureResult(err error, etag, storedLatest string) CheckResult {
	return CheckResult{
		State:         Unsuccessful,
		Err:           err,
		LatestVersion: storedLatest,
		ETag:          etag,
	}
}

// checkDue reports whether a check is due at now. It mirrors
// config.UpdateCheckDue with the same fail-safe semantics: an empty or
// unparseable stored timestamp keeps the check due.
func checkDue(now time.Time, lastCheck string, interval time.Duration) bool {
	if lastCheck == "" {
		return true
	}
	last, err := time.Parse(time.RFC3339, lastCheck)
	if err != nil {
		return true
	}
	return !now.Before(last.Add(interval))
}
