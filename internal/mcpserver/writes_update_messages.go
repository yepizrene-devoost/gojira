package mcpserver

import (
	"errors"

	"github.com/yepizrene-devoost/gojira/internal/issuefields"
)

// updateFieldMessages are fixed, input-free rules: an agent learns which field to fix
// without any ticket text being echoed back.
var updateFieldMessages = map[string]string{
	"summary":          "invalid summary; use a non-empty string",
	"priority":         "invalid priority; use a non-empty priority name",
	"description":      "invalid description; use a string, or an empty string to clear",
	"labels":           "invalid labels; use a non-empty array of non-empty, trimmed names",
	"dueDate":          "invalid dueDate; use a real calendar date in YYYY-MM-DD format, or an empty string to clear",
	"components":       "invalid components; use an array of non-empty, unique names, or [] to clear",
	"fixVersions":      "invalid fixVersions; use an array of non-empty, unique names, or [] to clear",
	"storyPoints":      "invalid storyPoints; use a finite decimal number, or null to clear, together with storyPointsField",
	"storyPointsField": "invalid storyPointsField; use a customfield_NNN identifier",
}

const genericUpdateMessage = "invalid update fields"

func updateFieldMessage(field string) string {
	if message, ok := updateFieldMessages[field]; ok {
		return message
	}
	return genericUpdateMessage
}

// updateValidationMessage names the failing field reported by the shared preparation.
func updateValidationMessage(err error) string {
	var invalid *issuefields.ValidationError
	if errors.As(err, &invalid) {
		return updateFieldMessage(invalid.Field)
	}
	return genericUpdateMessage
}
