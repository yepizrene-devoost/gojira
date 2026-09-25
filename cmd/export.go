package cmd

import (
	"encoding/json"
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
		client, _, err := BuildClient()
		if err != nil {
			return err
		}

		boardID, _ := cmd.Flags().GetInt("board")
		sprintID, _ := cmd.Flags().GetInt("sprint")

		if boardID == 0 {
			boards, err := client.GetBoards()
			if err != nil {
				return err
			}
			if len(boards) == 0 {
				return fmt.Errorf("no boards found")
			}
			boardID = boards[0].ID
			fmt.Fprintf(os.Stderr, "Using board: %s (id=%d)\n", boards[0].Name, boardID)
		}

		if sprintID == 0 {
			sprints, err := client.GetSprints(boardID)
			if err != nil {
				return err
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
			return err
		}

		tickets := jira.IssuesToTicketJSON(issues)

		b, _ := json.MarshalIndent(tickets, "", "  ")
		fmt.Println(string(b))
		return nil
	},
}

func init() {
	exportCmd.Flags().IntP("board", "b", 0, "Board ID (default: first board)")
	exportCmd.Flags().IntP("sprint", "s", 0, "Sprint ID (default: active sprint)")
	rootCmd.AddCommand(exportCmd)
}
