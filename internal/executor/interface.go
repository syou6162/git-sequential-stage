package executor

import (
	"context"
	"io"
)

// CommandExecutor defines the interface for executing external commands
type CommandExecutor interface {
	// Execute runs a command and returns its output
	Execute(ctx context.Context, name string, args ...string) ([]byte, error)

	// ExecuteWithStdin runs a command with stdin input and returns its output
	ExecuteWithStdin(ctx context.Context, name string, stdin io.Reader, args ...string) ([]byte, error)
}
