package cmd

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage GoJira configuration",
	Long:  "Initialize, read, or update your GoJira configuration (~/.config/gojira/config.yaml).",
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
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configTestCmd)
	rootCmd.AddCommand(configCmd)
}
