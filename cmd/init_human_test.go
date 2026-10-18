package cmd

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// Human-mode refusals must fail with a readable message; a nil cause used to
// make init exit 0 silently when no terminal and no flags were given.
func TestProjectInitHumanRefusalsFail(t *testing.T) {
	for _, tc := range []struct {
		name string
		repo bool
		args []string
		want string
	}{
		{"no terminal and no flags", true, nil, "--project KEY --yes"},
		{"no terminal without --yes", true, []string{"--project", "ABC"}, "--project KEY --yes"},
		{"outside a git work tree", false, []string{"--project", "ABC", "--yes"}, "Git work tree"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("GIT_CONFIG_GLOBAL", home+"/global")
			t.Setenv("GIT_CONFIG_SYSTEM", home+"/system")
			t.Setenv("GOJIRA_PROJECT", "")
			t.Chdir(t.TempDir())
			if tc.repo {
				if out, err := exec.Command("git", "init", "--quiet").CombinedOutput(); err != nil {
					t.Fatalf("git init: %s: %v", out, err)
				}
			}
			oldTerminal := isTerminal
			t.Cleanup(func() { isTerminal = oldTerminal })
			isTerminal = func() bool { return false }

			command := newProjectInitCommand()
			var stdout, stderr bytes.Buffer
			command.SetOut(&stdout)
			command.SetErr(&stderr)
			command.SetArgs(tc.args)
			err := command.Execute()
			if err == nil {
				t.Fatalf("expected an error, got nil (stdout=%q stderr=%q)", stdout.String(), stderr.String())
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err.Error(), tc.want)
			}
		})
	}
}
