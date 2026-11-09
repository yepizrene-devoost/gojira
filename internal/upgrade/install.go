package upgrade

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Outcome is the reported result of installing a staged binary, using the same
// vocabulary as the Jira mutation contract.
type Outcome string

const (
	// OutcomeApplied means the new binary is in place.
	OutcomeApplied Outcome = "applied"
	// OutcomeNotApplied means nothing on disk changed.
	OutcomeNotApplied Outcome = "not_applied"
	// OutcomeUnknown means the change may or may not be complete.
	OutcomeUnknown Outcome = "unknown"
)

// ErrNotOwned reports a binary located outside the directories the installers
// own. Such a binary is never replaced.
var ErrNotOwned = errors.New("not a gojira installer-owned location")

// ErrLocked reports a concurrent upgrade of the same directory.
var ErrLocked = errors.New("upgrade already running")

const lockFileName = ".gojira.upgrade.lock"

// OwnershipError refuses to replace a path GoJira does not own.
type OwnershipError struct {
	Path string
}

func (e *OwnershipError) Error() string {
	return "refusing to replace " + e.Path + ": not a gojira installer-owned location (set GOJIRA_INSTALL_DIR or run the installer)"
}

// Is lets errors.Is match ErrNotOwned without losing the explanatory message.
func (e *OwnershipError) Is(target error) bool { return target == ErrNotOwned }

// System bundles the operating-system effects of an upgrade so tests can drive
// the Windows sequence on any platform.
type System struct {
	GOOS       string
	Executable func() (string, error)
	Getenv     func(string) string
	HomeDir    func() (string, error)
	Lstat      func(string) (os.FileInfo, error)
	Rename     func(oldpath, newpath string) error
	Remove     func(path string) error
}

// OSSystem returns the real operating-system effects.
func OSSystem() System {
	return System{
		GOOS:       runtime.GOOS,
		Executable: os.Executable,
		Getenv:     os.Getenv,
		HomeDir:    os.UserHomeDir,
		Lstat:      os.Lstat,
		Rename:     os.Rename,
		Remove:     os.Remove,
	}
}

// Target is the resolved binary an upgrade would replace.
type Target struct {
	Path string
	Dir  string
}

// Placement reports what happened to a staged binary.
type Placement struct {
	Outcome      Outcome
	PendingPath  string
	LeftoverPath string
}

// Installer replaces installer-owned binaries.
type Installer struct {
	system System
}

// NewInstaller returns an installer using system.
func NewInstaller(system System) *Installer {
	return &Installer{system: system}
}

// Target resolves the running binary and refuses anything outside the
// installer-owned layouts: package-manager paths, go install locations,
// symlinks and non-regular files are never replaced.
func (i *Installer) Target() (Target, error) {
	executable, err := i.system.Executable()
	if err != nil {
		return Target{}, fmt.Errorf("locate the running gojira binary: %w", err)
	}
	return i.TargetFor(executable)
}

// TargetFor resolves one explicit binary path with the same ownership rules.
func (i *Installer) TargetFor(executable string) (Target, error) {
	absolute, err := filepath.Abs(executable)
	if err != nil {
		return Target{}, fmt.Errorf("resolve %s: %w", executable, err)
	}
	info, err := i.system.Lstat(absolute)
	if err != nil {
		return Target{}, fmt.Errorf("inspect %s: %w", absolute, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return Target{}, fmt.Errorf("refusing to replace %s: it is a symbolic link", absolute)
	}
	if !info.Mode().IsRegular() {
		return Target{}, fmt.Errorf("refusing to replace %s: it is not a regular file", absolute)
	}
	dir := filepath.Dir(absolute)
	dirInfo, err := i.system.Lstat(dir)
	if err != nil {
		return Target{}, fmt.Errorf("inspect %s: %w", dir, err)
	}
	if dirInfo.Mode()&os.ModeSymlink != 0 {
		return Target{}, fmt.Errorf("refusing to replace %s: %s is a symbolic link", absolute, dir)
	}
	if !i.ownsDirectory(dir) {
		return Target{}, &OwnershipError{Path: absolute}
	}
	return Target{Path: absolute, Dir: dir}, nil
}

func (i *Installer) ownsDirectory(dir string) bool {
	for _, owned := range i.ownedDirectories() {
		if sameDirectory(owned, dir) {
			return true
		}
	}
	return false
}

func (i *Installer) ownedDirectories() []string {
	owned := make([]string, 0, 3)
	if installDir := i.system.Getenv("GOJIRA_INSTALL_DIR"); installDir != "" {
		owned = append(owned, installDir)
	}
	if home, err := i.system.HomeDir(); err == nil && home != "" {
		owned = append(owned, filepath.Join(home, ".local", "bin"))
	}
	if i.system.GOOS == "windows" {
		if localAppData := i.system.Getenv("LOCALAPPDATA"); localAppData != "" {
			owned = append(owned, filepath.Join(localAppData, "Programs", "gojira"))
		}
	}
	return owned
}

func sameDirectory(left, right string) bool {
	leftAbsolute, leftErr := filepath.Abs(left)
	rightAbsolute, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return filepath.Clean(left) == filepath.Clean(right)
	}
	return filepath.Clean(leftAbsolute) == filepath.Clean(rightAbsolute)
}

