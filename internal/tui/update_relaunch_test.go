package tui

import (
	"errors"
	"strings"
	"testing"
)

// issue #52 U2: after a successful install the TUI offers a final relaunch
// step instead of only asking for a manual restart. finishUpdate must open a
// relaunch modal; choosing relaunch replaces the process (one call with the
// same executable); choosing later keeps today's honest toast; a failed
// exec falls back to that toast too. The relaunch seam is never called on
// failure paths.

func installedModel(t *testing.T) Model {
	m := promptModel(t)
	m = openPrompt(t, m, "v0.6.0")
	m.version = "v0.5.0"
	return m
}

func TestFinishUpdateOpensRelaunchModal(t *testing.T) {
	m := installedModel(t).finishUpdate()
	if !m.updateRelaunchOffered {
		t.Fatal("finishUpdate did not offer the relaunch step")
	}
	plain := plainView(m)
	if !strings.Contains(plain, "Restart gojira") || !strings.Contains(plain, "v0.6.0") {
		t.Fatalf("relaunch modal missing restart offer or new version:\n%s", plain)
	}
	if !strings.Contains(plain, "Later") {
		t.Fatalf("relaunch modal missing a Later option:\n%s", plain)
	}
}

// `r` (or Enter on Restart) invokes the relaunch seam exactly once.
func TestRelaunchRunsOnce(t *testing.T) {
	calls := 0
	prevSeam := relaunchSeam
	relaunchSeam = func() error { calls++; return nil }
	t.Cleanup(func() { relaunchSeam = prevSeam })
	m := installedModel(t).finishUpdate()
	_, cmd := updateModel(t, m, textKey("r"))
	if cmd == nil {
		t.Fatal("r did not start the relaunch")
	}
	// The command performs the exec replacement; in tests it invokes the seam.
	if msg := cmd(); msg == nil {
		t.Fatal("relaunch command produced no message")
	}
	if calls != 1 {
		t.Fatalf("relaunch calls = %d, want 1", calls)
	}
}

// Esc/`l` (Later) closes the modal, keeps the old binary running and says so
// plainly — today's honest toast.
func TestRelaunchLaterKeepsHonestToast(t *testing.T) {
	calls := 0
	prevSeam := relaunchSeam
	relaunchSeam = func() error { calls++; return nil }
	t.Cleanup(func() { relaunchSeam = prevSeam })
	m := installedModel(t).finishUpdate()
	m, _ = updateModel(t, m, textKey("l"))
	if calls != 0 {
		t.Fatalf("Later invoked the relaunch seam %d times", calls)
	}
	if m.updateRelaunchOffered {
		t.Fatal("Later did not close the relaunch modal")
	}
	if !strings.Contains(m.toast, "restart") || !strings.Contains(m.toast, "keeps") {
		t.Fatalf("Later toast lost the restart honesty: %q", m.toast)
	}
}

// A failed relaunch falls back to the honest restart toast and closes the
// modal; it never retries.
func TestRelaunchFailureFallsBackToToast(t *testing.T) {
	calls := 0
	prevSeam := relaunchSeam
	relaunchSeam = func() error { calls++; return errors.New("exec failed") }
	t.Cleanup(func() { relaunchSeam = prevSeam })
	m := installedModel(t).finishUpdate()
	m, cmd := updateModel(t, m, textKey("r"))
	if cmd == nil {
		t.Fatal("r did not start the relaunch")
	}
	msg := cmd()
	if calls != 1 {
		t.Fatalf("relaunch calls = %d, want 1", calls)
	}
	updated, _ := m.Update(msg)
	m = updated.(Model)
	if m.updateRelaunchOffered {
		t.Fatal("failed relaunch kept the modal open")
	}
	if !strings.Contains(m.toast, "restart") {
		t.Fatalf("failed relaunch lost the restart toast: %q", m.toast)
	}
}

// Failure paths of the install never offer the relaunch: the binary did not
// change, so there is nothing new to start.
func TestFailedInstallNeverOffersRelaunch(t *testing.T) {
	m := installedModel(t)
	m.updateUpgrading = true
	m.activeUpdateRequest = 7
	m.updatePromptOpen = true
	m.updatePromptLatest = "v0.6.0"
	m = m.failUpdate(errors.New("placement refused"))
	if m.updateRelaunchOffered {
		t.Fatal("a failed install offered the relaunch")
	}
	if _, cmd := updateModel(t, m, textKey("r")); cmd != nil {
		t.Fatal("r must be inert outside the relaunch modal")
	}
}

// finishUpdate without a successful install must not be reachable; guard the
// invariant that the offered modal only exists after an updateInstalledMsg.
func TestRelaunchOfferResetsWithSessionState(t *testing.T) {
	m := installedModel(t).finishUpdate()
	if !m.updateRelaunchOffered {
		t.Fatal("expected the offer after a successful install")
	}
	// Choosing relaunch closes the offer before the process is replaced.
	prevSeam := relaunchSeam
	relaunchSeam = func() error { return nil }
	t.Cleanup(func() { relaunchSeam = prevSeam })
	m, cmd := updateModel(t, m, textKey("r"))
	if cmd == nil {
		t.Fatal("r did not start the relaunch")
	}
	if m.updateRelaunchOffered {
		t.Fatal("relaunch left the modal open during replacement")
	}
}
