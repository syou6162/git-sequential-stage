package executor

import (
	"bytes"
	"io"
	"os/exec"
)

// RealCommandExecutor is the real implementation of CommandExecutor
type RealCommandExecutor struct{}

// NewRealCommandExecutor creates a new real executor
func NewRealCommandExecutor() *RealCommandExecutor {
	return &RealCommandExecutor{}
}

// Execute implements CommandExecutor.Execute
func (r *RealCommandExecutor) Execute(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	
	output, err := cmd.Output()
	if err != nil {
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
		// Return stderr content along with the error
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitErr.Stderr = stderr.Bytes()
		}
		return nil, err
	}
	
	return output, nil
}