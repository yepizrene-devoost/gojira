package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List your Jira projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := BuildClient()
		if err != nil {
			return err
		}

		projects, err := client.GetProjects()
		if err != nil {
			return err
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			b, _ := json.MarshalIndent(projects, "", "  ")
			fmt.Println(string(b))
		} else {
			for _, p := range projects {
				fmt.Printf("%-8s  %s\n", p.Key, p.Name)
			}
		}
		return nil
	},
}

func init() {
	projectsCmd.Flags().Bool("json", false, "Output as JSON")
	rootCmd.AddCommand(projectsCmd)
}
