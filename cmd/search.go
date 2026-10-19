package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

var searchCmd = &cobra.Command{
	Use:   "search <jql>",
	Short: "Search issues using JQL",
	Long: `Search Jira issues with a JQL query. Examples:

  gojira search "assignee = currentUser() AND status != Done"
  gojira search "project = ARA AND priority = High" --limit 10
  gojira search "text ~ 'login bug'" --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		meta, _ := cmd.Flags().GetBool("meta")
		all, _ := cmd.Flags().GetBool("all")
		if meta {
			_ = cmd.Flags().Set("json", "true")
		}
		if all && cmd.Flags().Changed("limit") {
			return commandError(cmd, "validation_error", "--all cannot be combined with --limit", "", jira.MutationNotApplied, fmt.Errorf("--all cannot be combined with --limit"))
		}
		client, domain, err := buildClient()
		if err != nil {
			return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", "", jira.MutationNotApplied, err)
		}

		jql := args[0]
		limit, _ := cmd.Flags().GetInt("limit")

		if all {
			limit = 10000
		}
		result, err := client.SearchIssues(jql, limit)
		if err != nil {
			return commandError(cmd, "read_failed", "issues could not be searched", "", jira.MutationNotApplied, err)
		}

		issues := result.Issues
		if wantsJSON(cmd) {
			var outputErr error
			if meta {
				outputErr = writeJSON(cmd, searchMetadata{"v1", jira.IssuesToTicketJSON(issues, domain), len(issues), result.Truncated, result.EstimatedTotal})
			} else {
				outputErr = writeSearchJSON(cmd, issues, domain)
			}
			if err := outputErr; err != nil {
				return reportJSONError(cmd, "output_failed", "search JSON could not be written", "", jira.MutationNotApplied, err)
			}
			if result.Truncated && !meta {
				if err := writeJSONTo(cmd.ErrOrStderr(), searchWarning{"v1", "results_truncated", len(issues), result.EstimatedTotal, searchTruncationHint}); err != nil {
					return reportJSONError(cmd, "output_failed", "search warning could not be written", "", jira.MutationNotApplied, err)
				}
			}
			return nil
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Showing %d issues\n\n", len(issues))
		for _, iss := range issues {
			status := ""
			priority := ""
			assignee := "Unassigned"
			if iss.Fields.Status != nil {
				status = iss.Fields.Status.Name
			}
			if iss.Fields.Priority != nil {
				priority = iss.Fields.Priority.Name
			}
			if iss.Fields.Assignee != nil {
				assignee = iss.Fields.Assignee.DisplayName
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %-20s %-12s %-15s %s\n",
				iss.Key, truncate(status, 20), truncate(priority, 12), truncate(assignee, 15), iss.Fields.Summary)
		}

		if result.Truncated {
			message := "More results exist"
			if result.EstimatedTotal != nil {
				message += fmt.Sprintf(" (about %d in total)", *result.EstimatedTotal)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), message)
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Warning:", searchTruncationHint)
		}

		return nil
	},
}

const searchTruncationHint = "Rerun with --all or a larger --limit, or narrow the JQL; do not report this list as complete."

type searchMetadata struct {
	SchemaVersion  string            `json:"schemaVersion"`
	Issues         []jira.TicketJSON `json:"issues"`
	Returned       int               `json:"returned"`
	Truncated      bool              `json:"truncated"`
	EstimatedTotal *int              `json:"estimatedTotal,omitempty"`
}

type searchWarning struct {
	SchemaVersion  string `json:"schemaVersion"`
	Code           string `json:"code"`
	Returned       int    `json:"returned"`
	EstimatedTotal *int   `json:"estimatedTotal,omitempty"`
	Hint           string `json:"hint"`
}

func writeSearchJSON(cmd *cobra.Command, issues []jira.Issue, domain string) error {
	return writeJSON(cmd, jira.IssuesToTicketJSON(issues, domain))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func init() {
	searchCmd.Flags().Int("limit", 50, "Maximum results to return")
	searchCmd.Flags().Bool("json", false, "Output as JSON")
	searchCmd.Flags().Bool("all", false, "Fetch all results up to 10,000 issues")
	searchCmd.Flags().Bool("meta", false, "Output JSON with completeness metadata")
	rootCmd.AddCommand(searchCmd)
}
