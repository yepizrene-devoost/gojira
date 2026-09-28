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

		if !showLogs && timeSpent == "" {
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

func init() {
	logCmd.Flags().StringP("time", "t", "", "Time spent (e.g. 30m, 2h, 1d)")
	logCmd.Flags().StringP("comment", "c", "", "Worklog comment")
	logCmd.Flags().Bool("show", false, "Show existing worklogs")
	logCmd.Flags().Bool("json", false, "Show worklogs or output the resulting issue as JSON")
	rootCmd.AddCommand(logCmd)
}
