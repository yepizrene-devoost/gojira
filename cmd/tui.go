package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/config"
	"github.com/yepizrene-devoost/gojira/internal/tui"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive TUI (default when no command is given)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTUI()
	},
}

func runTUI() error {
	// ── First-run wizard ──────────────────────────────────────────
	if !config.Exists() {
		fmt.Fprintln(os.Stderr, "No configuration found. Let's set things up.\n")
		m := tui.NewSetup()
		p := tea.NewProgram(m, tea.WithAltScreen())
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
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func init() {
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runTUI()
	}
	rootCmd.AddCommand(tuiCmd)
}

