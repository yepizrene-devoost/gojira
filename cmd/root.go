package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/config"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

var rootCmd = &cobra.Command{
	Use:           "gojira",
	Short:         "Manage Jira from the command line or interactive TUI",
	Long:          "GoJira — kanban boards, ticket transitions, worklog, and JSON export. All from your terminal.",
	SilenceErrors: true,
	SilenceUsage:  true,
}

var buildClient = BuildClient

// Execute runs the root command.
func Execute() {
	if code := runRootCommand(rootCmd, os.Args[1:]); code != 0 {
		os.Exit(code)
	}
}

func runRootCommand(root *cobra.Command, args []string) int {
	jsonIntent := hasJSONIntent(root, args)
	root.SetArgs(args)
	cmd, err := root.ExecuteC()
	if err == nil {
		return 0
	}
	if isReportedCommandError(err) {
		return 1
	}
	var flagErr *flagSyntaxError
	if errors.As(err, &flagErr) {
		if jsonIntent {
			target := cmd
			if target == nil {
				target = root
			}
			_ = reportJSONError(target, "validation_error", "command arguments are invalid", "", jira.MutationNotApplied, err)
		} else {
			_, _ = fmt.Fprintln(root.ErrOrStderr(), "Error: invalid command flags")
		}
		return 1
	}
	if jsonIntent || (cmd != nil && wantsJSON(cmd)) {
		target := cmd
		if target == nil {
			target = root
		}
		_ = reportJSONError(target, "validation_error", "command arguments are invalid", "", jira.MutationNotApplied, err)
		return 1
	}
	_, _ = fmt.Fprintf(root.ErrOrStderr(), "Error: %v\n", err)
	return 1
}

type flagSyntaxError struct {
	err error
}

func (e *flagSyntaxError) Error() string { return e.err.Error() }
func (e *flagSyntaxError) Unwrap() error { return e.err }

// hasJSONIntent parses only the selected command's flag grammar. Values for
// non-JSON flags are skipped without inspection, and -- terminates scanning.
func hasJSONIntent(root *cobra.Command, args []string) bool {
	command, commandArgs := commandFromArgs(root, args)
	if command == nil || command.Flags().Lookup("json") == nil {
		return false
	}

	intent := false
	for i := 0; i < len(commandArgs); i++ {
		arg := commandArgs[i]
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "--") {
			nameValue := strings.TrimPrefix(arg, "--")
			name, value, hasValue := strings.Cut(nameValue, "=")
			flag := command.Flags().Lookup(name)
			if flag == nil {
				continue
			}
			if name == "json" {
				if !hasValue {
					intent = true
				} else if parsed, err := strconv.ParseBool(value); err == nil {
					intent = parsed
				}
			}
			if !hasValue && flag.NoOptDefVal == "" && i+1 < len(commandArgs) {
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "-") && arg != "-" && shortFlagConsumesNext(command, arg) && i+1 < len(commandArgs) {
			i++
		}
	}
	return intent
}

func commandFromArgs(root *cobra.Command, args []string) (*cobra.Command, []string) {
	var selected *cobra.Command
	selectedIndex := len(args)
	for _, child := range root.Commands() {
		if index := commandIndexForCandidate(child, args); index >= 0 && index < selectedIndex {
			selected = child
			selectedIndex = index
		}
	}
	if selected == nil {
		return root, args
	}

	commandArgs := make([]string, 0, len(args)-1)
	commandArgs = append(commandArgs, args[:selectedIndex]...)
	commandArgs = append(commandArgs, args[selectedIndex+1:]...)
	return selected, commandArgs
}

func commandIndexForCandidate(command *cobra.Command, args []string) int {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return -1
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			if flagConsumesNext(command, arg) && i+1 < len(args) {
				i++
			}
			continue
		}
		if command.Name() == arg || command.HasAlias(arg) {
			return i
		}
		return -1
	}
	return -1
}

func flagConsumesNext(command *cobra.Command, arg string) bool {
	if strings.HasPrefix(arg, "--") {
		nameValue := strings.TrimPrefix(arg, "--")
		name, _, hasValue := strings.Cut(nameValue, "=")
		flag := command.Flags().Lookup(name)
		return flag != nil && !hasValue && flag.NoOptDefVal == ""
	}
	return shortFlagConsumesNext(command, arg)
}

func shortFlagConsumesNext(command *cobra.Command, arg string) bool {
	if !strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "--") {
		return false
	}
	shorthands := strings.TrimPrefix(arg, "-")
	for index, shorthand := range shorthands {
		flag := command.Flags().ShorthandLookup(string(shorthand))
		if flag == nil {
			return false
		}
		if flag.NoOptDefVal == "" {
			return index == len(shorthands)-1
		}
	}
	return false
}

func init() {
	cobra.OnInitialize(initEnv)
	rootCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &flagSyntaxError{err: err}
	})
}

func initEnv() {
	_ = godotenv.Load()
}

// BuildClient resolves credentials with precedence:
//
//	environment variables > config.yaml (~/.config/gojira/config.yaml)
func BuildClient() (*jira.Client, string, error) {
	email := os.Getenv("JIRA_EMAIL")
	token := os.Getenv("JIRA_API_TOKEN")
	domain := os.Getenv("JIRA_DOMAIN")

	// Fall back to config file
	if email == "" || token == "" || domain == "" {
		cfg, err := config.Load()
		if err != nil {
			return nil, "", err
		}
		if domain == "" {
			domain = cfg.Domain
		}
		if email == "" {
			email = cfg.Email
		}
		if token == "" {
			t, err := config.LoadToken()
			if err != nil {
				return nil, "", fmt.Errorf("no credentials found (set env vars or run: gojira config init)")
			}
			token = t
		}
	}

	if domain == "" || email == "" || token == "" {
		return nil, "", fmt.Errorf("incomplete credentials (run: gojira config init)")
	}

	return jira.NewClient("https://"+domain, email, token), domain, nil
}
