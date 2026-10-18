package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yepizrene-devoost/gojira/internal/config"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

var contextConfig = config.Load

// ContextIdentity deliberately omits private fields from Jira's UserRef.
type ContextIdentity struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Active      bool   `json:"active"`
}

type ContextJSON struct {
	SchemaVersion string                  `json:"schemaVersion"`
	Project       *config.ResolvedProject `json:"project"`
	NeedsProject  bool                    `json:"needsProject"`
	Identity      ContextIdentity         `json:"identity"`
}

// readContext is shared by CLI and MCP, using the process environment and cwd.
func readContext(client *jira.Client, flag string) (*ContextJSON, error) {
	cfg, err := contextConfig()
	if err != nil {
		return nil, err
	}
	project, err := config.ResolveProject(flag, cfg.Project)
	if err != nil {
		return nil, err
	}
	user, err := client.GetMyself()
	if err != nil {
		return nil, err
	}
	return &ContextJSON{SchemaVersion: jsonSchemaVersion, Project: project, NeedsProject: project == nil, Identity: ContextIdentity{AccountID: user.AccountID, DisplayName: user.DisplayName, Active: user.Active}}, nil
}

func newContextCommand() *cobra.Command {
	command := &cobra.Command{Use: "context", Short: "Show resolved project and authenticated Jira identity", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		client, _, err := buildClient()
		if err != nil {
			return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", "", jira.MutationNotApplied, err)
		}
		flag, _ := cmd.Flags().GetString("project")
		result, err := readContext(client, flag)
		if err != nil {
			code, message := "read_failed", "context could not be read"
			var invalid *config.ProjectValidationError
			if errors.As(err, &invalid) {
				code, message = "validation_error", invalid.Error()
			}
			return commandError(cmd, code, message, "", jira.MutationNotApplied, err)
		}
		if wantsJSON(cmd) {
			if err := writeJSON(cmd, result); err != nil {
				return reportJSONError(cmd, "output_failed", "context JSON could not be written", "", jira.MutationNotApplied, err)
			}
			return nil
		}
		projectSummary := "Project: not configured; ask the human for a project."
		if result.Project != nil {
			scope := ""
			if result.Project.Scope != "" {
				scope = ", scope: " + result.Project.Scope
			}
			projectSummary = fmt.Sprintf("Project: %s (source: %s%s)", result.Project.Key, result.Project.Source, scope)
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\nIdentity: %s (active: %t)\n", projectSummary, result.Identity.DisplayName, result.Identity.Active)
		return err
	}}
	command.Flags().String("project", "", "Override the project key")
	command.Flags().Bool("json", false, "Output as JSON")
	return command
}

func init() { rootCmd.AddCommand(newContextCommand()) }
