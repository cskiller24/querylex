// Command generate_completions generates static shell completion files
// for bash, zsh, fish, and PowerShell. It is run at build time via a
// GoReleaser before hook: go run ./cmd/generate_completions/
//
// The generated files are included in release archives under the
// completions/ directory so users can install them directly without
// needing the querylex completion subcommand.
package main

import (
	"os"
	"path/filepath"

	"github.com/cskiller24/querylex/internal/rootcmd"
)

func main() {
	// Create completions directory if it doesn't exist
	if err := os.MkdirAll("completions", 0755); err != nil {
		panic(err)
	}

	// Generate bash completion (V2 with descriptions)
	f, err := os.Create(filepath.Join("completions", "querylex.bash"))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := rootcmd.RootCmd.GenBashCompletionV2(f, true); err != nil {
		panic(err)
	}
	f.Close()

	// Generate zsh completion
	f, err = os.Create(filepath.Join("completions", "querylex.zsh"))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := rootcmd.RootCmd.GenZshCompletion(f); err != nil {
		panic(err)
	}
	f.Close()

	// Generate fish completion (with descriptions)
	f, err = os.Create(filepath.Join("completions", "querylex.fish"))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := rootcmd.RootCmd.GenFishCompletion(f, true); err != nil {
		panic(err)
	}
	f.Close()

	// Generate PowerShell completion (with descriptions)
	f, err = os.Create(filepath.Join("completions", "querylex.ps1"))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := rootcmd.RootCmd.GenPowerShellCompletionWithDesc(f); err != nil {
		panic(err)
	}
	f.Close()
}
