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
		client, domain, err := BuildClient()
		if err != nil {
			return err
		}

		issueKey := args[0]
		iss, err := client.GetIssueFull(issueKey)
		if err != nil {
			return err
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			return writeGetJSON(cmd, *iss, domain)
		}

		// Render readable output
		fmt.Println(renderIssueFull(iss))
		return nil
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
	rows = append(rows, [2]string{"Created", iss.Fields.Created})
	rows = append(rows, [2]string{"Updated", iss.Fields.Updated})

	for _, row := range rows {
		writef(&b, "%-12s %s\n", metaLabelStyle.Render(row[0]+":"), row[1])
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
	rootCmd.AddCommand(getCmd)
}
