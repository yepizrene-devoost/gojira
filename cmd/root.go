package cmd

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/config"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

var rootCmd = &cobra.Command{
	Use:   "gojira",
	Short: "Manage your Jira boards from the terminal",
	Long:  "GoJira — kanban boards, ticket transitions, worklog, and JSON export. All from your terminal.",
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initEnv)
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
