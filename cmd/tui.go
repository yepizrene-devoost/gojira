package cmd

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/config"
	"github.com/yepizrene-devoost/gojira/internal/tui"
)

var tuiCmd = newTUICommand(runTUI)

func newTUICommand(run func() error) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the interactive TUI",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run()
		},
	}
}

func runTUI() error {
	// ── First-run wizard ──────────────────────────────────────────
	if !config.Exists() {
		fmt.Fprintln(os.Stderr, "No configuration found. Let's set things up.")
		m := tui.NewSetup()
		p := tea.NewProgram(m)
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("setup wizard: %w", err)
		}
		fmt.Fprintln(os.Stderr)
	}

	// ── Load config & build client ────────────────────────────────
	client, domain, err := BuildClient()
	if err != nil {
		return fmt.Errorf("connection setup: %w", err)
	}

	info, err := client.TestConnection()
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	fmt.Fprintf(os.Stderr, "✓ Connected to %s\n", info)

	// ── Launch TUI ────────────────────────────────────────────────
	m := tui.New(client, domain)
	p := tea.NewProgram(m)
	_, err = p.Run()
	return err
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
