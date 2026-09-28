package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestWriteExportJSONUsesCobraOutputAsTopLevelArray(t *testing.T) {
	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	issues := []jira.Issue{{Key: "ARA-10"}}

	if err := writeExportJSON(cmd, issues, "example.atlassian.net"); err != nil {
		t.Fatalf("writeExportJSON() error = %v", err)
	}

	var tickets []jira.TicketJSON
	assertSingleJSONDocument(t, stdout.Bytes(), &tickets)
	if len(tickets) != 1 || tickets[0].Key != "ARA-10" || tickets[0].SchemaVersion != "v1" {
		t.Fatalf("tickets = %#v, want one v1 ARA-10 ticket", tickets)
	}
}
