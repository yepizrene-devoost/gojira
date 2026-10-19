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
			return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", "", jira.MutationNotApplied, err)
		}

		projects, err := client.GetProjects()
		if err != nil {
			return commandError(cmd, "read_failed", "projects could not be read", "", jira.MutationNotApplied, err)
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			if projects == nil {
				projects = []jira.Project{}
			}
			if err := writeJSON(cmd, projects); err != nil {
				return reportJSONError(cmd, "output_failed", "projects JSON could not be written", "", jira.MutationNotApplied, err)
			}
			return nil
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
