package tui

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func updateCheckModel(t *testing.T, version string) Model {
	t.Helper()
	// The closed port makes a board load fail immediately, so no test waits on
	// the network.
	return New(jira.NewClient("http://127.0.0.1:1", "email", "token"), "example.atlassian.net").WithVersion(version)
}

// batchMessages runs one Init command and returns every message its batch
// produces.
func batchMessages(t *testing.T, command tea.Cmd) []tea.Msg {
	t.Helper()
	if command == nil {
		return nil
	}
	batch, ok := command().(tea.BatchMsg)
	if !ok {
		return []tea.Msg{command()}
	}
	messages := make([]tea.Msg, 0, len(batch))
	for _, sub := range batch {
		if sub == nil {
			continue
		}
		messages = append(messages, sub())
	}
	return messages
}

func TestUpdateCheckNotifiesOnceForANewerRelease(t *testing.T) {
	recorded := ""
	m := updateCheckModel(t, "v0.5.0").WithUpdateCheck(
		func(ctx context.Context) (string, error) {
			if _, ok := ctx.Deadline(); !ok {
				t.Error("release check ran without a deadline")
			}
			return "v0.6.0", nil
		},
		func(latest string) { recorded = latest },
	)

	message := m.checkForUpdate()
	available, ok := message.(updateAvailableMsg)
	if !ok {
		t.Fatalf("checkForUpdate() = %T, want updateAvailableMsg", message)
	}
	if available.latest != "v0.6.0" {
		t.Fatalf("latest = %q, want v0.6.0", available.latest)
	}

	updated, _ := m.Update(message)
	model, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	if want := "GoJira v0.6.0 is available — run gojira upgrade"; model.toast != want {
		t.Fatalf("toast = %q, want %q", model.toast, want)
	}
	if recorded != "v0.6.0" {
		t.Fatalf("recorded = %q, want the checked release", recorded)
	}
}

func TestUpdateCheckStaysSilent(t *testing.T) {
	cases := []struct {
		name   string
		latest string
		err    error
	}{
		{name: "offline", err: errors.New("no network")},
		{name: "same version", latest: "v0.5.0"},
		{name: "older release", latest: "v0.4.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorded := 0
			m := updateCheckModel(t, "v0.5.0").WithUpdateCheck(
				func(context.Context) (string, error) { return tc.latest, tc.err },
				func(string) { recorded++ },
			)
			if message := m.checkForUpdate(); message != nil {
				t.Fatalf("checkForUpdate() = %T, want no notification", message)
			}
			if tc.err == nil && recorded != 1 {
				t.Fatalf("recorded %d successful checks, want 1", recorded)
			}
			if tc.err != nil && recorded != 0 {
				t.Fatalf("recorded %d failed checks, want 0", recorded)
			}
		})
	}
}

func TestUpdateCheckIgnoresDevelopmentBuilds(t *testing.T) {
	recorded := 0
	m := updateCheckModel(t, "dev").WithUpdateCheck(
		func(context.Context) (string, error) { return "v9.9.9", nil },
		func(string) { recorded++ },
	)
	if message := m.checkForUpdate(); message != nil {
		t.Fatalf("checkForUpdate() = %T, want no notification for a development build", message)
	}
	if recorded != 1 {
		t.Fatalf("recorded %d checks, want the successful check to be recorded", recorded)
	}
}

func TestInitStartsTheUpdateCheckOnlyWhenConfigured(t *testing.T) {
	withCheck := updateCheckModel(t, "v0.5.0").WithUpdateCheck(
		func(context.Context) (string, error) { return "v0.6.0", nil },
		nil,
	)
	found := false
	for _, message := range batchMessages(t, withCheck.Init()) {
		if available, ok := message.(updateAvailableMsg); ok {
			found = true
			if available.latest != "v0.6.0" {
				t.Fatalf("latest = %q, want v0.6.0", available.latest)
			}
		}
	}
	if !found {
		t.Fatal("Init did not run the configured update check")
	}

	for _, message := range batchMessages(t, updateCheckModel(t, "v0.5.0").Init()) {
		if _, ok := message.(updateAvailableMsg); ok {
			t.Fatal("Init ran an update check without a configured checker")
		}
	}
}
