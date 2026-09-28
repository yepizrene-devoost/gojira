package cmd

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

// writeJSON writes one indented JSON document through Cobra's configured
// output writer. Encoder and writer failures are returned to command callers.
func writeJSON(cmd *cobra.Command, value any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
