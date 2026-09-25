package jira

import (
	"strings"
	"time"
)

// ─── Board / Sprint ────────────────────────────────────────────────────

type Board struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Location *struct {
		ProjectName string `json:"projectName"`
		ProjectKey  string `json:"projectKey"`
	} `json:"location,omitempty"`
}

type BoardConfig struct {
	ColumnConfig *struct {
		Columns []struct {
			Name     string `json:"name"`
			Statuses []struct {
				ID string `json:"id"`
			} `json:"statuses"`
		} `json:"columns"`
	} `json:"columnConfig,omitempty"`
}

type Sprint struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

// ─── ADF (Atlassian Document Format) ──────────────────────────────────

// ADFAttrs carries node attributes. The shape depends on the node type:
// headings use Level (required, integer), mentions use ID (accountId) and
// Text (display name). Fields are omitempty so unused ones never reach the
// wire — the ADF schema rejects stray properties.
type ADFAttrs struct {
	Level int    `json:"level,omitempty"`
	ID    string `json:"id,omitempty"`
	Text  string `json:"text,omitempty"`
}

// ADFNode represents a node in an Atlassian Document Format tree.
// Text must stay omitempty: serializing "text":"" on container nodes
// (paragraph, heading, listItem, …) makes the whole document invalid ADF.
type ADFNode struct {
	Type    string    `json:"type"`
	Text    string    `json:"text,omitempty"`
	Attrs   *ADFAttrs `json:"attrs,omitempty"`
	Content []ADFNode `json:"content,omitempty"`
}

// ADFDoc is the root of an ADF document (used by descriptions and comment bodies).
type ADFDoc struct {
	Type    string    `json:"type"`
	Version int       `json:"version"`
	Content []ADFNode `json:"content"`
}

// Flatten converts ADF to plain text. Blocks (paragraphs, list items) are
// separated by newlines. Inline nodes (text, mentions) are joined within
// their block without extra newlines. Mentions are rendered as @Name.
func (d *ADFDoc) Flatten() string {
	if d == nil {
		return ""
	}
	var blocks []string
	for _, node := range d.Content {
		text := flattenBlock(node)
		if text != "" {
			blocks = append(blocks, text)
		}
	}
	return strings.Join(blocks, "\n")
}

func flattenBlock(node ADFNode) string {
	switch node.Type {
	case "paragraph":
		return flattenInline(node.Content)
	case "bulletList", "orderedList":
		var items []string
		for _, child := range node.Content {
			if child.Type == "listItem" {
				items = append(items, "- "+flattenInline(child.Content))
			}
		}
		return strings.Join(items, "\n")
	case "heading":
		return "## " + flattenInline(node.Content)
	case "codeBlock":
		return "```\n" + flattenInline(node.Content) + "\n```"
	case "rule":
		return "---"
	case "blockquote":
		lines := strings.Split(flattenInline(node.Content), "\n")
		quoted := make([]string, len(lines))
		for i, l := range lines {
			quoted[i] = "> " + l
		}
		return strings.Join(quoted, "\n")
	default:
		// Fallback: recurse into content
		if len(node.Content) > 0 {
			return flattenInline(node.Content)
		}
		return node.Text
	}
}

func flattenInline(nodes []ADFNode) string {
	var parts []string
	for _, n := range nodes {
		switch n.Type {
		case "text":
			parts = append(parts, n.Text)
		case "mention":
			// Render as @Name
			if n.Attrs != nil && n.Attrs.Text != "" {
				parts = append(parts, n.Attrs.Text)
			} else {
				parts = append(parts, "@user")
			}
		case "hardBreak":
			parts = append(parts, "\n")
		case "code":
			parts = append(parts, "`"+n.Text+"`")
		default:
			// Inline marks (strong, em, link, etc.) — just get the text
			if n.Text != "" {
				parts = append(parts, n.Text)
			}
			if len(n.Content) > 0 {
				parts = append(parts, flattenInline(n.Content))
			}
		}
	}
	return strings.Join(parts, "")
}

// ─── Issue fields ──────────────────────────────────────────────────────

