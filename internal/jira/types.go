package jira

import "strings"

// ─── Board / Sprint ────────────────────────────────────────────────────

type Board struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
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

type IssueTypeField struct {
	Name string `json:"name"`
}

type DescriptionField struct {
	Content []struct {
		Content []struct{ Text string } `json:"content"`
	} `json:"content"`
}

type IssueFieldData struct {
	Summary     string            `json:"summary"`
	Status      *StatusField      `json:"status,omitempty"`
	Priority    *PriorityField    `json:"priority,omitempty"`
	Assignee    *AssigneeField    `json:"assignee,omitempty"`
	IssueType   *IssueTypeField   `json:"issuetype,omitempty"`
	Created     string            `json:"created"`
	Updated     string            `json:"updated"`
	Description *DescriptionField `json:"description,omitempty"`
}

type Issue struct {
	Key    string         `json:"key"`
	Fields IssueFieldData `json:"fields"`
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
	IssueType   string   `json:"issueType"`
	Description string   `json:"description"`
	Created     string   `json:"created"`
	Updated     string   `json:"updated"`
	Labels      []string `json:"labels,omitempty"`
}

func IssueToTicketJSON(iss Issue) TicketJSON {
	t := TicketJSON{
		Key:      iss.Key,
		Summary:  iss.Fields.Summary,
		Created:  iss.Fields.Created,
		Updated:  iss.Fields.Updated,
		Assignee: "Unassigned",
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
	return t
}

func IssuesToTicketJSON(issues []Issue) []TicketJSON {
	out := make([]TicketJSON, len(issues))
	for i, iss := range issues {
		out[i] = IssueToTicketJSON(iss)
	}
	return out
}
