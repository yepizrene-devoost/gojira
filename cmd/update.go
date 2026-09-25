package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update <issue-key>",
	Short: "Update fields on a Jira issue",
	Long: `Update one or more fields on an existing issue. Examples:

  gojira update ARA-1892 --summary "New summary text"
  gojira update ARA-1892 --priority High
  gojira update ARA-1892 --labels "frontend,urgent"
  gojira update ARA-1892 --summary "..." --priority Critical --labels "p0"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := BuildClient()
		if err != nil {
			return err
		}

		issueKey := args[0]
		fields := map[string]interface{}{}

		if s, _ := cmd.Flags().GetString("summary"); s != "" {
			fields["summary"] = s
		}
		if p, _ := cmd.Flags().GetString("priority"); p != "" {
			fields["priority"] = map[string]string{"name": p}
		}
		if l, _ := cmd.Flags().GetString("labels"); l != "" {
			labels := strings.Split(l, ",")
			for i := range labels {
				labels[i] = strings.TrimSpace(labels[i])
			}
			fields["labels"] = labels
		}

		if len(fields) == 0 {
			return fmt.Errorf("specify at least one field to update (--summary, --priority, --labels)")
		}

		if err := client.UpdateIssue(issueKey, fields); err != nil {
			return err
		}

		updated := make([]string, 0, len(fields))
		for k := range fields {
			updated = append(updated, k)
		}
		fmt.Printf("✓ Updated %s: %s\n", issueKey, strings.Join(updated, ", "))
		return nil
	},
}

func init() {
	updateCmd.Flags().String("summary", "", "New summary")
	updateCmd.Flags().String("priority", "", "New priority name (e.g. High, Critical)")
	updateCmd.Flags().String("labels", "", "Labels (comma-separated, replaces existing)")
	rootCmd.AddCommand(updateCmd)
}
