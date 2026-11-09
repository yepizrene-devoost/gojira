package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yepizrene-devoost/gojira/internal/upgrade"
)

// releaseURLV060 is the release page the prompt must show and open.
const releaseURLV060 = "https://github.com/yepizrene-devoost/gojira/releases/tag/v0.6.0"

// promptModel returns the shared boards-view model sized for the prompt tests.
func promptModel(t *testing.T) Model {
	t.Helper()
	m := updateCheckModel(t, "v0.5.0")
	m.width, m.height = 120, 40
	return m
}

// openPrompt delivers one updateAvailableMsg the way the startup check does.
func openPrompt(t *testing.T, m Model, latest string) Model {
	t.Helper()
	updated, _ := updateModel(t, m, updateAvailableMsg{latest: latest})
	return updated
}

func plainView(m Model) string { return ansi.Strip(m.View().Content) }

// flatView drops the modal frame and collapses the modal's line wrapping so
// exact installer messages can be asserted as written.
func flatView(m Model) string {
	stripped := strings.ReplaceAll(plainView(m), "│", "")
	return strings.Join(strings.Fields(stripped), " ")
}

// Only an upgrade.Completed check with a version newer than the running build
// may open the prompt: Skipped and Unsuccessful outcomes stay silent and never
// render it, and a completed check without a newer release does nothing.
func TestUpdatePromptOpensOnlyForACompletedNewerRelease(t *testing.T) {
	t.Run("completed newer opens the prompt", func(t *testing.T) {
		recorded := ""
		m := promptModel(t).WithUpdateCheckResult(
			func(context.Context) upgrade.CheckResult {
				return upgrade.CheckResult{State: upgrade.Completed, LatestVersion: "v0.6.0"}
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
		if recorded != "v0.6.0" {
			t.Fatalf("recorded = %q, want the completed check's release", recorded)
		}
		plain := plainView(openPrompt(t, m, available.latest))
		if !strings.Contains(plain, "gojira v0.5.0 → v0.6.0") {
			t.Fatalf("prompt does not show the version line:\n%s", plain)
		}
	})

	t.Run("skipped never opens the prompt", func(t *testing.T) {
		recorded := 0
		m := promptModel(t).WithUpdateCheckResult(
			func(context.Context) upgrade.CheckResult {
				return upgrade.CheckResult{State: upgrade.Skipped, Reason: upgrade.SkipInsideInterval, LatestVersion: "v0.6.0"}
			},
			func(string) { recorded++ },
		)
		if message := m.checkForUpdate(); message != nil {
			t.Fatalf("checkForUpdate() = %T, want no message for a skipped check", message)
		}
		if recorded != 0 {
			t.Fatalf("recorded %d skipped checks, want 0", recorded)
		}
		if plain := plainView(m); strings.Contains(plain, "Update available") {
			t.Fatalf("skipped check opened the prompt:\n%s", plain)
		}
	})

	t.Run("unsuccessful never opens the prompt and stays silent", func(t *testing.T) {
		recorded := 0
		m := promptModel(t).WithUpdateCheckResult(
			func(context.Context) upgrade.CheckResult {
				return upgrade.CheckResult{State: upgrade.Unsuccessful, Err: errors.New("no network")}
			},
			func(string) { recorded++ },
		)
		if message := m.checkForUpdate(); message != nil {
			t.Fatalf("checkForUpdate() = %T, want silence for a failed check", message)
		}
		if recorded != 0 {
			t.Fatalf("recorded %d failed checks, want 0", recorded)
		}
		plain := plainView(m)
		if strings.Contains(plain, "Update available") || m.toast != "" {
			t.Fatalf("failed check was not silent: toast=%q\n%s", m.toast, plain)
		}
	})

	t.Run("completed without a newer release never opens the prompt", func(t *testing.T) {
		m := promptModel(t).WithUpdateCheckResult(
			func(context.Context) upgrade.CheckResult {
				return upgrade.CheckResult{State: upgrade.Completed, LatestVersion: "v0.5.0"}
			},
			nil,
		)
		if message := m.checkForUpdate(); message != nil {
			t.Fatalf("checkForUpdate() = %T, want no message for the running version", message)
		}
	})
}

// The prompt is built from the established modal helpers: it lists the
// installed and latest versions, the release URL and the three options.
func TestUpdatePromptListsVersionsURLOptions(t *testing.T) {
	m := openPrompt(t, promptModel(t), "v0.6.0")
	plain := plainView(m)
	for _, want := range []string{
		"gojira v0.5.0 → v0.6.0",
		releaseURLV060,
		"Update now",
		"View changes",
		"Keep current version",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("prompt does not contain %q:\n%s", want, plain)
		}
	}
}

// S6: the modal is centred with the existing helpers and stays inside the
// terminal at narrow and wide sizes; its content wraps instead of overflowing.
func TestUpdatePromptIsCentredAndInsideBounds(t *testing.T) {
	cases := []struct {
		name    string
		width   int
		height  int
		wrapped bool // URL must be broken across lines
	}{
		{name: "narrow", width: 60, height: 20, wrapped: true},
		{name: "wide", width: 200, height: 50},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := promptModel(t)
			m.width, m.height = tc.width, tc.height
			m = openPrompt(t, m, "v0.6.0")
			content := m.View().Content
			lines := strings.Split(content, "\n")
			if len(lines) > tc.height {
				t.Fatalf("view height=%d exceeds terminal height %d", len(lines), tc.height)
			}
			for i, line := range lines {
				if got := ansi.StringWidth(line); got > tc.width {
					t.Fatalf("line %d width=%d exceeds terminal width %d:\n%s", i+1, got, tc.width, line)
				}
			}
			plain := strings.Split(ansi.Strip(content), "\n")
			frame := "│"
			cellCol := func(s, marker string) int {
				idx := strings.Index(s, marker)
				if idx < 0 {
					return -1
				}
				return ansi.StringWidth(s[:idx])
			}
			row := -1
			for i, line := range plain {
				if strings.Contains(line, "gojira v0.5.0 → v0.6.0") {
					row = i
					break
				}
			}
			if row < 0 {
				t.Fatalf("prompt not rendered:\n%s", strings.Join(plain, "\n"))
			}
			line := plain[row]
			leftByte := strings.Index(line, frame)
			rightByte := strings.LastIndex(line, frame)
			if leftByte < 0 || rightByte <= leftByte {
				t.Fatalf("no modal frame on the version row:\n%s", line)
			}
			left := ansi.StringWidth(line[:leftByte])
			boxW := ansi.StringWidth(line[:rightByte+len(frame)]) - left
			if want := (tc.width - boxW) / 2; left != want {
				t.Errorf("modal starts at column %d, want centred at %d:\n%s", left, want, line)
			}
			// All four rounded corners of the modal frame survive the overlay.
			top, bottom := -1, -1
			for i := 0; i <= row; i++ {
				if cellCol(plain[i], "╭") == left {
					top = i
				}
			}
			for i := row; i < len(plain); i++ {
				if cellCol(plain[i], "╰") == left {
					bottom = i
				}
			}
			if top < 0 || bottom <= top {
				t.Fatalf("modal frame clipped vertically:\n%s", strings.Join(plain, "\n"))
			}
			if !strings.Contains(plain[top], "╮") || !strings.Contains(plain[bottom], "╯") {
				t.Fatalf("modal corners clipped:\n%s\n%s", plain[top], plain[bottom])
			}
			joined := strings.Join(plain, "\n")
			if tc.wrapped {
				if !strings.Contains(joined, "https://github.com/yepizrene-") ||
					strings.Contains(joined, releaseURLV060) {
					t.Fatalf("release URL was not wrapped to the modal width:\n%s", joined)
				}
			} else if !strings.Contains(joined, releaseURLV060) {
				t.Fatalf("release URL missing from wide prompt:\n%s", joined)
			}
			for _, want := range []string{"Update now", "View changes", "Keep current version"} {
				if !strings.Contains(joined, want) {
					t.Errorf("bounded prompt dropped option %q:\n%s", want, joined)
				}
			}
		})
	}
}

