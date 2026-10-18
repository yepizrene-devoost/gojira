package config

import (
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveProject(t *testing.T) {
	for _, tc := range []struct {
		name, flag, env, git, user, key, source, scope string
		invalid                                        bool
	}{
		{name: "flag", flag: "FLAG", env: "ENV", git: "local\tGIT", user: "USER", key: "FLAG", source: "flag"},
		{name: "env", env: "ENV", git: "local\tGIT", user: "USER", key: "ENV", source: "env"},
		{name: "git", git: "worktree\tGIT\n", user: "USER", key: "GIT", source: "git", scope: "worktree"},
		{name: "user", user: "USER", key: "USER", source: "user"},
		{name: "missing"},
		{name: "empty git", git: "local\t\n", user: "USER", key: "USER", source: "user"},
		{name: "invalid flag", flag: "bad", env: "ENV", invalid: true, source: "flag"},
		{name: "invalid env", env: " bad ", user: "USER", invalid: true, source: "env"},
		{name: "invalid git", git: "local\tbad", user: "USER", invalid: true, source: "git"},
		{name: "invalid user", user: "bad", invalid: true, source: "user"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := ResolveProjectWith(tc.flag, tc.user, func(string) string { return tc.env }, func() ([]byte, error) { return []byte(tc.git), nil })
			if tc.invalid {
				var invalid *ProjectValidationError
				if !errors.As(err, &invalid) || string(invalid.Source) != tc.source {
					t.Fatalf("error=%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.key == "" {
				if p != nil {
					t.Fatalf("project=%+v", p)
				}
				return
			}
			if p == nil || p.Key != tc.key || string(p.Source) != tc.source || p.Scope != tc.scope {
				t.Fatalf("project=%+v", p)
			}
		})
	}
}

func TestResolveProjectGitUnavailable(t *testing.T) {
	for _, err := range []error{exec.ErrNotFound, errors.New("exit status 1"), errors.New("not a repository")} {
		p, e := ResolveProjectWith("", "USER", func(string) string { return "" }, func() ([]byte, error) { return nil, err })
		if e != nil || p == nil || p.Key != "USER" {
			t.Fatalf("project=%+v error=%v", p, e)
		}
	}
}

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
		p, e := ResolveProjectWith("", "", func(string) string { return "" }, func() ([]byte, error) {
			c := exec.Command(git, "config", "--show-scope", "--get", "gojira.project")
			c.Dir = dir
			return c.Output()
		})
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
