package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func sprintPositiveFlag(cmd *cobra.Command, name string) (int, error) {
	raw, _ := cmd.Flags().GetString(name)
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("--%s must be a positive integer", name)
	}
	return value, nil
}

func init() {
	group := &cobra.Command{Use: "sprint", Short: "List sprints or move one issue into a sprint"}
	list := &cobra.Command{Use: "list", Short: "List board sprints", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		board, err := sprintPositiveFlag(cmd, "board")
		if err != nil {
			return commandError(cmd, "validation_error", "invalid board", "", jira.MutationNotApplied, err)
		}
		states, _ := cmd.Flags().GetString("state")
		for _, state := range strings.Split(states, ",") {
			if state != "active" && state != "future" && state != "closed" {
				return commandError(cmd, "validation_error", "invalid sprint state", "", jira.MutationNotApplied, fmt.Errorf("state must contain active, future, or closed"))
			}
		}
		client, _, err := buildClient()
		if err != nil {
			return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", "", jira.MutationNotApplied, err)
		}
		sprints, err := client.GetSprintsByState(board, states)
		if err != nil {
			return commandError(cmd, "read_failed", "sprint list failed", "", jira.MutationNotApplied, err)
		}
		if wantsJSON(cmd) {
			if err := writeJSON(cmd, sprints); err != nil {
				return commandError(cmd, "output_failed", "could not write sprints", "", jira.MutationNotApplied, err)
			}
			return nil
		}
		for _, s := range sprints {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\n", s.ID, s.Name, s.State); err != nil {
				return err
			}
		}
		return nil
	}}
	list.Flags().String("board", "", "Board ID (required, positive integer)")
	list.Flags().String("state", "active,future", "Comma-separated sprint states")
	move := &cobra.Command{Use: "move KEY", Short: "Move one issue into a sprint", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		id, err := sprintPositiveFlag(cmd, "to")
		if err == nil && !parentIssueKey.MatchString(key) {
			err = fmt.Errorf("issue key must be a Jira issue key (e.g. ARA-123)")
		}
		if err != nil {
			return commandError(cmd, "validation_error", "invalid sprint move", key, jira.MutationNotApplied, err)
		}
		client, domain, err := buildClient()
		if err != nil {
			return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", key, jira.MutationNotApplied, err)
		}
		if err := client.AddIssuesToSprint(id, []string{key}); err != nil {
			return commandError(cmd, "mutation_failed", "sprint move failed", key, jira.MutationStateOf(err), err)
		}
		if wantsJSON(cmd) {
			return writeMutationResult(cmd, client, domain, key)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Jira accepted the request to move %s to sprint %d\n", key, id)
		return nil
	}}
	move.Flags().String("to", "", "Target sprint ID (required, positive integer)")
	group.PersistentFlags().Bool("json", false, "Output JSON")
	group.AddCommand(list, move)
	rootCmd.AddCommand(group)
}
