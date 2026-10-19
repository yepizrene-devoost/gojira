package reporting

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ValidateUserQuery checks user lookup input without making network requests.
func ValidateUserQuery(query string) error {
	if strings.TrimSpace(query) == "" {
		return errors.New("query is required")
	}
	return nil
}

func SearchUsers(ctx context.Context, reader UserReader, query string) (UserSearchResult, error) {
	r := UserSearchResult{SchemaVersion: "v1", Query: query, Candidates: []User{}, Completeness: "partial", Warnings: []string{"User search covers active matches within the endpoint's first 1000 users only; missing Browse users permission may return an empty result. No organization-wide completeness is implied."}}
	if err := ValidateUserQuery(query); err != nil {
		return r, err
	}
	seen := map[string]bool{}
	for offset := 0; offset < 1000; {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		page, err := reader.ReportingUsers(ctx, query, offset)
		if err != nil {
			if ctx.Err() != nil {
				return r, ctx.Err()
			}
			if offset == 0 {
				return r, fmt.Errorf("user search failed: %w", err)
			}
			r.Warnings = append(r.Warnings, "User search interrupted; candidates are partial.")
			return r, nil
		}
		if len(page) > 100 {
			r.Warnings = append(r.Warnings, "Unexpected user page size; candidates are partial.")
			return r, nil
		}
		for _, u := range page {
			if u.AccountID == "" || seen[u.AccountID] {
				r.Warnings = append(r.Warnings, "Invalid or repeated user page; candidates are partial.")
				return r, nil
			}
			seen[u.AccountID] = true
			r.Candidates = append(r.Candidates, u)
		}
		offset += len(page)
		if len(page) < 100 {
			r.Completeness = "endpoint_window_exhausted"
			return r, nil
		}
	}
	r.Warnings = append(r.Warnings, "User search endpoint cap reached.")
	return r, nil
}
