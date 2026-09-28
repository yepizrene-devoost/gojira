package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

var assignCmd = &cobra.Command{
	Use:   "assign <issue-key> <email>",
	Short: "Assign an issue to a user by email",
	Long: `Assign a Jira issue to a user. Example:

  gojira assign ARA-1892 rene@devoost.com`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueKey := args[0]
		email := args[1]
		client, domain, err := buildClient()
		if err != nil {
			return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", issueKey, jira.MutationNotApplied, err)
		}

		accountID, displayName, err := client.ResolveAccountID(email)
		if err != nil {
			wrapped := fmt.Errorf("resolving user %s: %w", email, err)
			return commandError(cmd, "lookup_failed", "assignee could not be resolved", issueKey, jira.MutationNotApplied, wrapped)
		}
		if err := client.AssignIssue(issueKey, accountID); err != nil {
			return commandError(cmd, "mutation_failed", "issue assignment failed", issueKey, jira.MutationStateOf(err), err)
		}
		if wantsJSON(cmd) {
			return writeMutationResult(cmd, client, domain, issueKey)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Assigned %s → %s (%s)\n", issueKey, displayName, email)
		return nil
	},
}

func init() {
	assignCmd.Flags().Bool("json", false, "Output the resulting issue as TicketJSON v1")
	rootCmd.AddCommand(assignCmd)
}
