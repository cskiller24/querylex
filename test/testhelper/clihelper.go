package testhelper

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
	"time"
)

// RunQuerylex runs the compiled querylex binary with the given arguments
// and returns stdout, stderr, and the exit code. The binary path is
// "./bin/querylex" relative to the repo root. A 30-second timeout is
// applied to each invocation.
func RunQuerylex(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "./bin/querylex", args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run querylex: %v", err)
		}
	}

	return stdoutBuf.String(), stderrBuf.String(), exitCode
}
