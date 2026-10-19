package picker

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func key(m Model, code rune, text string) Model {
	next, _ := m.Update(tea.KeyPressMsg{Code: code, Text: text})
	return next.(Model)
}

func TestNavigation(t *testing.T) {
	m := New([]Item{{"A", "Alpha"}, {"B", "Beta"}, {"C", "Gamma"}})
	m = key(m, tea.KeyUp, "")
	if m.cursor != 0 {
		t.Fatal("up must clamp at start")
	}
	for range 4 {
		m = key(m, tea.KeyDown, "")
	}
	if m.cursor != 2 {
		t.Fatal("down must clamp at end")
	}
	m = key(m, 'k', "k")
	m = key(m, 'j', "j")
	if m.cursor != 2 || m.filter.Value() != "" {
		t.Fatal("j/k should navigate with empty filter")
	}
}

func TestFilter(t *testing.T) {
	for _, tt := range []struct {
		name, query string
		want        int
	}{
		{"key", "b", 1}, {"name case insensitive", "ALP", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := New([]Item{{"A", "Alpha"}, {"B", "Beta"}})
			m = key(m, tea.KeyDown, "")
			for _, r := range tt.query {
				m = key(m, r, string(r))
			}
			if len(m.matches) != 1 || m.matches[0].Key != []string{"A", "B"}[tt.want] || m.cursor != 0 {
				t.Fatalf("filter/clamp: %+v", m)
			}
		})
	}
	m := New([]Item{{"A", "Ajk"}, {"B", "Ajk"}})
	for _, r := range "ajk" {
		m = key(m, r, string(r))
	}
	if m.filter.Value() != "ajk" || len(m.matches) != 2 || m.matches[0].Key != "A" {
		t.Fatal("nonempty filter must accept j/k and preserve order")
	}
}

func TestSelectionAndCancellation(t *testing.T) {
	for _, tt := range []struct {
		name string
		code rune
	}{{"escape", tea.KeyEscape}, {"interrupt", 'c'}} {
		t.Run(tt.name, func(t *testing.T) {
			m := New([]Item{{"A", "Alpha"}})
			msg := tea.KeyPressMsg{Code: tt.code}
			if tt.name == "interrupt" {
				msg.Mod = tea.ModCtrl
			}
			next, cmd := m.Update(msg)
			m = next.(Model)
			if !errors.Is(m.Err(), ErrCancelled) || m.Selected() != "" || cmd == nil {
				t.Fatal("must cancel without selection")
			}
		})
	}
	m := key(New([]Item{{"A", "Alpha"}}), tea.KeyEnter, "")
	if m.Selected() != "A" || m.Err() != nil {
		t.Fatal("enter must select")
	}
	m = key(New([]Item{{"A", "Alpha"}}), 'z', "z")
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if next.(Model).Selected() != "" || cmd != nil || !strings.Contains(m.View().Content, "No matching projects") {
		t.Fatal("empty result must not select")
	}
}

func TestWindow(t *testing.T) {
	items := make([]Item, 500)
	for i := range items {
		items[i] = Item{fmt.Sprintf("P%d", i), "Project"}
	}
	m := New(items)
	for _, height := range []int{12, 8, 1, 30} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: height})
		m = next.(Model)
		for range 120 {
			m = key(m, tea.KeyDown, "")
		}
		view := m.View().Content
		rows := 0
		for _, line := range strings.Split(view, "\n") {
			if strings.Contains(line, "  Project") {
				rows++
			}
		}
		if rows > m.rows || rows == 0 || strings.Count(view, "\n") > m.rows+4 {
			t.Fatalf("unbounded view: %q", view)
		}
		if !strings.Contains(view, fmt.Sprintf("%d/500", m.cursor+1)) || !strings.Contains(view, "> "+items[m.cursor].Key) {
			t.Fatal("position or highlighted row missing")
		}
	}
}

func TestEmptyPick(t *testing.T) {
	if _, err := Pick(nil); err == nil || errors.Is(err, ErrCancelled) {
		t.Fatal("empty list needs a clear error before terminal startup")
	}
}
