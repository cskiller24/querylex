package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/querylex/querylex/internal/cli"
	_ "github.com/querylex/querylex/internal/db/mysql"
	_ "github.com/querylex/querylex/internal/db/postgresql"
	_ "github.com/querylex/querylex/internal/db/sqlite"
	_ "github.com/querylex/querylex/internal/db/mariadb"
	_ "github.com/querylex/querylex/internal/db/mssql"
)

// mergeTableArgs combines --table and --tables-json flags into a single slice.
func mergeTableArgs(tables []string, tablesJSON string) []string {
	if tablesJSON != "" {
		var jsonTables []string
		if err := json.Unmarshal([]byte(tablesJSON), &jsonTables); err == nil {
			seen := make(map[string]bool)
			for _, t := range tables {
				seen[t] = true
			}
			for _, t := range jsonTables {
				if !seen[t] {
					tables = append(tables, t)
				}
				seen[t] = true
			}
		}
	}
	return tables
}

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

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Show schema information for tables",
	Long:  "Displays complete column definitions for the specified tables or all tables if none specified.",
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		tables, _ := cmd.Flags().GetStringArray("table")
		tablesJSON, _ := cmd.Flags().GetString("tables-json")
		allTables := mergeTableArgs(tables, tablesJSON)
		resp := cli.RunSchema(allTables)
		resp.Complete(start)
		outputResponse(resp)
		if !resp.Success {
			os.Exit(1)
		}
	},
}

var tableStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show table statistics",
	Long:  "Displays row counts, cardinality, data size, index size, and freshness for the specified tables.",
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		tables, _ := cmd.Flags().GetStringArray("table")
		tablesJSON, _ := cmd.Flags().GetString("tables-json")
		allTables := mergeTableArgs(tables, tablesJSON)
		resp := cli.RunStatsTables(allTables)
		resp.Complete(start)
		outputResponse(resp)
		if !resp.Success {
			os.Exit(1)
		}
	},
}

var indexesCmd = &cobra.Command{
	Use:   "indexes",
	Short: "Show index information for tables",
	Long:  "Displays index metadata from schema_map.json by default. Use --live to query the database directly.",
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		tables, _ := cmd.Flags().GetStringArray("table")
		tablesJSON, _ := cmd.Flags().GetString("tables-json")
		live, _ := cmd.Flags().GetBool("live")
		allTables := mergeTableArgs(tables, tablesJSON)
		resp := cli.RunIndexes(allTables, live)
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
	rootCmd.AddCommand(schemaCmd)
	rootCmd.AddCommand(tableStatsCmd)
	rootCmd.AddCommand(indexesCmd)

	schemaCmd.Flags().StringArray("table", nil, "Table names (repeatable)")
	schemaCmd.Flags().String("tables-json", "", "Tables as JSON array")

	tableStatsCmd.Flags().StringArray("table", nil, "Table names (repeatable)")
	tableStatsCmd.Flags().String("tables-json", "", "Tables as JSON array")

	indexesCmd.Flags().StringArray("table", nil, "Table names (repeatable)")
	indexesCmd.Flags().String("tables-json", "", "Tables as JSON array")
	indexesCmd.Flags().Bool("live", false, "Query database live instead of reading from schema_map.json")
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

