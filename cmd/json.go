package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

const jsonSchemaVersion = "v1"

type mutationErrorJSON struct {
	SchemaVersion string             `json:"schemaVersion"`
	Code          string             `json:"code"`
	Message       string             `json:"message"`
	IssueKey      string             `json:"issueKey,omitempty"`
	MutationState jira.MutationState `json:"mutationState"`
}

type reportedCommandError struct {
	err error
}

func (e *reportedCommandError) Error() string { return e.err.Error() }
func (e *reportedCommandError) Unwrap() error { return e.err }

func isReportedCommandError(err error) bool {
	var reported *reportedCommandError
	return errors.As(err, &reported)
}

// writeJSON writes one indented JSON document through Cobra's configured
// output writer. Encoder and writer failures are returned to command callers.
func writeJSON(cmd *cobra.Command, value any) error {
	return writeJSONTo(cmd.OutOrStdout(), value)
}

func writeJSONTo(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func wantsJSON(cmd *cobra.Command) bool {
	flag := cmd.Flags().Lookup("json")
	if flag == nil {
		return false
	}
	enabled, err := cmd.Flags().GetBool("json")
	return err == nil && enabled
}

func reportJSONError(cmd *cobra.Command, code, message, issueKey string, state jira.MutationState, cause error) error {
	if cause == nil {
		cause = errors.New(message)
	}
	envelope := mutationErrorJSON{
		SchemaVersion: jsonSchemaVersion,
		Code:          code,
		Message:       message,
		IssueKey:      issueKey,
		MutationState: state,
	}
	if err := writeJSONTo(cmd.ErrOrStderr(), envelope); err != nil {
		return &reportedCommandError{err: fmt.Errorf("writing JSON error: %w", err)}
	}
	return &reportedCommandError{err: cause}
}

func commandError(cmd *cobra.Command, code, message, issueKey string, state jira.MutationState, cause error) error {
	if wantsJSON(cmd) {
		return reportJSONError(cmd, code, message, issueKey, state, cause)
	}
	return cause
}

func writeMutationResult(cmd *cobra.Command, client *jira.Client, domain, issueKey string) error {
	return writeMutationResultWithStoryPoints(cmd, client, domain, issueKey, "")
}

func writeMutationResultWithStoryPoints(cmd *cobra.Command, client *jira.Client, domain, issueKey, storyPointsField string) error {
	var (
		issue *jira.Issue
		err   error
	)
	if storyPointsField == "" {
		issue, err = client.GetIssueFull(issueKey)
	} else {
		issue, err = client.GetIssueFullWithStoryPoints(issueKey, storyPointsField)
	}
	if err != nil {
		return reportJSONError(cmd, "refetch_failed", "mutation applied but the resulting issue could not be fetched", issueKey, jira.MutationApplied, err)
	}
	if err := writeJSON(cmd, jira.IssueToTicketJSON(*issue, domain)); err != nil {
		return reportJSONError(cmd, "output_failed", "mutation applied but the JSON result could not be written", issueKey, jira.MutationApplied, err)
	}
	return nil
}
