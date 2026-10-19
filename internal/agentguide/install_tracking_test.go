package agentguide

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A guide written inside a Git working tree must never be left eligible for
// accidental staging: the user has to choose local exclusion explicitly.
func TestInstallRefusesTrackableTargetInsideRepository(t *testing.T) {
	root := repository(t)
	target := filepath.Join(root, "docs", "guide.md")
	_, err := Install(OSFilesystem{}, GitAdapter{}, Options{Path: target, Yes: true})
	if err == nil || !strings.Contains(err.Error(), "--exclude") {
		t.Fatalf("want a refusal that points to --exclude, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "docs")); !os.IsNotExist(statErr) {
		t.Fatalf("refused installation created %s: %v", root, statErr)
	}
}

type brokenGit struct{}

func (brokenGit) Prepare(string) (Exclusion, error) {
	return Exclusion{}, errors.New("git unavailable")
}
func (brokenGit) Apply(Exclusion) error           { return nil }
func (brokenGit) Ignored(Exclusion) (bool, error) { return true, nil }

// When the repository state cannot be determined, the installer fails closed
// instead of assuming the target is outside a working tree.
func TestInstallFailsClosedWhenRepositoryStateIsUnknown(t *testing.T) {
	target := filepath.Join(t.TempDir(), "guide.md")
	_, err := Install(OSFilesystem{}, brokenGit{}, Options{Path: target, Yes: true})
	if err == nil {
		t.Fatal("want a refusal when the repository state is unknown")
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("refused installation wrote the guide: %v", statErr)
	}
}

// Guard (no RED recorded, it passed on first run): the refusal is limited to
// targets inside a working tree; a plain directory still works without flags.
func TestInstallOutsideRepositoryStillWritesWithoutExclude(t *testing.T) {
	target := filepath.Join(t.TempDir(), "guide.md")
	if _, err := Install(OSFilesystem{}, GitAdapter{}, Options{Path: target, Yes: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("guide was not written: %v", err)
	}
}
