package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gojira",
	Short: "Manage your Jira boards from the terminal",
	Long:  "GoJira — kanban boards, ticket transitions, worklog, and JSON export. All from your terminal.",
	// Default: launch TUI
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTUI()
	},
}

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

func buildClient() (*JiraClient, string, error) {
	email := os.Getenv("JIRA_EMAIL")
	token := os.Getenv("JIRA_API_TOKEN")
	domain := os.Getenv("JIRA_DOMAIN")

	if email == "" || token == "" || domain == "" {
		return nil, "", fmt.Errorf("set JIRA_EMAIL, JIRA_API_TOKEN, JIRA_DOMAIN in .env")
	}

	client := NewJiraClient("https://"+domain, email, token)
	return client, domain, nil
}
