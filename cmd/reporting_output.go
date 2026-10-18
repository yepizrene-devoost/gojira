package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

// reportingError keeps transport details out of both human and JSON output.
func reportingError(cmd *cobra.Command, code, message string, cause error) error {
	if wantsJSON(cmd) {
		return reportJSONError(cmd, code, message, "", jira.MutationNotApplied, cause)
	}
	_, _, hint := permissionHint(cause, jira.MutationNotApplied)
	if hint != "" {
		return &humanPermissionError{err: errors.New(message), hint: hint}
	}
	return errors.New(message)
}

func reportingWarnings(cmd *cobra.Command, warnings []string) error {
	for _, warning := range warnings {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "WARNING: "+warning); err != nil {
			return err
		}
	}
	return nil
}
