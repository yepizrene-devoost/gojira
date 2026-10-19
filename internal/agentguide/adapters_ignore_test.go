package agentguide

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// ignored asks Git itself whether a path is ignored; exit 0 means ignored.
func ignored(t *testing.T, root, path string) bool {
	t.Helper()
	err := exec.Command("git", "-C", root, "check-ignore", "-q", path).Run()
	if err == nil {
		return true
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false
	}
	t.Fatalf("git check-ignore: %v", err)
	return false
}

func TestGitExclusionMatchesOnlyTheTarget(t *testing.T) {
	for _, name := range []string{"guide.md", "#notes.md", "!keep.md", "my guide.md", "g[1].md", "star*.md", "what?.md"} {
		t.Run(name, func(t *testing.T) {
			root := repository(t)
			target := filepath.Join(root, name)
			homonym := filepath.Join(root, "sub", name)
			for _, path := range []string{target, homonym} {
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			adapter := GitAdapter{}
			exclusion, err := adapter.Prepare(target)
			if err != nil {
				t.Fatal(err)
			}
			if err := adapter.Apply(exclusion); err != nil {
				t.Fatal(err)
			}
			if !ignored(t, root, name) {
				t.Fatalf("line %q does not exclude the target %q", exclusion.Line, name)
			}
			if ignored(t, root, filepath.Join("sub", name)) {
				t.Fatalf("line %q also excludes the homonym in another directory", exclusion.Line)
			}
		})
	}
}

// "Not a repository" and "git could not run" are different facts: only the first
// may allow an installation without --exclude.
func TestGitPrepareDistinguishesMissingGitFromOutsideRepository(t *testing.T) {
	target := filepath.Join(t.TempDir(), "guide.md")
	t.Setenv("PATH", t.TempDir())
	_, err := (GitAdapter{}).Prepare(target)
	if err == nil {
		t.Fatal("want an error when git cannot run")
	}
	if errors.Is(err, ErrOutsideWorkingTree) {
		t.Fatalf("a missing git was reported as outside a working tree: %v", err)
	}
}
