package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func runTUI() error {
	client, _, err := buildClient()
	if err != nil {
		return fmt.Errorf("connection setup: %w", err)
	}

	// Test connection
	info, err := client.TestConnection()
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	fmt.Fprintf(os.Stderr, "✓ Connected to %s\n", info)

	domain := os.Getenv("JIRA_DOMAIN")
	m := model{client: client, domain: domain, view: viewBoards}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