// The cursor starts on Keep current version, not on Update now.
func TestUpdatePromptCursorStartsOnKeep(t *testing.T) {
	plain := plainView(openPrompt(t, promptModel(t), "v0.6.0"))
	if !strings.Contains(plain, "→ Keep current version") {
		t.Fatalf("cursor is not on Keep current version:\n%s", plain)
	}
	if strings.Contains(plain, "→ Update now") || strings.Contains(plain, "→ View changes") {
		t.Fatalf("cursor is not on Keep:\n%s", plain)
	}
}

// Arrows move the cursor between the three options, and Enter confirms the
// selection: on Keep it closes the prompt while the status line stays.
func TestUpdatePromptArrowsMoveAndEnterConfirms(t *testing.T) {
	m := openPrompt(t, promptModel(t), "v0.6.0")

	m, _ = updateModel(t, m, specialKey(tea.KeyUp))
	if plain := plainView(m); !strings.Contains(plain, "→ View changes") {
		t.Fatalf("up from Keep did not select View changes:\n%s", plain)
	}
	m, _ = updateModel(t, m, specialKey(tea.KeyUp))
	if plain := plainView(m); !strings.Contains(plain, "→ Update now") {
		t.Fatalf("second up did not select Update now:\n%s", plain)
	}
	m, _ = updateModel(t, m, specialKey(tea.KeyDown))
	if plain := plainView(m); !strings.Contains(plain, "→ View changes") {
		t.Fatalf("down did not return to View changes:\n%s", plain)
	}
	m, _ = updateModel(t, m, specialKey(tea.KeyDown)) // back on Keep
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))

	plain := plainView(m)
	if strings.Contains(plain, "Keep current version") {
		t.Fatalf("enter on Keep did not close the prompt:\n%s", plain)
	}
	if !strings.Contains(plain, "⬆ GoJira v0.6.0 is available") {
		t.Fatalf("status line must stay after keeping:\n%s", plain)
	}
}

