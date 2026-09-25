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
		client, _, err := BuildClient()
		if err != nil {
			return err
		}

		issueKey := args[0]
		text := args[1]
		mentions, _ := cmd.Flags().GetStringArray("mention")

		// Build ADF body: one paragraph with the text followed by inline
		// mention nodes. A mention notifies only when attrs.id carries the
		// resolved accountId; attrs.text is the display name.
		inline := []jira.ADFNode{{Type: "text", Text: text}}
		for _, email := range mentions {
			accountID, displayName, err := client.ResolveAccountID(email)
			if err != nil {
				return fmt.Errorf("resolving mention %s: %w", email, err)
			}
			inline = append(inline,
				jira.ADFNode{Type: "text", Text: " "},
				jira.ADFNode{Type: "mention", Attrs: &jira.ADFAttrs{ID: accountID, Text: displayName}},
			)
		}
		nodes := []jira.ADFNode{
			{Type: "paragraph", Content: inline},
		}

		body := jira.ADFDoc{Type: "doc", Version: 1, Content: nodes}
		if err := client.AddComment(issueKey, body); err != nil {
			return err
		}

		fmt.Printf("✓ Commented on %s", issueKey)
		if len(mentions) > 0 {
			fmt.Printf(" (mentioned %d user(s))", len(mentions))
		}
		fmt.Println()
		return nil
	},
}

func init() {
	commentCmd.Flags().StringArray("mention", nil, "Email to mention (can be repeated)")
	rootCmd.AddCommand(commentCmd)
}
