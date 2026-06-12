// Package rootcmd provides the shared RootCmd cobra command definition
// for both the querylex binary binary and the build-time shell completion
// generator. Both packages import this library package to avoid Go's
// restriction on importing "package main".
package rootcmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/cskiller24/querylex/internal/cli"
	_ "github.com/cskiller24/querylex/internal/db/mariadb"
	_ "github.com/cskiller24/querylex/internal/db/mssql"
	_ "github.com/cskiller24/querylex/internal/db/mysql"
	_ "github.com/cskiller24/querylex/internal/db/postgresql"
	_ "github.com/cskiller24/querylex/internal/db/sqlite"
	"github.com/cskiller24/querylex/internal/state"
	"github.com/cskiller24/querylex/internal/version"
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

// RootCmd is the root cobra command for querylex. It is exported so that
// the build-time shell completion generator (cmd/generate_completions)
// can generate static completion files for inclusion in release archives.
var RootCmd = &cobra.Command{
	Use:   "querylex",
	Short: "Querylex — context-aware SQL query companion for MySQL, MariaDB, PostgreSQL, SQLite, and MSSQL",
	Long: `Querylex is a CLI-based, context-aware SQL query companion for database introspection
and analysis.

It helps users explore database schemas, validate SQL, explain query plans, analyze
indexes, discover join paths, and manage query memory across multiple database engines.
It supports MySQL, MariaDB, PostgreSQL, SQLite, and Microsoft SQL Server.

Getting Started:
  1. Add a database:     querylex add-db
  2. Check status:       querylex workspace-stats --human
  3. Explain a query:    querylex explain "SELECT ..."
  4. Save and search:    querylex save "my query"

Credential Management:
  querylex encrypt              Generate an encryption key for the credential store
  querylex encrypt --rotate     Rotate the encryption key and re-encrypt credentials
  querylex encrypt --force      Skip the confirmation prompt

Shell Completions:
  querylex completion bash > /path/to/completions`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initWorkspace()
	},
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var listDbsCmd = &cobra.Command{
	Use:   "list-dbs",
	Short: "List all connected databases",
	Long:  `Display all database connections registered in the workspace, including connection type, host, port, database name, indexing status, and which one is active. Connection details are read from each database.json file. Use --json for machine-readable JSON output.`,
	Run: func(cmd *cobra.Command, args []string) {
		useJSON, _ := cmd.Flags().GetBool("json")
		if !useJSON {
			resp := cli.RunListDBs()
			cli.RenderListDBsHuman(os.Stdout, resp.Data)
			return
		}
		start := time.Now()
		resp := cli.RunListDBs()
		resp.Complete(start)
		outputResponse(resp)
		if !resp.Success {
			os.Exit(1)
		}
	},
}

