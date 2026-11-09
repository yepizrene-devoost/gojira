package tui

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
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yepizrene-devoost/gojira/internal/upgrade"
)

// installSeams points the S8 install seams at test doubles and restores the
// originals afterwards. A nil fetcher keeps the current fetcher seam.
func installSeams(t *testing.T, installer func() *upgrade.Installer, baseURL string, fetcher func() *upgrade.Fetcher) {
	t.Helper()
	prevInstaller, prevBase, prevFetcher := upgradeInstaller, upgradeBaseURL, upgradeFetcher
	upgradeInstaller, upgradeBaseURL = installer, baseURL
	if fetcher != nil {
		upgradeFetcher = fetcher
	}
	t.Cleanup(func() {
		upgradeInstaller, upgradeBaseURL, upgradeFetcher = prevInstaller, prevBase, prevFetcher
	})
}

// writeTestBinary creates the running binary a fake installer resolves.
func writeTestBinary(t *testing.T, dir string) string {
	t.Helper()
	binary := filepath.Join(dir, "gojira")
	if err := os.WriteFile(binary, []byte("old gojira"), 0o755); err != nil {
		t.Fatalf("write test binary: %v", err)
	}
	return binary
}

// ownedInstaller builds an installer seam whose running binary sits inside an
// installer-owned directory, so Target, Lock and Place run against the real
// temp dir without touching the actual executable.
func ownedInstaller(t *testing.T, dir, goos string) func() *upgrade.Installer {
	t.Helper()
	binary := writeTestBinary(t, dir)
	return func() *upgrade.Installer {
		return upgrade.NewInstaller(upgrade.System{
			GOOS:       goos,
			Executable: func() (string, error) { return binary, nil },
			Getenv: func(key string) string {
				if key == "GOJIRA_INSTALL_DIR" {
					return dir
				}
				return ""
			},
			HomeDir: func() (string, error) { return dir, nil },
			Lstat:   os.Lstat,
			Rename:  os.Rename,
			Remove:  os.Remove,
		})
	}
}

// releaseServer serves a fake latest release whose archive is real, so the
// whole install pipeline runs locally without network access.
func releaseServer(t *testing.T, archive []byte, checksumStatus int) *httptest.Server {
	t.Helper()
	assetName, err := upgrade.CurrentArtifactName()
	if err != nil {
		t.Fatalf("CurrentArtifactName: %v", err)
	}
	mux := http.NewServeMux()
	var server *httptest.Server
	mux.HandleFunc("/repos/yepizrene-devoost/gojira/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"tag_name": "v0.6.0",
			"assets": []map[string]any{
				{"name": assetName, "browser_download_url": server.URL + "/archive"},
				{"name": "checksums.txt", "browser_download_url": server.URL + "/checksums"},
			},
		}
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			t.Errorf("encode release: %v", err)
		}
	})
	mux.HandleFunc("/checksums", func(w http.ResponseWriter, _ *http.Request) {
		if checksumStatus != http.StatusOK {
			w.WriteHeader(checksumStatus)
			return
		}
		sum := sha256.Sum256(archive)
		if _, err := fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), assetName); err != nil {
			t.Errorf("write checksums: %v", err)
		}
	})
	mux.HandleFunc("/archive", func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write(archive); err != nil {
			t.Errorf("write archive: %v", err)
		}
	})
	server = httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// countingServer records every release-check hit so "no network" refusals can
// assert that nothing was requested.
func countingServer(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	return server, &hits
}

