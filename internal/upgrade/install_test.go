package upgrade

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeSystem returns a system whose effects are fully controlled by the test.
type fakeSystem struct {
	goos       string
	executable string
	env        map[string]string
	home       string
	renames    []string
	renameErr  func(oldpath, newpath string) error
	removeErr  func(path string) error
}

func (f *fakeSystem) system() System {
	return System{
		GOOS: f.goos,
		Executable: func() (string, error) {
			if f.executable == "" {
				return "", errors.New("no executable")
			}
			return f.executable, nil
		},
		Getenv: func(key string) string { return f.env[key] },
		HomeDir: func() (string, error) {
			if f.home == "" {
				return "", errors.New("no home")
			}
			return f.home, nil
		},
		Lstat: os.Lstat,
		Rename: func(oldpath, newpath string) error {
			f.renames = append(f.renames, oldpath+" -> "+newpath)
			if f.renameErr != nil {
				if err := f.renameErr(oldpath, newpath); err != nil {
					return err
				}
			}
			return os.Rename(oldpath, newpath)
		},
		Remove: func(path string) error {
			if f.removeErr != nil {
				if err := f.removeErr(path); err != nil {
					return err
				}
			}
			return os.Remove(path)
		},
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func TestTargetAcceptsInstallerOwnedLayouts(t *testing.T) {
	t.Run("GOJIRA_INSTALL_DIR", func(t *testing.T) {
		installDir := t.TempDir()
		binary := filepath.Join(installDir, "gojira")
		writeFile(t, binary, "old")

		fake := &fakeSystem{goos: "linux", executable: binary, env: map[string]string{"GOJIRA_INSTALL_DIR": installDir}}
		target, err := NewInstaller(fake.system()).Target()
		if err != nil {
			t.Fatalf("Target returned unexpected error: %v", err)
		}
		if target.Path != binary || target.Dir != installDir {
			t.Fatalf("Target = %+v, want path %q and dir %q", target, binary, installDir)
		}
	})

	t.Run("user local bin", func(t *testing.T) {
		home := t.TempDir()
		binary := filepath.Join(home, ".local", "bin", "gojira")
		writeFile(t, binary, "old")

		fake := &fakeSystem{goos: "darwin", executable: binary, home: home}
		if _, err := NewInstaller(fake.system()).Target(); err != nil {
			t.Fatalf("Target returned unexpected error for ~/.local/bin: %v", err)
		}
	})

	t.Run("windows programs dir", func(t *testing.T) {
		localAppData := t.TempDir()
		installDir := filepath.Join(localAppData, "Programs", "gojira")
		binary := filepath.Join(installDir, "gojira.exe")
		writeFile(t, binary, "old")

		fake := &fakeSystem{
			goos:       "windows",
			executable: binary,
			env:        map[string]string{"LOCALAPPDATA": localAppData},
		}
		if _, err := NewInstaller(fake.system()).Target(); err != nil {
			t.Fatalf("Target returned unexpected error for the Windows programs dir: %v", err)
		}
	})
}

func TestTargetRefusesUnownedOrUnsafeBinaries(t *testing.T) {
	t.Run("outside the installer layouts", func(t *testing.T) {
		home := t.TempDir()
		binary := filepath.Join(home, "go", "bin", "gojira")
		writeFile(t, binary, "old")

		fake := &fakeSystem{goos: "linux", executable: binary, home: home}
		_, err := NewInstaller(fake.system()).Target()
		if !errors.Is(err, ErrNotOwned) {
			t.Fatalf("Target error = %v, want ErrNotOwned", err)
		}
		want := "refusing to replace " + binary + ": not a gojira installer-owned location (set GOJIRA_INSTALL_DIR or run the installer)"
		if err.Error() != want {
			t.Fatalf("Target error = %q, want %q", err, want)
		}
	})

	t.Run("symbolic link", func(t *testing.T) {
		installDir := t.TempDir()
		real := filepath.Join(installDir, "gojira-real")
		writeFile(t, real, "old")
		link := filepath.Join(installDir, "gojira")
		if err := os.Symlink(real, link); err != nil {
			t.Fatalf("create symlink: %v", err)
		}

		fake := &fakeSystem{goos: "linux", executable: link, env: map[string]string{"GOJIRA_INSTALL_DIR": installDir}}
		_, err := NewInstaller(fake.system()).Target()
		if err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Fatalf("Target error = %v, want a symbolic link refusal", err)
		}
	})

	t.Run("symlinked install directory", func(t *testing.T) {
		parent := t.TempDir()
		realDir := filepath.Join(parent, "real-install")
		if err := os.Mkdir(realDir, 0o755); err != nil {
			t.Fatalf("create real directory: %v", err)
		}
		binary := filepath.Join(realDir, "gojira")
		writeFile(t, binary, "old")
		linkDir := filepath.Join(parent, "link-install")
		if err := os.Symlink(realDir, linkDir); err != nil {
			t.Fatalf("create symlinked directory: %v", err)
		}

		throughLink := filepath.Join(linkDir, "gojira")
		fake := &fakeSystem{goos: "linux", executable: throughLink, env: map[string]string{"GOJIRA_INSTALL_DIR": linkDir}}
		_, err := NewInstaller(fake.system()).Target()
		if err == nil {
			t.Fatal("Target accepted a symlinked install directory")
		}
		if errors.Is(err, ErrNotOwned) {
			t.Fatalf("Target error = %v, want a refusal independent of ErrNotOwned", err)
		}
		want := "refusing to replace " + throughLink + ": " + linkDir + " is a symbolic link"
		if err.Error() != want {
			t.Fatalf("Target error = %q, want %q", err, want)
		}
		if got := readFile(t, binary); got != "old" {
			t.Fatalf("binary content = %q, want the previous binary untouched", got)
		}
	})

	t.Run("LOCALAPPDATA outside Windows", func(t *testing.T) {
		localAppData := t.TempDir()
		binary := filepath.Join(localAppData, "Programs", "gojira", "gojira")
		writeFile(t, binary, "old")

		fake := &fakeSystem{goos: "linux", executable: binary, env: map[string]string{"LOCALAPPDATA": localAppData}}
		_, err := NewInstaller(fake.system()).Target()
		if !errors.Is(err, ErrNotOwned) {
			t.Fatalf("Target error = %v, want ErrNotOwned when LOCALAPPDATA is set on a non-Windows GOOS", err)
		}
	})

	t.Run("directory", func(t *testing.T) {
		installDir := t.TempDir()
		directory := filepath.Join(installDir, "gojira")
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatalf("create directory: %v", err)
		}

		fake := &fakeSystem{goos: "linux", executable: directory, env: map[string]string{"GOJIRA_INSTALL_DIR": installDir}}
		_, err := NewInstaller(fake.system()).Target()
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("Target error = %v, want a regular file refusal", err)
		}
	})
}

