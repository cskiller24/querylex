package main

import (
	"fmt"
	"os"

	"github.com/querylex/querylex/internal/rootcmd"
)

func main() {
	if err := rootcmd.RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