// releaseArchive builds a tar.gz containing the release binary, the layout
// DownloadVerified extracts.
func releaseArchive(t *testing.T, body string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	payload := []byte(body)
	if err := tarWriter.WriteHeader(&tar.Header{Name: "gojira", Mode: 0o755, Size: int64(len(payload))}); err != nil {
		t.Fatalf("write archive header: %v", err)
	}
	if _, err := tarWriter.Write(payload); err != nil {
		t.Fatalf("write archive body: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buffer.Bytes()
}

// settleUpgrade runs the async install chain to its terminal message, the way
// the Bubble Tea runtime would.
func settleUpgrade(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	for step := 0; cmd != nil; step++ {
		if step > 20 {
			t.Fatal("install chain did not settle within 20 messages")
		}
		m, cmd = updateModel(t, m, cmd())
	}
	return m
}

// S8 happy path: Update now runs the existing pipeline to a completed
// placement — the binary on disk is replaced, the outcome is reported with
// both versions, the S10 status line clears, the recorder persists the
// version and the modal closes.
func TestUpgradeNowInstallsAndReportsSuccess(t *testing.T) {
	dir := t.TempDir()
	srv := releaseServer(t, releaseArchive(t, "new gojira binary"), http.StatusOK)
	installSeams(t, ownedInstaller(t, dir, runtime.GOOS), srv.URL, nil)

	recorded := ""
	m := promptModel(t)
	m.updateRecorder = func(latest string) { recorded = latest }
	m = openPrompt(t, m, "v0.6.0")

	m, cmd := updateModel(t, m, textKey("u"))
	m = settleUpgrade(t, m, cmd)

	plain := plainView(m)
	if strings.Contains(plain, "Keep current version") {
		t.Fatalf("prompt still open after a successful install:\n%s", plain)
	}
	if m.updateUpgrading {
		t.Fatal("install still marked busy after success")
	}
	if want := "Installed gojira v0.5.0 → v0.6.0"; !strings.Contains(m.toast, want) {
		t.Fatalf("toast = %q, want it to contain %q", m.toast, want)
	}
	if !strings.Contains(m.toast, "restart") || !strings.Contains(m.toast, "keeps v0.5.0") {
		t.Fatalf("toast must say plainly that a restart is needed and the session keeps the old version: %q", m.toast)
	}
	if m.updateAvailable != "" {
		t.Fatalf("updateAvailable = %q, want the S10 status line state cleared", m.updateAvailable)
	}
	if plain := plainView(m); strings.Contains(plain, "⬆ GoJira v0.6.0 is available") {
		t.Fatalf("status line must clear after the install:\n%s", plain)
	}
	if recorded != "v0.6.0" {
		t.Fatalf("recorder = %q, want the installed version persisted", recorded)
	}
	installed, err := os.ReadFile(filepath.Join(dir, "gojira"))
	if err != nil || string(installed) != "new gojira binary" {
		t.Fatalf("installed binary = %q, %v; want the staged release", installed, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gojira.upgrade.lock")); !os.IsNotExist(err) {
		t.Fatalf("upgrade lock left behind: %v", err)
	}
}

// S8 refusal: a binary outside the installer-owned directories surfaces the
// installer's exact text inside the modal before any network request happens.
func TestUpgradeRefusesForeignBinaryBeforeAnyNetwork(t *testing.T) {
	ownedDir, foreignDir := t.TempDir(), t.TempDir()
	binary := writeTestBinary(t, foreignDir)
	installer := func() *upgrade.Installer {
		return upgrade.NewInstaller(upgrade.System{
			GOOS:       runtime.GOOS,
			Executable: func() (string, error) { return binary, nil },
			Getenv: func(key string) string {
				if key == "GOJIRA_INSTALL_DIR" {
					return ownedDir
				}
				return ""
			},
			HomeDir: func() (string, error) { return foreignDir, nil },
			Lstat:   os.Lstat,
			Rename:  os.Rename,
			Remove:  os.Remove,
		})
	}
	srv, hits := countingServer(t)
	installSeams(t, installer, srv.URL, nil)

	m := openPrompt(t, promptModel(t), "v0.6.0")
	m, cmd := updateModel(t, m, textKey("u"))
	m = settleUpgrade(t, m, cmd)

	flat := flatView(m)
	if !strings.Contains(flat, "not a gojira installer-owned location") {
		t.Fatalf("modal does not carry the installer's ownership refusal:\n%s", flat)
	}
	if !strings.Contains(flat, "Update now") {
		t.Fatalf("refusal must render inside the still-open modal:\n%s", flat)
	}
	if *hits != 0 {
		t.Fatalf("ownership refusal made %d network requests, want 0", *hits)
	}
	if m.updateUpgrading {
		t.Fatal("install still marked busy after the refusal")
	}
}

// S8 refusal: a held upgrade lock surfaces the installer's ErrLocked text
// inside the modal.
func TestUpgradeSurfacesLockedDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gojira.upgrade.lock"), []byte("1\n"), 0o600); err != nil {
		t.Fatalf("pre-create lock: %v", err)
	}
	srv := releaseServer(t, nil, http.StatusOK)
	installSeams(t, ownedInstaller(t, dir, runtime.GOOS), srv.URL, nil)

	m := openPrompt(t, promptModel(t), "v0.6.0")
	m, cmd := updateModel(t, m, textKey("u"))
	m = settleUpgrade(t, m, cmd)

	flat := flatView(m)
	if !strings.Contains(flat, "upgrade already running") {
		t.Fatalf("modal does not carry the lock refusal:\n%s", flat)
	}
	if !strings.Contains(flat, "Update now") {
		t.Fatalf("refusal must render inside the still-open modal:\n%s", flat)
	}
	if m.updateUpgrading {
		t.Fatal("install still marked busy after the refusal")
	}
}

// S8 refusal: a development build is refused with the installer's exact
// message before any network request, mirroring cmd/upgrade.go's stage order.
func TestUpgradeSurfacesDevBuildRefusalBeforeNetwork(t *testing.T) {
	dir := t.TempDir()
	srv, hits := countingServer(t)
	installSeams(t, ownedInstaller(t, dir, runtime.GOOS), srv.URL, nil)

	m := promptModel(t).WithVersion("dev")
	m = openPrompt(t, m, "v0.6.0")
	m, cmd := updateModel(t, m, textKey("u"))
	m = settleUpgrade(t, m, cmd)

	flat := flatView(m)
	if !strings.Contains(flat, fmt.Sprintf("is a development build (version %q)", "dev")) {
		t.Fatalf("modal does not carry the dev-build refusal:\n%s", flat)
	}
	if !strings.Contains(flat, "scripts/install.sh") {
		t.Fatalf("dev-build refusal must keep the installer's instructions:\n%s", flat)
	}
	if *hits != 0 {
		t.Fatalf("dev-build refusal made %d network requests, want 0", *hits)
	}
	if m.updateUpgrading {
		t.Fatal("install still marked busy after the refusal")
	}
}

// S8 failure: a broken download surfaces "download failed … nothing was
// changed" and releases the upgrade lock it acquired.
func TestUpgradeSurfacesDownloadFailureUnchanged(t *testing.T) {
	dir := t.TempDir()
	srv := releaseServer(t, nil, http.StatusOK)
	failingFetcher := func() *upgrade.Fetcher {
		return upgrade.NewFetcher(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network down")
		})})
	}
	installSeams(t, ownedInstaller(t, dir, runtime.GOOS), srv.URL, failingFetcher)

	m := openPrompt(t, promptModel(t), "v0.6.0")
	m, cmd := updateModel(t, m, textKey("u"))
	m = settleUpgrade(t, m, cmd)

	flat := flatView(m)
	if !strings.Contains(flat, "download failed:") {
		t.Fatalf("modal does not carry the download failure:\n%s", flat)
	}
	if !strings.Contains(flat, "nothing was changed") {
		t.Fatalf("download failure must state that nothing was changed:\n%s", flat)
	}
	if m.updateUpgrading {
		t.Fatal("install still marked busy after the failure")
	}
	if _, err := os.Stat(filepath.Join(dir, ".gojira.upgrade.lock")); !os.IsNotExist(err) {
		t.Fatalf("upgrade lock left behind after the failure: %v", err)
	}
}

