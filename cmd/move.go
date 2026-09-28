package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

var moveCmd = &cobra.Command{
	Use:   "move <issue-key> --to <status>",
	Short: "Transition a ticket to a new status",
	Long:  "Move a ticket to a different status. Example: gojira move ARA-1892 --to '03 In Progress'",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueKey := args[0]
		client, domain, err := buildClient()
		if err != nil {
			return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", issueKey, jira.MutationNotApplied, err)
		}
		targetStatus, _ := cmd.Flags().GetString("to")

		transitions, err := client.GetTransitions(issueKey)
		if err != nil {
			return commandError(cmd, "read_failed", "issue transitions could not be read", issueKey, jira.MutationNotApplied, err)
		}
		if targetStatus == "" {
			if wantsJSON(cmd) {
				if err := writeJSON(cmd, transitions); err != nil {
					return reportJSONError(cmd, "output_failed", "transition JSON could not be written", issueKey, jira.MutationNotApplied, err)
				}
				return nil
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Available transitions for %s:\n\n", issueKey)
			for i, transition := range transitions {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n", i+1, transition.Name)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nUse --to \"<status>\" to move. Example: gojira move %s --to \"03 In Progress\"\n", issueKey)
			return nil
		}

		var matched *jira.Transition
		for i := range transitions {
			if transitions[i].Name == targetStatus {
				matched = &transitions[i]
				break
			}
		}
		if matched == nil {
			names := make([]string, len(transitions))
			for i, transition := range transitions {
				names[i] = transition.Name
			}
			err := fmt.Errorf("status %q not found. Available: %v", targetStatus, names)
			return commandError(cmd, "validation_error", "target status is not an available transition", issueKey, jira.MutationNotApplied, err)
		}
		if err := client.TransitionIssue(issueKey, matched.ID); err != nil {
			return commandError(cmd, "mutation_failed", "issue transition failed", issueKey, jira.MutationStateOf(err), err)
		}
		if wantsJSON(cmd) {
			return writeMutationResult(cmd, client, domain, issueKey)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Moved %s → %s\n", issueKey, matched.Name)
		return nil
	},
}

func init() {
	moveCmd.Flags().String("to", "", "Target status name")
	moveCmd.Flags().Bool("json", false, "List transitions or output the resulting issue as JSON")
	rootCmd.AddCommand(moveCmd)
}
