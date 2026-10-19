package agentguide

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The local exclude file has the lowest precedence in Git: a negation in the
// repository ignore rules overrides it. A successful exclusion write therefore
// does not prove the guide is ignored.
func TestInstallRefusesWhenRepositoryRulesOverrideTheExclusion(t *testing.T) {
	root := repository(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("!guide.md\n"), 0644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "guide.md")
	_, err := Install(OSFilesystem{}, GitAdapter{}, Options{Path: target, Yes: true, Exclude: true})
	if err == nil || !strings.Contains(err.Error(), "override") {
		t.Fatalf("want a refusal explaining that repository rules override the exclusion, got %v", err)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("refused installation wrote the guide: %v", statErr)
	}
}

type unverifiableGit struct{ GitAdapter }

func (unverifiableGit) Ignored(Exclusion) (bool, error) {
	return false, errors.New("cannot verify the effective Git ignore status")
}

// Guard (no RED recorded, it shares the branch of the test above): when Git
// cannot confirm the effective status, nothing is written either.
func TestInstallRefusesWhenIgnoreStatusCannotBeVerified(t *testing.T) {
	root := repository(t)
	target := filepath.Join(root, "guide.md")
	if _, err := Install(OSFilesystem{}, unverifiableGit{}, Options{Path: target, Yes: true, Exclude: true}); err == nil {
		t.Fatal("want a refusal when the ignore status cannot be verified")
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("refused installation wrote the guide: %v", statErr)
	}
}
