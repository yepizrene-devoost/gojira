package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
	"github.com/yepizrene-devoost/gojira/internal/upgrade"
)

// Seams kept swappable so tests never reach the real release endpoint or the
// real installed binary.
var (
	upgradeBaseURL      = ""
	upgradeInstaller    = func() *upgrade.Installer { return upgrade.NewInstaller(upgrade.OSSystem()) }
	upgradeFetcher      = func() *upgrade.Fetcher { return upgrade.NewFetcher(nil) }
	upgradeArtifactName = upgrade.CurrentArtifactName
)

// checksumNote states honestly what a verified checksum proves.
const checksumNote = "transfer integrity only; checksums.txt does not authenticate the publisher"

// upgradeReport is the machine-readable output of `gojira upgrade --json`.
type upgradeReport struct {
	SchemaVersion    string             `json:"schemaVersion"`
	CurrentVersion   string             `json:"currentVersion"`
	LatestVersion    string             `json:"latestVersion"`
	UpdateAvailable  bool               `json:"updateAvailable"`
	Action           string             `json:"action"`
	MutationState    jira.MutationState `json:"mutationState"`
	ExecutablePath   string             `json:"executablePath,omitempty"`
	AssetName        string             `json:"assetName,omitempty"`
	ChecksumVerified bool               `json:"checksumVerified"`
	ChecksumNote     string             `json:"checksumNote,omitempty"`
	PendingPath      string             `json:"pendingPath,omitempty"`
	LeftoverPath     string             `json:"leftoverPath,omitempty"`
}

func newUpgradeCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade gojira to the latest published release",
		Long: `Replace this gojira binary with the latest published release.

Only a binary installed by the GoJira installer is replaced: the directories
$GOJIRA_INSTALL_DIR, ~/.local/bin and, on Windows, %LOCALAPPDATA%\Programs\gojira.
Package-manager paths, go install locations, symlinks and unknown installations
are refused with instructions instead of being overwritten.

The download is verified against checksums.txt from the same release. A matching
checksum proves transfer integrity only: it does not authenticate the publisher,
because the checksum is served next to the artifact.

Use --check to report the available version without installing anything. Without
a terminal, --yes is required and no upgrade happens silently.`,
		Args: cobra.NoArgs,
		RunE: runUpgrade,
	}
	command.Flags().Bool("check", false, "Report the latest published release without installing it")
	command.Flags().Bool("yes", false, "Install without asking for confirmation")
	command.Flags().Bool("json", false, "Output as JSON")
	return command
}

func init() { rootCmd.AddCommand(newUpgradeCommand()) }

// upgradeFailure reports a refusal or failure with its outcome, and fails closed
// on the mutation state so nothing implies a change that did not happen.
func upgradeFailure(cmd *cobra.Command, code, message string, state jira.MutationState, cause error) error {
	if cause == nil {
		cause = errors.New(message)
	} else if !strings.Contains(cause.Error(), message) {
		cause = fmt.Errorf("%s: %w", message, cause)
	}
	return commandError(cmd, code, message, "", state, cause)
}

