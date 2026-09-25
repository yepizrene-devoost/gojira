package jira

import "strings"

// ─── Board / Sprint ────────────────────────────────────────────────────

type Board struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Project *struct {
		Key string `json:"key"`
	} `json:"location,omitempty"` // populated when using enriched queries
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

// ─── Issue fields ──────────────────────────────────────────────────────

type StatusField struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type PriorityField struct {
	Name string `json:"name"`
}

type AssigneeField struct {
	DisplayName string `json:"displayName"`
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

type DescriptionField struct {
	Content []struct {
		Content []struct{ Text string } `json:"content"`
	} `json:"content"`
}

type CommentField struct {
	Comments []Comment `json:"comments"`
}

type Comment struct {
	Author struct {
		DisplayName string `json:"displayName"`
	} `json:"author"`
	Body    string `json:"body"`
	Created string `json:"created"`
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
	Key         string   `json:"key"`
	Summary     string   `json:"summary"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	Assignee    string   `json:"assignee"`
	Reporter    string   `json:"reporter,omitempty"`
	IssueType   string   `json:"issueType"`
	Project     string   `json:"project,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	Components  []string `json:"components,omitempty"`
	Description string   `json:"description"`
	Created     string   `json:"created"`
	Updated     string   `json:"updated"`
	TimeLogged  string   `json:"timeLogged,omitempty"`
	Comments    []CommentJSON `json:"comments,omitempty"`
}

type CommentJSON struct {
	Author  string `json:"author"`
	Body    string `json:"body"`
	Created string `json:"created"`
}

func IssueToTicketJSON(iss Issue) TicketJSON {
	t := TicketJSON{
		Key:      iss.Key,
		Summary:  iss.Fields.Summary,
		Created:  iss.Fields.Created,
		Updated:  iss.Fields.Updated,
		Assignee: "Unassigned",
		Labels:   iss.Fields.Labels,
	}
	if iss.Fields.Status != nil {
		t.Status = iss.Fields.Status.Name
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
		for _, block := range iss.Fields.Description.Content {
			for _, c := range block.Content {
				if c.Text != "" {
					t.Description += c.Text + "\n"
				}
			}
		}
		t.Description = strings.TrimRight(t.Description, "\n")
	}
	if iss.Fields.Comment != nil && len(iss.Fields.Comment.Comments) > 0 {
		t.Comments = make([]CommentJSON, len(iss.Fields.Comment.Comments))
		for i, c := range iss.Fields.Comment.Comments {
			t.Comments[i] = CommentJSON{
				Author:  c.Author.DisplayName,
				Body:    c.Body,
				Created: c.Created,
			}
		}
	}
	return t
}

func IssuesToTicketJSON(issues []Issue) []TicketJSON {
	out := make([]TicketJSON, len(issues))
	for i, iss := range issues {
		out[i] = IssueToTicketJSON(iss)
	}
	return out
}
