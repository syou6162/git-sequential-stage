package executor

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
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
		fmt.Fprintf(os.Stderr, "[ERROR] Command failed: %s %s\n", name, strings.Join(args, " "))
		if stderr.Len() > 0 {
			fmt.Fprintf(os.Stderr, "[ERROR] stderr: %s\n", stderr.String())
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
		fmt.Fprintf(os.Stderr, "[ERROR] Command failed: %s %s (with stdin)\n", name, strings.Join(args, " "))
		if stderr.Len() > 0 {
			fmt.Fprintf(os.Stderr, "[ERROR] stderr: %s\n", stderr.String())
		}
		
		// Return stderr content along with the error
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitErr.Stderr = stderr.Bytes()
		}
		return nil, err
	}
	
	return output, nil
}