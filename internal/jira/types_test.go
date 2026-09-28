package jira

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestIssueToTicketJSONRichFixture(t *testing.T) {
	statusCategory := struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	}{Key: "indeterminate", Name: "In Progress"}
	description := ADFDoc{Content: []ADFNode{
		{Type: "heading", Content: []ADFNode{{Type: "text", Text: "Acceptance"}}},
		{Type: "paragraph", Content: []ADFNode{
			{Type: "text", Text: "Pair with "},
			{Type: "mention", Attrs: &ADFAttrs{Text: "@Alex"}},
		}},
	}}
	comment := Comment{
		Body: ADFDoc{Content: []ADFNode{{
			Type: "paragraph", Content: []ADFNode{{Type: "text", Text: "Ready"}},
		}}},
		Created: "2026-09-23T08:09:10-0300",
	}
	comment.Author.DisplayName = "Alex"

	dueDate := "2026-10-31"
	issue := Issue{Key: "ARA-42"}
	issue.Fields = IssueFieldData{
		Summary:     "Version ticket JSON",
		Status:      &StatusField{Name: "In Progress", Category: &statusCategory},
		Priority:    &PriorityField{Name: "High"},
		Assignee:    &AssigneeField{DisplayName: "René"},
		Reporter:    &ReporterField{DisplayName: "Sam"},
		IssueType:   &IssueTypeField{Name: "Story"},
		Project:     &ProjectField{Key: "ARA", Name: "Agent Roadmap"},
		Labels:      []string{"agent", "json"},
		Components:  []ComponentField{{Name: "CLI"}, {Name: "TUI"}},
		FixVersions: []FixVersionField{{Name: "v1.0"}, {Name: "v1.1"}},
		DueDate:     &dueDate,
		Description: &description,
		Created:     "2026-09-22T20:12:41.934-0600",
		Updated:     "2026-09-23T10:11:12.000Z",
		Comment:     &CommentField{Comments: []Comment{comment}},
	}

	got := IssueToTicketJSON(issue, "example.atlassian.net")
	want := TicketJSON{
		SchemaVersion:  "v1",
		Key:            "ARA-42",
		URL:            "https://example.atlassian.net/browse/ARA-42",
		Summary:        "Version ticket JSON",
		Status:         "In Progress",
		StatusCategory: "indeterminate",
		Priority:       "High",
		Assignee:       "René",
		Reporter:       "Sam",
		IssueType:      "Story",
		Project:        "ARA",
		Labels:         []string{"agent", "json"},
		Components:     []string{"CLI", "TUI"},
		FixVersions:    []string{"v1.0", "v1.1"},
		DueDate:        &dueDate,
		Description:    "## Acceptance\nPair with @Alex",
		Created:        "2026-09-22T20:12:41-06:00",
		Updated:        "2026-09-23T10:11:12Z",
		Comments: []CommentJSON{{
			Author: "Alex", Body: "Ready", Created: "2026-09-23T08:09:10-03:00",
		}},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("IssueToTicketJSON() = %#v, want %#v", got, want)
	}
}

func TestIssueToTicketJSONSparseFixturePreservesDefaultsAndOmissions(t *testing.T) {
	got := IssueToTicketJSON(Issue{Key: "ARA-43"}, "")

	if got.SchemaVersion != "v1" {
		t.Fatalf("SchemaVersion = %q, want v1", got.SchemaVersion)
	}
	if got.Assignee != "Unassigned" {
		t.Fatalf("Assignee = %q, want Unassigned", got.Assignee)
	}
	for name, value := range map[string]string{
		"summary": got.Summary, "status": got.Status, "priority": got.Priority,
		"issueType": got.IssueType, "description": got.Description,
		"created": got.Created, "updated": got.Updated,
	} {
		if value != "" {
			t.Errorf("%s = %q, want empty string", name, value)
		}
	}

	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	for _, key := range []string{"schemaVersion", "key", "summary", "status", "priority", "assignee", "issueType", "dueDate", "description", "created", "updated"} {
		if _, ok := object[key]; !ok {
			t.Errorf("required field %q was omitted from %s", key, encoded)
		}
	}
	if object["dueDate"] != nil {
		t.Errorf("dueDate = %#v, want JSON null", object["dueDate"])
	}
	for _, key := range []string{"url", "statusCategory", "reporter", "project", "labels", "components", "fixVersions", "timeLogged", "comments"} {
		if _, ok := object[key]; ok {
			t.Errorf("omitempty field %q was present in %s", key, encoded)
		}
	}
}

func TestIssueToTicketJSONStoryPointsTriState(t *testing.T) {
	number := json.Number("9007199254740993.125")
	tests := []struct {
		name        string
		points      *NullableNumber
		wantPresent bool
		wantValue   any
	}{
		{name: "selector omitted", points: nil, wantPresent: false},
		{name: "selected null", points: &NullableNumber{}, wantPresent: true, wantValue: nil},
		{name: "selected number preserves precision", points: &NullableNumber{Value: &number}, wantPresent: true, wantValue: json.Number("9007199254740993.125")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			issue := Issue{Key: "A-1", StoryPoints: tc.points}
			encoded, err := json.Marshal(IssueToTicketJSON(issue, ""))
			if err != nil {
				t.Fatal(err)
			}
			decoder := json.NewDecoder(strings.NewReader(string(encoded)))
			decoder.UseNumber()
			var object map[string]any
			if err := decoder.Decode(&object); err != nil {
				t.Fatal(err)
			}
			value, present := object["storyPoints"]
			if present != tc.wantPresent {
				t.Fatalf("storyPoints presence = %v, want %v in %s", present, tc.wantPresent, encoded)
			}
			if tc.wantPresent && !reflect.DeepEqual(value, tc.wantValue) {
				t.Fatalf("storyPoints = %#v, want %#v", value, tc.wantValue)
			}
		})
	}
}

func TestIssuesToTicketJSONVersionsEveryElement(t *testing.T) {
	got := IssuesToTicketJSON([]Issue{{Key: "ARA-1"}, {Key: "ARA-2"}}, "example.atlassian.net")
	if len(got) != 2 {
		t.Fatalf("len(IssuesToTicketJSON()) = %d, want 2", len(got))
	}
	for i, ticket := range got {
		if ticket.SchemaVersion != "v1" {
			t.Errorf("ticket %d SchemaVersion = %q, want v1", i, ticket.SchemaVersion)
		}
	}
}
