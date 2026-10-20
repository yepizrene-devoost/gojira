package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/config"
	"github.com/yepizrene-devoost/gojira/internal/secretprompt"
)

// connectionTester is the one thing `config init` needs from a Jira client.
type connectionTester interface {
	TestConnection() (string, error)
}

// configInitDeps are the side effects of `config init`. They are injected so tests never
// touch the real keychain, the real config file, the network or a terminal.
type configInitDeps struct {
	isTerminal func() bool
	readSecret func(prompt string) (string, error)
	saveConfig func(*config.Config) error
	saveToken  func(string) error
	newClient  func() (connectionTester, error)
}

func defaultConfigInitDeps() configInitDeps {
	return configInitDeps{
		isTerminal: func() bool { return isTerminal() },
		readSecret: func(prompt string) (string, error) { return secretprompt.Read(prompt, os.Stdin, os.Stdout) },
		saveConfig: func(cfg *config.Config) error { return cfg.Save() },
		saveToken:  config.SaveToken,
		newClient: func() (connectionTester, error) {
			client, _, err := BuildClient()
			if err != nil {
				return nil, err
			}
			return client, nil
		},
	}
}

var configInitCmd = newConfigInitCommand(defaultConfigInitDeps())

func newConfigInitCommand(deps configInitDeps) *cobra.Command {
	command := &cobra.Command{
		Use:   "init",
		Short: "Set up GoJira configuration interactively",
		Long: "Set up GoJira configuration. On a terminal the API token is typed or pasted into a masked " +
			"prompt and is never displayed. Without a terminal it is read as a line. To give the token " +
			"non-interactively without leaving it in your shell history, pipe it with --token-stdin " +
			"(requires --domain and --email).",
		RunE: func(command *cobra.Command, _ []string) error { return runConfigInit(command, deps) },
	}
	command.Flags().String("domain", "", "Jira domain (e.g. mycompany.atlassian.net)")
	command.Flags().String("email", "", "Jira email")
	command.Flags().String("token", "", "Jira API token (discouraged: visible in shell history and process lists; prefer --token-stdin or the masked prompt)")
	command.Flags().Bool("token-stdin", false, "Read the Jira API token from standard input (requires --domain and --email)")
	return command
}

func runConfigInit(command *cobra.Command, deps configInitDeps) error {
	domain, _ := command.Flags().GetString("domain")
	email, _ := command.Flags().GetString("email")
	token, _ := command.Flags().GetString("token")
	tokenStdin, _ := command.Flags().GetBool("token-stdin")
	out := command.OutOrStdout()
	reader := bufio.NewReader(command.InOrStdin())

	if tokenStdin && command.Flags().Changed("token") {
		return fmt.Errorf("--token and --token-stdin cannot be used together")
	}
	if tokenStdin && (domain == "" || email == "") {
		return fmt.Errorf("--token-stdin reads the token from standard input, so --domain and --email are required")
	}
	if token != "" {
		_, _ = fmt.Fprintln(command.ErrOrStderr(), "warning: --token leaves the token in your shell history and in the process list; prefer --token-stdin or the masked prompt")
	}
	if tokenStdin {
		data, err := io.ReadAll(reader)
		if err != nil {
			return fmt.Errorf("reading token from standard input: %w", err)
		}
		token = strings.TrimSpace(string(data))
		if token == "" {
			return fmt.Errorf("no token received on standard input")
		}
	}

	if domain == "" {
		_, _ = fmt.Fprint(out, "Jira domain (e.g. mycompany.atlassian.net): ")
		value, err := readConfigInput(reader, "domain")
		if err != nil {
			return err
		}
		domain = value
	}
	if email == "" {
		_, _ = fmt.Fprint(out, "Jira email: ")
		value, err := readConfigInput(reader, "email")
		if err != nil {
			return err
		}
		email = value
	}
	if token == "" {
		value, err := readInitToken(deps, reader, out)
		if err != nil {
			return err
		}
		token = value
	}

	if domain == "" || email == "" || token == "" {
		return fmt.Errorf("domain, email, and token are all required")
	}

	if err := deps.saveConfig(&config.Config{Domain: domain, Email: email}); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	if err := deps.saveToken(token); err != nil {
		return fmt.Errorf("saving token: %w", err)
	}

	// Test connection with the just-saved credentials
	client, err := deps.newClient()
	if err != nil {
		_, _ = fmt.Fprintf(out, "⚠ Config saved but could not build client: %v\n", err)
		_, _ = fmt.Fprintln(out, "  Try: gojira config test")
		return nil
	}
	info, err := client.TestConnection()
	if err != nil {
		_, _ = fmt.Fprintf(out, "⚠ Config saved but connection failed: %v\n", err)
		_, _ = fmt.Fprintln(out, "  Check your credentials and try: gojira config test")
		return nil
	}

	_, _ = fmt.Fprintf(out, "✓ Configured and connected to %s\n", info)
	return nil
}

// readInitToken reads the token through the masked prompt on a terminal, and as a plain
// line otherwise, so pipes and scripts keep working exactly as before.
func readInitToken(deps configInitDeps, reader *bufio.Reader, out io.Writer) (string, error) {
	if deps.isTerminal() {
		value, err := deps.readSecret("Jira API token: ")
		if err != nil {
			return "", fmt.Errorf("reading token: %w", err)
		}
		return value, nil
	}
	_, _ = fmt.Fprint(out, "Jira API token: ")
	return readConfigInput(reader, "token")
}
