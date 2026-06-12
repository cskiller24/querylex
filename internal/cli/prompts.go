package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/AlecAivazis/survey/v2"
	"golang.org/x/term"

	"github.com/cskiller24/querylex/internal/state"
)

type DBSetupAnswers struct {
	DBType   string
	Name     string
	Host     string
	Port     int
	Database string
	Username string
	Password string
	SSLMode  string
}

func DefaultPort(dbType string) int {
	switch dbType {
	case "mysql":
		return 3306
	case "postgres":
		return 5432
	default:
		return 3306
	}
}

func PromptDatabaseSetup() (*DBSetupAnswers, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, fmt.Errorf("querylex add-db requires an interactive terminal")
	}

	answers := &DBSetupAnswers{}

	dbTypeQs := &survey.Select{
		Message: "Select database type:",
		Options: []string{"mysql", "postgres"},
		Default: "mysql",
	}
	if err := survey.AskOne(dbTypeQs, &answers.DBType); err != nil {
		return nil, err
	}

	defaultPort := DefaultPort(answers.DBType)

	portQs := []*survey.Question{
		{
			Name: "name",
			Prompt: &survey.Input{
				Message: "Display name (e.g., Production MySQL):",
			},
			Validate: survey.Required,
		},
		{
			Name: "host",
			Prompt: &survey.Input{
				Message: "Host:",
				Default: "localhost",
			},
			Validate: survey.Required,
		},
		{
			Name: "port",
			Prompt: &survey.Input{
				Message: "Port:",
				Default: fmt.Sprintf("%d", defaultPort),
			},
			Validate: survey.Required,
		},
		{
			Name: "database",
			Prompt: &survey.Input{
				Message: "Database name:",
			},
			Validate: survey.Required,
		},
		{
			Name: "username",
			Prompt: &survey.Input{
				Message: "Username:",
			},
			Validate: survey.Required,
		},
	}

	raw := struct {
		Name     string
		Host     string
		Port     string
		Database string
		Username string
	}{}

	if err := survey.Ask(portQs, &raw); err != nil {
		return nil, err
	}

	answers.Name = raw.Name
	answers.Host = raw.Host
	answers.Database = raw.Database
	answers.Username = raw.Username

	port, err := strconv.Atoi(raw.Port)
	if err != nil || port < 1 || port > 65535 {
		port = defaultPort
	}
	answers.Port = port

	pwQs := &survey.Password{
		Message: "Password:",
	}
	if err := survey.AskOne(pwQs, &answers.Password); err != nil {
		return nil, err
	}

	sslQs := &survey.Select{
		Message: "SSL mode:",
		Options: []string{"require", "disable", "verify-ca", "verify-full"},
		Default: "require",
	}
	if err := survey.AskOne(sslQs, &answers.SSLMode); err != nil {
		return nil, err
	}

	return answers, nil
}

// DBEditAnswers captures the user's responses for editing an existing database connection.
type DBEditAnswers struct {
	Name     string
	Host     string
	Port     int
	Database string
	Username string
	Password string // empty means keep existing
	SSLMode  string
}

// PromptDatabaseEdit prompts the user to edit an existing database connection.
// It takes the current config values as defaults. An empty password means the
// existing password should be preserved.
func PromptDatabaseEdit(current *DBConnectionConfig) (*DBEditAnswers, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, fmt.Errorf("querylex edit-db requires an interactive terminal")
	}

	answers := &DBEditAnswers{}

	defaultPort := DefaultPort(current.Type)

	qs := []*survey.Question{
		{
			Name: "name",
			Prompt: &survey.Input{
				Message: "Display name:",
				Default: current.Name,
			},
			Validate: survey.Required,
		},
		{
			Name: "host",
			Prompt: &survey.Input{
				Message: "Host:",
				Default: current.Host,
			},
			Validate: survey.Required,
		},
		{
			Name: "port",
			Prompt: &survey.Input{
				Message: "Port:",
				Default: fmt.Sprintf("%d", current.Port),
			},
			Validate: survey.Required,
		},
		{
			Name: "database",
			Prompt: &survey.Input{
				Message: "Database name:",
				Default: current.Database,
			},
			Validate: survey.Required,
		},
		{
			Name: "username",
			Prompt: &survey.Input{
				Message: "Username:",
				Default: current.Username,
			},
			Validate: survey.Required,
		},
	}

	raw := struct {
		Name     string
		Host     string
		Port     string
		Database string
		Username string
	}{}

	if err := survey.Ask(qs, &raw); err != nil {
		return nil, err
	}

	answers.Name = raw.Name
	answers.Host = raw.Host
	answers.Database = raw.Database
	answers.Username = raw.Username

	port, err := strconv.Atoi(raw.Port)
	if err != nil || port < 1 || port > 65535 {
		port = defaultPort
	}
	answers.Port = port

	// Password: empty means keep existing
	pwQs := &survey.Password{
		Message: "Password (leave empty to keep existing):",
	}
	if err := survey.AskOne(pwQs, &answers.Password); err != nil {
		return nil, err
	}

	sslQs := &survey.Select{
		Message: "SSL mode:",
		Options: []string{"require", "disable", "verify-ca", "verify-full"},
		Default: current.SSLMode,
	}
	if err := survey.AskOne(sslQs, &answers.SSLMode); err != nil {
		return nil, err
	}

	return answers, nil
}

// PromptConfirm asks the user for a yes/no confirmation and returns true if confirmed.
func PromptConfirm(message string, defaultYes bool) (bool, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, fmt.Errorf("interactive terminal required for confirmation")
	}

	confirm := false
	prompt := &survey.Confirm{
		Message: message,
		Default: defaultYes,
	}
	if err := survey.AskOne(prompt, &confirm); err != nil {
		return false, err
	}

	return confirm, nil
}

// PromptSelectDatabase prompts the user to select a database from connected databases.
// It returns the ID of the selected database or an error if no databases are connected
// or if the user is not in a terminal.
func PromptSelectDatabase(message string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("interactive terminal required for database selection")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}

	querylexDir := filepath.Join(home, ".querylex")
	workspaceFile := filepath.Join(querylexDir, "querylex.json")
	wsStore := state.NewFileWorkspaceStore(workspaceFile)

	ws, err := wsStore.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load workspace state: %w", err)
	}

	if len(ws.ConnectedDatabases) == 0 {
		return "", fmt.Errorf("No databases connected. Run 'querylex add-db' to add one.")
	}

	// Build options: "NAME  (TYPE — ID)"
	options := make([]string, len(ws.ConnectedDatabases))
	for i, entry := range ws.ConnectedDatabases {
		options[i] = fmt.Sprintf("%s  (%s — %s)", entry.Name, entry.Type, entry.ID)
	}

	var selected int
	prompt := &survey.Select{
		Message: message,
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return "", err
	}

	if selected < 0 || selected >= len(ws.ConnectedDatabases) {
		return "", fmt.Errorf("invalid database selection")
	}

	return ws.ConnectedDatabases[selected].ID, nil
}
