package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var boardsCmd = &cobra.Command{
	Use:   "boards",
	Short: "List your Jira boards",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := BuildClient()
		if err != nil {
			return err
		}

		boards, err := client.GetBoards()
		if err != nil {
			return err
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			b, _ := json.MarshalIndent(boards, "", "  ")
			fmt.Println(string(b))
		} else {
			for _, b := range boards {
				fmt.Printf("%-6d  %-30s  [%s]\n", b.ID, b.Name, b.Type)
			}
		}
		return nil
	},
}

func init() {
	boardsCmd.Flags().Bool("json", false, "Output as JSON")
	rootCmd.AddCommand(boardsCmd)
}
