package jira_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yepizrene-devoost/gojira/internal/jira"
	"github.com/yepizrene-devoost/gojira/internal/reporting"
)

func reportFilters() reporting.TimeFilters {
	return reporting.TimeFilters{AccountID: "a", Project: "P", From: "2026-03-08", To: "2026-03-08", Timezone: "America/New_York"}
}

func TestReportingInitialStatus(t *testing.T) {
	for _, status := range []int{401, 403, 404} {
		for _, users := range []bool{true, false} {
			t.Run(fmt.Sprintf("%d/users=%t", status, users), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "private response", status) }))
				defer server.Close()
				client := jira.NewClient(server.URL, "", "")
				var err error
				if users {
					_, err = reporting.SearchUsers(context.Background(), client, "name")
				} else {
					_, err = reporting.TimeReport(context.Background(), client, reportFilters())
				}
				var api *jira.APIError
				if !errors.As(err, &api) || api.Status != status || api.Body != "" {
					t.Fatalf("expected sanitized APIError %d, got %#v", status, err)
				}
			})
		}
	}
}

func TestReportingInvalidJSON(t *testing.T) {
	for _, body := range []string{"null", "[] {}", "[] malformed"} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, body) }))
			defer server.Close()
			_, err := reporting.SearchUsers(context.Background(), jira.NewClient(server.URL, "", ""), "name")
			if err == nil {
				t.Fatal("invalid response accepted")
			}
		})
	}
}

func TestReportingInflightCancellation(t *testing.T) {
	for _, users := range []bool{true, false} {
		t.Run(fmt.Sprint(users), func(t *testing.T) {
			entered := make(chan struct{})
			stopped := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done(); close(stopped) }))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			client := jira.NewClient(server.URL, "", "")
			go func() {
				var err error
				if users {
					_, err = reporting.SearchUsers(ctx, client, "name")
				} else {
					_, err = reporting.TimeReport(ctx, client, reportFilters())
				}
				result <- err
			}()
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("request did not start")
			}
			cancel()
			select {
			case err := <-result:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("got %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("operation did not cancel")
			}
			select {
			case <-stopped:
			case <-time.After(2 * time.Second):
				t.Fatal("HTTP request did not cancel")
			}
		})
	}
}
