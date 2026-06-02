package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/cskiller24/querylex/internal/cli"
)

func main() {
	start := time.Now()
	resp := cli.RunStats()
	resp.Complete(start)

	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, `{"success":false,"error":{"code":"INTERNAL_ERROR","message":"failed to serialize response","retryable":false}}`)
		os.Exit(1)
	}
	fmt.Println(string(data))

	if !resp.Success {
		os.Exit(1)
	}
}
