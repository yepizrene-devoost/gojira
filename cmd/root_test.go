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
