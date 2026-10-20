package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// executablePath is a seam so tests can pin the binary location.
var executablePath = os.Executable

// mcpClientRenderers maps a --client value to the format that client reads.
// Adding a client means adding one entry here; nothing else changes.
var mcpClientRenderers = map[string]func(command string, args []string) (string, error){
	"codex": renderCodexTOML,
	"json":  renderMCPServersJSON,
}

func renderCodexTOML(command string, args []string) (string, error) {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = strconv.Quote(arg)
	}
	return fmt.Sprintf("[mcp_servers.gojira]\ncommand = %s\nargs = [%s]\n", strconv.Quote(command), strings.Join(quoted, ", ")), nil
}

func renderMCPServersJSON(command string, args []string) (string, error) {
	document := map[string]any{"mcpServers": map[string]any{"gojira": map[string]any{"command": command, "args": args}}}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
}

func supportedMCPClients() string {
	names := make([]string, 0, len(mcpClientRenderers))
	for name := range mcpClientRenderers {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// newMCPConfigCommand prints a ready-to-paste MCP client entry. It never
// includes credentials: gojira mcp reads them from gojira config init.
func newMCPConfigCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "config",
		Short: "Print the MCP client configuration for this gojira binary",
		Long:  "Print a ready-to-paste MCP server entry pointing at this binary. The output contains no credentials: run gojira config init once, and gojira mcp reads them from there. Use --client codex for the Codex TOML format, or the default json for the mcpServers format most other clients read.",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, _ := command.Flags().GetString("client")
			render, ok := mcpClientRenderers[client]
			if !ok {
				return fmt.Errorf("unknown client %q (supported: %s)", client, supportedMCPClients())
			}
			binary, err := executablePath()
			if err != nil {
				return fmt.Errorf("cannot determine the gojira binary path: %w", err)
			}
			args := []string{"mcp"}
			if allowWrites, _ := command.Flags().GetBool("allow-writes"); allowWrites {
				args = append(args, "--allow-writes")
			}
			output, err := render(binary, args)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(command.OutOrStdout(), output)
			return err
		},
	}
	command.Flags().String("client", "json", "Client format to print: "+supportedMCPClients())
	command.Flags().Bool("allow-writes", false, "Include --allow-writes (grants broad write authority to the connected agent)")
	return command
}
