package agentguide

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repository(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("temporary Git integration")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	return root
}

func TestGitApplyPreservesAndDeduplicates(t *testing.T) {
	for _, initial := range []string{"", "# existing\nkeep", "# existing\nkeep\n"} {
		t.Run(initial, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "exclude")
			if initial != "" {
				if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
					t.Fatal(err)
				}
			}
			adapter := GitAdapter{}
			exclusion := Exclusion{File: path, Line: "docs/guide.md"}
			for range 2 {
				if err := adapter.Apply(exclusion); err != nil {
					t.Fatal(err)
				}
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := initial
			if want != "" && !strings.HasSuffix(want, "\n") {
				want += "\n"
			}
			want += "docs/guide.md\n"
			if string(data) != want {
				t.Fatalf("got %q; want %q", data, want)
			}
		})
	}
}

func TestGitPrepareResolvesRelativeExclusion(t *testing.T) {
	root := repository(t)
	target := filepath.Join(root, "new", "guide.md")
	exclusion, err := (GitAdapter{}).Prepare(target)
	if err != nil {
		t.Fatal(err)
	}
	if exclusion.Line != "/new/guide.md" {
		t.Fatalf("line: %q", exclusion.Line)
	}
	out, err := exec.Command("git", "-C", root, "rev-parse", "--git-path", "info/exclude").Output()
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(string(out))
	if !filepath.IsAbs(want) {
		want = filepath.Join(root, want)
	}
	if exclusion.File != want {
		t.Fatalf("exclude file: %q, want %q", exclusion.File, want)
	}
	if _, err := os.Stat(filepath.Dir(target)); !os.IsNotExist(err) {
		t.Fatalf("prepare created target directory: %v", err)
	}
}
