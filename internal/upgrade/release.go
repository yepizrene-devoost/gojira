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

const (
	// DefaultAPIBaseURL is the API host of the published repository.
	DefaultAPIBaseURL = "https://api.github.com"

	// latestReleasePath is pinned on purpose: an upgrade must never be
	// redirected to an arbitrary repository by configuration or environment.
	latestReleasePath = "/repos/yepizrene-devoost/gojira/releases/latest"

	// DefaultTimeout bounds one release metadata request, including the
	// unauthenticated offline case.
	DefaultTimeout = 10 * time.Second

	maxMetadataBytes = 1 << 20
)

// Asset is one file attached to a published release.
type Asset struct {
	Name string
	URL  string
	Size int64
}

// Release is a published release with its attached assets.
type Release struct {
	TagName string
	Assets  []Asset
}

// Asset returns the asset with the given name, or nil when it is absent.
func (r Release) Asset(name string) *Asset {
	for index := range r.Assets {
		if r.Assets[index].Name == name {
			return &r.Assets[index]
		}
	}
	return nil
}

// Client resolves the published releases of this project. It never sends
// credentials.
type Client struct {
	url  string
	http *http.Client
}

// NewClient returns a release client that queries the pinned latest-release
// path under baseURL, falling back to DefaultAPIBaseURL and a bounded default
// HTTP client when either is omitted.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = DefaultAPIBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultTimeout}
	}
	return &Client{url: baseURL + latestReleasePath, http: httpClient}
}

// releasePayload is the subset of the GitHub release document this command
// reads.
type releasePayload struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	PreRelease bool   `json:"prerelease"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

// LatestRelease returns the most recent published release. Drafts,
// pre-releases and uncomparable tags are refused, and the response body is
// never echoed in an error.
func (c *Client) LatestRelease(ctx context.Context) (Release, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return Release{}, fmt.Errorf("build release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")

	response, err := c.http.Do(request)
	if err != nil {
		return Release{}, fmt.Errorf("fetch latest release: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("fetch latest release: unexpected HTTP status %d", response.StatusCode)
	}

	var payload releasePayload
	if err := json.NewDecoder(io.LimitReader(response.Body, maxMetadataBytes)).Decode(&payload); err != nil {
		return Release{}, fmt.Errorf("decode latest release: %w", err)
	}
	if payload.Draft || payload.PreRelease {
		return Release{}, errors.New("latest release is not published")
	}
	if payload.TagName == "" {
		return Release{}, errors.New("latest release has no tag")
	}
	if _, err := ParseVersion(payload.TagName); err != nil {
		return Release{}, fmt.Errorf("latest release tag %q: %w", payload.TagName, err)
	}

	release := Release{TagName: payload.TagName, Assets: make([]Asset, 0, len(payload.Assets))}
	for _, asset := range payload.Assets {
		release.Assets = append(release.Assets, Asset{Name: asset.Name, URL: asset.URL, Size: asset.Size})
	}
	return release, nil
}
