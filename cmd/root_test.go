package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommandWithoutArgsShowsHelpWithoutConfiguration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("JIRA_EMAIL", "")
	t.Setenv("JIRA_API_TOKEN", "")
	t.Setenv("JIRA_DOMAIN", "")

	// Keep godotenv initialization, if Cobra invokes it for help, isolated from
	// the repository and any developer-specific environment file.
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("change to isolated working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(workingDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	var output bytes.Buffer
	oldOut := rootCmd.OutOrStdout()
	oldErr := rootCmd.ErrOrStderr()
	rootCmd.SetOut(&output)
	rootCmd.SetErr(&output)
	rootCmd.SetArgs([]string{})
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(oldOut)
		rootCmd.SetErr(oldErr)
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute bare root command: %v", err)
	}

	got := output.String()
	for _, want := range []string{"Usage:", "Available Commands:", "tui"} {
		if !strings.Contains(got, want) {
			t.Errorf("bare root output does not contain %q:\n%s", want, got)
		}
	}

	configPath := filepath.Join(home, ".config", "gojira", "config.yaml")
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("bare root command touched configuration %q: %v", configPath, err)
	}
}

func TestHasJSONIntentRespectsCommandFlagGrammar(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "implicit true", args: []string{"create", "--json"}, want: true},
		{name: "numeric true", args: []string{"create", "--json=1"}, want: true},
		{name: "uppercase true", args: []string{"create", "--json=TRUE"}, want: true},
		{name: "true before command", args: []string{"--json=true", "create"}, want: true},
		{name: "false before command overridden after", args: []string{"--json=false", "create", "--json=TRUE"}, want: true},
		{name: "true before command overridden after", args: []string{"--json=1", "create", "--json=false"}, want: false},
		{name: "flag value before command is skipped", args: []string{"--summary", "value", "create", "--json"}, want: true},
		{name: "command name consumed as flag value", args: []string{"--summary", "create", "--json"}, want: false},
		{name: "terminator before command", args: []string{"--", "create", "--json"}, want: false},
		{name: "JSON token consumed as summary value", args: []string{"create", "--summary", "--json"}, want: false},
		{name: "flag terminator", args: []string{"create", "--", "--json"}, want: false},
		{name: "later false overrides true", args: []string{"create", "--json", "--json=false"}, want: false},
		{name: "later numeric false overrides true", args: []string{"create", "--json=TRUE", "--json=0"}, want: false},
		{name: "later true overrides false", args: []string{"create", "--json=false", "--json=T"}, want: true},
		{name: "unrelated value is not sniffed", args: []string{"create", "--summary=contains--json=true"}, want: false},
		{name: "command without JSON flag", args: []string{"tui", "--json"}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := hasJSONIntent(rootCmd, test.args); got != test.want {
				t.Fatalf("hasJSONIntent(%q) = %v, want %v", test.args, got, test.want)
			}
		})
	}
}

func TestTUIRunsOnlyWhenExplicitlySelected(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantCalls int
	}{
		{name: "bare command only shows help", args: []string{}, wantCalls: 0},
		{name: "explicit tui command launches TUI", args: []string{"tui"}, wantCalls: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			root := &cobra.Command{Use: "gojira"}
			root.SetOut(&bytes.Buffer{})
			root.SetErr(&bytes.Buffer{})
			root.SetArgs(tc.args)
			root.AddCommand(newTUICommand(func() error {
				calls++
				return nil
			}))

			if err := root.Execute(); err != nil {
				t.Fatalf("execute command: %v", err)
			}
			if calls != tc.wantCalls {
				t.Fatalf("TUI launch calls = %d, want %d", calls, tc.wantCalls)
			}
		})
	}
}

func TestTUICommandReturnsLaunchError(t *testing.T) {
	launchErr := errors.New("launch failed")
	root := &cobra.Command{Use: "gojira", SilenceUsage: true, SilenceErrors: true}
	root.SetArgs([]string{"tui"})
	root.AddCommand(newTUICommand(func() error { return launchErr }))

	if err := root.Execute(); !errors.Is(err, launchErr) {
		t.Fatalf("execute tui error = %v, want %v", err, launchErr)
	}
}
