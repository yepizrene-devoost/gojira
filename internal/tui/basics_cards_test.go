package tui

import (
	"strings"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

// basicsCards returns a model whose board column and search results list TEST-1 (with a
// status the refreshed ticket does not carry) next to an unrelated TEST-2.
func basicsCards(t *testing.T, j *basicsJira, origin view) Model {
	t.Helper()
	m, _ := basicsModel(t, origin, j.handler(t))
	card := *basicsIssue()
	card.Fields.Status = &jira.StatusField{Name: "In Review"}
	other := jira.Issue{Key: "TEST-2", Fields: jira.IssueFieldData{Summary: "Unrelated", Labels: []string{"keep"}}}
	m.columns = []column{{name: "To Do", issues: []jira.Issue{card, other}}}
	m.searchResults = []jira.Issue{card, other}
	return m
}

func saveEverything(t *testing.T, m Model) Model {
	t.Helper()
	m, _ = basicsPress(m, "U")
	m = basicsSet(basicsSet(basicsSet(m, 0, "New summary"), 1, "High"), 2, "a,b")
	m, cmd := basicsPress(m, "ctrl+s")
	if cmd == nil {
		t.Fatalf("Ctrl+S sent nothing:\n%s", m.View().Content)
	}
	return basicsDeliver(m, cmd)
}

func TestBasicsSaveRefreshesTheBoardCardAndTheSearchRowInPlace(t *testing.T) {
	for name, origin := range map[string]view{"board": viewKanban, "search": viewSearch} {
		t.Run(name, func(t *testing.T) {
			m := saveEverything(t, basicsCards(t, newBasicsJira(), origin))
			for where, issues := range map[string][]jira.Issue{"board card": m.columns[0].issues, "search row": m.searchResults} {
				card, other := issues[0], issues[1]
				if card.Fields.Summary != "New summary" || card.Fields.Priority == nil || card.Fields.Priority.Name != "High" || strings.Join(card.Fields.Labels, ",") != "a,b" {
					t.Errorf("%s was not refreshed: %+v", where, card.Fields)
				}
				if card.Fields.Status == nil || card.Fields.Status.Name != "In Review" {
					t.Errorf("%s lost a field the refresh does not carry: %+v", where, card.Fields.Status)
				}
				if other.Fields.Summary != "Unrelated" || strings.Join(other.Fields.Labels, ",") != "keep" {
					t.Errorf("%s changed an unrelated ticket: %+v", where, other.Fields)
				}
			}
		})
	}
}

func TestBasicsClosedDetailShowsTheNewSummaryOnTheBoard(t *testing.T) {
	m := saveEverything(t, basicsCards(t, newBasicsJira(), viewKanban))
	m, _ = basicsPress(m, "esc")
	content := m.View().Content
	if !strings.Contains(content, "New summary") || strings.Contains(content, "Old summary") {
		t.Fatalf("the board still shows the old summary:\n%s", content)
	}
}

// Bubble Tea passes the model by value: an earlier copy must not see the new values.
func TestBasicsPatchingTheCardsDoesNotMutateEarlierModelCopies(t *testing.T) {
	m := basicsCards(t, newBasicsJira(), viewKanban)
	before := m
	_ = saveEverything(t, m)
	if before.columns[0].issues[0].Fields.Summary != "Old summary" || before.searchResults[0].Fields.Summary != "Old summary" {
		t.Fatal("saving mutated the slices shared with an earlier copy of the model")
	}
}

func TestBasicsFailedSaveLeavesTheCardsUntouched(t *testing.T) {
	j := newBasicsJira()
	j.putStatus = 400
	m := basicsCards(t, j, viewKanban)
	m = saveEverything(t, m)
	if m.columns[0].issues[0].Fields.Summary != "Old summary" || m.searchResults[0].Fields.Summary != "Old summary" {
		t.Fatalf("a rejected update changed the cards: %+v", m.columns[0].issues[0].Fields)
	}
}
