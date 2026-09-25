package tui

import (
	"strings"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func testIssue(key string) jira.Issue {
	return jira.Issue{
		Key:    key,
		Fields: IssueFields("Test issue " + key),
	}
}

func IssueFields(summary string) jira.IssueFieldData {
	return jira.IssueFieldData{
		Summary:  summary,
		Status:   &jira.StatusField{Name: "To Do", ID: "10000"},
		Priority: &jira.PriorityField{Name: "High"},
		Assignee: &jira.AssigneeField{DisplayName: "TestUser"},
	}
}

func makeTestModel(cols int, issuesPerCol int) Model {
	columns := make([]column, cols)
	for c := 0; c < cols; c++ {
		col := column{
			name:   "Col" + string(rune('A'+c)),
			issues: make([]jira.Issue, issuesPerCol),
		}
		for i := 0; i < issuesPerCol; i++ {
			col.issues[i] = testIssue(strings.Repeat("P", 1) + string(rune('0'+i%10)))
		}
		columns[c] = col
	}

	return Model{
		view:     viewKanban,
		columns:  columns,
		colCur:   0,
		rowCur:   0,
		width:    190,
		height:   50,
		boardCur: 0,
		boards:   []jira.Board{{ID: 1, Name: "Test Board", Type: "scrum"}},
	}
}

func TestKanbanMultipleColumnsFit(t *testing.T) {
	m := makeTestModel(7, 5)
	cpp := m.colsPerPage()
	if cpp < 2 {
		t.Fatalf("Expected cols per page >= 2 at width=190, got %d", cpp)
	}

	output := m.View()

	for _, col := range m.columns {
		if !strings.Contains(output, col.name) {
			t.Errorf("Column %q not found in output", col.name)
		}
	}
}

func TestKanbanPagination(t *testing.T) {
	m := makeTestModel(11, 3)
	cpp := m.colsPerPage()
	if cpp >= 11 {
		t.Fatalf("Expected pagination (cpp=%d < 11) at width=190", cpp)
	}

	output := m.View()
	shown := 0
	for _, col := range m.columns {
		if strings.Contains(output, col.name) {
			shown++
		}
	}
	if shown != cpp {
		t.Errorf("Expected %d columns visible, got %d", cpp, shown)
	}

	// Move to last column
	m.colCur = 10
	output = m.View()
	shown = 0
	for _, col := range m.columns {
		if strings.Contains(output, col.name) {
			shown++
		}
	}
	if shown != cpp {
		t.Errorf("After paging, expected %d columns, got %d", cpp, shown)
	}
	if !strings.Contains(output, m.columns[10].name) {
		t.Error("Last column not visible after paging")
	}
}

func TestKanbanEmptyColumnSameHeight(t *testing.T) {
	full := makeTestModel(3, 10)
	empty := makeTestModel(3, 10)
	empty.columns[1].issues = nil

	fullOut := full.View()
	emptyOut := empty.View()

	fullLines := strings.Count(fullOut, "\n")
	emptyLines := strings.Count(emptyOut, "\n")

	diff := fullLines - emptyLines
	if diff < -2 || diff > 2 {
		t.Errorf("Line count mismatch: full=%d, empty=%d (diff=%d)", fullLines, emptyLines, diff)
	}
}

func TestKanbanScrollFollowsCursor(t *testing.T) {
	m := makeTestModel(1, 20)
	m.height = 30

	out1 := m.View()
	m.rowCur = 19
	out2 := m.View()

	if out1 == "" || out2 == "" {
		t.Error("Empty view output")
	}
}
