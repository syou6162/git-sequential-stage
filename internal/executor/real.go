package executor

import (
	"bytes"
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
func (r *RealCommandExecutor) Execute(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
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
func (r *RealCommandExecutor) ExecuteWithStdin(name string, stdin io.Reader, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
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

// ExecuteInDir implements CommandExecutor.ExecuteInDir
func (r *RealCommandExecutor) ExecuteInDir(dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		r.logger.Error("Command failed in dir %s: %s %s", dir, name, strings.Join(args, " "))
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
