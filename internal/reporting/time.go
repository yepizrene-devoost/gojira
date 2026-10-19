package reporting

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

var projectPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

func interval(f TimeFilters) (time.Time, time.Time, error) {
	if strings.TrimSpace(f.AccountID) == "" || !projectPattern.MatchString(f.Project) || f.Timezone == "" || f.Timezone == "Local" {
		return time.Time{}, time.Time{}, errors.New("account ID, project key and IANA timezone are required")
	}
	loc, err := time.LoadLocation(f.Timezone)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("invalid IANA timezone")
	}
	from, e1 := time.ParseInLocation("2006-01-02", f.From, loc)
	to, e2 := time.ParseInLocation("2006-01-02", f.To, loc)
	if e1 != nil || e2 != nil || to.Before(from) {
		return from, to, errors.New("invalid inclusive date range")
	}
	return from, to.AddDate(0, 0, 1), nil
}

// ValidateTimeFilters checks report scope and dates without network access.
func ValidateTimeFilters(f TimeFilters) error {
	_, _, err := interval(f)
	return err
}

func TimeReport(ctx context.Context, reader TimeReader, f TimeFilters) (TimeReportResult, error) {
	r := TimeReportResult{SchemaVersion: "v1", Filters: f, Rows: []TimeRow{}, CompletenessScope: "visible_to_authenticated_user", Warnings: []string{"Only issues and worklogs visible to the authenticated user are included; permissions may hide data. This scan is not a transactional snapshot."}}
	from, to, err := interval(f)
	if err != nil {
		return r, err
	}
	totals := map[string]int64{}
	issues := map[string]bool{}
	keys := map[string]bool{}
	logs := map[string]bool{}
	tokens := map[string]bool{}
	token := ""
	finish := func(reason string) (TimeReportResult, error) {
		if reason != "" {
			r.Warnings = append(r.Warnings, reason)
		} else {
			r.Complete = true
		}
		for k, v := range totals {
			r.Rows = append(r.Rows, TimeRow{Key: k, Seconds: v})
		}
		sort.Slice(r.Rows, func(i, j int) bool { return r.Rows[i].Key < r.Rows[j].Key })
		return r, nil
	}
	failure := func(cause error) (TimeReportResult, error) {
		if ctx.Err() != nil {
			return r, ctx.Err()
		}
		if r.IssuesScanned == 0 {
			return r, fmt.Errorf("report scan failed: %w", cause)
		}
		var status interface{ HTTPStatus() int }
		if errors.As(cause, &status) {
			return finish(fmt.Sprintf("Page request failed (HTTP %d); totals are partial. Check Jira access for authentication or permission failures.", status.HTTPStatus()))
		}
		return finish("Page request failed; totals are partial.")
	}
	for {
		if ctx.Err() != nil {
			return r, ctx.Err()
		}
		if r.PageRequests >= MaxPageRequests {
			return finish("Page request budget reached; totals are partial.")
		}
		r.PageRequests++
		page, err := reader.ReportingIssues(ctx, `project = "`+f.Project+`" ORDER BY key ASC`, token)
		if err != nil {
			return failure(err)
		}
		if page.IsLast == nil || page.Issues == nil || len(page.Issues) > 100 {
			return finish("Invalid issue paging metadata; totals are partial.")
		}
		for _, issue := range page.Issues {
			if ctx.Err() != nil {
				return r, ctx.Err()
			}
			if r.IssuesScanned >= MaxIssues {
				return finish("Visible issue cap reached; totals are partial.")
			}
			if issue.ID == "" || issue.Key == "" || issues[issue.ID] || keys[issue.Key] {
				return finish("Invalid or duplicate issue; totals are partial.")
			}
			issues[issue.ID] = true
			keys[issue.Key] = true
			r.IssuesScanned++
			offset := 0
			for {
				if ctx.Err() != nil {
					return r, ctx.Err()
				}
				if r.PageRequests >= MaxPageRequests {
					return finish("Page request budget reached; totals are partial.")
				}
				r.PageRequests++
				wp, err := reader.ReportingWorklogs(ctx, issue.ID, offset)
				if err != nil {
					return failure(err)
				}
				if wp.Worklogs == nil || wp.StartAt == nil || wp.MaxResults == nil || wp.Total == nil || *wp.StartAt != offset || *wp.MaxResults <= 0 || *wp.Total < offset || len(wp.Worklogs) > *wp.MaxResults || len(wp.Worklogs) > *wp.Total-offset {
					return finish("Invalid worklog paging metadata; totals are partial.")
				}
				for _, w := range wp.Worklogs {
					if w.ID == "" || logs[w.ID] || w.Seconds == nil || *w.Seconds < 0 || w.Author.AccountID == "" {
						return finish("Invalid or duplicate worklog; totals are partial.")
					}
					started, err := time.Parse(time.RFC3339Nano, w.Started)
					if err != nil {
						started, err = time.Parse("2006-01-02T15:04:05.999999999-0700", w.Started)
					}
					if err != nil {
						return finish("Invalid worklog started instant; totals are partial.")
					}
					logs[w.ID] = true
					r.WorklogsScanned++
					if w.Author.AccountID == f.AccountID && !started.Before(from) && started.Before(to) {
						if *w.Seconds > math.MaxInt64-r.TotalSeconds {
							return finish("Duration overflow; totals are partial.")
						}
						totals[issue.Key] += *w.Seconds
						r.TotalSeconds += *w.Seconds
					}
				}
				offset += len(wp.Worklogs)
				if offset == *wp.Total {
					break
				}
				if len(wp.Worklogs) == 0 {
					return finish("Non-progressing worklog page; totals are partial.")
				}
			}
		}
		if *page.IsLast {
			return finish("")
		}
		if page.NextPageToken == "" || tokens[page.NextPageToken] || len(page.Issues) == 0 {
			return finish("Non-progressing issue page; totals are partial.")
		}
		tokens[page.NextPageToken] = true
		token = page.NextPageToken
	}
}
