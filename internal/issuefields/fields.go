// Package issuefields prepares issue edits without performing I/O.
package issuefields

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

// Input uses pointers to distinguish omission from explicit clearing.
type Input struct {
	Summary, Description, Priority, Labels      *string
	DueDate, Components, FixVersions            *string
	StoryPoints, StoryPointsField               *string
	ComponentNames, FixVersionNames, LabelNames *[]string
}

// Prepared contains the mutation and its preflight metadata requirements.
type Prepared struct {
	Fields           map[string]interface{}
	MetadataFields   map[string][]string
	StoryPointsField string
}

// ValidationError preserves the CLI's contextual error message.
type ValidationError struct {
	Message string
	Cause   error
	// Field is the canonical input name that failed (for example dueDate).
	Field string
}

func (e *ValidationError) Error() string { return e.Cause.Error() }
func (e *ValidationError) Unwrap() error { return e.Cause }

func invalid(field, message string, cause error) error {
	return &ValidationError{Message: message, Cause: cause, Field: field}
}

// Prepare preserves the CLI's omission, empty-clear and list semantics.
func Prepare(in Input) (Prepared, error) {
	p := Prepared{Fields: map[string]interface{}{}, MetadataFields: map[string][]string{}}
	if (in.StoryPoints != nil) != (in.StoryPointsField != nil) {
		return p, invalid("storyPoints", "story points flags must be paired", fmt.Errorf("--story-points and --story-points-field must be provided together"))
	}
	if in.StoryPoints != nil {
		p.StoryPointsField = *in.StoryPointsField
		if err := jira.ValidateCustomFieldID(p.StoryPointsField); err != nil {
			return p, invalid("storyPointsField", "invalid story points field", err)
		}
		points, err := ParseStoryPoints(*in.StoryPoints)
		if err != nil {
			return p, invalid("storyPoints", "invalid story points", err)
		}
		p.Fields[p.StoryPointsField] = points
	}
	if in.Summary != nil && *in.Summary != "" {
		p.Fields["summary"] = *in.Summary
	}
	if in.Description != nil {
		if *in.Description == "" {
			p.Fields["description"] = nil
		} else {
			p.Fields["description"] = jira.TextToADF(*in.Description)
		}
	}
	if in.Priority != nil && *in.Priority != "" {
		p.Fields["priority"] = map[string]string{"name": *in.Priority}
	}
	if in.Labels != nil && *in.Labels != "" {
		labels := strings.Split(*in.Labels, ",")
		for i := range labels {
			labels[i] = strings.TrimSpace(labels[i])
		}
		p.Fields["labels"] = labels
	}
	if in.LabelNames != nil {
		p.Fields["labels"] = *in.LabelNames
	}
	if in.DueDate != nil {
		if err := ValidateDueDate(*in.DueDate); err != nil {
			return p, invalid("dueDate", "invalid due date", err)
		}
		if *in.DueDate == "" {
			p.Fields["duedate"] = nil
		} else {
			p.Fields["duedate"] = *in.DueDate
		}
		p.MetadataFields["duedate"] = nil
	}
	for _, entry := range []struct {
		raw         *string
		names       *[]string
		flag, field string
	}{{in.Components, in.ComponentNames, "components", "components"}, {in.FixVersions, in.FixVersionNames, "fix-versions", "fixVersions"}} {
		if entry.raw == nil && entry.names == nil {
			continue
		}
		var names []string
		var err error
		if entry.raw != nil {
			names, err = ParseCommaNames(*entry.raw, entry.flag)
		} else {
			names, err = ValidateNames(*entry.names, entry.flag)
		}
		if err != nil {
			return p, invalid(entry.field, "invalid "+entry.flag, err)
		}
		values := make([]map[string]string, len(names))
		for i, name := range names {
			values[i] = map[string]string{"name": name}
		}
		p.Fields[entry.field] = values
		p.MetadataFields[entry.field] = names
	}
	if len(p.Fields) == 0 {
		return p, invalid("fields", "at least one field must be specified", fmt.Errorf("specify at least one field to update (--summary, --description, --priority, --labels, --due-date, --components, --fix-versions, --story-points with --story-points-field)"))
	}
	return p, nil
}

