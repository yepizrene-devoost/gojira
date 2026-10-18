package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yepizrene-devoost/gojira/internal/reporting"
)

func newReportCommand() *cobra.Command {
	parent := &cobra.Command{Use: "report", Short: "Report visible Jira data"}
	command := &cobra.Command{Use: "time", Short: "Sum visible worklogs for an explicit account and date range", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		get := func(name string) string { v, _ := cmd.Flags().GetString(name); return v }
		filters := reporting.TimeFilters{AccountID: get("account-id"), Project: get("project"), From: get("from"), To: get("to"), Timezone: get("timezone")}
		if err := reporting.ValidateTimeFilters(filters); err != nil {
			return reportingError(cmd, "validation_error", err.Error(), err)
		}
		client, _, err := buildClient()
		if err != nil {
			return reportingError(cmd, "read_failed", "Jira credentials unavailable", err)
		}
		value, err := reporting.TimeReport(cmd.Context(), client, filters)
		if err != nil {
			return reportingError(cmd, "read_failed", "Time report failed", err)
		}
		if wantsJSON(cmd) {
			return writeJSON(cmd, value)
		}
		if !value.Complete {
			if _, err = fmt.Fprintln(cmd.OutOrStdout(), "PARTIAL — totals are incomplete"); err != nil {
				return err
			}
		}
		if _, err = fmt.Fprintln(cmd.OutOrStdout(), "ISSUE\tSECONDS"); err != nil {
			return err
		}
		for _, row := range value.Rows {
			if _, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d\n", row.Key, row.Seconds); err != nil {
				return err
			}
		}
		if _, err = fmt.Fprintf(cmd.OutOrStdout(), "TOTAL SECONDS\t%d\n", value.TotalSeconds); err != nil {
			return err
		}
		return reportingWarnings(cmd, value.Warnings)
	}}
	for _, flag := range []string{"account-id", "project", "from", "to", "timezone"} {
		command.Flags().String(flag, "", "Explicit "+flag+" filter")
	}
	command.Flags().Bool("json", false, "Print structured JSON")
	parent.AddCommand(command)
	return parent
}

func init() { rootCmd.AddCommand(newReportCommand()) }
