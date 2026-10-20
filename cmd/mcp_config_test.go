package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runMCPConfig(t *testing.T, executable string, args ...string) (string, string, error) {
	t.Helper()
	previous := executablePath
	executablePath = func() (string, error) { return executable, nil }
	t.Cleanup(func() { executablePath = previous })
	command := newMCPConfigCommand()
	// Mirror rootCmd, which silences usage and errors for every subcommand.
	command.SilenceErrors = true
	command.SilenceUsage = true
	var out, errOut bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&errOut)
	command.SetArgs(args)
	err := command.Execute()
	return out.String(), errOut.String(), err
}

func TestMCPConfigCodexPrintsTOML(t *testing.T) {
	out, _, err := runMCPConfig(t, "/opt/gojira/gojira", "--client", "codex")
	if err != nil {
		t.Fatal(err)
	}
	want := "[mcp_servers.gojira]\ncommand = \"/opt/gojira/gojira\"\nargs = [\"mcp\"]\n"
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestMCPConfigDefaultsToUniversalJSON(t *testing.T) {
	out, _, err := runMCPConfig(t, "/opt/gojira/gojira")
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("stdout is not valid JSON: %v: %q", err, out)
	}
	server := parsed.MCPServers["gojira"]
	if server.Command != "/opt/gojira/gojira" || len(server.Args) != 1 || server.Args[0] != "mcp" {
		t.Fatalf("unexpected server entry: %+v", server)
	}
}

func TestMCPConfigAllowWritesAddsFlagToEveryClient(t *testing.T) {
	for _, client := range []string{"codex", "json"} {
		out, _, err := runMCPConfig(t, "/opt/gojira/gojira", "--client", client, "--allow-writes")
		if err != nil {
			t.Fatalf("%s: %v", client, err)
		}
		if !strings.Contains(out, `"--allow-writes"`) {
			t.Fatalf("%s: missing --allow-writes in %q", client, out)
		}
	}
}

func TestMCPConfigReadOnlyByDefault(t *testing.T) {
	for _, client := range []string{"codex", "json"} {
		out, _, err := runMCPConfig(t, "/opt/gojira/gojira", "--client", client)
		if err != nil {
			t.Fatalf("%s: %v", client, err)
		}
		if strings.Contains(out, "allow-writes") {
			t.Fatalf("%s: read-only default must not grant writes: %q", client, out)
		}
	}
}

func TestMCPConfigEscapesWindowsPaths(t *testing.T) {
	const path = `C:\Users\pm\gojira.exe`
	toml, _, err := runMCPConfig(t, path, "--client", "codex")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(toml, `command = "C:\\Users\\pm\\gojira.exe"`) {
		t.Fatalf("TOML path not escaped: %q", toml)
	}
	jsonOut, _, err := runMCPConfig(t, path)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]map[string]struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["mcpServers"]["gojira"].Command != path {
		t.Fatalf("JSON path altered: %q", parsed["mcpServers"]["gojira"].Command)
	}
}

func TestMCPConfigNeverPrintsCredentials(t *testing.T) {
	t.Setenv("JIRA_API_TOKEN", "super-secret-token")
	t.Setenv("JIRA_EMAIL", "pm@example.com")
	for _, client := range []string{"codex", "json"} {
		out, errOut, err := runMCPConfig(t, "/opt/gojira/gojira", "--client", client)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"super-secret-token", "pm@example.com"} {
			if strings.Contains(out, secret) || strings.Contains(errOut, secret) {
				t.Fatalf("%s: leaked %q", client, secret)
			}
		}
	}
}

func TestMCPConfigRejectsUnknownClient(t *testing.T) {
	out, _, err := runMCPConfig(t, "/opt/gojira/gojira", "--client", "bogus")
	if err == nil {
		t.Fatal("expected an error for an unknown client")
	}
	for _, valid := range []string{"codex", "json"} {
		if !strings.Contains(err.Error(), valid) {
			t.Fatalf("error should list %q: %v", valid, err)
		}
	}
	if out != "" {
		t.Fatalf("stdout must stay empty on error: %q", out)
	}
}

func TestMCPCommandStillRejectsUnknownSubcommand(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"mcp", "bogus"})
	if err != nil {
		t.Fatal(err)
	}
	if command.Name() != "mcp" {
		t.Fatalf("resolved %q, want mcp", command.Name())
	}
	if err := command.Args(command, []string{"bogus"}); err == nil {
		t.Fatal("mcp must keep rejecting positional arguments")
	}
	if command.Flags().Lookup("allow-writes") == nil {
		t.Fatal("mcp lost its --allow-writes flag")
	}
}
