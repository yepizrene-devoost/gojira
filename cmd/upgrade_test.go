package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
	"github.com/yepizrene-devoost/gojira/internal/upgrade"
)

// releaseFixture describes what the fake release server publishes.
type releaseFixture struct {
	tag       string
	archive   []byte
	digest    string
	status    int
	metadata  int
	requests  *int32
	downloads *int32
}

func newReleaseArchive(t *testing.T, binary string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	body := []byte(binary)
	if err := tarWriter.WriteHeader(&tar.Header{Name: "README.md", Mode: 0o644, Size: 4, Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("write README header: %v", err)
	}
	if _, err := tarWriter.Write([]byte("docs")); err != nil {
		t.Fatalf("write README body: %v", err)
	}
	if err := tarWriter.WriteHeader(&tar.Header{Name: "gojira", Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("write binary header: %v", err)
	}
	if _, err := tarWriter.Write(body); err != nil {
		t.Fatalf("write binary body: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buffer.Bytes()
}

func testAssetName(t *testing.T) string {
	t.Helper()
	name, err := upgrade.ArtifactName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatalf("ArtifactName: %v", err)
	}
	return name
}

// serveRelease starts a fake release server and returns its base URL. The
// server answers the metadata, checksum and archive requests an upgrade makes.
func serveRelease(t *testing.T, fixture *releaseFixture) string {
	t.Helper()
	var baseURL string
	requests := new(int32)
	downloads := new(int32)
	fixture.requests, fixture.downloads = requests, downloads

	sum := sha256.Sum256(fixture.archive)
	archiveDigest := fixture.digest
	if archiveDigest == "" {
		archiveDigest = hex.EncodeToString(sum[:])
	}
	assetName := testAssetName(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(requests, 1)
		if fixture.status != 0 {
			w.WriteHeader(fixture.status)
			return
		}
		switch {
		case r.URL.Path == "/checksums.txt":
			atomic.AddInt32(downloads, 1)
			_, _ = fmt.Fprintf(w, "%s  %s\n", archiveDigest, assetName)
		case r.URL.Path == "/"+assetName:
			atomic.AddInt32(downloads, 1)
			_, _ = w.Write(fixture.archive)
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			metadataStatus := fixture.metadata
			if metadataStatus != 0 {
				w.WriteHeader(metadataStatus)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name":   fixture.tag,
				"draft":      false,
				"prerelease": false,
				"assets": []map[string]any{
					{"name": "checksums.txt", "browser_download_url": baseURL + "/checksums.txt", "size": 128},
					{"name": assetName, "browser_download_url": baseURL + "/" + assetName, "size": len(fixture.archive)},
				},
			})
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	baseURL = server.URL
	t.Cleanup(server.Close)

	oldBaseURL := upgradeBaseURL
	upgradeBaseURL = server.URL
	t.Cleanup(func() { upgradeBaseURL = oldBaseURL })
	return server.URL
}

// useFakeInstallation installs the upgrade installer seams for one temp binary.
func useFakeInstallation(t *testing.T, binary, ownedDir string) {
	t.Helper()
	old := upgradeInstaller
	upgradeInstaller = func() *upgrade.Installer {
		return upgrade.NewInstaller(upgrade.System{
			GOOS:       runtime.GOOS,
			Executable: func() (string, error) { return binary, nil },
			Getenv:     func(string) string { return ownedDir },
			HomeDir:    func() (string, error) { return "", errors.New("no home directory") },
			Lstat:      os.Lstat,
			Rename:     os.Rename,
			Remove:     os.Remove,
		})
	}
	t.Cleanup(func() { upgradeInstaller = old })
}

func useTerminal(t *testing.T, terminal bool) {
	t.Helper()
	old := isTerminal
	isTerminal = func() bool { return terminal }
	t.Cleanup(func() { isTerminal = old })
}

func useStdin(t *testing.T, input string) {
	t.Helper()
	old := rootCmd.InOrStdin()
	rootCmd.SetIn(strings.NewReader(input))
	t.Cleanup(func() { rootCmd.SetIn(old) })
}

func TestUpgradeCheckReportsTheAvailableReleaseWithoutInstalling(t *testing.T) {
	setTestVersion(t, "v0.5.0")
	installDir := t.TempDir()
	binary := filepath.Join(installDir, "gojira")
	if err := os.WriteFile(binary, []byte("current"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	useFakeInstallation(t, binary, installDir)
	fixture := &releaseFixture{tag: "v0.6.0", archive: newReleaseArchive(t, "next")}
	serveRelease(t, fixture)

	code, stdout, stderr := runTestCLI(t, jira.NewClient("http://unused.example", "email", "token"), "upgrade", "--check", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	var report upgradeReport
	assertSingleJSONDocument(t, []byte(stdout), &report)
	if report.Action != "available" || !report.UpdateAvailable {
		t.Fatalf("report = %+v, want an available update", report)
	}
	if report.CurrentVersion != "v0.5.0" || report.LatestVersion != "v0.6.0" {
		t.Fatalf("report versions = %q -> %q", report.CurrentVersion, report.LatestVersion)
	}
	if report.MutationState != jira.MutationNotApplied {
		t.Fatalf("mutationState = %q, want not_applied", report.MutationState)
	}
	if got := atomic.LoadInt32(fixture.downloads); got != 0 {
		t.Fatalf("check mode performed %d downloads, want 0", got)
	}
	if got, err := os.ReadFile(binary); err != nil || string(got) != "current" {
		t.Fatalf("binary content = %q (err %v), want it untouched", got, err)
	}
}

func TestUpgradeRefusesNonInteractiveInstallWithoutYes(t *testing.T) {
	setTestVersion(t, "v0.5.0")
	installDir := t.TempDir()
	binary := filepath.Join(installDir, "gojira")
	if err := os.WriteFile(binary, []byte("current"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	useFakeInstallation(t, binary, installDir)
	useTerminal(t, false)
	fixture := &releaseFixture{tag: "v0.6.0", archive: newReleaseArchive(t, "next")}
	serveRelease(t, fixture)

	code, stdout, stderr := runTestCLI(t, jira.NewClient("http://unused.example", "email", "token"), "upgrade", "--json")
	if code != 1 || stdout != "" {
		t.Fatalf("exit = %d, stdout = %q", code, stdout)
	}
	envelope := decodeMutationError(t, stderr)
	want := "refusing to install without confirmation; rerun with --yes or use --check"
	if envelope.Code != "confirmation_required" || envelope.Message != want {
		t.Fatalf("envelope = %#v, want code confirmation_required and message %q", envelope, want)
	}
	if envelope.MutationState != jira.MutationNotApplied {
		t.Fatalf("mutationState = %q, want not_applied", envelope.MutationState)
	}
	if got := atomic.LoadInt32(fixture.requests); got != 0 {
		t.Fatalf("refused upgrade made %d requests, want 0", got)
	}
}

func TestUpgradeInstallsTheVerifiedRelease(t *testing.T) {
	setTestVersion(t, "v0.5.0")
	installDir := t.TempDir()
	binary := filepath.Join(installDir, "gojira")
	if err := os.WriteFile(binary, []byte("current"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	useFakeInstallation(t, binary, installDir)
	fixture := &releaseFixture{tag: "v0.6.0", archive: newReleaseArchive(t, "replacement")}
	serveRelease(t, fixture)

	code, stdout, stderr := runTestCLI(t, jira.NewClient("http://unused.example", "email", "token"), "upgrade", "--yes", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	var report upgradeReport
	assertSingleJSONDocument(t, []byte(stdout), &report)
	if report.Action != "installed" || report.MutationState != jira.MutationApplied {
		t.Fatalf("report = %+v, want an installed release", report)
	}
	if !report.ChecksumVerified {
		t.Fatal("checksumVerified = false, want true for an installed release")
	}
	if report.ExecutablePath != binary || report.AssetName != testAssetName(t) {
		t.Fatalf("report = %+v, want the resolved binary and asset", report)
	}
	if got, err := os.ReadFile(binary); err != nil || string(got) != "replacement" {
		t.Fatalf("binary content = %q (err %v), want the replacement", got, err)
	}
	entries, err := os.ReadDir(installDir)
	if err != nil {
		t.Fatalf("read install dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(binary) {
		t.Fatalf("install dir holds %v, want only the binary", entries)
	}
}

func TestUpgradeReportsAnUpToDateInstallation(t *testing.T) {
	setTestVersion(t, "v0.6.0")
	installDir := t.TempDir()
	binary := filepath.Join(installDir, "gojira")
	if err := os.WriteFile(binary, []byte("current"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	useFakeInstallation(t, binary, installDir)
	serveRelease(t, &releaseFixture{tag: "v0.6.0", archive: newReleaseArchive(t, "next")})

	code, stdout, stderr := runTestCLI(t, jira.NewClient("http://unused.example", "email", "token"), "upgrade", "--yes")
	if code != 0 || stderr != "" {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if want := "✓ gojira v0.6.0 is the latest published release\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestUpgradeRefusesABinaryItDoesNotOwn(t *testing.T) {
	setTestVersion(t, "v0.5.0")
	installDir := t.TempDir()
	binary := filepath.Join(installDir, "gojira")
	if err := os.WriteFile(binary, []byte("current"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	useFakeInstallation(t, binary, filepath.Join(t.TempDir(), "elsewhere"))
	fixture := &releaseFixture{tag: "v0.6.0", archive: newReleaseArchive(t, "next")}
	serveRelease(t, fixture)

	code, stdout, stderr := runTestCLI(t, jira.NewClient("http://unused.example", "email", "token"), "upgrade", "--yes", "--json")
	if code != 1 || stdout != "" {
		t.Fatalf("exit = %d, stdout = %q", code, stdout)
	}
	envelope := decodeMutationError(t, stderr)
	want := "refusing to replace " + binary + ": not a gojira installer-owned location (set GOJIRA_INSTALL_DIR or run the installer)"
	if envelope.Code != "not_owned" || envelope.Message != want {
		t.Fatalf("envelope = %#v, want %q", envelope, want)
	}
	if envelope.MutationState != jira.MutationNotApplied {
		t.Fatalf("mutationState = %q, want not_applied", envelope.MutationState)
	}
	if got := atomic.LoadInt32(fixture.requests); got != 0 {
		t.Fatalf("refused upgrade made %d requests, want 0", got)
	}
	if got, err := os.ReadFile(binary); err != nil || string(got) != "current" {
		t.Fatalf("binary content = %q (err %v), want it untouched", got, err)
	}
}

func TestUpgradeRefusesADevelopmentBuild(t *testing.T) {
	setTestVersion(t, "dev")
	installDir := t.TempDir()
	binary := filepath.Join(installDir, "gojira")
	if err := os.WriteFile(binary, []byte("current"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	useFakeInstallation(t, binary, installDir)
	serveRelease(t, &releaseFixture{tag: "v0.6.0", archive: newReleaseArchive(t, "next")})

	code, _, stderr := runTestCLI(t, jira.NewClient("http://unused.example", "email", "token"), "upgrade", "--yes")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, `is a development build (version "dev")`) {
		t.Fatalf("stderr = %q, want the development build refusal", stderr)
	}
	if got, err := os.ReadFile(binary); err != nil || string(got) != "current" {
		t.Fatalf("binary content = %q (err %v), want it untouched", got, err)
	}
}

func TestUpgradeRefusesAnUnverifiedDownload(t *testing.T) {
	setTestVersion(t, "v0.5.0")
	installDir := t.TempDir()
	binary := filepath.Join(installDir, "gojira")
	if err := os.WriteFile(binary, []byte("current"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	useFakeInstallation(t, binary, installDir)
	serveRelease(t, &releaseFixture{
		tag:     "v0.6.0",
		archive: newReleaseArchive(t, "replacement"),
		digest:  strings.Repeat("f", 64),
	})

	code, stdout, stderr := runTestCLI(t, jira.NewClient("http://unused.example", "email", "token"), "upgrade", "--yes", "--json")
	if code != 1 || stdout != "" {
		t.Fatalf("exit = %d, stdout = %q", code, stdout)
	}
	envelope := decodeMutationError(t, stderr)
	if envelope.Code != "download_failed" || envelope.MutationState != jira.MutationNotApplied {
		t.Fatalf("envelope = %#v, want download_failed and not_applied", envelope)
	}
	if !strings.Contains(envelope.Message, "checksum mismatch") {
		t.Fatalf("message = %q, want a checksum mismatch explanation", envelope.Message)
	}
	if got, err := os.ReadFile(binary); err != nil || string(got) != "current" {
		t.Fatalf("binary content = %q (err %v), want it untouched", got, err)
	}
	entries, err := os.ReadDir(installDir)
	if err != nil {
		t.Fatalf("read install dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("install dir holds %v, want only the binary", entries)
	}
}

func TestUpgradeReportsAReleaseCheckFailure(t *testing.T) {
	setTestVersion(t, "v0.5.0")
	installDir := t.TempDir()
	binary := filepath.Join(installDir, "gojira")
	if err := os.WriteFile(binary, []byte("current"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	useFakeInstallation(t, binary, installDir)
	serveRelease(t, &releaseFixture{tag: "v0.6.0", archive: newReleaseArchive(t, "next"), status: http.StatusInternalServerError})

	code, stdout, stderr := runTestCLI(t, jira.NewClient("http://unused.example", "email", "token"), "upgrade", "--check", "--json")
	if code != 1 || stdout != "" {
		t.Fatalf("exit = %d, stdout = %q", code, stdout)
	}
	envelope := decodeMutationError(t, stderr)
	if envelope.Code != "release_check_failed" || envelope.MutationState != jira.MutationNotApplied {
		t.Fatalf("envelope = %#v, want release_check_failed and not_applied", envelope)
	}
	if !strings.HasPrefix(envelope.Message, "release check failed: ") || !strings.HasSuffix(envelope.Message, "; nothing was changed") {
		t.Fatalf("message = %q, want the release check failure contract", envelope.Message)
	}
}

func TestUpgradeCancelledByTheHumanChangesNothing(t *testing.T) {
	setTestVersion(t, "v0.5.0")
	installDir := t.TempDir()
	binary := filepath.Join(installDir, "gojira")
	if err := os.WriteFile(binary, []byte("current"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	useFakeInstallation(t, binary, installDir)
	useTerminal(t, true)
	useStdin(t, "n\n")
	fixture := &releaseFixture{tag: "v0.6.0", archive: newReleaseArchive(t, "next")}
	serveRelease(t, fixture)

	code, stdout, stderr := runTestCLI(t, jira.NewClient("http://unused.example", "email", "token"), "upgrade")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if want := "Upgrade cancelled; nothing was changed.\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	if want := "Install gojira v0.6.0 at " + binary + "? [y/N] "; stderr != want {
		t.Fatalf("stderr = %q, want the confirmation prompt %q", stderr, want)
	}
	if got := atomic.LoadInt32(fixture.downloads); got != 0 {
		t.Fatalf("cancelled upgrade performed %d downloads, want 0", got)
	}
	if got, err := os.ReadFile(binary); err != nil || string(got) != "current" {
		t.Fatalf("binary content = %q (err %v), want it untouched", got, err)
	}
}

func TestUpgradeRejectsArguments(t *testing.T) {
	setTestVersion(t, "v0.5.0")
	code, _, stderr := runTestCLI(t, jira.NewClient("http://unused.example", "email", "token"), "upgrade", "extra")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if stderr == "" {
		t.Fatal("stderr is empty, want an argument rejection")
	}
}
