package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestWriteSearchJSONUsesCobraOutputAsTopLevelArray(t *testing.T) {
	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	issues := []jira.Issue{{Key: "ARA-1"}, {Key: "ARA-2"}}

	if err := writeSearchJSON(cmd, issues, "example.atlassian.net"); err != nil {
		t.Fatalf("writeSearchJSON() error = %v", err)
	}

	var tickets []jira.TicketJSON
	assertSingleJSONDocument(t, stdout.Bytes(), &tickets)
	if len(tickets) != 2 {
		t.Fatalf("len(tickets) = %d, want 2", len(tickets))
	}
	for i, ticket := range tickets {
		if ticket.SchemaVersion != "v1" {
			t.Errorf("ticket %d SchemaVersion = %q, want v1", i, ticket.SchemaVersion)
		}
	}
}
