package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export tickets from a board/sprint as JSON",
	Long:  "Export tickets for agent consumption. Example: gojira export --board 1",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, domain, err := buildClient()
		if err != nil {
			return reportJSONError(cmd, "configuration_error", "Jira client configuration is unavailable", "", jira.MutationNotApplied, err)
		}

		boardID, _ := cmd.Flags().GetInt("board")
		sprintID, _ := cmd.Flags().GetInt("sprint")

		if boardID == 0 {
			boards, err := client.GetBoards()
			if err != nil {
				return reportJSONError(cmd, "read_failed", "boards could not be read", "", jira.MutationNotApplied, err)
			}
			if len(boards) == 0 {
				return reportJSONError(cmd, "validation_error", "no boards found", "", jira.MutationNotApplied, fmt.Errorf("no boards found"))
			}
			boardID = boards[0].ID
			fmt.Fprintf(os.Stderr, "Using board: %s (id=%d)\n", boards[0].Name, boardID)
		}

		if sprintID == 0 {
			sprints, err := client.GetSprints(boardID)
			if err != nil {
				return reportJSONError(cmd, "read_failed", "sprints could not be read", "", jira.MutationNotApplied, err)
			}
			for _, s := range sprints {
				if s.State == "active" {
					sprintID = s.ID
					fmt.Fprintf(os.Stderr, "Using sprint: %s (id=%d)\n", s.Name, sprintID)
					break
				}
			}
		}

		issues, err := client.GetBoardIssues(boardID, sprintID)
		if err != nil {
			return reportJSONError(cmd, "read_failed", "board issues could not be read", "", jira.MutationNotApplied, err)
		}

		if err := writeExportJSON(cmd, issues, domain); err != nil {
			return reportJSONError(cmd, "output_failed", "export JSON could not be written", "", jira.MutationNotApplied, err)
		}
		return nil
	},
}

func writeExportJSON(cmd *cobra.Command, issues []jira.Issue, domain string) error {
	return writeJSON(cmd, jira.IssuesToTicketJSON(issues, domain))
}

func init() {
	exportCmd.Flags().IntP("board", "b", 0, "Board ID (default: first board)")
	exportCmd.Flags().IntP("sprint", "s", 0, "Sprint ID (default: active sprint)")
	rootCmd.AddCommand(exportCmd)
}