type StatusField struct {
	Name     string `json:"name"`
	ID       string `json:"id"`
	Category *struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"statusCategory,omitempty"`
}

type PriorityField struct {
	Name string `json:"name"`
}

type AssigneeField struct {
	DisplayName string `json:"displayName"`
}

// UserRef is a Jira user as returned by the user endpoints. EmailAddress is
// only present when the site exposes emails to API clients.
type UserRef struct {
	AccountID    string `json:"accountId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	Active       bool   `json:"active"`
}

type ReporterField struct {
	DisplayName string `json:"displayName"`
}

type IssueTypeField struct {
	Name string `json:"name"`
}

type ProjectField struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type ComponentField struct {
	Name string `json:"name"`
}

type DescriptionField = ADFDoc

type Comment struct {
	Author struct {
		DisplayName string `json:"displayName"`
	} `json:"author"`
	Body    ADFDoc `json:"body"`
	Comment string `json:"comment,omitempty"`
	Created string `json:"created"`
}

type CommentField struct {
	Comments []Comment `json:"comments"`
}

type IssueFieldData struct {
	Summary     string            `json:"summary"`
	Status      *StatusField      `json:"status,omitempty"`
	Priority    *PriorityField    `json:"priority,omitempty"`
	Assignee    *AssigneeField    `json:"assignee,omitempty"`
	Reporter    *ReporterField    `json:"reporter,omitempty"`
	IssueType   *IssueTypeField   `json:"issuetype,omitempty"`
	Project     *ProjectField     `json:"project,omitempty"`
	Labels      []string          `json:"labels,omitempty"`
	Components  []ComponentField  `json:"components,omitempty"`
	Created     string            `json:"created"`
	Updated     string            `json:"updated"`
	Description *DescriptionField `json:"description,omitempty"`
	Comment     *CommentField     `json:"comment,omitempty"`
}

type Issue struct {
	Key    string         `json:"key"`
	Fields IssueFieldData `json:"fields"`
}

// ─── Project ───────────────────────────────────────────────────────────

type Project struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// ─── Transitions / Worklog ─────────────────────────────────────────────

type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Worklog struct {
	Author struct {
		DisplayName string `json:"displayName"`
	} `json:"author"`
	Comment   string `json:"comment"`
	TimeSpent string `json:"timeSpent"`
	Started   string `json:"started"`
}

// ─── TicketJSON (curated for agent consumption) ────────────────────────

type TicketJSON struct {
	Key            string   `json:"key"`
	URL            string   `json:"url,omitempty"`
	Summary        string   `json:"summary"`
	Status         string   `json:"status"`
	StatusCategory string   `json:"statusCategory,omitempty"` // "new", "indeterminate", "done"
	Priority       string   `json:"priority"`
	Assignee       string   `json:"assignee"`
	Reporter       string   `json:"reporter,omitempty"`
	IssueType      string   `json:"issueType"`
	Project        string   `json:"project,omitempty"`
	Labels         []string `json:"labels,omitempty"`
	Components     []string `json:"components,omitempty"`
	Description    string   `json:"description"`
	Created        string   `json:"created"`
	Updated        string   `json:"updated"`
	TimeLogged     string   `json:"timeLogged,omitempty"`
	Comments       []CommentJSON `json:"comments,omitempty"`
}

type CommentJSON struct {
	Author  string `json:"author"`
	Body    string `json:"body"`
	Created string `json:"created"`
}

// normalizeDate converts Jira's date format to RFC3339.
// Input: "2026-09-22T20:12:41.934-0600" → Output: "2026-09-22T20:12:41-06:00"
// Falls back to the original string if parsing fails.
func normalizeDate(s string) string {
	if s == "" {
		return ""
	}
	// Jira uses non-RFC3339 offsets (no colon): 2026-09-22T20:12:41.934-0600
	layouts := []string{
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05.000Z07:00",
		"2006-01-02T15:04:05Z07:00",
	}
	for _, layout := range layouts {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t.Format(time.RFC3339)
		}
	}
	return s
}

