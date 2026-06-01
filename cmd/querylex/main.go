package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/querylex/querylex/internal/cli"
)

var rootCmd = &cobra.Command{
	Use:   "querylex",
	Short: "Querylex — AI-augmented SQL generation and optimization",
	Long: `Querylex is a CLI-based, AI-augmented SQL query generation and optimization system.

It helps users generate SQL from natural language using live database context
and optimize existing SQL using explain plans, schema data, statistics, indexes,
and dialect-aware rewrite heuristics. It supports MySQL, MariaDB, PostgreSQL,
SQLite, and Microsoft SQL Server.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initWorkspace()
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Querylex: use 'querylex-add-db' to add a database, or 'querylex --help' for commands")
	},
}

var addDbCmd = &cobra.Command{
	Use:   "add-db",
	Short: "Add a new database connection through guided setup",
	Long:  "Interactively add and configure a new database connection for Querylex via guided prompts.",
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		resp := cli.RunAddDB()
		resp.Complete(start)
		outputResponse(resp)
		if !resp.Success {
			os.Exit(1)
		}
	},
}

var statsCmd = &cobra.Command{
	Use:   "workspace-stats",
	Short: "Show workspace status across connected databases",
	Long:  "Display Querylex workspace status including connected databases and their indexing status.",
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		resp := cli.RunStats()
		resp.Complete(start)
		outputResponse(resp)
		if !resp.Success {
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(addDbCmd)
	rootCmd.AddCommand(statsCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func initWorkspace() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	querylexDir := filepath.Join(home, ".querylex")
	logsDir := filepath.Join(querylexDir, "logs")

	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("cannot create .querylex directory: %w", err)
	}

	entries, err := os.ReadDir(querylexDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".tmp" {
				os.Remove(filepath.Join(querylexDir, entry.Name()))
			}
		}
	}

	return nil
}

func outputResponse(resp any) {
	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintln(os.Stderr, `{"success":false,"error":{"code":"INTERNAL_ERROR","message":"failed to serialize response","retryable":false},"meta":{"protocol_version":"1.0.0"}}`)
		return
	}
	fmt.Println(string(data))
}

