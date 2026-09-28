package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

var boardsCmd = &cobra.Command{
	Use:   "boards",
	Short: "List your Jira boards",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := buildClient()
		if err != nil {
			return err
		}

		boards, err := client.GetBoards()
		if err != nil {
			return err
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			if boards == nil {
				boards = []jira.Board{}
			}
			return writeJSON(cmd, boards)
		} else {
			for _, b := range boards {
				project := ""
				if b.Location != nil && b.Location.ProjectKey != "" {
					project = fmt.Sprintf("  (%s)", b.Location.ProjectName)
				}
				fmt.Printf("%-6d  %-30s  [%s]%s\n", b.ID, b.Name, b.Type, project)
			}
		}
		return nil
	},
}

func init() {
	boardsCmd.Flags().Bool("json", false, "Output as JSON")
	rootCmd.AddCommand(boardsCmd)
}
