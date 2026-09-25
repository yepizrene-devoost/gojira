package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var assignCmd = &cobra.Command{
	Use:   "assign <issue-key> <email>",
	Short: "Assign an issue to a user by email",
	Long: `Assign a Jira issue to a user. Example:

  gojira assign ARA-1892 rene@devoost.com`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := BuildClient()
		if err != nil {
			return err
		}

		issueKey := args[0]
		email := args[1]

		accountID, displayName, err := client.ResolveAccountID(email)
		if err != nil {
			return fmt.Errorf("resolving user %s: %w", email, err)
		}

		if err := client.AssignIssue(issueKey, accountID); err != nil {
			return err
		}

		fmt.Printf("✓ Assigned %s → %s (%s)\n", issueKey, displayName, email)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(assignCmd)
}
