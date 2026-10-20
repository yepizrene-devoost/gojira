package cmd

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/yepizrene-devoost/gojira/internal/jira"
	"github.com/yepizrene-devoost/gojira/internal/mcpserver"
)

// newMCPServer keeps the CLI test seam while injecting CLI policy into the adapter.
func newMCPServer(client *jira.Client, domain string, allowWrites ...bool) *mcp.Server {
	enabled := len(allowWrites) > 0 && allowWrites[0]
	// A nil *jira.Client must not become a non-nil Writer interface.
	var writer mcpserver.Writer
	if client != nil {
		writer = client
	}
	return mcpserver.New(mcpserver.Options{
		Client:        client,
		Writer:        writer,
		AllowWrites:   enabled,
		Domain:        domain,
		SchemaVersion: jsonSchemaVersion,
		MapError:      func(err error) (string, int, string) { return permissionHint(err, jira.MutationNotApplied) },
		Context:       func(identity mcpserver.IdentityReader) (any, error) { return readContext(identity, "") },
	})
}

func init() {
	command := &cobra.Command{Use: "mcp", Short: "Serve Jira tools over MCP stdio (read-only by default)", Long: "Serve Jira tools over MCP stdio. Stdout is reserved for protocol frames. Configure credentials with gojira config init first. Only read tools are registered unless --allow-writes is passed; that flag grants broad write authority to the connected agent, and a delete confirmation token is not proof of human approval.", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		client, domain, err := buildClient()
		if err != nil {
			return fmt.Errorf("MCP credentials unavailable; configure credentials with gojira config init")
		}
		allowWrites, _ := cmd.Flags().GetBool("allow-writes")
		return newMCPServer(client, domain, allowWrites).Run(cmd.Context(), &mcp.StdioTransport{})
	}}
	command.Flags().Bool("allow-writes", false, "Enable guarded Jira write tools (default read-only)")
	command.AddCommand(newMCPConfigCommand())
	rootCmd.AddCommand(command)
}