// roundTripFunc adapts a function to http.RoundTripper for seam tests.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// S8 honesty: an uncertain placement surfaces the mutation-unknown wording
// instead of claiming either result.
func TestUpgradeSurfacesPlacementUnknownHonesty(t *testing.T) {
	dir := t.TempDir()
	binary := writeTestBinary(t, dir)
	renames := 0
	installer := func() *upgrade.Installer {
		return upgrade.NewInstaller(upgrade.System{
			// The Windows sequence is the only one that can report unknown:
			// the running image moves aside, placement fails and the rollback
			// fails too.
			GOOS:       "windows",
			Executable: func() (string, error) { return binary, nil },
			Getenv: func(key string) string {
				if key == "GOJIRA_INSTALL_DIR" {
					return dir
				}
				return ""
			},
			HomeDir: func() (string, error) { return dir, nil },
			Lstat:   os.Lstat,
			Rename: func(oldpath, newpath string) error {
				renames++
				if renames == 1 {
					return os.Rename(oldpath, newpath)
				}
				return errors.New("rename refused")
			},
			Remove: os.Remove,
		})
	}
	srv := releaseServer(t, releaseArchive(t, "new gojira binary"), http.StatusOK)
	installSeams(t, installer, srv.URL, nil)

	m := openPrompt(t, promptModel(t), "v0.6.0")
	m, cmd := updateModel(t, m, textKey("u"))
	m = settleUpgrade(t, m, cmd)

	flat := flatView(m)
	for _, want := range []string{"may have been applied", "check before retrying"} {
		if !strings.Contains(flat, want) {
			t.Fatalf("modal does not carry the mutation-unknown honesty %q:\n%s", want, flat)
		}
	}
	if m.updateUpgrading {
		t.Fatal("install still marked busy after the uncertain placement")
	}
	if !strings.Contains(flat, "Update now") {
		t.Fatalf("uncertain placement must render inside the still-open modal:\n%s", flat)
	}
}

