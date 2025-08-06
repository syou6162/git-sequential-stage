package executor

import "io"

// CommandExecutor defines the interface for executing external commands
type CommandExecutor interface {
	// Execute runs a command and returns its output
	Execute(name string, args ...string) ([]byte, error)

	// ExecuteWithStdin runs a command with stdin input and returns its output
	ExecuteWithStdin(name string, stdin io.Reader, args ...string) ([]byte, error)

	// ExecuteInDir runs a command in a specific directory and returns its output
	ExecuteInDir(dir string, name string, args ...string) ([]byte, error)
}
