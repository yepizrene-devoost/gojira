package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

var commentCmd = &cobra.Command{
	Use:   "comment <issue-key> <text>",
	Short: "Add a comment to a Jira issue",
	Long: `Add a comment to an issue, with optional mentions. Examples:

  gojira comment ARA-1892 "This is fixed, ready for review"
  gojira comment ARA-1892 "Can you check this?" --mention pm@devoost.com
  gojira comment ARA-1892 "LGTM" --mention rene@devoost.com --mention dev@devoost.com`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueKey := args[0]
		text := args[1]
		mentions, _ := cmd.Flags().GetStringArray("mention")
		client, domain, err := buildClient()
		if err != nil {
			return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", issueKey, jira.MutationNotApplied, err)
		}

		inline := []jira.ADFNode{{Type: "text", Text: text}}
		for _, email := range mentions {
			accountID, displayName, err := client.ResolveAccountID(email)
			if err != nil {
				wrapped := fmt.Errorf("resolving mention %s: %w", email, err)
				return commandError(cmd, "lookup_failed", "mentioned user could not be resolved", issueKey, jira.MutationNotApplied, wrapped)
			}
			inline = append(inline,
				jira.ADFNode{Type: "text", Text: " "},
				jira.ADFNode{Type: "mention", Attrs: &jira.ADFAttrs{ID: accountID, Text: displayName}},
			)
		}
		body := jira.ADFDoc{
			Type:    "doc",
			Version: 1,
			Content: []jira.ADFNode{{Type: "paragraph", Content: inline}},
		}
		if err := client.AddComment(issueKey, body); err != nil {
			return commandError(cmd, "mutation_failed", "comment creation failed", issueKey, jira.MutationStateOf(err), err)
		}
		if wantsJSON(cmd) {
			return writeMutationResult(cmd, client, domain, issueKey)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Commented on %s", issueKey)
		if len(mentions) > 0 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), " (mentioned %d user(s))", len(mentions))
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout())
		return nil
	},
}

func init() {
	commentCmd.Flags().StringArray("mention", nil, "Email to mention (can be repeated)")
	commentCmd.Flags().Bool("json", false, "Output the resulting issue as TicketJSON v1")
	rootCmd.AddCommand(commentCmd)
}
