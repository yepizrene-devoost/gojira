package cmd

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

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
