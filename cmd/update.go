package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/issuefields"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

var updateCmd = &cobra.Command{
	Use:   "update <issue-key>",
	Short: "Update fields on a Jira issue",
	Long: `Update one or more fields on an existing issue. Examples:

  gojira update ARA-1892 --summary "New summary text"
  gojira update ARA-1892 --description "Updated context"
  gojira update ARA-1892 --due-date 2026-10-31
  gojira update ARA-1892 --components "Web,API" --fix-versions "v1.0,v1.1"
  gojira update ARA-1892 --story-points 5.5 --story-points-field customfield_10016
  gojira update ARA-1892 --due-date= --components= --fix-versions=
  gojira update ARA-1892 --story-points= --story-points-field customfield_10016
  gojira update ARA-1892 --priority High
  gojira update ARA-1892 --labels "frontend,urgent"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueKey := args[0]
		flag := func(name string) *string {
			if !cmd.Flags().Changed(name) {
				return nil
			}
			value, _ := cmd.Flags().GetString(name)
			return &value
		}
		prepared, err := issuefields.Prepare(issuefields.Input{
			Summary: flag("summary"), Description: flag("description"), Priority: flag("priority"), Labels: flag("labels"),
			DueDate: flag("due-date"), Components: flag("components"), FixVersions: flag("fix-versions"),
			StoryPoints: flag("story-points"), StoryPointsField: flag("story-points-field"),
		})
		if err != nil {
			return commandError(cmd, "validation_error", err.(*issuefields.ValidationError).Message, issueKey, jira.MutationNotApplied, err)
		}
		client, domain, err := buildClient()
		if err != nil {
			return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", issueKey, jira.MutationNotApplied, err)
		}
		if prepared.NeedsMetadata() {
			meta, err := client.GetIssueEditMeta(issueKey)
			if err != nil {
				return commandError(cmd, "metadata_failed", "issue edit metadata is unavailable", issueKey, jira.MutationNotApplied, err)
			}
			if err := prepared.ValidateMetadata(meta); err != nil {
				return commandError(cmd, "validation_error", err.(*issuefields.ValidationError).Message, issueKey, jira.MutationNotApplied, err)
			}
		}
		if err := client.UpdateIssue(issueKey, prepared.Fields); err != nil {
			return commandError(cmd, "mutation_failed", "issue update failed", issueKey, jira.MutationStateOf(err), err)
		}
		if wantsJSON(cmd) {
			return writeMutationResultWithStoryPoints(cmd, client, domain, issueKey, prepared.StoryPointsField)
		}
		updated := make([]string, 0, len(prepared.Fields))
		for key := range prepared.Fields {
			updated = append(updated, key)
		}
		sort.Strings(updated)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Updated %s: %s\n", issueKey, strings.Join(updated, ", "))
		return nil
	},
}

func validateStoryPointsField(value string) error {
	return jira.ValidateCustomFieldID(value)
}

func init() {
	updateCmd.Flags().String("summary", "", "New summary")
	updateCmd.Flags().String("description", "", "New description (plain text; explicitly empty clears it)")
	updateCmd.Flags().String("priority", "", "New priority name (e.g. High, Critical)")
	updateCmd.Flags().String("labels", "", "Labels (comma-separated, replaces existing)")
	updateCmd.Flags().String("due-date", "", "Due date in YYYY-MM-DD format (explicitly empty clears it)")
	updateCmd.Flags().String("components", "", "Component names (comma-separated, replaces existing; explicitly empty clears)")
	updateCmd.Flags().String("fix-versions", "", "Fix version names (comma-separated, replaces existing; explicitly empty clears)")
	updateCmd.Flags().String("story-points", "", "Story points decimal (requires --story-points-field; explicitly empty clears)")
	updateCmd.Flags().String("story-points-field", "", "Jira story points field ID such as customfield_10016 (requires --story-points)")
	updateCmd.Flags().Bool("json", false, "Output the resulting issue as TicketJSON v1")
	rootCmd.AddCommand(updateCmd)
}
