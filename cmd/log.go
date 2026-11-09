package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

var logCmd = &cobra.Command{
	Use:   "log <issue-key> --time <duration>",
	Short: "Add worklog to a ticket",
	Long:  "Log time spent on a ticket. Example: gojira log ARA-1892 --time 2h --comment 'Fixed auth bug'",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueKey := args[0]
		timeSpent, _ := cmd.Flags().GetString("time")
		comment, _ := cmd.Flags().GetString("comment")
		showLogs, _ := cmd.Flags().GetBool("show")
		worklogID, _ := cmd.Flags().GetString("id")
		deleteWorklog, _ := cmd.Flags().GetBool("delete")
		confirm, _ := cmd.Flags().GetString("confirm")
		clearComment, _ := cmd.Flags().GetBool("clear-comment")

		// S13: reading history and addressing one entry are exclusive intents.
		if showLogs && worklogID != "" {
			err := fmt.Errorf("--show cannot be combined with --id")
			return commandError(cmd, "validation_error", "--show cannot be combined with --id", issueKey, jira.MutationNotApplied, err)
		}
		if deleteWorklog {
			if worklogID == "" {
				err := fmt.Errorf("--delete requires --id")
				return commandError(cmd, "validation_error", "--delete requires --id", issueKey, jira.MutationNotApplied, err)
			}
			if timeSpent != "" || comment != "" || clearComment {
				err := fmt.Errorf("--delete cannot be combined with --time or --comment")
				return commandError(cmd, "validation_error", "--delete cannot be combined with --time or --comment", issueKey, jira.MutationNotApplied, err)
			}
			if confirm != worklogID {
				err := fmt.Errorf("--confirm must repeat the worklog id")
				return commandError(cmd, "validation_error", "--confirm must repeat the worklog id", issueKey, jira.MutationNotApplied, err)
			}
		}
		// S14: clearing the comment is a deliberate act that still targets one entry.
		if clearComment {
			if worklogID == "" {
				err := fmt.Errorf("--clear-comment requires --id")
				return commandError(cmd, "validation_error", "--clear-comment requires --id", issueKey, jira.MutationNotApplied, err)
			}
			if comment != "" {
				err := fmt.Errorf("--clear-comment cannot be combined with --comment")
				return commandError(cmd, "validation_error", "--clear-comment cannot be combined with --comment", issueKey, jira.MutationNotApplied, err)
			}
		}
		if worklogID != "" && !deleteWorklog && timeSpent == "" && comment == "" && !clearComment {
			err := fmt.Errorf("--time or --comment is required with --id")
			return commandError(cmd, "validation_error", "--time or --comment is required with --id", issueKey, jira.MutationNotApplied, err)
		}

		if !showLogs && timeSpent == "" && worklogID == "" {
			err := fmt.Errorf("--time is required (e.g. 30m, 2h, 1d)")
			return commandError(cmd, "validation_error", "time spent is required", issueKey, jira.MutationNotApplied, err)
		}
		client, domain, err := buildClient()
		if err != nil {
			return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", issueKey, jira.MutationNotApplied, err)
		}

		if showLogs {
			worklogs, err := client.GetWorklog(issueKey)
			if err != nil {
				return commandError(cmd, "read_failed", "worklogs could not be read", issueKey, jira.MutationNotApplied, err)
			}
			if wantsJSON(cmd) {
				if worklogs == nil {
					worklogs = []jira.Worklog{}
				}
				if err := writeJSON(cmd, worklogs); err != nil {
					return reportJSONError(cmd, "output_failed", "worklog JSON could not be written", issueKey, jira.MutationNotApplied, err)
				}
				return nil
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Worklog for %s:\n\n", issueKey)
			for _, worklog := range worklogs {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s  %s\n", worklog.TimeSpent, worklog.Author.DisplayName, worklog.Comment)
			}
			return nil
		}

		if deleteWorklog {
			if err := client.DeleteWorklog(issueKey, worklogID); err != nil {
				return commandError(cmd, "mutation_failed", "worklog deletion failed", issueKey, jira.MutationStateOf(err), err)
			}
			if wantsJSON(cmd) {
				return writeMutationResult(cmd, client, domain, issueKey)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Deleted worklog %s on %s\n", worklogID, issueKey)
			return nil
		}

		if worklogID != "" {
			// A clear is one mutation whether or not the duration also changes; a
			// second request could leave the worklog half-edited. `--comment` is
			// impossible here (rejected above).
			var err error
			switch {
			case clearComment && timeSpent != "":
				err = client.UpdateWorklogAndClearComment(issueKey, worklogID, timeSpent)
			case clearComment:
				err = client.ClearWorklogComment(issueKey, worklogID)
			default:
				err = client.UpdateWorklog(issueKey, worklogID, timeSpent, comment)
			}
			if err != nil {
				return commandError(cmd, "mutation_failed", "worklog update failed", issueKey, jira.MutationStateOf(err), err)
			}
			// S12: re-read the entry so a duration Jira did not keep is reported
			// as a warning; the mutation itself was accepted, so the exit code
			// stays 0 and `--json` stdout remains a single valid document.
			var warning error
			if timeSpent != "" {
				warning = client.VerifyWorklogUpdate(issueKey, worklogID, timeSpent)
			}
			if wantsJSON(cmd) {
				if err := writeMutationResult(cmd, client, domain, issueKey); err != nil {
					return err
				}
				writeUpdateWarning(cmd, warning)
				return nil
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Updated worklog %s on %s\n", worklogID, issueKey)
			writeUpdateWarning(cmd, warning)
			return nil
		}

		if err := client.AddWorklog(issueKey, timeSpent, comment); err != nil {
			return commandError(cmd, "mutation_failed", "worklog creation failed", issueKey, jira.MutationStateOf(err), err)
		}
		if wantsJSON(cmd) {
			return writeMutationResult(cmd, client, domain, issueKey)
		}

		message := fmt.Sprintf("✓ Logged %s on %s", timeSpent, issueKey)
		if comment != "" {
			message += fmt.Sprintf(" (%s)", strings.TrimSpace(comment))
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), message)
		return nil
	},
}

// writeUpdateWarning reports a post-write verification mismatch on stderr.
// The mutation itself was accepted, so the command still exits successfully
// and stdout (text or `--json`) is never mixed with the warning.
func writeUpdateWarning(cmd *cobra.Command, warning error) {
	if warning != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning)
	}
}

func init() {
	logCmd.Flags().StringP("time", "t", "", "Time spent (e.g. 30m, 2h, 1d)")
	logCmd.Flags().StringP("comment", "c", "", "Worklog comment (sent as an ADF document)")
	logCmd.Flags().String("id", "", "Worklog id: update it with --time/--comment, or delete it with --delete")
	logCmd.Flags().Bool("delete", false, "Delete the worklog given by --id (requires --confirm <id>)")
	logCmd.Flags().String("confirm", "", "Confirmation for --delete: must repeat the worklog id")
	logCmd.Flags().Bool("show", false, "Show existing worklogs (cannot be combined with --id)")
	logCmd.Flags().Bool("clear-comment", false, "Clear the comment of the worklog given by --id (cannot be combined with --comment)")
	logCmd.Flags().Bool("json", false, "Show worklogs or output the resulting issue as JSON")
	rootCmd.AddCommand(logCmd)
}
