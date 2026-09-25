package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new Jira issue",
	Long: `Create a new issue. Example:

  gojira create --project ARA --type Task --summary "Fix login bug"
  gojira create --project ARA --type Bug --summary "Crash on load" --description-file report.md
  gojira create --project ARA --type Task --summary "New sprint work" --board 1

With --board, the new issue is also added to that board's active sprint;
without it, sprint-board issues land in the backlog.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := BuildClient()
		if err != nil {
			return err
		}

		project, _ := cmd.Flags().GetString("project")
		issueType, _ := cmd.Flags().GetString("type")
		summary, _ := cmd.Flags().GetString("summary")
		descRaw, _ := cmd.Flags().GetString("description")
		descFile, _ := cmd.Flags().GetString("description-file")

		if project == "" || summary == "" {
			return fmt.Errorf("--project and --summary are required")
		}
		if issueType == "" {
			issueType = "Task"
		}

		// Build description from flag or file
		var desc *jira.ADFDoc
		text := descRaw
		if descFile != "" {
			b, err := os.ReadFile(descFile)
			if err != nil {
				return fmt.Errorf("reading description file: %w", err)
			}
			text = string(b)
		}
		if text != "" {
			doc := jira.TextToADF(text)
			desc = &doc
		}

		key, err := client.CreateIssue(project, issueType, summary, desc)
		if err != nil {
			return err
		}
		fmt.Printf("✓ Created %s\n", key)

		// Optionally place the new issue in a board's active sprint.
		boardID, _ := cmd.Flags().GetInt("board")
		if boardID > 0 {
			sprints, err := client.GetSprints(boardID)
			if err != nil {
				return fmt.Errorf("created %s but could not read sprints for board %d: %w", key, boardID, err)
			}
			var active *jira.Sprint
			for i := range sprints {
				if sprints[i].State == "active" {
					active = &sprints[i]
					break
				}
			}
			if active == nil {
				return fmt.Errorf("created %s but board %d has no active sprint (issue is in the backlog)", key, boardID)
			}
			if err := client.AddIssuesToSprint(active.ID, []string{key}); err != nil {
				return fmt.Errorf("created %s but could not add it to sprint %q: %w", key, active.Name, err)
			}
			fmt.Printf("✓ Added %s to active sprint %q\n", key, active.Name)
		}
		return nil
	},
}

func init() {
	createCmd.Flags().String("project", "", "Project key (e.g. ARA)")
	createCmd.Flags().String("type", "", "Issue type (default: Task)")
	createCmd.Flags().String("summary", "", "Issue summary")
	createCmd.Flags().String("description", "", "Description text")
	createCmd.Flags().String("description-file", "", "Path to description file (markdown)")
	createCmd.Flags().Int("board", 0, "Also add the new issue to this board's active sprint")
	rootCmd.AddCommand(createCmd)
}
