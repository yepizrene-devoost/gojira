package cmd

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

var updateCmd = &cobra.Command{
	Use:   "update <issue-key>",
	Short: "Update fields on a Jira issue",
	Long: `Update one or more fields on an existing issue. Examples:

  gojira update ARA-1892 --summary "New summary text"
  gojira update ARA-1892 --description "Updated context"
  gojira update ARA-1892 --due-date 2026-10-31
  gojira update ARA-1892 --components "Web,API" --fix-versions "v1.0,v1.1"
  gojira update ARA-1892 --due-date= --components= --fix-versions=
  gojira update ARA-1892 --priority High
  gojira update ARA-1892 --labels "frontend,urgent"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueKey := args[0]
		fields := map[string]interface{}{}
		metadataFields := map[string][]string{}

		if s, _ := cmd.Flags().GetString("summary"); s != "" {
			fields["summary"] = s
		}
		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			if description == "" {
				fields["description"] = nil
			} else {
				fields["description"] = jira.TextToADF(description)
			}
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

		if cmd.Flags().Changed("due-date") {
			dueDate, _ := cmd.Flags().GetString("due-date")
			if err := validateDueDate(dueDate); err != nil {
				return commandError(cmd, "validation_error", "invalid due date", issueKey, jira.MutationNotApplied, err)
			}
			if dueDate == "" {
				fields["duedate"] = nil
			} else {
				fields["duedate"] = dueDate
			}
			metadataFields["duedate"] = nil
		}
		for _, flagField := range []struct {
			flag  string
			field string
		}{
			{flag: "components", field: "components"},
			{flag: "fix-versions", field: "fixVersions"},
		} {
			if !cmd.Flags().Changed(flagField.flag) {
				continue
			}
			raw, _ := cmd.Flags().GetString(flagField.flag)
			names, err := parseCommaNames(raw, flagField.flag)
			if err != nil {
				return commandError(cmd, "validation_error", "invalid "+flagField.flag, issueKey, jira.MutationNotApplied, err)
			}
			values := make([]map[string]string, len(names))
			for i, name := range names {
				values[i] = map[string]string{"name": name}
			}
			fields[flagField.field] = values
			metadataFields[flagField.field] = names
		}

		if len(fields) == 0 {
			err := fmt.Errorf("specify at least one field to update (--summary, --description, --priority, --labels, --due-date, --components, --fix-versions)")
			return commandError(cmd, "validation_error", "at least one field must be specified", issueKey, jira.MutationNotApplied, err)
		}

		client, domain, err := buildClient()
		if err != nil {
			return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", issueKey, jira.MutationNotApplied, err)
		}
		if len(metadataFields) > 0 {
			meta, err := client.GetIssueEditMeta(issueKey)
			if err != nil {
				return commandError(cmd, "metadata_failed", "issue edit metadata is unavailable", issueKey, jira.MutationNotApplied, err)
			}
			if err := validateEditMeta(meta, metadataFields); err != nil {
				return commandError(cmd, "validation_error", "requested field update is not allowed", issueKey, jira.MutationNotApplied, err)
			}
		}
		if err := client.UpdateIssue(issueKey, fields); err != nil {
			return commandError(cmd, "mutation_failed", "issue update failed", issueKey, jira.MutationStateOf(err), err)
		}
		if wantsJSON(cmd) {
			return writeMutationResult(cmd, client, domain, issueKey)
		}

		updated := make([]string, 0, len(fields))
		for key := range fields {
			updated = append(updated, key)
		}
		sort.Strings(updated)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Updated %s: %s\n", issueKey, strings.Join(updated, ", "))
		return nil
	},
}

func validateDueDate(value string) error {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return fmt.Errorf("--due-date must be a real calendar date in YYYY-MM-DD format")
	}
	return nil
}

func parseCommaNames(value, flag string) ([]string, error) {
	if value == "" {
		return []string{}, nil
	}
	parts := strings.Split(value, ",")
	names := make([]string, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for i, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			return nil, fmt.Errorf("--%s contains an empty entry", flag)
		}
		canonical := strings.ToLower(name)
		if _, exists := seen[canonical]; exists {
			return nil, fmt.Errorf("--%s contains duplicate entry %q", flag, name)
		}
		seen[canonical] = struct{}{}
		names[i] = name
	}
	return names, nil
}

func validateEditMeta(meta *jira.EditMeta, requested map[string][]string) error {
	if meta == nil {
		return fmt.Errorf("Jira returned empty edit metadata")
	}
	for field, names := range requested {
		fieldMeta, editable := meta.Fields[field]
		if !editable {
			return fmt.Errorf("field %q is not editable for this issue", field)
		}
		if len(names) == 0 || fieldMeta.AllowedValues == nil {
			continue
		}
		allowed := make(map[string]struct{}, len(*fieldMeta.AllowedValues))
		for _, value := range *fieldMeta.AllowedValues {
			allowed[value.Name] = struct{}{}
		}
		for _, name := range names {
			if _, ok := allowed[name]; !ok {
				return fmt.Errorf("%q is not an allowed value for field %q", name, field)
			}
		}
	}
	return nil
}

func init() {
	updateCmd.Flags().String("summary", "", "New summary")
	updateCmd.Flags().String("description", "", "New description (plain text; explicitly empty clears it)")
	updateCmd.Flags().String("priority", "", "New priority name (e.g. High, Critical)")
	updateCmd.Flags().String("labels", "", "Labels (comma-separated, replaces existing)")
	updateCmd.Flags().String("due-date", "", "Due date in YYYY-MM-DD format (explicitly empty clears it)")
	updateCmd.Flags().String("components", "", "Component names (comma-separated, replaces existing; explicitly empty clears)")
	updateCmd.Flags().String("fix-versions", "", "Fix version names (comma-separated, replaces existing; explicitly empty clears)")
	updateCmd.Flags().Bool("json", false, "Output the resulting issue as TicketJSON v1")
	rootCmd.AddCommand(updateCmd)
}
