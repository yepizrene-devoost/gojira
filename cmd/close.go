package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yepizrene-devoost/gojira/internal/jira"
	"github.com/yepizrene-devoost/gojira/internal/lifecycle"
)

func init() {
	for _, action := range []string{"close", "reopen"} {
		cmd := &cobra.Command{Use: action + " KEY", Short: "Transition an issue by destination status category", Args: cobra.ExactArgs(1)}
		cmd.Flags().Bool("json", false, "Output JSON")
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			key := args[0]
			if !parentIssueKey.MatchString(key) {
				return commandError(cmd, "validation_error", "invalid issue key", key, jira.MutationNotApplied, fmt.Errorf("issue key must be a Jira issue key (e.g. ARA-123)"))
			}
			client, domain, err := buildClient()
			if err != nil {
				return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", key, jira.MutationNotApplied, err)
			}
			transitions, err := client.GetTransitions(key)
			if err != nil {
				return commandError(cmd, "read_failed", "available transitions could not be read", key, jira.MutationNotApplied, err)
			}
			transition, err := lifecycle.Select(lifecycle.Action(action), transitions)
			if err != nil {
				return commandError(cmd, "validation_error", err.Error(), key, jira.MutationNotApplied, err)
			}
			if err := client.TransitionIssue(key, transition.ID); err != nil {
				state := jira.MutationStateOf(err)
				message := action + " failed"
				if state == jira.MutationUnknown {
					message += "; whether the change was applied is unknown; verify before retrying"
					err = fmt.Errorf("%s: %w", message, err)
				}
				return commandError(cmd, "mutation_failed", message, key, state, err)
			}
			if wantsJSON(cmd) {
				return writeMutationResult(cmd, client, domain, key)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Jira accepted the request to %s %s\n", action, key)
			return nil
		}
		rootCmd.AddCommand(cmd)
	}
}
