package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/yepizrene-devoost/gojira/internal/config"
	"github.com/yepizrene-devoost/gojira/internal/jira"
	"github.com/yepizrene-devoost/gojira/internal/picker"
)

var isTerminal = func() bool { return isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd()) }
var pickProject = picker.Pick

type projectInitJSON struct {
	SchemaVersion string                  `json:"schemaVersion"`
	Project       string                  `json:"project"`
	Written       string                  `json:"written"`
	Effective     *config.ResolvedProject `json:"effective"`
	Warning       string                  `json:"warning,omitempty"`
}

func newProjectInitCommand() *cobra.Command {
	command := &cobra.Command{Use: "init", Short: "Store this clone's Jira project in local Git configuration", Args: cobra.NoArgs, RunE: runProjectInit}
	command.Flags().String("project", "", "Project key (skips the picker)")
	command.Flags().Bool("yes", false, "Allow non-interactive initialization")
	command.Flags().Bool("json", false, "Output as JSON")
	return command
}

func runProjectInit(cmd *cobra.Command, _ []string) error {
	refuse := func(message string, cause error) error {
		// A nil cause would make commandError return nil in human mode and exit 0.
		if cause == nil {
			cause = errors.New(message)
		} else if !strings.Contains(cause.Error(), message) {
			cause = fmt.Errorf("%s: %w", message, cause)
		}
		return commandError(cmd, "validation_error", message, "", jira.MutationNotApplied, cause)
	}
	if err := exec.Command("git", "rev-parse", "--git-dir").Run(); err != nil {
		return refuse("init requires a Git work tree", err)
	}
	key, _ := cmd.Flags().GetString("project")
	yes, _ := cmd.Flags().GetBool("yes")
	terminal := isTerminal()
	if !terminal && (key == "" || !yes) {
		return refuse("non-interactive init requires --project KEY --yes", nil)
	}
	if key != "" {
		if _, err := config.ResolveProject(key, ""); err != nil {
			return refuse(err.Error(), err)
		}
	}
	client, _, err := buildClient()
	if err != nil {
		return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", "", jira.MutationNotApplied, err)
	}
	if key == "" {
		projects, err := client.GetProjects()
		if err != nil {
			return commandError(cmd, "read_failed", "projects could not be read", "", jira.MutationNotApplied, err)
		}
		items := make([]picker.Item, 0, len(projects))
		for _, project := range projects {
			items = append(items, picker.Item{Key: project.Key, Name: project.Name})
		}
		key, err = pickProject(items)
		if err != nil {
			return fmt.Errorf("project selection cancelled: %w", err)
		}
		if key == "" {
			return picker.ErrCancelled
		}
	}
	if _, err := config.ResolveProject(key, ""); err != nil {
		return refuse(err.Error(), err)
	}
	if _, err := client.GetProject(key); err != nil {
		return commandError(cmd, "read_failed", "project could not be verified", "", jira.MutationNotApplied, err)
	}
	if err := exec.Command("git", "config", "--local", "gojira.project", key).Run(); err != nil {
		return refuse("write_failed: local project configuration could not be written", err)
	}
	userDefault := ""
	if cfg, err := config.Load(); err == nil {
		userDefault = cfg.Project
	}
	effective, err := config.ResolveProject("", userDefault)
	warning := ""
	if err != nil {
		warning = "local project was written, but effective project resolution failed: " + err.Error()
	} else if effective == nil || effective.Key != key || effective.Source != config.ProjectSourceGit || effective.Scope != "local" {
		warning = "local project was written, but no effective project could be resolved"
		if effective != nil {
			warning = fmt.Sprintf("local project %s was written, but effective project is %s (source: %s, scope: %s)", key, effective.Key, effective.Source, effective.Scope)
		}
	}
	result := projectInitJSON{SchemaVersion: jsonSchemaVersion, Project: key, Written: "local", Effective: effective, Warning: warning}
	if warning != "" {
		if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "Warning: "+warning); err != nil {
			return err
		}
	}
	if wantsJSON(cmd) {
		if err := writeJSON(cmd, result); err != nil {
			return reportJSONError(cmd, "output_failed", "init JSON could not be written", "", jira.MutationNotApplied, err)
		}
		return nil
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Stored project %s in local Git configuration.\n", key)
	return err
}

func init() { rootCmd.AddCommand(newProjectInitCommand()) }
