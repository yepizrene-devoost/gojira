package agentguide

import "testing"

func TestPlanRejectsReservedNames(t *testing.T) {
	for _, path := range []string{"AGENTS.md", "nested/agents.MD", "CLAUDE.md", "nested/ClAuDe.Md"} {
		t.Run(path, func(t *testing.T) {
			if _, err := Plan(path, false, false); err == nil {
				t.Fatal("reserved target accepted")
			}
		})
	}
}

func TestPlanRejectsDirectory(t *testing.T) {
	if _, err := Plan("guide", true, true); err == nil {
		t.Fatal("directory accepted")
	}
}

func TestPlanRejectsExisting(t *testing.T) {
	if _, err := Plan("guide.md", true, false); err == nil {
		t.Fatal("existing target accepted")
	}
}

func TestPlanRejectsEmptyPath(t *testing.T) {
	if _, err := Plan("", false, false); err == nil {
		t.Fatal("empty path accepted")
	}
}
