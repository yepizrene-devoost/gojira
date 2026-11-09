package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

// commentADFWarning states the supported editing semantics (issue #39 D1):
// editing replaces the whole body with a new plain-text ADF document, so rich
// formatting, mentions and visibility restrictions of the previous body are
// lost. It is printed on stderr only, never on `--json` stdout.
const commentADFWarning = "warning: editing replaces the whole comment body with plain text as ADF; rich formatting, mentions and visibility restrictions of the previous body are lost"

var commentCmd = &cobra.Command{
	Use:   "comment <issue-key> [text]",
	Short: "Add, list, read, edit or delete comments on a Jira issue",
	Long: `Add a comment to an issue, with optional mentions, or manage existing
comments by their stable id.

  gojira comment ARA-1892 "This is fixed, ready for review"
  gojira comment ARA-1892 "Can you check this?" --mention pm@devoost.com
  gojira comment ARA-1892 --list
  gojira comment ARA-1892 --id 10001
  gojira comment ARA-1892 --id 10001 --text "updated text"
  gojira comment ARA-1892 --id 10001 --delete --confirm 10001

--id alone reads one comment; with --text it edits that comment. Editing
replaces the whole comment body with the given text as an ADF
document; rich formatting, mentions and visibility restrictions of the
previous body are lost. Deleting a comment is destructive and requires
repeating the comment id with --confirm.`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueKey := args[0]
		text := ""
		if len(args) == 2 {
			text = args[1]
		}
		mentions, _ := cmd.Flags().GetStringArray("mention")
		list, _ := cmd.Flags().GetBool("list")
		commentID, _ := cmd.Flags().GetString("id")
		newText, _ := cmd.Flags().GetString("text")
		deleteComment, _ := cmd.Flags().GetBool("delete")
		confirm, _ := cmd.Flags().GetString("confirm")

		// Intent validation happens before the client is built, so no request
		// is ever sent for an inconsistent combination (issue #39 S4/S5).
		if list {
			if commentID != "" {
				return commandError(cmd, "validation_error", "--list cannot be combined with --id", issueKey, jira.MutationNotApplied, fmt.Errorf("--list cannot be combined with --id"))
			}
			if text != "" || newText != "" || deleteComment || confirm != "" {
				return commandError(cmd, "validation_error", "--list cannot be combined with write flags", issueKey, jira.MutationNotApplied, fmt.Errorf("--list cannot be combined with write flags"))
			}
		}
		if deleteComment {
			if commentID == "" {
				return commandError(cmd, "validation_error", "--delete requires --id", issueKey, jira.MutationNotApplied, fmt.Errorf("--delete requires --id"))
			}
			if text != "" || newText != "" {
				return commandError(cmd, "validation_error", "--delete cannot be combined with --text", issueKey, jira.MutationNotApplied, fmt.Errorf("--delete cannot be combined with --text"))
			}
			if confirm != commentID {
				return commandError(cmd, "validation_error", "--confirm must repeat the comment id", issueKey, jira.MutationNotApplied, fmt.Errorf("--confirm must repeat the comment id"))
			}
		}
		if !deleteComment && confirm != "" {
			return commandError(cmd, "validation_error", "--confirm is only valid with --delete", issueKey, jira.MutationNotApplied, fmt.Errorf("--confirm is only valid with --delete"))
		}
		if commentID == "" && (newText != "" || deleteComment) {
			return commandError(cmd, "validation_error", "--text and --delete require --id", issueKey, jira.MutationNotApplied, fmt.Errorf("--text and --delete require --id"))
		}
		if commentID == "" && (newText != "" || deleteComment) {
			return commandError(cmd, "validation_error", "--text and --delete require --id", issueKey, jira.MutationNotApplied, fmt.Errorf("--text and --delete require --id"))
		}
		if commentID != "" && (text != "" || len(mentions) > 0) {
			return commandError(cmd, "validation_error", "--id cannot be combined with an inline comment text or --mention", issueKey, jira.MutationNotApplied, fmt.Errorf("--id cannot be combined with an inline comment text or --mention"))
		}
		// Plain add keeps its original two-argument shape.
		if commentID == "" && !list && text == "" {
			return commandError(cmd, "validation_error", "comment text is required", issueKey, jira.MutationNotApplied, fmt.Errorf("comment text is required"))
		}

		client, domain, err := buildClient()
		if err != nil {
			return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", issueKey, jira.MutationNotApplied, err)
		}

		switch {
		case list:
			return runCommentList(cmd, client, issueKey)
		case deleteComment:
			return runCommentDelete(cmd, client, domain, issueKey, commentID)
		case commentID != "" && newText != "":
			return runCommentEdit(cmd, client, domain, issueKey, commentID, newText)
		case commentID != "":
			return runCommentRead(cmd, client, issueKey, commentID)
		default:
			return runCommentAdd(cmd, client, domain, issueKey, text, mentions)
		}
	},
}

