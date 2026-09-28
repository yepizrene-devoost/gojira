package cmd

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestWriteGetJSONUsesCobraOutputAsSingleDocument(t *testing.T) {
	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	issue := jira.Issue{Key: "ARA-9"}
	issue.Fields.Summary = "Agent JSON"

	if err := writeGetJSON(cmd, issue, "example.atlassian.net"); err != nil {
		t.Fatalf("writeGetJSON() error = %v", err)
	}

	var ticket jira.TicketJSON
	assertSingleJSONDocument(t, stdout.Bytes(), &ticket)
	if ticket.SchemaVersion != "v1" || ticket.Key != "ARA-9" {
		t.Fatalf("ticket = %#v, want v1 ARA-9", ticket)
	}
}

// Comment previews are cut on rune boundaries: Jira bodies in this workspace
// carry accented Spanish, and a byte slice used to split those characters.
func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"short body is untouched", "hola", 200, "hola"},
		{"exactly max is untouched", strings.Repeat("a", 200), 200, strings.Repeat("a", 200)},
		{"one over max loses three runes to the ellipsis", strings.Repeat("a", 201), 200, strings.Repeat("a", 197) + "..."},
		{"long accented body stays valid utf-8", strings.Repeat("á", 300), 200, strings.Repeat("á", 197) + "..."},
		{"accented body exactly at max is untouched", strings.Repeat("ñ", 200), 200, strings.Repeat("ñ", 200)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateRunes(tc.in, tc.max)
			if got != tc.want {
				t.Fatalf("truncateRunes(%d runes, %d) = %q, want %q", utf8.RuneCountInString(tc.in), tc.max, got, tc.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("truncateRunes returned invalid utf-8: %q", got)
			}
			if n := utf8.RuneCountInString(got); n > tc.max {
				t.Fatalf("result has %d runes, want at most %d", n, tc.max)
			}
		})
	}
}

func TestRenderIssueFullShowsDueDateComponentsAndFixVersionsWhenPresent(t *testing.T) {
	dueDate := "2026-10-31"
	issue := &jira.Issue{Key: "PROJ-1"}
	issue.Fields.Components = []jira.ComponentField{{Name: "API"}, {Name: "Web"}}
	issue.Fields.FixVersions = []jira.FixVersionField{{Name: "v1.0"}, {Name: "v1.1"}}
	issue.Fields.DueDate = &dueDate

	out := renderIssueFull(issue)
	for _, want := range []string{"Components:", "API, Web", "Fix versions:", "v1.0, v1.1", "Due date:", "2026-10-31"} {
		if !strings.Contains(out, want) {
			t.Fatalf("rendered output missing %q: %q", want, out)
		}
	}

	sparse := renderIssueFull(&jira.Issue{Key: "PROJ-2"})
	for _, absent := range []string{"Components:", "Fix versions:", "Due date:"} {
		if strings.Contains(sparse, absent) {
			t.Fatalf("sparse output unexpectedly contains %q: %q", absent, sparse)
		}
	}
}

func TestRenderIssueFullCommentCreatedValues(t *testing.T) {
	tests := []struct {
		name    string
		created string
		want    string
	}{
		{name: "normal Jira timestamp", created: "2026-09-26T10:00:00.000-0300", want: "2026-09-26"},
		{name: "short value", created: "2026-", want: "2026-"},
		{name: "empty value", created: "", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			iss := &jira.Issue{Key: "PROJ-1"}
			iss.Fields.Comment = &jira.CommentField{Comments: []jira.Comment{{Created: tc.created}}}

			out := renderIssueFull(iss)
			if !strings.Contains(out, "("+tc.want+")") {
				t.Fatalf("rendered output does not contain comment date %q: %q", tc.want, out)
			}
		})
	}
}

// The renderer is the only caller, so the bound has to hold end to end: a long
// comment must be previewed, not printed whole.
func TestRenderIssueFullTruncatesComments(t *testing.T) {
	const created = "2026-09-26T10:00:00.000-0300"
	body := strings.Repeat("á", 300)

	iss := &jira.Issue{Key: "PROJ-1"}
	iss.Fields.Summary = "Una historia"
	iss.Fields.Created = created
	iss.Fields.Updated = created
	comment := jira.Comment{Created: created}
	comment.Author.DisplayName = "René"
	comment.Body = jira.ADFDoc{Content: []jira.ADFNode{{
		Type:    "paragraph",
		Content: []jira.ADFNode{{Type: "text", Text: body}},
	}}}
	iss.Fields.Comment = &jira.CommentField{Comments: []jira.Comment{comment}}

	out := renderIssueFull(iss)

	if strings.Contains(out, body) {
		t.Fatalf("renderer printed the full %d-rune comment instead of a preview", utf8.RuneCountInString(body))
	}
	want := strings.Repeat("á", commentPreviewMax-len(ellipsis)) + ellipsis
	if !strings.Contains(out, want) {
		t.Fatalf("renderer did not emit the expected %d-rune preview", commentPreviewMax)
	}
	if !utf8.ValidString(out) {
		t.Fatal("rendered output is not valid utf-8")
	}
}
