package mcpserver

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/yepizrene-devoost/gojira/internal/issuefields"
)

var writeKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*-[1-9][0-9]*$`)
var writeProjectPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
var writeTimePattern = regexp.MustCompile(`^[1-9][0-9]*[wdhm]( +[1-9][0-9]*[wdhm])*$`)

type writeRequest struct {
	values map[string]string
	update *issuefields.Prepared
}

func validateWrite(entry writeTool, raw json.RawMessage) (*writeRequest, string) {
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
	var update *issuefields.Prepared
	for name, raw := range fields {
		if !allowed[name] {
			return nil, "unexpected argument"
		}
		if string(raw) == "null" {
			return nil, "arguments must not be null"
		}
		if name == "fields" {
			var message string
			update, message = prepareUpdate(raw)
			if message != "" {
				return nil, message
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
	return &writeRequest{values: values, update: update}, ""
}

// prepareUpdate decodes the heterogeneous update object once at the boundary.
func prepareUpdate(raw json.RawMessage) (*issuefields.Prepared, string) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) == 0 {
		return nil, "fields must be a non-empty object of strings"
	}
	in := issuefields.Input{}
	for name, value := range fields {
		switch name {
		case "summary", "priority", "description", "dueDate", "storyPointsField":
			if string(value) == "null" {
				return nil, "fields must not be null"
			}
			var s string
			if json.Unmarshal(value, &s) != nil {
				return nil, updateFieldMessage(name)
			}
			switch name {
			case "summary", "priority":
				if strings.TrimSpace(s) == "" {
					return nil, "only non-empty summary and priority fields are allowed"
				}
				if name == "summary" {
					in.Summary = &s
				} else {
					in.Priority = &s
				}
			case "description":
				in.Description = &s
			case "dueDate":
				in.DueDate = &s
			case "storyPointsField":
				in.StoryPointsField = &s
			}
		case "labels", "components", "fixVersions":
			var names []string
			if string(value) == "null" || json.Unmarshal(value, &names) != nil {
				return nil, updateFieldMessage(name)
			}
			// JSON null array entries otherwise decode silently as empty strings.
			for _, s := range names {
				if strings.TrimSpace(s) == "" {
					return nil, updateFieldMessage(name)
				}
			}
			switch name {
			case "labels":
				if len(names) == 0 {
					return nil, updateFieldMessage(name)
				}
				for _, s := range names {
					if strings.TrimSpace(s) != s {
						return nil, updateFieldMessage(name)
					}
				}
				in.LabelNames = &names
			case "components":
				in.ComponentNames = &names
			default:
				in.FixVersionNames = &names
			}
		case "storyPoints":
			s := ""
			if string(value) != "null" {
				if len(value) == 0 || (value[0] != '-' && (value[0] < '0' || value[0] > '9')) {
					return nil, updateFieldMessage(name)
				}
				s = string(value)
			}
			in.StoryPoints = &s
		default:
			return nil, "only non-empty summary and priority fields are allowed"
		}
	}
	prepared, err := issuefields.Prepare(in)
	if err != nil {
		return nil, updateValidationMessage(err)
	}
	return &prepared, ""
}
