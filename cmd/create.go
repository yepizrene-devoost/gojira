package cmd

import (
	"fmt"
	"os"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

var parentIssueKey = regexp.MustCompile(`^[A-Z][A-Z0-9_]*-[1-9][0-9]*$`)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new Jira issue",
	Long: `Create a new issue. Example:

  gojira create --project ARA --type Task --summary "Fix login bug"
  gojira create --project ARA --type Bug --summary "Crash on load" --description-file report.md
  gojira create --project ARA --type Task --summary "New sprint work" --board 1
  gojira create --project ARA --parent ARA-123 --summary "Follow up"

With --parent, the issue type defaults to Sub-task and --board is not supported.
With --board, the new issue is also added to that board's active sprint;
without it, sprint-board issues land in the backlog.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		issueType, _ := cmd.Flags().GetString("type")
		summary, _ := cmd.Flags().GetString("summary")
		descRaw, _ := cmd.Flags().GetString("description")
		descFile, _ := cmd.Flags().GetString("description-file")
		parent, _ := cmd.Flags().GetString("parent")

		if project == "" || summary == "" {
			err := fmt.Errorf("--project and --summary are required")
			return commandError(cmd, "validation_error", "project and summary are required", "", jira.MutationNotApplied, err)
		}
		if cmd.Flags().Changed("parent") {
			if !parentIssueKey.MatchString(parent) {
				err := fmt.Errorf("--parent must be a Jira issue key (e.g. ARA-123)")
				return commandError(cmd, "validation_error", "parent issue key is invalid", "", jira.MutationNotApplied, err)
			}
			if cmd.Flags().Changed("board") {
				err := fmt.Errorf("--parent cannot be combined with --board")
				return commandError(cmd, "validation_error", "parent and board cannot be combined", "", jira.MutationNotApplied, err)
			}
			if cmd.Flags().Changed("type") && issueType != "Sub-task" {
				err := fmt.Errorf("--parent requires --type Sub-task")
				return commandError(cmd, "validation_error", "parent requires issue type Sub-task", "", jira.MutationNotApplied, err)
			}
			issueType = "Sub-task"
		} else if issueType == "" {
			issueType = "Task"
		}

		var desc *jira.ADFDoc
		text := descRaw
		if descFile != "" {
			b, err := os.ReadFile(descFile)
			if err != nil {
				wrapped := fmt.Errorf("reading description file: %w", err)
				return commandError(cmd, "input_error", "description file could not be read", "", jira.MutationNotApplied, wrapped)
			}
			text = string(b)
		}
		if text != "" {
			doc := jira.TextToADF(text)
			desc = &doc
		}

		client, domain, err := buildClient()
		if err != nil {
			return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", "", jira.MutationNotApplied, err)
		}

		var key string
		if parent != "" {
			key, err = client.CreateIssueWithParent(project, issueType, summary, desc, parent)
		} else {
			key, err = client.CreateIssue(project, issueType, summary, desc)
		}
		if err != nil {
			return commandError(cmd, "mutation_failed", "issue creation failed", key, jira.MutationStateOf(err), err)
		}
		if !wantsJSON(cmd) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Created %s\n", key)
		}

		boardID, _ := cmd.Flags().GetInt("board")
		if boardID > 0 {
			sprints, err := client.GetSprints(boardID)
			if err != nil {
				wrapped := fmt.Errorf("created %s but could not read sprints for board %d: %w", key, boardID, err)
				return commandError(cmd, "partial_failure", "issue created but active sprint lookup failed", key, jira.MutationApplied, wrapped)
			}
			var active *jira.Sprint
			for i := range sprints {
				if sprints[i].State == "active" {
					active = &sprints[i]
					break
				}
			}
			if active == nil {
				err := fmt.Errorf("created %s but board %d has no active sprint (issue is in the backlog)", key, boardID)
				return commandError(cmd, "partial_failure", "issue created but the board has no active sprint", key, jira.MutationApplied, err)
			}
			if err := client.AddIssuesToSprint(active.ID, []string{key}); err != nil {
				wrapped := fmt.Errorf("created %s but could not add it to sprint %q: %w", key, active.Name, err)
				return commandError(cmd, "partial_failure", "issue created but adding it to the active sprint failed", key, jira.MutationApplied, wrapped)
			}
			if !wantsJSON(cmd) {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Added %s to active sprint %q\n", key, active.Name)
			}
		}

		if wantsJSON(cmd) {
			return writeMutationResult(cmd, client, domain, key)
		}
		return nil
	},
}

func init() {
	createCmd.Flags().String("project", "", "Project key (e.g. ARA)")
	createCmd.Flags().String("type", "", "Issue type (default: Task, or Sub-task with --parent)")
	createCmd.Flags().String("parent", "", "Parent issue key for a Sub-task (cannot be used with --board)")
	createCmd.Flags().String("summary", "", "Issue summary")
	createCmd.Flags().String("description", "", "Description text")
	createCmd.Flags().String("description-file", "", "Path to description file (markdown)")
	createCmd.Flags().Int("board", 0, "Also add the new issue to this board's active sprint")
	createCmd.Flags().Bool("json", false, "Output the resulting issue as TicketJSON v1")
	rootCmd.AddCommand(createCmd)
}
