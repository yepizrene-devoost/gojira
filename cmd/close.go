package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yepizrene-devoost/gojira/internal/jira"
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
			var candidates []jira.Transition
			var descriptions []string
			for _, tr := range transitions {
				if tr.To == nil || tr.To.Category == nil {
					continue
				}
				category := tr.To.Category.Key
				if (action == "close" && category == "done") || (action == "reopen" && (category == "new" || category == "indeterminate")) {
					candidates = append(candidates, tr)
					descriptions = append(descriptions, fmt.Sprintf("%s: %s -> %s (%s)", tr.ID, tr.Name, tr.To.Name, category))
				}
			}
			if len(candidates) != 1 {
				listing := "none"
				if len(descriptions) > 0 {
					listing = strings.Join(descriptions, "; ")
				}
				message := fmt.Sprintf("%s requires exactly one available transition; candidates: %s", action, listing)
				return commandError(cmd, "validation_error", message, key, jira.MutationNotApplied, fmt.Errorf("%s", message))
			}
			if err := client.TransitionIssue(key, candidates[0].ID); err != nil {
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
