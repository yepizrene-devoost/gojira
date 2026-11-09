package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/config"
	"github.com/yepizrene-devoost/gojira/internal/tui"
	"github.com/yepizrene-devoost/gojira/internal/upgrade"
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
	// The release check is optional startup decoration: any config failure
	// simply leaves it off and never disturbs the TUI.
	cfg, cfgErr := config.Load()
	m := tui.New(client, domain).WithVersion(version)
	if cfgErr == nil && cfg.UpdateCheckEnabled() && cfg.UpdateCheckDue(time.Now(), config.UpdateCheckInterval) {
		m = m.WithUpdateCheck(
			func(ctx context.Context) (string, error) {
				release, err := upgrade.NewClient("", &http.Client{Timeout: 4 * time.Second}).LatestRelease(ctx)
				return release.TagName, err
			},
			func(latest string) {
				recordUpdateCheck(latest)
			},
		)
	}
	p := tea.NewProgram(m)
	_, err = p.Run()
	return err
}

// recordUpdateCheck stores the result of a release check through the
// serialized configuration writer, so it cannot race a concurrent config
// writer in another process. Failures are ignored: the release check is
// optional startup decoration and never disturbs the TUI.
func recordUpdateCheck(latest string) {
	_ = config.Update(func(cfg *config.Config) error {
		cfg.RecordUpdateCheck(time.Now(), latest)
		return nil
	})
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
