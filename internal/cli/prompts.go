package cli

import (
	"fmt"
	"os"
	"strconv"

	"github.com/AlecAivazis/survey/v2"
	"golang.org/x/term"
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
		return nil, fmt.Errorf("querylex-add-db requires an interactive terminal")
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
