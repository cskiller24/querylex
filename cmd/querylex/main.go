package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		human, _ := cmd.Flags().GetBool("human")
		if human {
			resp := cli.RunStats()
			cli.RenderStatsHuman(os.Stdout, resp.Data)
			return
		}
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

var explainCmd = &cobra.Command{
	Use:   "explain <sql>",
	Short: "Show execution plan for a SQL query",
	Long:  "Returns a dialect-normalized execution plan with heuristic analysis. Use --analyze to execute for actual runtime timing.",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		analyze, _ := cmd.Flags().GetBool("analyze")
		resp := cli.RunExplain(args[0], analyze)
		resp.Complete(start)
		outputResponse(resp)
		if !resp.Success {
			os.Exit(1)
		}
	},
}

var validateCmd = &cobra.Command{
	Use:   "validate <sql>",
	Short: "Validate SQL against active database schema",
	Long:  "Validates SQL without executing it. Layer 1 rejects DML/DCL statements. Layer 2 checks against the database schema.",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		resp := cli.RunValidate(args[0])
		resp.Complete(start)
		outputResponse(resp)
		if !resp.Success {
			os.Exit(1)
		}
	},
}

var joinsCmd = &cobra.Command{
	Use:   "joins",
	Short: "Show join relationships for tables",
	Long:  "Returns declared and inferred join relationships. Use --table once for all joins, twice for specific path.",
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		tables, _ := cmd.Flags().GetStringArray("table")
		tablesJSON, _ := cmd.Flags().GetString("tables-json")
		allTables := mergeTableArgs(tables, tablesJSON)
		resp := cli.RunJoins(allTables)
		resp.Complete(start)
		outputResponse(resp)
		if !resp.Success {
			os.Exit(1)
		}
	},
}

var saveCmd = &cobra.Command{
	Use:   "save <input> <sql>",
	Short: "Save a query to memory",
	Long:  "Upserts a SQL query into persistent memory with the given natural language input. The entry is stored in memory.sqlite and the keyword index is rebuilt.",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		resp := cli.RunSave(args[0], args[1])
		resp.Complete(start)
		outputResponse(resp)
		if !resp.Success {
			os.Exit(1)
		}
	},
}

var memoryCmd = &cobra.Command{
	Use:   "memory <input>",
	Short: "Search memory for matching queries",
	Long:  "Searches saved memory entries for strong matches (similarity >= 0.86) to the given input. Returns the best match or match_found: false.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		resp := cli.RunMemory(args[0])
		resp.Complete(start)
		outputResponse(resp)
		if !resp.Success {
			os.Exit(1)
		}
	},
}

var historyCmd = &cobra.Command{
	Use:   "history <topic>",
	Short: "Browse query history by topic",
	Long:  "Searches saved memory entries related to the given topic and returns them ranked by similarity and recency.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		resp := cli.RunHistory(args[0])
		resp.Complete(start)
		outputResponse(resp)
		if !resp.Success {
			os.Exit(1)
		}
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete <input>",
	Short: "Delete a memory entry",
	Long:  "Removes a saved memory entry by its normalized input text. Deleting a non-existent entry succeeds silently with deleted: false.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		resp := cli.RunDelete(args[0])
		resp.Complete(start)
		outputResponse(resp)
		if !resp.Success {
			os.Exit(1)
		}
	},
}

var aiConfigCmd = &cobra.Command{
	Use:   "ai-config",
	Short: "Configure AI provider settings",
	Long:  "Interactively set up AI provider credentials (stored in OS keychain) and model preferences via guided prompts.",
	Run: func(cmd *cobra.Command, args []string) {
		if err := cli.RunAIConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
		fmt.Println("AI configuration saved successfully.")
	},
}

var sqlCmd = &cobra.Command{
	Use:   "sql <question>",
	Short: "Generate SQL from natural language (AI-powered)",
	Long:  "Uses AI to generate dialect-correct SQL from a natural language question, leveraging live database context including schema, terminology, joins, statistics, and indexes.",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		question := strings.Join(args, " ")
		if err := cli.RunSQLGeneration(context.Background(), question); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
	},
}

var resolveCmd = &cobra.Command{
	Use:   "resolve <question>",
	Short: "Resolve natural language to table/column candidates",
	Long:  "Uses multi-pass deterministic matching against schema metadata to find relevant tables and columns. No database connection needed.",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		question := strings.Join(args, " ")
		resp := cli.RunResolve(question)
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
	rootCmd.AddCommand(explainCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(joinsCmd)
	rootCmd.AddCommand(saveCmd)
	rootCmd.AddCommand(memoryCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(resolveCmd)
	rootCmd.AddCommand(aiConfigCmd)
	rootCmd.AddCommand(sqlCmd)

	statsCmd.Flags().Bool("human", false, "Render as human-readable summary")

	schemaCmd.Flags().StringArray("table", nil, "Table names (repeatable)")
	schemaCmd.Flags().String("tables-json", "", "Tables as JSON array")

	tableStatsCmd.Flags().StringArray("table", nil, "Table names (repeatable)")
	tableStatsCmd.Flags().String("tables-json", "", "Tables as JSON array")

	indexesCmd.Flags().StringArray("table", nil, "Table names (repeatable)")
	indexesCmd.Flags().String("tables-json", "", "Tables as JSON array")
	indexesCmd.Flags().Bool("live", false, "Query database live instead of reading from schema_map.json")

	explainCmd.Flags().Bool("analyze", false, "Execute query for actual runtime plan (with warning)")
	explainCmd.Flags().String("tables-json", "", "Tables as JSON array")

	validateCmd.Flags().String("tables-json", "", "Tables as JSON array")

	joinsCmd.Flags().StringArray("table", nil, "Table names (repeatable)")
	joinsCmd.Flags().String("tables-json", "", "Tables as JSON array")

	resolveCmd.Flags().String("tables-json", "", "Tables as JSON array")
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

