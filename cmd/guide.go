package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yepizrene-devoost/gojira/internal/agentguide"
)

func newGuideCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "guide",
		Short: "Print the optional agent guide",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(command.OutOrStdout(), agentguide.Content())
			return err
		},
	}
	command.AddCommand(newGuideInstallCommand())
	return command
}

func init() { rootCmd.AddCommand(newGuideCommand()) }