var editDbCmd = &cobra.Command{
	Use:   "edit-db <id>",
	Short: "Edit a database connection",
	Long:  `Interactively edit an existing database connection. You will be prompted for updated connection details with the current values as defaults. If you enter a new password, the old credential is deleted and the new one is stored.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		resp := cli.RunEditDB(args[0])
		resp.Complete(start)
		outputResponse(resp)
		if !resp.Success {
			os.Exit(1)
		}
	},
}

var deleteDbCmd = &cobra.Command{
	Use:   "delete-db [id]",
	Short: "Delete a database connection",
	Long:  `Remove a database connection from the workspace including its credential, indexed artifacts, and configuration. Use --force/-y to skip the confirmation prompt. If no ID is provided, you will be prompted to select from connected databases.`,
	Args:  cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		force, _ := cmd.Flags().GetBool("force")

		id := ""
		if len(args) == 0 {
			selected, err := cli.PromptSelectDatabase("Select database to delete:")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			id = selected
		} else {
			id = args[0]
		}

		resp := cli.RunDeleteDB(id, force)
		resp.Complete(start)
		outputResponse(resp)
		if !resp.Success {
			os.Exit(1)
		}
	},
}

var addDbCmd = &cobra.Command{
	Use:   "add-db",
	Short: "Add a new database connection through guided setup",
	Long:  `Interactively add a new database connection for Querylex via guided prompts. You will be asked for database type (MySQL or PostgreSQL), connection details (host, port, database name, username), and password. The password is stored in your OS keychain, never in plaintext. After setup, Querylex automatically indexes the database schema.

Flags for non-interactive use: --type (mysql|postgres), --name, --host, --port, --database, --username, --password, --ssl-mode. When all required flags are provided, the interactive prompts are skipped.`,
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()

		dbType, _ := cmd.Flags().GetString("type")
		name, _ := cmd.Flags().GetString("name")
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		database, _ := cmd.Flags().GetString("database")
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")
		sslMode, _ := cmd.Flags().GetString("ssl-mode")

		var flags *cli.AddDBFlags
		if dbType != "" || name != "" || host != "" || database != "" || username != "" || password != "" {
			flags = &cli.AddDBFlags{
				Type:     dbType,
				Name:     name,
				Host:     host,
				Port:     port,
				Database: database,
				Username: username,
				Password: password,
				SSLMode:  sslMode,
			}
		}

		resp := cli.RunAddDB(flags)
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
	Long:  `Display Querylex workspace status including all connected databases, their indexing status and progress, active database indicator, and workspace health information. Use --human flag for a readable summary instead of JSON output.`,
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
	Long:  `Displays row counts, cardinality estimates, data size, index size, and freshness metadata for the specified tables. Requires an indexed active database. Use --table flag (repeatable) to target specific tables or omit for all tables.`,
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
	Long:  `Displays index metadata including index type (BTREE, HASH, GIN, etc.), uniqueness, columns with their order, cardinality, and visibility. By default reads from schema_map.json for speed. Use --live to query the database directly for real-time index information.`,
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
	Use:     "explain <sql>",
	Short:   "Show execution plan for a SQL query",
	Long:    `Returns a dialect-normalized execution plan with heuristic analysis. The plan format adapts to each database engine — JSON for MySQL/PostgreSQL, structured text for SQLite, and XML-to-structured for MSSQL. Use --analyze to execute the query for actual runtime timing and row counts (with confirmation prompt).`,
	Example: `  querylex explain --analyze "SELECT o.id, c.name FROM orders o JOIN customers c ON o.customer_id = c.id"`,
	Args:    cobra.MinimumNArgs(1),
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
	Long:  `Validates SQL without executing it. Layer 1 checks: rejects DML (INSERT/UPDATE/DELETE) and DCL (GRANT/REVOKE) statements. Layer 2 checks: resolves table and column references against the active database schema. Returns normalized SQL or specific validation errors.`,
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
	Long:  `Returns declared foreign key relationships (confidence=1.0) and inferred column-name pattern matches (confidence<1.0) for the specified tables. Use --table once to see all joins from that table. Use --table twice with two different tables to see the join path between them.`,
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

var useDbCmd = &cobra.Command{
	Use:   "use-db [id]",
	Short: "Switch the active database",
	Long:  `Set the specified database as the active database for all subsequent commands. The database ID can be found using 'querylex list-dbs'. If no ID is provided, you will be prompted to select from connected databases.`,
	Args:  cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()

		id := ""
		if len(args) == 0 {
			selected, err := cli.PromptSelectDatabase("Select database to use:")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			id = selected
		} else {
			id = args[0]
		}

		resp := cli.RunUseDB(id)
		resp.Complete(start)
		outputResponse(resp)
		if !resp.Success {
			os.Exit(1)
		}
	},
}

var resolveCmd = &cobra.Command{
	Use:     "resolve <question>",
	Short:   "Resolve natural language to table/column candidates",
	Long:    "Uses multi-pass deterministic matching against schema metadata to find relevant tables and columns. No database connection needed.",
	Example: `  querylex resolve "find customer orders"`,
	Args:    cobra.MinimumNArgs(1),
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

var completionCmd = &cobra.Command{
	Use:                   "completion [bash|zsh|fish|powershell]",
	Short:                 "Generate shell completion script",
	Long:                  "Generate the autocompletion script for querylex for the specified shell.\n\nSee each sub-command's help for details on how to use the generated script.",
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			cmd.Root().GenBashCompletionV2(os.Stdout, true)
		case "zsh":
			cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
	},
}

var encryptCmd = &cobra.Command{
	Use:   "encrypt",
	Short: "Manage encryption key for the encrypted credential store",
	Long:  "Generate or rotate the AES-256 encryption key for the encrypted credential store. Without flags, generates a new key (and re-encrypts any existing credentials). Use --rotate to rotate the key (re-encrypt all credentials with a fresh key). Use --force/-y to skip the confirmation prompt.",
	Run: func(cmd *cobra.Command, args []string) {
		rotate, _ := cmd.Flags().GetBool("rotate")
		force, _ := cmd.Flags().GetBool("force")
		useJSON, _ := cmd.Flags().GetBool("json")

		start := time.Now()
		resp := cli.RunEncrypt(rotate, force)
		resp.Complete(start)

		if useJSON {
			outputResponse(resp)
			if !resp.Success {
				os.Exit(1)
			}
		} else {
			cli.RenderEncryptHuman(os.Stdout, resp.Data)
			if !resp.Success {
				os.Exit(1)
			}
		}
	},
}

func init() {
	RootCmd.AddCommand(listDbsCmd)
	RootCmd.AddCommand(addDbCmd)
	RootCmd.AddCommand(editDbCmd)
	RootCmd.AddCommand(deleteDbCmd)
	RootCmd.AddCommand(statsCmd)
	RootCmd.AddCommand(schemaCmd)
	RootCmd.AddCommand(tableStatsCmd)
	RootCmd.AddCommand(indexesCmd)
	RootCmd.AddCommand(explainCmd)
	RootCmd.AddCommand(validateCmd)
	RootCmd.AddCommand(joinsCmd)
	RootCmd.AddCommand(saveCmd)
	RootCmd.AddCommand(memoryCmd)
	RootCmd.AddCommand(historyCmd)
	RootCmd.AddCommand(deleteCmd)
	RootCmd.AddCommand(useDbCmd)
	RootCmd.AddCommand(resolveCmd)
	RootCmd.AddCommand(completionCmd)
	RootCmd.AddCommand(encryptCmd)

	RootCmd.CompletionOptions.HiddenDefaultCmd = true
	RootCmd.Version = version.Version
	RootCmd.SetVersionTemplate(
		fmt.Sprintf("querylex version %s (commit %s, built %s)\n",
			version.Version, version.Commit, version.BuildDate),
	)

	statsCmd.Flags().Bool("human", false, "Render as human-readable summary")

	listDbsCmd.Flags().Bool("json", false, "Output as JSON instead of human-readable")

	encryptCmd.Flags().Bool("rotate", false, "Rotate the encryption key (re-encrypt all credentials)")
	encryptCmd.Flags().BoolP("force", "y", false, "Skip confirmation prompt")
	encryptCmd.Flags().Bool("json", false, "Output as JSON instead of human-readable")

	schemaCmd.Flags().StringArray("table", nil, "Table names (repeatable)")
	schemaCmd.Flags().String("tables-json", "", "Tables as JSON array")
	_ = schemaCmd.RegisterFlagCompletionFunc("tables-json", cobra.NoFileCompletions)

	tableStatsCmd.Flags().StringArray("table", nil, "Table names (repeatable)")
	tableStatsCmd.Flags().String("tables-json", "", "Tables as JSON array")
	_ = tableStatsCmd.RegisterFlagCompletionFunc("tables-json", cobra.NoFileCompletions)

	indexesCmd.Flags().StringArray("table", nil, "Table names (repeatable)")
	indexesCmd.Flags().String("tables-json", "", "Tables as JSON array")
	indexesCmd.Flags().Bool("live", false, "Query database live instead of reading from schema_map.json")
	_ = indexesCmd.RegisterFlagCompletionFunc("tables-json", cobra.NoFileCompletions)

	explainCmd.Flags().Bool("analyze", false, "Execute query for actual runtime plan (with warning)")
	explainCmd.Flags().String("tables-json", "", "Tables as JSON array")
	_ = explainCmd.RegisterFlagCompletionFunc("tables-json", cobra.NoFileCompletions)

	validateCmd.Flags().String("tables-json", "", "Tables as JSON array")
	_ = validateCmd.RegisterFlagCompletionFunc("tables-json", cobra.NoFileCompletions)

	joinsCmd.Flags().StringArray("table", nil, "Table names (repeatable)")
	joinsCmd.Flags().String("tables-json", "", "Tables as JSON array")
	_ = joinsCmd.RegisterFlagCompletionFunc("tables-json", cobra.NoFileCompletions)

	resolveCmd.Flags().String("tables-json", "", "Tables as JSON array")
	_ = resolveCmd.RegisterFlagCompletionFunc("tables-json", cobra.NoFileCompletions)

	deleteDbCmd.Flags().BoolP("force", "y", false, "Skip confirmation prompt")

	addDbCmd.Flags().String("type", "", "Database type (mysql|postgres)")
	addDbCmd.Flags().String("name", "", "Display name for the connection")
	addDbCmd.Flags().String("host", "", "Database host")
	addDbCmd.Flags().Int("port", 0, "Database port (default: 3306 for mysql, 5432 for postgres)")
	addDbCmd.Flags().String("database", "", "Database name")
	addDbCmd.Flags().String("username", "", "Database username")
	addDbCmd.Flags().String("password", "", "Database password")
	addDbCmd.Flags().String("ssl-mode", "", "SSL mode (require, disable, verify-ca, verify-full)")
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

	// Best-effort cleanup of orphaned temp and lock files from crashes.
	state.CleanupTempFiles(querylexDir)
	state.CleanupLockFiles(querylexDir)

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