func TestLockRefusesConcurrentUpgrades(t *testing.T) {
	installDir := t.TempDir()
	installer := NewInstaller(OSSystem())

	lock, err := installer.Lock(installDir)
	if err != nil {
		t.Fatalf("Lock returned unexpected error: %v", err)
	}
	if _, err := installer.Lock(installDir); err == nil {
		t.Fatal("Lock accepted a concurrent upgrade")
	} else if !strings.Contains(err.Error(), "another upgrade is already running for "+installDir) {
		t.Fatalf("Lock error = %q, want the concurrent-upgrade message", err)
	} else {
		lockPath := filepath.Join(installDir, ".gojira.upgrade.lock")
		want := "another upgrade is already running for " + installDir + " (remove " + lockPath + " if no upgrade is running)"
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Lock error = %q, want it to name the lock file to remove: %q", err, want)
		}
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, ".gojira.upgrade.lock")); !os.IsNotExist(err) {
		t.Fatalf("lock file still exists after release: %v", err)
	}
	second, err := installer.Lock(installDir)
	if err != nil {
		t.Fatalf("Lock after release returned unexpected error: %v", err)
	}
	if err := second.Release(); err != nil {
		t.Fatalf("Release of the second lock returned unexpected error: %v", err)
	}
}

func TestPlaceAtomicallyRenamesTheStagedBinary(t *testing.T) {
	installDir := t.TempDir()
	destination := filepath.Join(installDir, "gojira")
	writeFile(t, destination, "old")
	staged := filepath.Join(installDir, ".gojira.new.123")
	writeFile(t, staged, "new")

	fake := &fakeSystem{goos: "linux"}
	placement, err := NewInstaller(fake.system()).Place(Target{Path: destination, Dir: installDir}, staged)
	if err != nil {
		t.Fatalf("Place returned unexpected error: %v", err)
	}
	if placement.Outcome != OutcomeApplied {
		t.Fatalf("Outcome = %q, want %q", placement.Outcome, OutcomeApplied)
	}
	if placement.PendingPath != "" || placement.LeftoverPath != "" {
		t.Fatalf("Placement = %+v, want no pending or leftover path", placement)
	}
	if got := readFile(t, destination); got != "new" {
		t.Fatalf("destination content = %q, want the staged binary", got)
	}
	if _, err := os.Stat(staged); !os.IsNotExist(err) {
		t.Fatalf("staged file still exists after placement: %v", err)
	}
}

