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
			return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", "", jira.MutationNotApplied, err)
		}

		boards, err := client.GetBoards()
		if err != nil {
			return commandError(cmd, "read_failed", "boards could not be read", "", jira.MutationNotApplied, err)
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			if boards == nil {
				boards = []jira.Board{}
			}
			if err := writeJSON(cmd, boards); err != nil {
				return reportJSONError(cmd, "output_failed", "boards JSON could not be written", "", jira.MutationNotApplied, err)
			}
			return nil
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
