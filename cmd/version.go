package cmd

import (
	"encoding/json"
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is the channel/version marker, overridden at build time via
// -ldflags "-X main.version=...". Development builds keep "dev".
var version = "dev"

// SetVersion stores the marker injected by main and enables `gojira --version`.
func SetVersion(v string) {
	version = v
	rootCmd.Version = v
	rootCmd.SetVersionTemplate("gojira {{.Version}}\n")
}

// versionReport is the machine-readable output of `gojira version --json`.
type versionReport struct {
	Version  string `json:"version"`
	Revision string `json:"revision"`
	Dirty    bool   `json:"dirty"`
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the gojira version",
	Long: `Show the version marker and the commit this binary was built from.

The marker is "dev" for development builds or the release tag injected at
build time. The revision comes from the binary's own VCS stamp, suffixed
"-dirty" when the build tree had uncommitted changes. Pass --json for one
machine-readable document.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()

		if jsonOutput, _ := cmd.Flags().GetBool("json"); jsonOutput {
			report := versionReport{
				Version:  version,
				Revision: buildRevision(),
				Dirty:    buildDirty(),
			}
			b, _ := json.MarshalIndent(report, "", "  ")
			_, err := fmt.Fprintln(out, string(b))
			return err
		}

		line := "gojira " + version
		if rev := shortRevision(); rev != "" {
			line += " (" + rev
			if buildDirty() {
				line += "-dirty"
			}
			line += ")"
		}
		_, err := fmt.Fprintln(out, line)
		return err
	},
}

func init() {
	versionCmd.Flags().Bool("json", false, "Print the version as a single JSON document")
	rootCmd.AddCommand(versionCmd)
}

// buildInfo returns the embedded build/VCS info, or nil when absent
// (for example, a build without a git stamp).
func buildInfo() *debug.BuildInfo {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	return info
}

func settingValue(settings []debug.BuildSetting, key string) string {
	for _, s := range settings {
		if s.Key == key {
			return s.Value
		}
	}
	return ""
}

// buildRevision returns the full 40-char commit hash, or "" when the binary
// carries no VCS stamp.
func buildRevision() string {
	info := buildInfo()
	if info == nil {
		return ""
	}
	return settingValue(info.Settings, "vcs.revision")
}

func shortRevision() string {
	rev := buildRevision()
	if len(rev) > 7 {
		return rev[:7]
	}
	return rev
}

// buildDirty reports whether the tree had uncommitted changes at build time.
func buildDirty() bool {
	info := buildInfo()
	if info == nil {
		return false
	}
	return settingValue(info.Settings, "vcs.modified") == "true"
}