func TestPlaceKeepsTheDestinationWhenTheRenameFails(t *testing.T) {
	installDir := t.TempDir()
	destination := filepath.Join(installDir, "gojira")
	writeFile(t, destination, "old")
	staged := filepath.Join(installDir, ".gojira.new.123")
	writeFile(t, staged, "new")

	fake := &fakeSystem{goos: "linux", renameErr: func(oldpath, newpath string) error {
		return errors.New("permission denied")
	}}
	placement, err := NewInstaller(fake.system()).Place(Target{Path: destination, Dir: installDir}, staged)
	if err == nil {
		t.Fatal("Place accepted a failed rename")
	}
	if placement.Outcome != OutcomeNotApplied {
		t.Fatalf("Outcome = %q, want %q", placement.Outcome, OutcomeNotApplied)
	}
	if placement.PendingPath != staged {
		t.Fatalf("PendingPath = %q, want the staged binary %q", placement.PendingPath, staged)
	}
	if got := readFile(t, destination); got != "old" {
		t.Fatalf("destination content = %q, want the previous binary", got)
	}
	if got := readFile(t, staged); got != "new" {
		t.Fatalf("staged content = %q, want the new binary preserved", got)
	}
}

func TestPlaceOnWindowsRenamesTheRunningImageAside(t *testing.T) {
	installDir := t.TempDir()
	destination := filepath.Join(installDir, "gojira.exe")
	writeFile(t, destination, "old")
	staged := filepath.Join(installDir, ".gojira.new.123")
	writeFile(t, staged, "new")

	fake := &fakeSystem{goos: "windows"}
	placement, err := NewInstaller(fake.system()).Place(Target{Path: destination, Dir: installDir}, staged)
	if err != nil {
		t.Fatalf("Place returned unexpected error: %v", err)
	}
	if placement.Outcome != OutcomeApplied {
		t.Fatalf("Outcome = %q, want %q", placement.Outcome, OutcomeApplied)
	}
	if got := readFile(t, destination); got != "new" {
		t.Fatalf("destination content = %q, want the new binary", got)
	}
	if len(fake.renames) != 2 {
		t.Fatalf("recorded %d renames, want the two-step swap: %v", len(fake.renames), fake.renames)
	}
	if !strings.HasPrefix(fake.renames[0], destination+" -> "+destination+".old.") {
		t.Fatalf("first rename = %q, want the running image moved aside", fake.renames[0])
	}
}

func TestPlaceOnWindowsReportsTheLeftoverItCannotRemove(t *testing.T) {
	installDir := t.TempDir()
	destination := filepath.Join(installDir, "gojira.exe")
	writeFile(t, destination, "old")
	staged := filepath.Join(installDir, ".gojira.new.123")
	writeFile(t, staged, "new")

	fake := &fakeSystem{goos: "windows", removeErr: func(path string) error {
		return errors.New("file is in use")
	}}
	placement, err := NewInstaller(fake.system()).Place(Target{Path: destination, Dir: installDir}, staged)
	if err != nil {
		t.Fatalf("Place returned unexpected error: %v", err)
	}
	if placement.Outcome != OutcomeApplied {
		t.Fatalf("Outcome = %q, want %q", placement.Outcome, OutcomeApplied)
	}
	if !strings.HasPrefix(placement.LeftoverPath, destination+".old.") {
		t.Fatalf("LeftoverPath = %q, want the moved-aside binary", placement.LeftoverPath)
	}
	if _, err := os.Stat(placement.LeftoverPath); err != nil {
		t.Fatalf("leftover binary is missing: %v", err)
	}
}