// S8 busy state: the modal reports progress while the pipeline runs, says that
// esc cannot cancel it, and repeated u/enter submissions are no-ops.
func TestUpgradeBusyReportsProgressAndIgnoresRepeatSubmission(t *testing.T) {
	m := openPrompt(t, promptModel(t), "v0.6.0")
	m, cmd := updateModel(t, m, textKey("u"))

	if !m.updateUpgrading {
		t.Fatal("u did not mark the upgrade busy")
	}
	flat := flatView(m)
	if !strings.Contains(flat, updateStageChecking) {
		t.Fatalf("busy modal does not report progress:\n%s", flat)
	}
	if !strings.Contains(flat, "cannot be cancelled") {
		t.Fatalf("busy modal must say that esc cannot cancel an install in flight:\n%s", flat)
	}
	if strings.Contains(flat, "Keep current version") {
		t.Fatalf("busy modal must show progress instead of the options:\n%s", flat)
	}
	if cmd == nil {
		t.Fatal("u produced no upgrade command")
	}
	request := m.activeUpdateRequest

	again, secondCmd := updateModel(t, m, textKey("u"))
	if secondCmd != nil {
		t.Fatal("repeated u while busy produced another command")
	}
	entered, thirdCmd := updateModel(t, again, specialKey(tea.KeyEnter))
	if thirdCmd != nil {
		t.Fatal("enter while busy produced another command")
	}
	if entered.activeUpdateRequest != request {
		t.Fatalf("request id = %d after re-submissions, want the original %d", entered.activeUpdateRequest, request)
	}
	if !entered.updateUpgrading {
		t.Fatal("re-submissions disturbed the busy install")
	}
}

// S8 esc: while the install runs, esc dismisses the modal without pretending
// to cancel the HTTP mutation already in flight — the pipeline still finishes
// and reports its outcome.
func TestUpgradeEscWhileBusyDismissesWithoutCancelling(t *testing.T) {
	dir := t.TempDir()
	srv := releaseServer(t, releaseArchive(t, "new gojira binary"), http.StatusOK)
	installSeams(t, ownedInstaller(t, dir, runtime.GOOS), srv.URL, nil)

	recorded := ""
	m := promptModel(t)
	m.updateRecorder = func(latest string) { recorded = latest }
	m = openPrompt(t, m, "v0.6.0")

	m, cmd := updateModel(t, m, textKey("u"))
	if cmd == nil {
		t.Fatal("u produced no upgrade command")
	}
	if plain := plainView(m); !strings.Contains(plain, updateStageChecking) {
		t.Fatalf("upgrade did not start as busy:\n%s", plain)
	}

	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	if strings.Contains(plainView(m), "Keep current version") {
		t.Fatal("esc did not dismiss the modal while busy")
	}
	if !m.updateUpgrading {
		t.Fatal("esc must not pretend to cancel the install already in flight")
	}

	m = settleUpgrade(t, m, cmd)
	if want := "Installed gojira v0.5.0 → v0.6.0"; !strings.Contains(m.toast, want) {
		t.Fatalf("toast = %q, want it to contain %q", m.toast, want)
	}
	if m.updateAvailable != "" {
		t.Fatalf("updateAvailable = %q, want the status line resolved by the finished install", m.updateAvailable)
	}
	if recorded != "v0.6.0" {
		t.Fatalf("recorder = %q, want the installed version persisted", recorded)
	}
}
