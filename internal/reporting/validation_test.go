package reporting_test

import (
	"github.com/yepizrene-devoost/gojira/internal/reporting"
	"testing"
)

func TestSharedValidation(t *testing.T) {
	for _, query := range []string{"", " \t"} {
		if err := reporting.ValidateUserQuery(query); err == nil || err.Error() != "query is required" {
			t.Fatalf("query %q: %v", query, err)
		}
	}
	if err := reporting.ValidateUserQuery("Alex"); err != nil {
		t.Fatal(err)
	}
	valid := reporting.TimeFilters{AccountID: "person", Project: "DEMO", From: "2026-03-08", To: "2026-03-08", Timezone: "America/New_York"}
	if err := reporting.ValidateTimeFilters(valid); err != nil {
		t.Fatal(err)
	}
	invalid := valid
	invalid.To = "2026-03-07"
	if err := reporting.ValidateTimeFilters(invalid); err == nil || err.Error() != "invalid inclusive date range" {
		t.Fatalf("invalid range: %v", err)
	}
}
