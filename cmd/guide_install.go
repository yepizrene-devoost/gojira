package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yepizrene-devoost/gojira/internal/agentguide"
)

func newGuideInstallCommand() *cobra.Command {
	var options agentguide.Options
	command := &cobra.Command{
		Use:   "install --path FILE",
		Short: "Preview an optional guide installation; --yes writes it",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			plan, err := agentguide.Install(agentguide.OSFilesystem{}, agentguide.GitAdapter{}, options)
			if err != nil {
				return err
			}
			action := "Preview"
			if options.Yes {
				action = "Installed"
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "%s: %s (%d bytes)\n", action, plan.Path, plan.Size)
			if err != nil {
				return err
			}
			if plan.Exclusion != "" {
				_, err = fmt.Fprintf(command.OutOrStdout(), "Exclude: %s\n", plan.Exclusion)
			}
			return err
		},
	}
	command.Flags().StringVar(&options.Path, "path", "", "Destination Markdown file")
	command.Flags().BoolVar(&options.Yes, "yes", false, "Perform the installation")
	command.Flags().BoolVar(&options.Exclude, "exclude", false, "Exclude the repository-relative destination locally")
	return command
}
