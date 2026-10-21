package issuefields

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func text(s string) *string       { return &s }
func names(s ...string) *[]string { return &s }

func TestPrepare(t *testing.T) {
	for _, tc := range []struct {
		name    string
		in      Input
		want    string
		invalid bool
	}{
		{"empty", Input{}, "", true},
		{"array_names", Input{ComponentNames: names("API, Web")}, `{"components":[{"name":"API, Web"}]}`, false},
		{"scalars", Input{Summary: text("New"), Priority: text("High"), Labels: text("one, two")}, `{"labels":["one","two"],"priority":{"name":"High"},"summary":"New"}`, false},
		{"description", Input{Description: text("Context")}, `{"description":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Context"}]}]}}`, false},
		{"clear", Input{Description: text(""), DueDate: text(""), Components: text(""), FixVersions: text("")}, `{"components":[],"description":null,"duedate":null,"fixVersions":[]}`, false},
		{"date", Input{DueDate: text("2028-02-29")}, `{"duedate":"2028-02-29"}`, false},
		{"invalid_date", Input{DueDate: text("2027-02-29")}, "", true},
		{"names", Input{Components: text("API, Web"), FixVersions: text("v1")}, `{"components":[{"name":"API"},{"name":"Web"}],"fixVersions":[{"name":"v1"}]}`, false},
		{"duplicate", Input{Components: text("API,api")}, "", true},
		{"empty_entry", Input{FixVersions: text("v1,")}, "", true},
		{"points", Input{StoryPoints: text("9007199254740993.125"), StoryPointsField: text("customfield_10016")}, `{"customfield_10016":9007199254740993.125}`, false},
		{"clear_points", Input{StoryPoints: text(""), StoryPointsField: text("customfield_10016")}, `{"customfield_10016":null}`, false},
		{"invalid_points", Input{StoryPoints: text("1e3"), StoryPointsField: text("customfield_10016")}, "", true},
		{"invalid_field", Input{StoryPoints: text("5"), StoryPointsField: text("points")}, "", true},
		{"unpaired_value", Input{StoryPoints: text("5")}, "", true},
		{"unpaired_field", Input{StoryPointsField: text("customfield_10016")}, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := Prepare(tc.in)
			if tc.invalid {
				if err == nil {
					t.Fatal("expected validation failure")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(p.Fields)
			if err != nil || string(got) != tc.want {
				t.Fatalf("fields=%s error=%v, want %s", got, err, tc.want)
			}
		})
	}
}

func TestPrepareReportsTheFailingField(t *testing.T) {
	for _, tc := range []struct {
		name  string
		in    Input
		field string
	}{
		{"due_date", Input{DueDate: text("2027-02-29")}, "dueDate"},
		{"components", Input{ComponentNames: names("API", "api")}, "components"},
		{"fix_versions", Input{FixVersionNames: names("v1", "V1")}, "fixVersions"},
		{"story_points", Input{StoryPoints: text("1e3"), StoryPointsField: text("customfield_10016")}, "storyPoints"},
		{"story_points_unpaired", Input{StoryPoints: text("5")}, "storyPoints"},
		{"story_points_field", Input{StoryPoints: text("5"), StoryPointsField: text("points")}, "storyPointsField"},
		{"nothing_to_update", Input{}, "fields"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Prepare(tc.in)
			var invalid *ValidationError
			if !errors.As(err, &invalid) || invalid.Field != tc.field {
				t.Fatalf("error=%v, want a ValidationError for field %q", err, tc.field)
			}
		})
	}
}

func TestMetadata(t *testing.T) {
	p, err := Prepare(Input{Components: text("API"), StoryPoints: text("5"), StoryPointsField: text("customfield_10016")})
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`null`, `{"fields":{}}`, `{"fields":{"components":{"allowedValues":[{"name":"Web"}]}}}`, `{"fields":{"components":{},"customfield_10016":{"schema":{"type":"string"},"operations":["set"]}}}`, `{"fields":{"components":{},"customfield_10016":{"schema":{"type":"number"},"operations":["add"]}}}`} {
		var meta *jira.EditMeta
		if err := json.Unmarshal([]byte(raw), &meta); err != nil {
			t.Fatal(err)
		}
		if err := p.ValidateMetadata(meta); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	var meta jira.EditMeta
	if err := json.Unmarshal([]byte(`{"fields":{"components":{},"customfield_10016":{"schema":{"type":"number"},"operations":["set"]}}}`), &meta); err != nil {
		t.Fatal(err)
	}
	if err := p.ValidateMetadata(&meta); err != nil {
		t.Fatal(err)
	}
}
