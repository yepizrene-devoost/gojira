package jira

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxDiagnosticBody = 64 * 1024
const maxDiagnosticEntries = 8
const maxDiagnosticText = 256

// Diagnostics contains only bounded, credential-redacted Jira error fields.
// It is attached exclusively to mutation HTTP failures, never read responses.
type Diagnostics struct {
	HTTPStatus    int               `json:"httpStatus,omitempty"`
	Errors        map[string]string `json:"errors,omitempty"`
	ErrorMessages []string          `json:"errorMessages,omitempty"`
}

// MutationDiagnostics returns sanitized diagnostics through wrapped errors.
func MutationDiagnostics(err error) Diagnostics {
	var api *APIError
	if errors.As(err, &api) && api.Diagnostics != nil {
		return *api.Diagnostics
	}
	return Diagnostics{}
}

func (c *Client) diagnostics(status int, body []byte) Diagnostics {
	result := Diagnostics{HTTPStatus: status}
	if len(body) > maxDiagnosticBody {
		return result
	}
	var fields struct {
		Errors        map[string]string `json:"errors"`
		ErrorMessages []string          `json:"errorMessages"`
	}
	if json.Unmarshal(body, &fields) != nil {
		return result
	}
	email, token, _ := strings.Cut(c.auth, ":")
	secrets := []string{c.auth, email, token, base64.StdEncoding.EncodeToString([]byte(c.auth + ":")), base64.StdEncoding.EncodeToString([]byte(c.auth))}
	sort.Slice(secrets, func(i, j int) bool { return len(secrets[i]) > len(secrets[j]) })
	pairs := []string{}
	for _, secret := range secrets {
		if secret != "" {
			pairs = append(pairs, secret, "[REDACTED]")
		}
	}
	replacer := strings.NewReplacer(pairs...)
	clean := func(text string) string {
		text = replacer.Replace(text)
		if len(text) > maxDiagnosticText {
			text = text[:maxDiagnosticText]
			for !utf8.ValidString(text) {
				text = text[:len(text)-1]
			}
		}
		return text
	}
	keys := make([]string, 0, len(fields.Errors))
	for key := range fields.Errors {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for i, key := range keys {
		if i == maxDiagnosticEntries {
			break
		}
		if result.Errors == nil {
			result.Errors = map[string]string{}
		}
		result.Errors[clean(key)] = clean(fields.Errors[key])
	}
	for i, message := range fields.ErrorMessages {
		if i == maxDiagnosticEntries {
			break
		}
		result.ErrorMessages = append(result.ErrorMessages, clean(message))
	}
	return result
}
