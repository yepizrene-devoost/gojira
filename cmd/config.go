package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage GoJira configuration",
	Long:  "Initialize, read, or update your GoJira configuration (~/.config/gojira/config.yaml).",
}

// ─── config init ───────────────────────────────────────────────────────

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up GoJira configuration interactively",
	RunE: func(cmd *cobra.Command, args []string) error {
		domain, _ := cmd.Flags().GetString("domain")
		email, _ := cmd.Flags().GetString("email")
		token, _ := cmd.Flags().GetString("token")

		reader := bufio.NewReader(os.Stdin)

		if domain == "" {
			fmt.Print("Jira domain (e.g. mycompany.atlassian.net): ")
			value, err := readConfigInput(reader, "domain")
			if err != nil {
				return err
			}
			domain = value
		}
		if email == "" {
			fmt.Print("Jira email: ")
			value, err := readConfigInput(reader, "email")
			if err != nil {
				return err
			}
			email = value
		}
		if token == "" {
			fmt.Print("Jira API token: ")
			value, err := readConfigInput(reader, "token")
			if err != nil {
				return err
			}
			token = value
		}

		if domain == "" || email == "" || token == "" {
			return fmt.Errorf("domain, email, and token are all required")
		}

		// Save config
		cfg := &config.Config{Domain: domain, Email: email}
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		if err := config.SaveToken(token); err != nil {
			return fmt.Errorf("saving token: %w", err)
		}

		// Test connection with the just-saved credentials
		client, _, err := BuildClient()
		if err != nil {
			fmt.Printf("⚠ Config saved but could not build client: %v\n", err)
			fmt.Println("  Try: gojira config test")
			return nil
		}
		info, err := client.TestConnection()
		if err != nil {
			fmt.Printf("⚠ Config saved but connection failed: %v\n", err)
			fmt.Println("  Check your credentials and try: gojira config test")
			return nil
		}

		fmt.Printf("✓ Configured and connected to %s\n", info)
		return nil
	},
}

// ─── config set ────────────────────────────────────────────────────────

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long:  "Keys: domain, email, board, project, token, people.<alias>",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		switch {
		case key == "domain":
			cfg.Domain = value
		case key == "email":
			cfg.Email = value
		case key == "board":
			cfg.Board = value
		case key == "project":
			cfg.Project = value
		case key == "token":
			return config.SaveToken(value)
		case strings.HasPrefix(key, "people."):
			alias := strings.TrimPrefix(key, "people.")
			if cfg.People == nil {
				cfg.People = make(map[string]string)
			}
			cfg.People[alias] = value
		default:
			return fmt.Errorf("unknown key %q (valid: domain, email, board, project, token, people.<alias>)", key)
		}

		return cfg.Save()
	},
}

// ─── config get ────────────────────────────────────────────────────────

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		switch {
		case key == "domain":
			fmt.Println(cfg.Domain)
		case key == "email":
			fmt.Println(cfg.Email)
		case key == "board":
			fmt.Println(cfg.Board)
		case key == "project":
			fmt.Println(cfg.Project)
		case key == "token":
			token, err := config.LoadToken()
			if err != nil {
				return err
			}
			if len(token) > 8 {
				fmt.Printf("%s...%s\n", token[:4], token[len(token)-4:])
			} else {
				fmt.Println("****")
			}
		case strings.HasPrefix(key, "people."):
			alias := strings.TrimPrefix(key, "people.")
			if cfg.People != nil {
				if email, ok := cfg.People[alias]; ok {
					fmt.Println(email)
					return nil
				}
			}
			return fmt.Errorf("alias %q not found", alias)
		default:
			return fmt.Errorf("unknown key %q", key)
		}
		return nil
	},
}

// ─── config path ───────────────────────────────────────────────────────

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show config file path",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.ConfigPath()
		if err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	},
}

// ─── config test ───────────────────────────────────────────────────────

var configTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test Jira connection with current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, domain, err := BuildClient()
		if err != nil {
			return err
		}

		_ = domain // used for display
		info, err := client.TestConnection()
		if err != nil {
			return fmt.Errorf("connection failed: %w", err)
		}
		fmt.Printf("✓ Connected to %s\n", info)
		return nil
	},
}

func readConfigInput(reader *bufio.Reader, field string) (string, error) {
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", field, err)
	}
	return strings.TrimSpace(value), nil
}

func init() {
	configInitCmd.Flags().String("domain", "", "Jira domain (e.g. mycompany.atlassian.net)")
	configInitCmd.Flags().String("email", "", "Jira email")
	configInitCmd.Flags().String("token", "", "Jira API token")

	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configTestCmd)
	rootCmd.AddCommand(configCmd)
}