func runCommentList(cmd *cobra.Command, client *jira.Client, issueKey string) error {
	comments, total, err := client.ListComments(issueKey)
	if err != nil {
		return commandError(cmd, "read_failed", "comments could not be read", issueKey, jira.MutationNotApplied, err)
	}
	if wantsJSON(cmd) {
		if comments == nil {
			comments = []jira.Comment{}
		}
		docs := make([]jira.CommentJSON, len(comments))
		for i, c := range comments {
			docs[i] = jira.CommentJSON{ID: c.ID, Author: c.Author.DisplayName, Body: c.Body.Flatten(), Created: c.Created}
		}
		result := struct {
			SchemaVersion string             `json:"schemaVersion"`
			IssueKey      string             `json:"issueKey"`
			Total         int                `json:"total"`
			Comments      []jira.CommentJSON `json:"comments"`
		}{SchemaVersion: "v1", IssueKey: issueKey, Total: total, Comments: docs}
		if err := writeJSON(cmd, result); err != nil {
			return reportJSONError(cmd, "output_failed", "comment JSON could not be written", issueKey, jira.MutationNotApplied, err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Comments for %s (%d):\n\n", issueKey, total)
	for _, c := range comments {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  [%s]  %s  %s\n      %s\n", c.ID, c.Created, c.Author.DisplayName, strings.ReplaceAll(c.Body.Flatten(), "\n", "\n      "))
	}
	return nil
}

func runCommentRead(cmd *cobra.Command, client *jira.Client, issueKey, commentID string) error {
	comment, err := client.GetComment(issueKey, commentID)
	if err != nil {
		return commandError(cmd, "read_failed", "comment could not be read", issueKey, jira.MutationNotApplied, err)
	}
	if wantsJSON(cmd) {
		if err := writeJSON(cmd, jira.CommentJSON{ID: comment.ID, Author: comment.Author.DisplayName, Body: comment.Body.Flatten(), Created: comment.Created}); err != nil {
			return reportJSONError(cmd, "output_failed", "comment JSON could not be written", issueKey, jira.MutationNotApplied, err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[%s]  %s  %s\n%s\n", comment.ID, comment.Created, comment.Author.DisplayName, comment.Body.Flatten())
	return nil
}

func runCommentEdit(cmd *cobra.Command, client *jira.Client, domain, issueKey, commentID, text string) error {
	if err := client.UpdateComment(issueKey, commentID, text); err != nil {
		return commandError(cmd, "mutation_failed", "comment update failed", issueKey, jira.MutationStateOf(err), err)
	}
	if wantsJSON(cmd) {
		if err := writeMutationResult(cmd, client, domain, issueKey); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), commentADFWarning)
		return nil
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Updated comment %s on %s\n", commentID, issueKey)
	_, _ = fmt.Fprintln(cmd.ErrOrStderr(), commentADFWarning)
	return nil
}

func runCommentDelete(cmd *cobra.Command, client *jira.Client, domain, issueKey, commentID string) error {
	if err := client.DeleteComment(issueKey, commentID); err != nil {
		return commandError(cmd, "mutation_failed", "comment deletion failed", issueKey, jira.MutationStateOf(err), err)
	}
	if wantsJSON(cmd) {
		return writeMutationResult(cmd, client, domain, issueKey)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Deleted comment %s on %s\n", commentID, issueKey)
	return nil
}

func runCommentAdd(cmd *cobra.Command, client *jira.Client, domain, issueKey, text string, mentions []string) error {
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
}

func init() {
	commentCmd.Flags().StringArray("mention", nil, "Email to mention (can be repeated)")
	commentCmd.Flags().Bool("json", false, "Output the resulting issue as TicketJSON v1")
	commentCmd.Flags().Bool("list", false, "List every comment of the issue (cannot be combined with --id)")
	commentCmd.Flags().String("id", "", "Comment id: read it, edit it with --text, or delete it with --delete")
	commentCmd.Flags().String("text", "", "Replacement text for the comment given by --id (sent as an ADF document; omit --text to read the comment)")
	commentCmd.Flags().Bool("delete", false, "Delete the comment given by --id (requires --confirm <id>)")
	commentCmd.Flags().String("confirm", "", "Confirmation for --delete: must repeat the comment id")
	rootCmd.AddCommand(commentCmd)
}