func TestPlaceOnWindowsRollsBackWhenTheNewBinaryCannotBePlaced(t *testing.T) {
	installDir := t.TempDir()
	destination := filepath.Join(installDir, "gojira.exe")
	writeFile(t, destination, "old")
	staged := filepath.Join(installDir, ".gojira.new.123")
	writeFile(t, staged, "new")

	fake := &fakeSystem{goos: "windows", renameErr: func(oldpath, newpath string) error {
		if oldpath == staged {
			return errors.New("file is in use")
		}
		return nil
	}}
	placement, err := NewInstaller(fake.system()).Place(Target{Path: destination, Dir: installDir}, staged)
	if err == nil {
		t.Fatal("Place accepted a failed placement")
	}
	if placement.Outcome != OutcomeNotApplied {
		t.Fatalf("Outcome = %q, want %q", placement.Outcome, OutcomeNotApplied)
	}
	if got := readFile(t, destination); got != "old" {
		t.Fatalf("destination content = %q, want the previous binary restored", got)
	}
	if got := readFile(t, staged); got != "new" {
		t.Fatalf("staged content = %q, want the new binary preserved", got)
	}
}

func TestPlaceOnWindowsReportsUnknownWhenRollbackAlsoFails(t *testing.T) {
	installDir := t.TempDir()
	destination := filepath.Join(installDir, "gojira.exe")
	writeFile(t, destination, "old")
	staged := filepath.Join(installDir, ".gojira.new.123")
	writeFile(t, staged, "new")

	fake := &fakeSystem{goos: "windows", renameErr: func(oldpath, newpath string) error {
		if oldpath == staged || strings.Contains(oldpath, ".old.") {
			return errors.New("file is in use")
		}
		return nil
	}}
	placement, err := NewInstaller(fake.system()).Place(Target{Path: destination, Dir: installDir}, staged)
	if err == nil {
		t.Fatal("Place accepted a failed placement and rollback")
	}
	if placement.Outcome != OutcomeUnknown {
		t.Fatalf("Outcome = %q, want %q", placement.Outcome, OutcomeUnknown)
	}
	if !strings.Contains(err.Error(), "verify") {
		t.Fatalf("error %q does not tell the user to verify the state", err)
	}
}

func TestCleanLeftoversRemovesStalePreviousAndStagedFiles(t *testing.T) {
	installDir := t.TempDir()
	destination := filepath.Join(installDir, "gojira.exe")
	writeFile(t, destination, "current")
	removable := destination + ".old.aaaa"
	writeFile(t, removable, "previous")
	locked := destination + ".old.bbbb"
	writeFile(t, locked, "previous")
	staleStaged := filepath.Join(installDir, ".gojira.new.aaaa")
	writeFile(t, staleStaged, "staged")
	lockedStaged := filepath.Join(installDir, ".gojira.new.bbbb")
	writeFile(t, lockedStaged, "staged")

	fake := &fakeSystem{goos: "windows", removeErr: func(path string) error {
		if path == locked || path == lockedStaged {
			return errors.New("file is in use")
		}
		return nil
	}}
	remaining := NewInstaller(fake.system()).CleanLeftovers(destination)
	wantRemaining := map[string]bool{locked: true, lockedStaged: true}
	if len(remaining) != len(wantRemaining) {
		t.Fatalf("CleanLeftovers = %v, want the unremovable %v", remaining, wantRemaining)
	}
	for _, path := range remaining {
		if !wantRemaining[path] {
			t.Fatalf("CleanLeftovers = %v, unexpected entry %q", remaining, path)
		}
	}
	for _, gone := range []string{removable, staleStaged} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Fatalf("leftover still exists: %s", gone)
		}
	}
	if got := readFile(t, destination); got != "current" {
		t.Fatalf("destination content = %q, want the current binary untouched", got)
	}
}
