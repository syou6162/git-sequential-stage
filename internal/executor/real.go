package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/syou6162/git-sequential-stage/internal/logger"
)

// RealCommandExecutor is the real implementation of CommandExecutor
type RealCommandExecutor struct {
	logger *logger.Logger
}

// NewRealCommandExecutor creates a new real executor
func NewRealCommandExecutor() *RealCommandExecutor {
	return &RealCommandExecutor{
		logger: logger.NewFromEnv(),
	}
}

// Execute implements CommandExecutor.Execute
func (r *RealCommandExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		r.logger.Error("Command failed: %s %s", name, strings.Join(args, " "))
		if stderr.Len() > 0 {
			r.logger.Error("stderr: %s", stderr.String())
		}

		// Return stderr content along with the error
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitErr.Stderr = stderr.Bytes()
		}
		return nil, err
	}

	return output, nil
}

// ExecuteWithStdin implements CommandExecutor.ExecuteWithStdin
func (r *RealCommandExecutor) ExecuteWithStdin(ctx context.Context, name string, stdin io.Reader, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		r.logger.Error("Command failed: %s %s (with stdin)", name, strings.Join(args, " "))
		if stderr.Len() > 0 {
			r.logger.Error("stderr: %s", stderr.String())
		}

		// Return stderr content along with the error
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitErr.Stderr = stderr.Bytes()
		}
		return nil, err
	}

	return output, nil
}

// WrapGitError wraps a git command error with a user-friendly message based on stderr content
func WrapGitError(err error, commandDesc string) error {
	if err == nil {
		return nil
	}

	// Check if it's an ExitError with stderr
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		// Not an ExitError, return as-is with generic message
		return fmt.Errorf("failed to execute %s: %w", commandDesc, err)
	}

	stderr := string(exitErr.Stderr)

	// Check for common git errors and provide user-friendly messages
	if strings.Contains(stderr, "fatal: not a git repository") ||
		strings.Contains(stderr, "Not a git repository") {
		return fmt.Errorf("not in a git repository. Please run this command from within a git repository")
	}

	if strings.Contains(stderr, "git: command not found") ||
		strings.Contains(stderr, "executable file not found") {
		return fmt.Errorf("git command not found. Please install git:\n  macOS: brew install git\n  Ubuntu/Debian: sudo apt-get install git\n  Fedora/RHEL: sudo yum install git")
	}

	if strings.Contains(stderr, "fatal: ambiguous argument 'HEAD'") {
		return fmt.Errorf("no commits yet in this repository. Please make an initial commit first")
	}

	// Return original error with stderr content for other cases
	if stderr != "" {
		return fmt.Errorf("failed to execute %s: %w\nstderr: %s", commandDesc, err, strings.TrimSpace(stderr))
	}

	return fmt.Errorf("failed to execute %s: %w", commandDesc, err)
}
