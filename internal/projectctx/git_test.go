package projectctx

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveProjectRealGit(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git unavailable")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "global"))
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GOJIRA_PROJECT", "")
	// Explicitly isolate inherited command-line Git configuration too.
	t.Setenv("GIT_CONFIG_COUNT", "0")
	run := func(dir string, args ...string) string {
		t.Helper()
		c := exec.Command(git, args...)
		c.Dir = dir
		b, e := c.CombinedOutput()
		if e != nil {
			t.Fatalf("git %v: %v %s", args, e, b)
		}
		return string(b)
	}
	repo := t.TempDir()
	run(repo, "init")
	run(repo, "config", "user.email", "test@example.invalid")
	run(repo, "config", "user.name", "Test")
	run(repo, "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "test")
	check := func(dir, key, scope string) {
		t.Helper()
		p, e := ResolveProjectWith("", "", func(string) string { return "" }, gitFunc(func() ([]byte, error) {
			c := exec.Command(git, "config", "--show-scope", "--get", "gojira.project")
			c.Dir = dir
			return c.Output()
		}))
		if e != nil || p == nil || p.Key != key || p.Scope != scope {
			t.Fatalf("project=%+v error=%v", p, e)
		}
	}
	run(repo, "config", "--global", "gojira.project", "GLOBAL")
	t.Run("outside repository", func(t *testing.T) {
		t.Chdir(t.TempDir())
		p, e := ResolveProject("", "")
		if e != nil || p != nil {
			t.Fatalf("outside repo project=%+v error=%v", p, e)
		}
	})
	t.Run("missing binary", func(t *testing.T) {
		t.Setenv("PATH", "")
		p, e := ResolveProject("", "USER")
		if e != nil || p == nil || p.Key != "USER" {
			t.Fatalf("missing binary project=%+v error=%v", p, e)
		}
	})
	check(repo, "GLOBAL", "global")
	run(repo, "config", "--local", "gojira.project", "LOCAL")
	check(repo, "LOCAL", "local")
	worktree := filepath.Join(t.TempDir(), "linked")
	run(repo, "worktree", "add", "-b", "linked", worktree)
	check(worktree, "LOCAL", "local")
	run(repo, "config", "extensions.worktreeConfig", "true")
	run(worktree, "config", "--worktree", "gojira.project", "WORKTREE")
	check(worktree, "WORKTREE", "worktree")
	sub := t.TempDir()
	run(sub, "init")
	run(sub, "config", "user.email", "test@example.invalid")
	run(sub, "config", "user.name", "Test")
	run(sub, "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "test")
	run(repo, "-c", "protocol.file.allow=always", "submodule", "add", sub, "child")
	child := filepath.Join(repo, "child")
	run(child, "config", "--local", "gojira.project", "SUB")
	check(child, "SUB", "local")
}
