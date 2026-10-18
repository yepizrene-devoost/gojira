package cmd

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/yepizrene-devoost/gojira/internal/jira"
	"github.com/yepizrene-devoost/gojira/internal/mcpserver"
)

// newMCPServer keeps the CLI test seam while injecting CLI policy into the adapter.
func newMCPServer(client *jira.Client, domain string) *mcp.Server {
	return mcpserver.New(mcpserver.Options{
		Client:        client,
		Domain:        domain,
		SchemaVersion: jsonSchemaVersion,
		MapError:      func(err error) (string, int, string) { return permissionHint(err, jira.MutationNotApplied) },
		Context:       func(identity mcpserver.IdentityReader) (any, error) { return readContext(identity, "") },
	})
}

func init() {
	rootCmd.AddCommand(&cobra.Command{Use: "mcp", Short: "Serve read-only Jira tools over MCP stdio", Long: "Serve read-only Jira tools over MCP stdio. Stdout is reserved for protocol frames. Configure credentials with gojira config init first. No write tools are registered.", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		client, domain, err := buildClient()
		if err != nil {
			return fmt.Errorf("MCP credentials unavailable; configure credentials with gojira config init")
		}
		return newMCPServer(client, domain).Run(cmd.Context(), &mcp.StdioTransport{})
	}})
}
