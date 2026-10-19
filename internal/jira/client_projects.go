package jira

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

// GetProjects reads all visible projects. The 1000-page cap bounds requests
// against a broken server; repeated pages fail rather than claiming completeness.
// projectEntry keeps the numeric ID only long enough to restore the order of
// the legacy /project endpoint; it is never part of Project's JSON shape.
type projectEntry struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// sortedProjects orders by ascending numeric project ID, which is how the
// legacy /project endpoint listed them and which no orderBy of /project/search
// reproduces. Non-numeric IDs sort last in their API order.
func sortedProjects(entries []projectEntry) []Project {
	numericID := func(e projectEntry) (int, bool) {
		id, err := strconv.Atoi(e.ID)
		return id, err == nil
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, aOK := numericID(entries[i])
		b, bOK := numericID(entries[j])
		if aOK != bOK {
			return aOK
		}
		return aOK && a < b
	})
	projects := make([]Project, 0, len(entries))
	for _, e := range entries {
		projects = append(projects, Project{Key: e.Key, Name: e.Name})
	}
	return projects
}

func (c *Client) GetProjects() ([]Project, error) {
	entries := []projectEntry{}
	seen := map[string]bool{}
	startAt := 0
	for range 1000 {
		b, err := c.get(fmt.Sprintf(pathProjectSearch, startAt))
		if err != nil {
			return nil, err
		}
		var page struct {
			StartAt    int            `json:"startAt"`
			MaxResults int            `json:"maxResults"`
			IsLast     bool           `json:"isLast"`
			Values     []projectEntry `json:"values"`
		}
		if err := json.Unmarshal(b, &page); err != nil {
			return nil, err
		}
		if len(page.Values) == 0 {
			if !page.IsLast {
				// Never present an interrupted listing as the complete one.
				return nil, fmt.Errorf("project pagination returned an empty page that is not the last")
			}
			return sortedProjects(entries), nil
		}
		fingerprint, _ := json.Marshal(page.Values)
		if seen[string(fingerprint)] {
			return nil, fmt.Errorf("project pagination repeated a page")
		}
		seen[string(fingerprint)] = true
		entries = append(entries, page.Values...)
		if page.IsLast {
			return sortedProjects(entries), nil
		}
		step := page.MaxResults
		if step <= 0 {
			step = 50
		}
		next := page.StartAt + step
		if next <= startAt {
			return nil, fmt.Errorf("project pagination did not advance")
		}
		startAt = next
	}
	return nil, fmt.Errorf("project pagination exceeded 1000-page safety cap")
}

func (c *Client) GetProject(key string) (*Project, error) {
	b, err := c.get(issuePath(pathProjectByKey, key))
	if err != nil {
		return nil, err
	}
	var project Project
	if err := json.Unmarshal(b, &project); err != nil {
		return nil, err
	}
	return &project, nil
}
