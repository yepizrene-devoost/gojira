package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

var backlogCmd = &cobra.Command{
	Use:   "backlog <issue-key>",
	Short: "Move an issue from future and active sprints to the backlog",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueKey := args[0]
		if !parentIssueKey.MatchString(issueKey) {
			return commandError(cmd, "validation_error", "issue key is invalid", issueKey, jira.MutationNotApplied,
				fmt.Errorf("issue key must be a Jira issue key (e.g. ARA-123)"))
		}
		client, domain, err := buildClient()
		if err != nil {
			return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", issueKey, jira.MutationNotApplied, err)
		}
		if err := client.MoveIssueToBacklog(issueKey); err != nil {
			return commandError(cmd, "mutation_failed", "backlog move failed", issueKey, jira.MutationStateOf(err), err)
		}
		if wantsJSON(cmd) {
			return writeMutationResult(cmd, client, domain, issueKey)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Jira accepted the request to move %s to the backlog\n", issueKey)
		return nil
	},
}

func init() {
	backlogCmd.Flags().Bool("json", false, "Output the resulting issue as TicketJSON v1")
	rootCmd.AddCommand(backlogCmd)
}
