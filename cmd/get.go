package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

var keyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Bold(true)
var metaLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Bold(true)

// commentPreviewMax bounds a rendered comment preview, counted in runes so a
// multi-byte body (accented Spanish is common in this workspace) is never cut
// in the middle of a character.
const commentPreviewMax = 200

// ellipsis marks a truncated preview and counts toward the bound.
const ellipsis = "..."

// truncateRunes shortens s to at most max runes, replacing the tail with an
// ellipsis that counts toward max. max is expected to be at least len(ellipsis).
func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	keep := max - len(ellipsis)
	if keep < 0 {
		keep = 0
	}
	return string(runes[:keep]) + ellipsis
}

func commentCreatedDate(created string) string {
	if len(created) < 10 {
		return created
	}
	return created[:10]
}

var getCmd = &cobra.Command{
	Use:   "get <issue-key>",
	Short: "View a ticket's full details",
	Long:  "Display complete ticket information including project, labels, components, reporter, comments, and more.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		storyPointsField, _ := cmd.Flags().GetString("story-points-field")
		if cmd.Flags().Changed("story-points-field") {
			if err := validateStoryPointsField(storyPointsField); err != nil {
				return err
			}
		}
		issueKey := args[0]
		client, domain, err := buildClient()
		if err != nil {
			return commandError(cmd, "configuration_error", "Jira client configuration is unavailable", issueKey, jira.MutationNotApplied, err)
		}
		var iss *jira.Issue
		if storyPointsField == "" {
			iss, err = client.GetIssueFull(issueKey)
		} else {
			iss, err = client.GetIssueFullWithStoryPoints(issueKey, storyPointsField)
		}
		if err != nil {
			return commandError(cmd, "read_failed", "issue could not be read", issueKey, jira.MutationNotApplied, err)
		}

		if wantsJSON(cmd) {
			if err := writeGetJSON(cmd, *iss, domain); err != nil {
				return reportJSONError(cmd, "output_failed", "issue JSON could not be written", issueKey, jira.MutationNotApplied, err)
			}
			return nil
		}

		// Render readable output
		_, err = fmt.Fprintln(cmd.OutOrStdout(), renderIssueFull(iss))
		return err
	},
}

func writeGetJSON(cmd *cobra.Command, issue jira.Issue, domain string) error {
	return writeJSON(cmd, jira.IssueToTicketJSON(issue, domain))
}

// writef appends a formatted line to b. strings.Builder.Write never returns a
// non-nil error, so the discarded result is the documented outcome here instead
// of an unexplained `_, _ =` repeated at every call site.
func writef(b *strings.Builder, format string, args ...any) {
	_, _ = fmt.Fprintf(b, format, args...)
}

func renderIssueFull(iss *jira.Issue) string {
	var b strings.Builder
	ticket := jira.IssueToTicketJSON(*iss, "")

	b.WriteString(keyStyle.Render(fmt.Sprintf("📋 %s", iss.Key)))
	b.WriteString("\n\n")
	b.WriteString(iss.Fields.Summary)
	b.WriteString("\n\n")

	// Metadata table
	rows := [][2]string{}
	if iss.Fields.IssueType != nil {
		rows = append(rows, [2]string{"Type", iss.Fields.IssueType.Name})
	}
	if iss.Fields.Status != nil {
		rows = append(rows, [2]string{"Status", iss.Fields.Status.Name})
	}
	if iss.Fields.Priority != nil {
		rows = append(rows, [2]string{"Priority", iss.Fields.Priority.Name})
	}
	if iss.Fields.Assignee != nil {
		rows = append(rows, [2]string{"Assignee", iss.Fields.Assignee.DisplayName})
	} else {
		rows = append(rows, [2]string{"Assignee", "Unassigned"})
	}
	if iss.Fields.Reporter != nil {
		rows = append(rows, [2]string{"Reporter", iss.Fields.Reporter.DisplayName})
	}
	if iss.Fields.Project != nil {
		rows = append(rows, [2]string{"Project", fmt.Sprintf("%s (%s)", iss.Fields.Project.Key, iss.Fields.Project.Name)})
	}
	if parent := ticket.Parent; parent != nil {
		rows = append(rows, [2]string{"Parent", fmt.Sprintf("%s %s (%s)", parent.Key, parent.Summary, parent.Status)})
	}
	if len(iss.Fields.Labels) > 0 {
		rows = append(rows, [2]string{"Labels", strings.Join(iss.Fields.Labels, ", ")})
	}
	if len(iss.Fields.Components) > 0 {
		names := make([]string, len(iss.Fields.Components))
		for i, c := range iss.Fields.Components {
			names[i] = c.Name
		}
		rows = append(rows, [2]string{"Components", strings.Join(names, ", ")})
	}
	if len(iss.Fields.FixVersions) > 0 {
		names := make([]string, len(iss.Fields.FixVersions))
		for i, version := range iss.Fields.FixVersions {
			names[i] = version.Name
		}
		rows = append(rows, [2]string{"Fix versions", strings.Join(names, ", ")})
	}
	if iss.Fields.DueDate != nil {
		rows = append(rows, [2]string{"Due date", *iss.Fields.DueDate})
	}
	if iss.StoryPoints != nil {
		value := "None"
		if iss.StoryPoints.Value != nil {
			value = iss.StoryPoints.Value.String()
		}
		rows = append(rows, [2]string{"Story points", value})
	}
	rows = append(rows, [2]string{"Created", iss.Fields.Created})
	rows = append(rows, [2]string{"Updated", iss.Fields.Updated})

	for _, row := range rows {
		writef(&b, "%-12s %s\n", metaLabelStyle.Render(row[0]+":"), row[1])
	}

	// Hierarchy is read from the same issue response; no per-child requests.
	if ticket.Subtasks != nil {
		b.WriteString("\n── Subtasks ──\n")
		for _, child := range *ticket.Subtasks {
			writef(&b, "  %s %s (%s)\n", child.Key, child.Summary, child.Status)
		}
		if ticket.SubtaskProgress != nil {
			writef(&b, "  Progress: %d/%d done\n", ticket.SubtaskProgress.Done, ticket.SubtaskProgress.Total)
		}
	}

	// Description
	if iss.Fields.Description != nil {
		b.WriteString("\n── Description ──\n")
		for _, block := range iss.Fields.Description.Content {
			for _, c := range block.Content {
				if c.Text != "" {
					b.WriteString(c.Text)
					b.WriteString("\n")
				}
			}
		}
	}

	// Comments
	if iss.Fields.Comment != nil && len(iss.Fields.Comment.Comments) > 0 {
		writef(&b, "\n── Comments (%d) ──\n", len(iss.Fields.Comment.Comments))
		for i, c := range iss.Fields.Comment.Comments {
			body := truncateRunes(strings.TrimSpace(c.Body.Flatten()), commentPreviewMax)
			writef(&b, "  [%d] %s (%s): %q\n", i+1, c.Author.DisplayName, commentCreatedDate(c.Created), body)
		}
	}

	return b.String()
}

func init() {
	getCmd.Flags().Bool("json", false, "Output as JSON")
	getCmd.Flags().String("story-points-field", "", "Include story points from this Jira customfield_N ID")
	rootCmd.AddCommand(getCmd)
}