// NeedsMetadata reports whether this edit requires a preflight fetch.
func (p Prepared) NeedsMetadata() bool { return len(p.MetadataFields) > 0 || p.StoryPointsField != "" }

// ValidateMetadata must succeed before a mutation is attempted.
func (p Prepared) ValidateMetadata(meta *jira.EditMeta) error {
	if err := ValidateEditMeta(meta, p.MetadataFields); err != nil {
		return invalid("fields", "requested field update is not allowed", err)
	}
	if p.StoryPointsField != "" {
		if err := ValidateStoryPointsEditMeta(meta, p.StoryPointsField); err != nil {
			return invalid("storyPointsField", "story points field is not editable", err)
		}
	}
	return nil
}

var storyPointsValuePattern = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)

// ParseStoryPoints preserves decimal precision; explicitly empty clears.
func ParseStoryPoints(value string) (any, error) {
	if value == "" {
		return nil, nil
	}
	if !storyPointsValuePattern.MatchString(value) {
		return nil, fmt.Errorf("--story-points must be a finite decimal number or explicitly empty")
	}
	if _, ok := new(big.Rat).SetString(value); !ok {
		return nil, fmt.Errorf("--story-points must be a finite decimal number or explicitly empty")
	}
	return json.Number(value), nil
}

// ValidateStoryPointsEditMeta requires a numeric field supporting set.
func ValidateStoryPointsEditMeta(meta *jira.EditMeta, field string) error {
	if meta == nil {
		return fmt.Errorf("jira returned empty edit metadata")
	}
	fieldMeta, present := meta.Fields[field]
	if !present {
		return fmt.Errorf("field %q is not editable for this issue", field)
	}
	if fieldMeta.Schema.Type != "number" {
		return fmt.Errorf("field %q has schema type %q, want number", field, fieldMeta.Schema.Type)
	}
	for _, operation := range fieldMeta.Operations {
		if operation == "set" {
			return nil
		}
	}
	return fmt.Errorf("field %q does not support the set operation", field)
}

// ValidateDueDate accepts only real YYYY-MM-DD dates or an explicit clear.
func ValidateDueDate(value string) error {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return fmt.Errorf("--due-date must be a real calendar date in YYYY-MM-DD format")
	}
	return nil
}

// ParseCommaNames trims names and rejects empty entries and case-folded duplicates.
func ParseCommaNames(value, flag string) ([]string, error) {
	if value == "" {
		return []string{}, nil
	}
	return ValidateNames(strings.Split(value, ","), flag)
}

// ValidateNames applies the same name-list rules without splitting literal commas.
func ValidateNames(parts []string, flag string) ([]string, error) {
	names := make([]string, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for i, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			return nil, fmt.Errorf("--%s contains an empty entry", flag)
		}
		canonical := strings.ToLower(name)
		if _, exists := seen[canonical]; exists {
			return nil, fmt.Errorf("--%s contains duplicate entry %q", flag, name)
		}
		seen[canonical] = struct{}{}
		names[i] = name
	}
	return names, nil
}

// ValidateEditMeta checks editability and allowed names when Jira supplies them.
func ValidateEditMeta(meta *jira.EditMeta, requested map[string][]string) error {
	if meta == nil {
		return fmt.Errorf("jira returned empty edit metadata")
	}
	for field, names := range requested {
		fieldMeta, editable := meta.Fields[field]
		if !editable {
			return fmt.Errorf("field %q is not editable for this issue", field)
		}
		if len(names) == 0 || fieldMeta.AllowedValues == nil {
			continue
		}
		allowed := make(map[string]struct{}, len(*fieldMeta.AllowedValues))
		for _, value := range *fieldMeta.AllowedValues {
			allowed[value.Name] = struct{}{}
		}
		for _, name := range names {
			if _, ok := allowed[name]; !ok {
				return fmt.Errorf("%q is not an allowed value for field %q", name, field)
			}
		}
	}
	return nil
}
