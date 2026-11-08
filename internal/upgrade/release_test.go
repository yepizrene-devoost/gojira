package upgrade

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLatestReleaseQueriesOnlyThePinnedLatestEndpoint(t *testing.T) {
	var gotPath, gotMethod, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod, gotAuth = r.URL.Path, r.Method, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name": "v0.6.0",
			"draft": false,
			"prerelease": false,
			"assets": [
				{"name": "checksums.txt", "browser_download_url": "https://example.test/checksums.txt", "size": 128},
				{"name": "gojira_Linux_x86_64.tar.gz", "browser_download_url": "https://example.test/gojira_Linux_x86_64.tar.gz", "size": 4096}
			]
		}`))
	}))
	defer server.Close()

	release, err := NewClient(server.URL, server.Client()).LatestRelease(context.Background())
	if err != nil {
		t.Fatalf("LatestRelease returned unexpected error: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("request method = %q, want GET", gotMethod)
	}
	if gotPath != latestReleasePath {
		t.Fatalf("request path = %q, want the pinned latest-release path %q", gotPath, latestReleasePath)
	}
	if gotAuth != "" {
		t.Fatalf("request sent an Authorization header %q, want none", gotAuth)
	}
	if release.TagName != "v0.6.0" {
		t.Fatalf("TagName = %q, want v0.6.0", release.TagName)
	}
	if len(release.Assets) != 2 {
		t.Fatalf("got %d assets, want 2", len(release.Assets))
	}
	if asset := release.Asset("gojira_Linux_x86_64.tar.gz"); asset == nil {
		t.Fatal("Asset lookup did not find the released archive")
	} else if asset.URL != "https://example.test/gojira_Linux_x86_64.tar.gz" || asset.Size != 4096 {
		t.Fatalf("unexpected asset payload: %+v", *asset)
	}
	if asset := release.Asset("missing"); asset != nil {
		t.Fatalf("Asset lookup returned %+v for an absent name", *asset)
	}
}

func TestLatestReleaseRefusesUnpublishedReleases(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "draft", body: `{"tag_name":"v0.6.0","draft":true,"assets":[]}`},
		{name: "prerelease", body: `{"tag_name":"v0.6.0","prerelease":true,"assets":[]}`},
		{name: "missing tag", body: `{"tag_name":"","assets":[]}`},
		{name: "uncomparable tag", body: `{"tag_name":"nightly","assets":[]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			if _, err := NewClient(server.URL, server.Client()).LatestRelease(context.Background()); err == nil {
				t.Fatal("LatestRelease accepted a release that is not published")
			}
		})
	}
}

func TestLatestReleaseReportsHTTPFailureWithoutLeakingTheBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"secret-token-value"}`))
	}))
	defer server.Close()

	_, err := NewClient(server.URL, server.Client()).LatestRelease(context.Background())
	if err == nil {
		t.Fatal("LatestRelease accepted a 404 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("error %q does not report the HTTP status", err)
	}
	if strings.Contains(err.Error(), "secret-token-value") {
		t.Fatalf("error %q leaked the response body", err)
	}
}

func TestLatestReleaseReportsMalformedPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not json`))
	}))
	defer server.Close()

	if _, err := NewClient(server.URL, server.Client()).LatestRelease(context.Background()); err == nil {
		t.Fatal("LatestRelease accepted malformed JSON")
	}
}

func TestLatestReleaseReportsTransportFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	client := server.Client()
	server.Close()

	if _, err := NewClient(url, client).LatestRelease(context.Background()); err == nil {
		t.Fatal("LatestRelease accepted a closed server")
	}
}

func TestLatestReleaseHonorsContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if _, err := NewClient(server.URL, server.Client()).LatestRelease(ctx); err == nil {
		t.Fatal("LatestRelease ignored the cancelled context")
	}
}

func TestNewClientDefaultsToThePinnedReleasesEndpoint(t *testing.T) {
	client := NewClient("", nil)
	if want := DefaultAPIBaseURL + latestReleasePath; client.url != want {
		t.Fatalf("default endpoint = %q, want %q", client.url, want)
	}
	if client.http.Timeout <= 0 {
		t.Fatalf("default client timeout = %v, want a bounded timeout", client.http.Timeout)
	}
	if DefaultAPIBaseURL != "https://api.github.com" {
		t.Fatalf("DefaultAPIBaseURL = %q, want the published API host", DefaultAPIBaseURL)
	}
	if latestReleasePath != "/repos/yepizrene-devoost/gojira/releases/latest" {
		t.Fatalf("latestReleasePath = %q, want the published repository path", latestReleasePath)
	}
}
