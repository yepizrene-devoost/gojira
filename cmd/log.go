package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var logCmd = &cobra.Command{
	Use:   "log <issue-key> --time <duration>",
	Short: "Add worklog to a ticket",
	Long:  "Log time spent on a ticket. Example: gojira log ARA-1892 --time 2h --comment 'Fixed auth bug'",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := BuildClient()
		if err != nil {
			return err
		}

		issueKey := args[0]
		timeSpent, _ := cmd.Flags().GetString("time")
		comment, _ := cmd.Flags().GetString("comment")
		showLogs, _ := cmd.Flags().GetBool("show")

		if showLogs {
			worklogs, err := client.GetWorklog(issueKey)
			if err != nil {
				return err
			}

			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				b, _ := json.MarshalIndent(worklogs, "", "  ")
				fmt.Println(string(b))
			} else {
				fmt.Printf("Worklog for %s:\n\n", issueKey)
				for _, wl := range worklogs {
					fmt.Printf("  %s  %s  %s\n", wl.TimeSpent, wl.Author.DisplayName, wl.Comment)
				}
			}
			return nil
		}

		if timeSpent == "" {
			return fmt.Errorf("--time is required (e.g. 30m, 2h, 1d)")
		}

		if err := client.AddWorklog(issueKey, timeSpent, comment); err != nil {
			return err
		}

		msg := fmt.Sprintf("✓ Logged %s on %s", timeSpent, issueKey)
		if comment != "" {
			msg += fmt.Sprintf(" (%s)", strings.TrimSpace(comment))
		}
		fmt.Println(msg)
		return nil
	},
}

func init() {
	logCmd.Flags().StringP("time", "t", "", "Time spent (e.g. 30m, 2h, 1d)")
	logCmd.Flags().StringP("comment", "c", "", "Worklog comment")
	logCmd.Flags().Bool("show", false, "Show existing worklogs")
	logCmd.Flags().Bool("json", false, "Output as JSON")
	rootCmd.AddCommand(logCmd)
}
