package mcpserver

import (
	"encoding/json"
	"regexp"
	"strings"
)

var writeKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*-[1-9][0-9]*$`)
var writeProjectPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
var writeTimePattern = regexp.MustCompile(`^[1-9][0-9]*[wdhm]( +[1-9][0-9]*[wdhm])*$`)

func validateWrite(entry writeTool, raw json.RawMessage) (map[string]string, string) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return nil, "arguments must be an object"
	}
	allowed := map[string]bool{}
	for _, name := range entry.required {
		allowed[name] = true
		if _, ok := fields[name]; !ok {
			return nil, "required argument missing"
		}
	}
	for _, name := range entry.optional {
		allowed[name] = true
	}
	values := map[string]string{}
	for name, raw := range fields {
		if !allowed[name] {
			return nil, "unexpected argument"
		}
		if string(raw) == "null" {
			return nil, "arguments must not be null"
		}
		if name == "fields" {
			var updates map[string]string
			if json.Unmarshal(raw, &updates) != nil || len(updates) == 0 {
				return nil, "fields must be a non-empty object of strings"
			}
			for field, value := range updates {
				if (field != "summary" && field != "priority") || strings.TrimSpace(value) == "" {
					return nil, "only non-empty summary and priority fields are allowed"
				}
			}
			// Unmarshal into strings accepts null values; reject them explicitly.
			var nested map[string]json.RawMessage
			_ = json.Unmarshal(raw, &nested)
			for _, value := range nested {
				if string(value) == "null" {
					return nil, "fields must not be null"
				}
			}
			values[name] = string(raw)
			continue
		}
		var value string
		if json.Unmarshal(raw, &value) != nil {
			return nil, "arguments must be strings"
		}
		values[name] = value
	}
	for _, name := range entry.required {
		if strings.TrimSpace(values[name]) == "" {
			return nil, "required arguments must not be empty"
		}
	}
	if key, ok := values["key"]; ok {
		key = strings.ToUpper(strings.TrimSpace(key))
		if !writeKeyPattern.MatchString(key) {
			return nil, "invalid issue key"
		}
		values["key"] = key
	}
	if project, ok := values["project"]; ok {
		if !writeProjectPattern.MatchString(project) {
			return nil, "invalid project"
		}
	}
	if account, ok := values["accountId"]; ok && strings.TrimSpace(account) != account {
		return nil, "accountId must be trimmed"
	}
	if spent, ok := values["timeSpent"]; ok && !writeTimePattern.MatchString(spent) {
		return nil, "invalid timeSpent; use positive Jira durations such as 2h 30m"
	}
	if token, ok := values["confirmationToken"]; ok && strings.TrimSpace(token) == "" {
		return nil, "confirmationToken must not be empty"
	}
	return values, ""
}