// browseURL returns the Jira browse URL for an issue given a domain.
func browseURL(domain, key string) string {
	if domain == "" || key == "" {
		return ""
	}
	return "https://" + domain + "/browse/" + key
}

// TextToADF converts plain text to an ADF document.
// Supports: paragraphs (separated by newlines), bullet lists (- prefix),
// headings (# prefix), and code blocks (``` delimiters).
func TextToADF(text string) ADFDoc {
	if text == "" {
		return ADFDoc{}
	}
	lines := strings.Split(text, "\n")
	var nodes []ADFNode
	inCodeBlock := false
	var codeLines []string

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				nodes = append(nodes, ADFNode{
					Type:    "codeBlock",
					Content: []ADFNode{{Type: "text", Text: strings.Join(codeLines, "\n")}},
				})
				codeLines = nil
				inCodeBlock = false
			} else {
				inCodeBlock = true
			}
			continue
		}
		if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}
		if strings.HasPrefix(line, "- ") {
			nodes = append(nodes, ADFNode{
				Type: "bulletList",
				Content: []ADFNode{{
					Type:    "listItem",
					Content: []ADFNode{{Type: "paragraph", Content: []ADFNode{{Type: "text", Text: line[2:]}}}},
				}},
			})
		} else if strings.HasPrefix(line, "# ") {
			nodes = append(nodes, ADFNode{
				Type:    "heading",
				Attrs:   &ADFAttrs{Level: 2},
				Content: []ADFNode{{Type: "text", Text: line[2:]}},
			})
		} else if line == "" {
			continue // skip empty lines (each non-empty line is its own paragraph)
		} else {
			nodes = append(nodes, ADFNode{
				Type:    "paragraph",
				Content: []ADFNode{{Type: "text", Text: line}},
			})
		}
	}
	return ADFDoc{Type: "doc", Version: 1, Content: nodes}
}

func IssueToTicketJSON(iss Issue, domain string) TicketJSON {
	t := TicketJSON{
		Key:      iss.Key,
		URL:      browseURL(domain, iss.Key),
		Summary:  iss.Fields.Summary,
		Created:  normalizeDate(iss.Fields.Created),
		Updated:  normalizeDate(iss.Fields.Updated),
		Assignee: "Unassigned",
		Labels:   iss.Fields.Labels,
	}
	if iss.Fields.Status != nil {
		t.Status = iss.Fields.Status.Name
		if iss.Fields.Status.Category != nil {
			t.StatusCategory = iss.Fields.Status.Category.Key
		}
	}
	if iss.Fields.Priority != nil {
		t.Priority = iss.Fields.Priority.Name
	}
	if iss.Fields.IssueType != nil {
		t.IssueType = iss.Fields.IssueType.Name
	}
	if iss.Fields.Assignee != nil {
		t.Assignee = iss.Fields.Assignee.DisplayName
	}
	if iss.Fields.Reporter != nil {
		t.Reporter = iss.Fields.Reporter.DisplayName
	}
	if iss.Fields.Project != nil {
		t.Project = iss.Fields.Project.Key
	}
	if len(iss.Fields.Components) > 0 {
		t.Components = make([]string, len(iss.Fields.Components))
		for i, c := range iss.Fields.Components {
			t.Components[i] = c.Name
		}
	}
	if iss.Fields.Description != nil {
		t.Description = iss.Fields.Description.Flatten()
	}
	if iss.Fields.Comment != nil && len(iss.Fields.Comment.Comments) > 0 {
		t.Comments = make([]CommentJSON, len(iss.Fields.Comment.Comments))
		for i, c := range iss.Fields.Comment.Comments {
			t.Comments[i] = CommentJSON{
				Author:  c.Author.DisplayName,
				Body:    c.Body.Flatten(),
				Created: normalizeDate(c.Created),
			}
		}
	}
	return t
}

func IssuesToTicketJSON(issues []Issue, domain string) []TicketJSON {
	out := make([]TicketJSON, len(issues))
	for i, iss := range issues {
		out[i] = IssueToTicketJSON(iss, domain)
	}
	return out
}
