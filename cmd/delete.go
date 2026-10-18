package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func init() {
	cmd := &cobra.Command{Use: "delete KEY --confirm KEY", Short: "Permanently delete an issue after exact key confirmation", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		confirm, _ := cmd.Flags().GetString("confirm")
		if !parentIssueKey.MatchString(key) || confirm != key {
			return commandError(cmd, "validation_error", "delete requires a valid issue key and --confirm matching it exactly", key, jira.MutationNotApplied, fmt.Errorf("permanent deletion requires a valid issue key and --confirm matching it exactly"))
		}
		client, _, err := buildClient()
		if err != nil {
			return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", key, jira.MutationNotApplied, err)
		}
		if err := client.DeleteIssue(key); err != nil {
			state := jira.MutationStateOf(err)
			message := "delete failed"
			if state == jira.MutationUnknown {
				message += "; whether deletion was applied is unknown; verify before retrying"
				err = fmt.Errorf("%s: %w", message, err)
			}
			return commandError(cmd, "mutation_failed", message, key, state, err)
		}
		if wantsJSON(cmd) {
			result := struct {
				SchemaVersion string `json:"schemaVersion"`
				Key           string `json:"key"`
				Deleted       bool   `json:"deleted"`
			}{jsonSchemaVersion, key, true}
			if err := writeJSON(cmd, result); err != nil {
				return commandError(cmd, "output_failed", "deletion applied but the JSON result could not be written", key, jira.MutationApplied, err)
			}
			return nil
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Permanently deleted %s\n", key)
		return nil
	}}
	cmd.Flags().String("confirm", "", "Confirm permanent deletion by repeating the exact issue key")
	cmd.Flags().Bool("json", false, "Output JSON")
	rootCmd.AddCommand(cmd)
}