// The u shortcut is the single dispatch path for Update now (S8): it starts
// the install pipeline inside the modal, which stays open reporting progress.
// The completed outcomes are covered in update_install_test.go.
func TestUpdatePromptUpdateShortcutStartsUpgradeInsideModal(t *testing.T) {
	m := openPrompt(t, promptModel(t), "v0.6.0")
	m, cmd := updateModel(t, m, textKey("u"))

	if cmd == nil {
		t.Fatal("u produced no upgrade command")
	}
	if !m.updateUpgrading {
		t.Fatal("u did not mark the upgrade busy")
	}
	plain := plainView(m)
	if !strings.Contains(plain, updateStageChecking) {
		t.Fatalf("u did not start the upgrade inside the modal:\n%s", plain)
	}
	if strings.Contains(plain, "Keep current version") {
		t.Fatalf("busy modal must replace the options with progress:\n%s", plain)
	}
}

// The c shortcut keeps the current version directly: the prompt closes and the
// status line keeps the newer version visible.
func TestUpdatePromptKeepShortcutClosesPrompt(t *testing.T) {
	m := openPrompt(t, promptModel(t), "v0.6.0")
	m, _ = updateModel(t, m, textKey("c"))

	plain := plainView(m)
	if strings.Contains(plain, "Keep current version") {
		t.Fatalf("c did not close the prompt:\n%s", plain)
	}
	if !strings.Contains(plain, "⬆ GoJira v0.6.0 is available") {
		t.Fatalf("status line must stay after keeping:\n%s", plain)
	}
}

// esc keeps the current version: the prompt closes, the status line stays.
func TestUpdatePromptEscKeepsAndCloses(t *testing.T) {
	m := openPrompt(t, promptModel(t), "v0.6.0")
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))

	plain := plainView(m)
	if strings.Contains(plain, "Keep current version") {
		t.Fatalf("esc did not close the prompt:\n%s", plain)
	}
	if !strings.Contains(plain, "⬆ GoJira v0.6.0 is available") {
		t.Fatalf("status line must stay after esc:\n%s", plain)
	}
}

// v (and confirming View changes) opens the release URL through the browser
// helper and leaves the modal open.
func TestUpdatePromptViewChangesOpensReleaseURLAndKeepsPrompt(t *testing.T) {
	cases := []struct {
		name string
		act  func(Model) (Model, tea.Cmd)
	}{
		{name: "shortcut", act: func(m Model) (Model, tea.Cmd) { return updateModel(t, m, textKey("v")) }},
		{name: "enter", act: func(m Model) (Model, tea.Cmd) {
			m, _ = updateModel(t, m, specialKey(tea.KeyUp)) // cursor on View changes
			return updateModel(t, m, specialKey(tea.KeyEnter))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := promptModel(t)
			opened := ""
			m.releaseBrowser = func(url string) tea.Cmd {
				opened = url
				return func() tea.Msg { return nil }
			}
			m = openPrompt(t, m, "v0.6.0")
			var cmd tea.Cmd
			m, cmd = tc.act(m)
			if opened != releaseURLV060 {
				t.Fatalf("browser opened %q, want %q", opened, releaseURLV060)
			}
			if cmd == nil {
				t.Fatal("view changes produced no command")
			}
			if plain := plainView(m); !strings.Contains(plain, "Keep current version") {
				t.Fatalf("prompt closed after viewing changes:\n%s", plain)
			}
		})
	}
}

