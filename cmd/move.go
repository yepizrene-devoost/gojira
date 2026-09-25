package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

var moveCmd = &cobra.Command{
	Use:   "move <issue-key> --to <status>",
	Short: "Transition a ticket to a new status",
	Long:  "Move a ticket to a different status. Example: gojira move ARA-1892 --to '03 In Progress'",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := BuildClient()
		if err != nil {
			return err
		}

		issueKey := args[0]
		targetStatus, _ := cmd.Flags().GetString("to")

		if targetStatus == "" {
			transitions, err := client.GetTransitions(issueKey)
			if err != nil {
				return err
			}

			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				b, _ := json.MarshalIndent(transitions, "", "  ")
				fmt.Println(string(b))
			} else {
				fmt.Printf("Available transitions for %s:\n\n", issueKey)
				for i, t := range transitions {
					fmt.Printf("  %d. %s\n", i+1, t.Name)
				}
				fmt.Printf("\nUse --to \"<status>\" to move. Example: gojira move %s --to \"03 In Progress\"\n", issueKey)
			}
			return nil
		}

		transitions, err := client.GetTransitions(issueKey)
		if err != nil {
			return err
		}

		var matched *jira.Transition
		for _, t := range transitions {
			if t.Name == targetStatus {
				matched = &t
				break
			}
		}

		if matched == nil {
			names := make([]string, len(transitions))
			for i, t := range transitions {
				names[i] = t.Name
			}
			return fmt.Errorf("status %q not found. Available: %v", targetStatus, names)
		}

		if err := client.TransitionIssue(issueKey, matched.ID); err != nil {
			return err
		}

		fmt.Printf("✓ Moved %s → %s\n", issueKey, matched.Name)
		return nil
	},
}

func init() {
	moveCmd.Flags().String("to", "", "Target status name")
	moveCmd.Flags().Bool("json", false, "List transitions as JSON")
	rootCmd.AddCommand(moveCmd)
}
