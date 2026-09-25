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
  gojira create --project ARA --type Bug --summary "Crash on load" --description-file report.md`,
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
		return nil
	},
}

func init() {
	createCmd.Flags().String("project", "", "Project key (e.g. ARA)")
	createCmd.Flags().String("type", "", "Issue type (default: Task)")
	createCmd.Flags().String("summary", "", "Issue summary")
	createCmd.Flags().String("description", "", "Description text")
	createCmd.Flags().String("description-file", "", "Path to description file (markdown)")
	rootCmd.AddCommand(createCmd)
}
