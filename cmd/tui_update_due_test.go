package cmd

import (
	"testing"
	"time"

	"github.com/yepizrene-devoost/gojira/internal/config"
)

// Issue #52: the launch decision must consult the configured interval, not
// the hardcoded 24h constant. updateTUIUpdateCheck wires the check only when
// it is due; a fresh install (nothing recorded) is always due.
func TestUpdateCheckDueUsesConfiguredInterval(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name    string
		cfg     *config.Config
		wantDue bool
	}{
		{name: "fresh install with no interval: due", cfg: &config.Config{}, wantDue: true},
		{name: "checked 1h ago, default interval 0: due", cfg: func() *config.Config {
			c := &config.Config{}
			c.RecordUpdateCheck(now.Add(-1*time.Hour), "v0.8.1")
			return c
		}(), wantDue: true},
		{name: "checked 1h ago, configured 6h: not due", cfg: func() *config.Config {
			c := &config.Config{UpdateCheckInterval: "6h"}
			c.RecordUpdateCheck(now.Add(-1*time.Hour), "v0.8.1")
			return c
		}(), wantDue: false},
		{name: "checked 7h ago, configured 6h: due", cfg: func() *config.Config {
			c := &config.Config{UpdateCheckInterval: "6h"}
			c.RecordUpdateCheck(now.Add(-7*time.Hour), "v0.8.1")
			return c
		}(), wantDue: true},
		{name: "checked 25h ago, hardcoded 24h would be due: due with 0", cfg: func() *config.Config {
			c := &config.Config{}
			c.RecordUpdateCheck(now.Add(-25*time.Hour), "v0.8.1")
			return c
		}(), wantDue: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := updateCheckDueForTUI(tc.cfg, now)
			if got != tc.wantDue {
				t.Fatalf("updateCheckDueForTUI() = %v, want %v", got, tc.wantDue)
			}
		})
	}
}

// The launch wiring must derive the interval from the loaded config (the
// configured value), never from the hardcoded constant. A disabled check or
// a config failure never wires the check.
func TestTUIUpdateCheckWiringUsesConfiguredInterval(t *testing.T) {
	now := time.Now()
	fresh := &config.Config{} // nothing recorded: due with any interval
	if !updateCheckDueForTUI(fresh, now) {
		t.Fatal("fresh install must be due")
	}
	hourAgo := &config.Config{UpdateCheckInterval: "6h"}
	hourAgo.RecordUpdateCheck(now.Add(-1*time.Hour), "v0.8.1")
	if updateCheckDueForTUI(hourAgo, now) {
		t.Fatal("an hour-old check must not be due with a 6h interval")
	}
}
