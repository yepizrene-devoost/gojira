package upgrade

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func execCommand(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}

// Issue #55: on macOS, a binary a non-notarized app downloads carries the
// quarantine-family xattrs, and the quarantine + provenance pair makes
// Gatekeeper kill the binary on launch (SIGKILL, repro: exit 137 with both,
// exit 0 with quarantine removed). Place must clear the removable quarantine
// attribute from the staged binary before the rename — provenance is
// immutable on macOS 26 and harmless alone — so the placed file and any
// in-place relaunch inherit a launchable binary. The clearing is best-effort:
// a failure never fails the install. Outside macOS it is a no-op.

func setXattr(t *testing.T, path, name, value string) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("xattrs under test are a macOS behavior")
	}
	if out, err := execCommand("xattr", "-w", name, value, path); err != nil {
		t.Fatalf("setting %s on %s: %v %s", name, path, err, out)
	}
}

func hasXattr(t *testing.T, path, name string) bool {
	t.Helper()
	// xattr -p exits 1 and prints "No such xattr" when absent; any other
	// outcome means the attribute is present.
	_, err := execCommand("xattr", "-p", name, path)
	return err == nil
}

func TestPlaceClearsMacOSLaunchBlockers(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launch-blocker clearing is a macOS behavior")
	}
	installDir := t.TempDir()
	destination := filepath.Join(installDir, "gojira")
	writeFile(t, destination, "old")
	staged := filepath.Join(installDir, ".gojira.new.123")
	writeFile(t, staged, "new")
	setXattr(t, staged, "com.apple.quarantine", "0081;0000;/usr/bin/curl;")

	fake := &fakeSystem{goos: "darwin"}
	placement, err := NewInstaller(fake.system()).Place(Target{Path: destination, Dir: installDir}, staged)
	if err != nil {
		t.Fatalf("Place returned unexpected error: %v", err)
	}
	if placement.Outcome != OutcomeApplied {
		t.Fatalf("Outcome = %q, want %q", placement.Outcome, OutcomeApplied)
	}
	if hasXattr(t, destination, "com.apple.quarantine") {
		t.Fatal("placed binary still carries com.apple.quarantine; with provenance present Gatekeeper would kill it")
	}
}

// A clearing failure must not fail the install: the binary is already usable,
// and the user can clear the attribute by hand.
func TestPlaceToleratesClearingFailure(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launch-blocker clearing is a macOS behavior")
	}
	installDir := t.TempDir()
	destination := filepath.Join(installDir, "gojira")
	writeFile(t, destination, "old")
	staged := filepath.Join(installDir, ".gojira.new.123")
	writeFile(t, staged, "new")

	fake := &fakeSystem{goos: "darwin", clearErr: func(path string) error {
		return os.ErrPermission
	}}
	placement, err := NewInstaller(fake.system()).Place(Target{Path: destination, Dir: installDir}, staged)
	if err != nil {
		t.Fatalf("Place failed on a clearing error: %v", err)
	}
	if placement.Outcome != OutcomeApplied {
		t.Fatalf("Outcome = %q, want %q", placement.Outcome, OutcomeApplied)
	}
}

// Outside macOS the clearing must not run at all.
func TestPlaceSkipsClearingOutsideMacOS(t *testing.T) {
	goos := runtime.GOOS
	if goos == "darwin" {
		t.Skip("this test asserts the no-op on non-macOS platforms")
	}
	installDir := t.TempDir()
	destination := filepath.Join(installDir, "gojira")
	writeFile(t, destination, "old")
	staged := filepath.Join(installDir, ".gojira.new.123")
	writeFile(t, staged, "new")

	fake := &fakeSystem{goos: goos, clearCalls: new(int)}
	placement, err := NewInstaller(fake.system()).Place(Target{Path: destination, Dir: installDir}, staged)
	if err != nil || placement.Outcome != OutcomeApplied {
		t.Fatalf("Place = %+v, %v", placement, err)
	}
	if *fake.clearCalls != 0 {
		t.Fatalf("clear ran %d times outside macOS", *fake.clearCalls)
	}
}