// S5: a version the user already dismissed or viewed never reopens the prompt,
// not even when the next start checks the same release again; the dismissal is
// persisted through the config recorder.
func TestDismissedOrViewedVersionNeverReopens(t *testing.T) {
	cases := []struct {
		name      string
		staysOpen bool // viewing changes keeps the prompt open by design (S9)
		act       func(Model) (Model, tea.Cmd)
	}{
		{name: "dismissed with esc", act: func(m Model) (Model, tea.Cmd) { return updateModel(t, m, specialKey(tea.KeyEsc)) }},
		{name: "dismissed with c", act: func(m Model) (Model, tea.Cmd) { return updateModel(t, m, textKey("c")) }},
		{name: "viewed with v", staysOpen: true, act: func(m Model) (Model, tea.Cmd) { return updateModel(t, m, textKey("v")) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorded := ""
			m := promptModel(t).WithUpdateCheck(
				func(context.Context) (string, error) { return "v0.6.0", nil },
				func(latest string) { recorded = latest },
			)
			m.releaseBrowser = func(string) tea.Cmd { return func() tea.Msg { return nil } }
			m = openPrompt(t, m, "v0.6.0")
			m, _ = tc.act(m)
			isOpen := strings.Contains(plainView(m), "Keep current version")
			if isOpen != tc.staysOpen {
				t.Fatalf("prompt open=%v after %s, want %v", isOpen, tc.name, tc.staysOpen)
			}
			if isOpen { // close it after viewing so only the dismissal guard is tested
				m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
			}
			if recorded != "v0.6.0" {
				t.Fatalf("recorder = %q, want the dismissed or viewed version persisted", recorded)
			}

			// The next start checks the same release again.
			message := m.checkForUpdate()
			available, ok := message.(updateAvailableMsg)
			if !ok {
				t.Fatalf("re-check = %T, want updateAvailableMsg", message)
			}
			m = openPrompt(t, m, available.latest)
			if plain := plainView(m); strings.Contains(plain, "Keep current version") {
				t.Fatalf("prompt reopened for the dismissed version:\n%s", plain)
			}
			if plain := plainView(m); !strings.Contains(plain, "⬆ GoJira v0.6.0 is available") {
				t.Fatalf("status line must survive the re-check:\n%s", plain)
			}
		})
	}
}

// S10: while a newer version is known and unresolved the boards header shows a
// status line; it disappears once the running version is current. The
// open prompt overlays the header, so this render check starts from the
// resolved model state; the message-to-status wiring is exercised end to end
// by the dismissal tests below.
func TestBoardsHeaderShowsUpdateStatusUntilCurrent(t *testing.T) {
	t.Run("appears while a newer version is known", func(t *testing.T) {
		m := promptModel(t)
		m.updateAvailable = "v0.6.0"
		if plain := plainView(m); !strings.Contains(plain, "⬆ GoJira v0.6.0 is available") {
			t.Fatalf("status line missing from boards header:\n%s", plain)
		}
	})

	t.Run("disappears when the running version is current", func(t *testing.T) {
		m := promptModel(t)
		m.updateAvailable = "v0.6.0"
		m = m.WithVersion("v0.6.0")
		if plain := plainView(m); strings.Contains(plain, "⬆ GoJira v0.6.0 is available") {
			t.Fatalf("status line shown for a current version:\n%s", plain)
		}
	})

	t.Run("never appears for a check without a newer release", func(t *testing.T) {
		m := promptModel(t).WithUpdateCheck(
			func(context.Context) (string, error) { return "v0.5.0", nil },
			nil,
		)
		if message := m.checkForUpdate(); message != nil {
			t.Fatalf("checkForUpdate() = %T, want no message", message)
		}
		if plain := plainView(m); strings.Contains(plain, "⬆ GoJira") {
			t.Fatalf("status line shown without a newer version:\n%s", plain)
		}
	})
}

// Startup never blocks: Init only schedules the check as a command, and the
// command performs it asynchronously under the startup timeout.
func TestInitRunsTheUpdateCheckAsynchronously(t *testing.T) {
	calls := 0
	m := updateCheckModel(t, "v0.5.0").WithUpdateCheck(
		func(context.Context) (string, error) {
			calls++
			return "v0.6.0", nil
		},
		nil,
	)

	init := m.Init()
	if calls != 0 {
		t.Fatalf("Init ran the release check synchronously (%d calls)", calls)
	}
	found := false
	for _, message := range batchMessages(t, init) {
		if _, ok := message.(updateAvailableMsg); ok {
			found = true
		}
	}
	if calls == 0 {
		t.Fatal("Init's batch never ran the release check")
	}
	if !found {
		t.Fatal("Init's batch did not report the newer release")
	}
}
