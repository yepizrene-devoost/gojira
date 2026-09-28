package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List your Jira projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := buildClient()
		if err != nil {
			return err
		}

		projects, err := client.GetProjects()
		if err != nil {
			return err
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			if projects == nil {
				projects = []jira.Project{}
			}
			return writeJSON(cmd, projects)
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