func runUpgrade(cmd *cobra.Command, _ []string) error {
	checkOnly, _ := cmd.Flags().GetBool("check")
	assumeYes, _ := cmd.Flags().GetBool("yes")

	// Ownership is decided before any network access: a binary this command must
	// not replace is never examined further.
	installer := upgradeInstaller()
	var target upgrade.Target
	if !checkOnly {
		resolved, err := installer.Target()
		if err != nil {
			return upgradeFailure(cmd, "not_owned", err.Error(), jira.MutationNotApplied, err)
		}
		target = resolved
		if !assumeYes && !isTerminal() {
			const message = "refusing to install without confirmation; rerun with --yes or use --check"
			return upgradeFailure(cmd, "confirmation_required", message, jira.MutationNotApplied, nil)
		}
	}

	client := upgrade.NewClient(upgradeBaseURL, nil)
	release, err := client.LatestRelease(cmd.Context())
	if err != nil {
		message := fmt.Sprintf("release check failed: %v; nothing was changed", err)
		return upgradeFailure(cmd, "release_check_failed", message, jira.MutationNotApplied, err)
	}

	newer, compareErr := upgrade.NewerThan(version, release.TagName)
	comparable := compareErr == nil
	report := upgradeReport{
		SchemaVersion:   jsonSchemaVersion,
		CurrentVersion:  version,
		LatestVersion:   release.TagName,
		UpdateAvailable: newer,
		MutationState:   jira.MutationNotApplied,
	}

	if checkOnly {
		return reportUpgradeCheck(cmd, report, comparable)
	}
	if !comparable {
		return upgradeFailure(cmd, "not_comparable",
			fmt.Sprintf("%s is a development build (version %q); install a published release with scripts/install.sh or scripts/install.ps1", target.Path, version),
			jira.MutationNotApplied, nil)
	}
	if !newer {
		report.Action = "up_to_date"
		return writeUpgradeReport(cmd, report)
	}

	if !assumeYes {
		confirmed, err := readUpgradeConfirmation(cmd, release.TagName, target.Path)
		if err != nil {
			return upgradeFailure(cmd, "confirmation_failed", "confirmation could not be read; nothing was changed", jira.MutationNotApplied, err)
		}
		if !confirmed {
			report.Action = "declined"
			return writeUpgradeReport(cmd, report)
		}
	}

	assetName, err := upgradeArtifactName()
	if err != nil {
		return upgradeFailure(cmd, "unsupported_platform", err.Error(), jira.MutationNotApplied, err)
	}
	report.AssetName = assetName
	asset := release.Asset(assetName)
	if asset == nil {
		return upgradeFailure(cmd, "release_asset_missing",
			fmt.Sprintf("release %s has no asset named %s", release.TagName, assetName), jira.MutationNotApplied, nil)
	}
	checksums := release.Asset("checksums.txt")
	if checksums == nil {
		return upgradeFailure(cmd, "release_asset_missing",
			fmt.Sprintf("release %s has no checksums file", release.TagName), jira.MutationNotApplied, nil)
	}

	lock, err := installer.Lock(target.Dir)
	if err != nil {
		code := "lock_failed"
		if errors.Is(err, upgrade.ErrLocked) {
			code = "upgrade_in_progress"
		}
		return upgradeFailure(cmd, code, err.Error(), jira.MutationNotApplied, err)
	}
	defer func() { _ = lock.Release() }()

	installer.CleanLeftovers(target.Path)

	fetcher := upgradeFetcher()
	digest, err := fetcher.FetchChecksum(cmd.Context(), checksums.URL, assetName)
	if err != nil {
		message := fmt.Sprintf("download failed: %v; nothing was changed", err)
		return upgradeFailure(cmd, "download_failed", message, jira.MutationNotApplied, err)
	}
	staged, err := fetcher.DownloadVerified(cmd.Context(), asset.URL, assetName, digest, target.Dir)
	if err != nil {
		message := fmt.Sprintf("download failed: %v; nothing was changed", err)
		return upgradeFailure(cmd, "download_failed", message, jira.MutationNotApplied, err)
	}

	placement, err := installer.Place(target, staged)
	if err != nil {
		if placement.Outcome == upgrade.OutcomeUnknown {
			return upgradeFailure(cmd, "placement_unknown", err.Error(), jira.MutationUnknown, err)
		}
		return upgradeFailure(cmd, "placement_failed", err.Error(), jira.MutationNotApplied, err)
	}

	report.Action = "installed"
	report.UpdateAvailable = true
	report.MutationState = jira.MutationApplied
	report.ExecutablePath = target.Path
	report.ChecksumVerified = true
	report.ChecksumNote = checksumNote
	report.PendingPath = placement.PendingPath
	report.LeftoverPath = placement.LeftoverPath
	return writeUpgradeReport(cmd, report)
}

// reportUpgradeCheck writes the result of --check. A build that is not a
// published release is reported as such instead of pretending to be current.
func reportUpgradeCheck(cmd *cobra.Command, report upgradeReport, comparable bool) error {
	switch {
	case !comparable:
		report.UpdateAvailable = false
		report.Action = "unknown"
	case report.UpdateAvailable:
		report.Action = "available"
	default:
		report.Action = "up_to_date"
	}
	return writeUpgradeReport(cmd, report)
}

// writeUpgradeReport emits the JSON document or the human line for one outcome.
func writeUpgradeReport(cmd *cobra.Command, report upgradeReport) error {
	if wantsJSON(cmd) {
		if err := writeJSON(cmd, report); err != nil {
			return reportJSONError(cmd, "output_failed", "upgrade JSON could not be written", "", report.MutationState, err)
		}
		return nil
	}
	out := cmd.OutOrStdout()
	switch report.Action {
	case "installed":
		if _, err := fmt.Fprintf(out, "✓ Upgraded gojira %s → %s (restart any running session)\n", report.CurrentVersion, report.LatestVersion); err != nil {
			return err
		}
		if report.LeftoverPath != "" {
			_, err := fmt.Fprintf(out, "The previous binary remains at %s until it can be removed.\n", report.LeftoverPath)
			return err
		}
		return nil
	case "available":
		_, err := fmt.Fprintf(out, "gojira %s → %s available; run gojira upgrade to install it\n", report.CurrentVersion, report.LatestVersion)
		return err
	case "unknown":
		_, err := fmt.Fprintf(out, "gojira %s is not a published release; the latest published release is %s\n", report.CurrentVersion, report.LatestVersion)
		return err
	case "declined":
		_, err := fmt.Fprintln(out, "Upgrade cancelled; nothing was changed.")
		return err
	default:
		_, err := fmt.Fprintf(out, "✓ gojira %s is the latest published release\n", report.CurrentVersion)
		return err
	}
}

// readUpgradeConfirmation asks the human before replacing the binary. The
// prompt goes to stderr to keep stdout machine-readable.
func readUpgradeConfirmation(cmd *cobra.Command, tag, path string) (bool, error) {
	if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "Install gojira %s at %s? [y/N] ", tag, path); err != nil {
		return false, err
	}
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && line == "" {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
