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
		client, domain, err := buildClient()
		if err != nil {
			return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", "", jira.MutationNotApplied, err)
		}

		jql := args[0]
		limit, _ := cmd.Flags().GetInt("limit")

		issues, total, err := client.SearchJQL(jql, limit)
		if err != nil {
			return commandError(cmd, "read_failed", "issues could not be searched", "", jira.MutationNotApplied, err)
		}

		if wantsJSON(cmd) {
			if err := writeSearchJSON(cmd, issues, domain); err != nil {
				return reportJSONError(cmd, "output_failed", "search JSON could not be written", "", jira.MutationNotApplied, err)
			}
			return nil
		}

		// Table output
		shown := len(issues)
		if total == 0 {
			total = shown // API sometimes omits total
		}
		fmt.Printf("Found %d issues (showing %d):\n\n", total, shown)
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
			fmt.Printf("  %-12s %-20s %-12s %-15s %s\n",
				iss.Key, truncate(status, 20), truncate(priority, 12), truncate(assignee, 15), iss.Fields.Summary)
		}

		if total > len(issues) {
			fmt.Printf("\n  ... and %d more (use --limit to show more)\n", total-len(issues))
		}

		return nil
	},
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
	rootCmd.AddCommand(searchCmd)
}
