package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yepizrene-devoost/gojira/internal/reporting"
)

func newUsersCommand() *cobra.Command {
	parent := &cobra.Command{Use: "users", Short: "Find visible Jira user candidates"}
	search := &cobra.Command{Use: "search QUERY", Short: "Search user candidates without selecting an identity", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := reporting.ValidateUserQuery(args[0]); err != nil {
			return reportingError(cmd, "validation_error", err.Error(), err)
		}
		client, _, err := buildClient()
		if err != nil {
			return reportingError(cmd, "read_failed", "Jira credentials unavailable", err)
		}
		value, err := reporting.SearchUsers(cmd.Context(), client, args[0])
		if err != nil {
			return reportingError(cmd, "read_failed", "User search failed", err)
		}
		if wantsJSON(cmd) {
			return writeJSON(cmd, value)
		}
		if value.Completeness == "partial" {
			if _, err = fmt.Fprintln(cmd.OutOrStdout(), "PARTIAL — candidates are incomplete"); err != nil {
				return err
			}
		}
		if _, err = fmt.Fprintln(cmd.OutOrStdout(), "ACCOUNT ID\tDISPLAY NAME\tACTIVE"); err != nil {
			return err
		}
		for _, u := range value.Candidates {
			if _, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%t\n", u.AccountID, u.DisplayName, u.Active); err != nil {
				return err
			}
		}
		if len(value.Candidates) == 0 {
			if _, err = fmt.Fprintln(cmd.OutOrStdout(), "No visible candidates returned; this does not prove that no matching user exists."); err != nil {
				return err
			}
		}
		return reportingWarnings(cmd, value.Warnings)
	}}
	search.Flags().Bool("json", false, "Print structured JSON")
	parent.AddCommand(search)
	return parent
}

func init() { rootCmd.AddCommand(newUsersCommand()) }
