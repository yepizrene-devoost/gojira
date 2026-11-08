package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/attachments"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

type attachmentJSON struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

type attachResultJSON struct {
	SchemaVersion string           `json:"schemaVersion"`
	IssueKey      string           `json:"issueKey"`
	Attachments   []attachmentJSON `json:"attachments"`
}

// attachState reports the mutation state of a failed upload. Validation and
// settings errors never reach a write request, so they are not applied.
func attachState(err error) jira.MutationState {
	var mutationErr *jira.MutationError
	if errors.As(err, &mutationErr) {
		return mutationErr.State
	}
	return jira.MutationNotApplied
}

// partialState is the mutation state of a run that stopped after some files
// were already uploaded: those uploads are applied whatever happened to the
// failing one, unless that one is itself unknown.
func partialState(err error) jira.MutationState {
	if state := attachState(err); state == jira.MutationUnknown {
		return state
	}
	return jira.MutationApplied
}

// uploadedNames lists the local files that were sent, not the names Jira
// echoes back, because Jira may answer with no details at all.
func uploadedNames(results []attachments.Result) []string {
	names := make([]string, 0, len(results))
	for _, result := range results {
		names = append(names, filepath.Base(result.Path))
	}
	return names
}

var attachCmd = &cobra.Command{
	Use:   "attach <issue-key> <file>...",
	Short: "Attach one or more files to a Jira issue",
	Long: `Upload files as attachments to an issue. Every path and the instance
limits are checked before the first upload; each file is then sent in its own
request. Examples:

  gojira attach ARA-1892 ./screenshot.png
  gojira attach ARA-1892 ./logs/app.log ./report.pdf --json`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueKey := args[0]
		paths := args[1:]
		client, _, err := buildClient()
		if err != nil {
			return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", issueKey, jira.MutationNotApplied, err)
		}

		results, err := attachments.Upload(client, issueKey, paths)
		if err != nil {
			if len(results) > 0 {
				names := uploadedNames(results)
				message := fmt.Sprintf("attachment upload stopped after %d of %d file(s); uploaded: %s", len(results), len(paths), strings.Join(names, ", "))
				wrapped := fmt.Errorf("%s: %w", message, err)
				return commandError(cmd, "partial_failure", message, issueKey, partialState(err), wrapped)
			}
			return commandError(cmd, "upload_failed", "attachment upload failed", issueKey, attachState(err), err)
		}
		if wantsJSON(cmd) {
			out := attachResultJSON{SchemaVersion: jsonSchemaVersion, IssueKey: issueKey, Attachments: []attachmentJSON{}}
			for _, result := range results {
				for _, uploaded := range result.Attachments {
					out.Attachments = append(out.Attachments, attachmentJSON{ID: uploaded.ID, Filename: uploaded.Filename, Size: uploaded.Size})
				}
			}
			return writeJSON(cmd, out)
		}
		for _, result := range results {
			if len(result.Attachments) == 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Attached %s to %s\n", filepath.Base(result.Path), issueKey)
				continue
			}
			for _, uploaded := range result.Attachments {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Attached %s to %s (id %s, %d bytes)\n", uploaded.Filename, issueKey, uploaded.ID, uploaded.Size)
			}
		}
		return nil
	},
}

func init() {
	attachCmd.Flags().Bool("json", false, "Output the uploaded attachments as JSON")
	rootCmd.AddCommand(attachCmd)
}