// Lock takes the exclusive upgrade lock of dir. The lock is a file created
// exclusively, so two upgrades of the same installation cannot interleave.
func (i *Installer) Lock(dir string) (*Lock, error) {
	path := filepath.Join(dir, lockFileName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("%w: another upgrade is already running for %s (remove %s if no upgrade is running)", ErrLocked, dir, path)
		}
		return nil, fmt.Errorf("lock %s: %w", dir, err)
	}
	_, _ = fmt.Fprintf(file, "%d\n", os.Getpid())
	return &Lock{path: path, file: file}, nil
}

// Lock is a held upgrade lock.
type Lock struct {
	path string
	file *os.File
}

// Release drops the lock. Releasing twice is harmless.
func (l *Lock) Release() error {
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
	if err := os.Remove(l.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("release upgrade lock: %w", err)
	}
	return nil
}

// Place installs a staged binary over target. On Unix a single rename is
// atomic. On Windows the running image is first moved aside, so the destination
// is never left missing: a failed step is rolled back, and an unremovable
// previous binary is reported instead of hidden.
func (i *Installer) Place(target Target, staged string) (Placement, error) {
	if i.system.GOOS == "windows" {
		return i.placeWindows(target, staged)
	}
	if err := i.system.Rename(staged, target.Path); err != nil {
		return Placement{Outcome: OutcomeNotApplied, PendingPath: staged},
			fmt.Errorf("replace %s: %w", target.Path, err)
	}
	return Placement{Outcome: OutcomeApplied}, nil
}

func (i *Installer) placeWindows(target Target, staged string) (Placement, error) {
	backup := target.Path + ".old." + randomSuffix()
	if err := i.system.Rename(target.Path, backup); err != nil {
		return Placement{Outcome: OutcomeNotApplied, PendingPath: staged},
			fmt.Errorf("move the running binary at %s aside: %w", target.Path, err)
	}
	if err := i.system.Rename(staged, target.Path); err != nil {
		if rollbackErr := i.system.Rename(backup, target.Path); rollbackErr != nil {
			return Placement{Outcome: OutcomeUnknown, PendingPath: staged, LeftoverPath: backup},
				fmt.Errorf("place the new binary at %s: %w; verify the installed binary before retrying (previous binary at %s)",
					target.Path, err, backup)
		}
		return Placement{Outcome: OutcomeNotApplied, PendingPath: staged},
			fmt.Errorf("place the new binary at %s: %w", target.Path, err)
	}
	if err := i.system.Remove(backup); err != nil {
		return Placement{Outcome: OutcomeApplied, LeftoverPath: backup}, nil
	}
	return Placement{Outcome: OutcomeApplied}, nil
}

// CleanLeftovers removes previous binaries left behind by an interrupted or
// locked Windows replacement together with stale staged files, and returns
// those it could not remove. The destination itself is never removed.
func (i *Installer) CleanLeftovers(destination string) []string {
	patterns := []string{
		destination + ".old.*",
		filepath.Join(filepath.Dir(destination), ".gojira.new.*"),
	}
	remaining := make([]string, 0, 1)
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			if sameDirectory(match, destination) {
				continue
			}
			if err := i.system.Remove(match); err != nil {
				remaining = append(remaining, match)
			}
		}
	}
	return remaining
}

func randomSuffix() string {
	buffer := make([]byte, 4)
	if _, err := rand.Read(buffer); err != nil {
		return strings.Repeat("0", 8)
	}
	return hex.EncodeToString(buffer)
}
