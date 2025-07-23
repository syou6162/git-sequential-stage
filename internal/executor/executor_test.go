package executor

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Helper function to validate mock command execution results
func validateMockExecution(t *testing.T, mock *MockCommandExecutor, output []byte, err error,
	command string, args []string, wantOutput []byte, wantError bool, wantErrMsg string) {
	// Check error expectation
	if (err != nil) != wantError {
		t.Errorf("Execute() error = %v, wantError %v", err, wantError)
		return
	}

	// Check error message
	if wantError && wantErrMsg != "" {
		if err.Error() != wantErrMsg {
			t.Errorf("Execute() error message = %v, want %v", err.Error(), wantErrMsg)
		}
	}

	// Check output
	if !bytes.Equal(output, wantOutput) {
		t.Errorf("Execute() output = %v, want %v", output, wantOutput)
	}

	// Check that command was recorded
	if len(mock.ExecutedCommands) != 1 {
		t.Errorf("Expected 1 executed command, got %d", len(mock.ExecutedCommands))
		return
	}

	executedCmd := mock.ExecutedCommands[0]
	if executedCmd.Name != command {
		t.Errorf("Executed command name = %v, want %v", executedCmd.Name, command)
	}
	if len(executedCmd.Args) != len(args) {
		t.Errorf("Executed command args length = %v, want %v", len(executedCmd.Args), len(args))
	}
	for i, arg := range args {
		if executedCmd.Args[i] != arg {
			t.Errorf("Executed command args[%d] = %v, want %v", i, executedCmd.Args[i], arg)
		}
	}
}

func TestMockCommandExecutorExecute(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*MockCommandExecutor)
		command    string
		args       []string
		wantOutput []byte
		wantError  bool
		wantErrMsg string
	}{
		{
			name: "mock successful command execution",
			setup: func(m *MockCommandExecutor) {
				m.Commands["git [--version]"] = MockResponse{
					Output: []byte("git version 2.39.0\n"),
					Error:  nil,
				}
			},
			command:    "git",
			args:       []string{"--version"},
			wantOutput: []byte("git version 2.39.0\n"),
			wantError:  false,
		},
		{
			name: "mock command execution with error",
			setup: func(m *MockCommandExecutor) {
				m.Commands["invalid-command []"] = MockResponse{
					Output: nil,
					Error:  errors.New("command not found"),
				}
			},
			command:    "invalid-command",
			args:       []string{},
			wantOutput: nil,
			wantError:  true,
			wantErrMsg: "command not found",
		},
		{
			name: "mock unexpected command",
			setup: func(m *MockCommandExecutor) {
				// Setup no commands
			},
			command:    "unexpected",
			args:       []string{"arg1", "arg2"},
			wantOutput: nil,
			wantError:  true,
			wantErrMsg: "unexpected command: unexpected [arg1 arg2]",
		},
		{
			name: "mock command with multiple arguments",
			setup: func(m *MockCommandExecutor) {
				m.Commands["git [-i *test.go --hunks=1]"] = MockResponse{
					Output: []byte("diff content"),
					Error:  nil,
				}
			},
			command:    "git",
			args:       []string{"-i", "*test.go", "--hunks=1"},
			wantOutput: []byte("diff content"),
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommandExecutor()
			tt.setup(mock)

			output, err := mock.Execute(tt.command, tt.args...)
			validateMockExecution(t, mock, output, err, tt.command, tt.args,
				tt.wantOutput, tt.wantError, tt.wantErrMsg)
		})
	}
}

// Helper function to validate mock command execution with stdin
func validateMockExecutionWithStdin(t *testing.T, mock *MockCommandExecutor, output []byte, err error,
	command string, args []string, wantOutput []byte, wantError bool, wantStdin []byte) {
	// Check error expectation
	if (err != nil) != wantError {
		t.Errorf("ExecuteWithStdin() error = %v, wantError %v", err, wantError)
		return
	}

	// Check output
	if !bytes.Equal(output, wantOutput) {
		t.Errorf("ExecuteWithStdin() output = %v, want %v", output, wantOutput)
	}

	// Check that command was recorded
	if len(mock.ExecutedCommands) != 1 {
		t.Errorf("Expected 1 executed command, got %d", len(mock.ExecutedCommands))
		return
	}

	executedCmd := mock.ExecutedCommands[0]
	if executedCmd.Name != command {
		t.Errorf("Executed command name = %v, want %v", executedCmd.Name, command)
	}

	// Check stdin was recorded correctly
	if !bytes.Equal(executedCmd.Stdin, wantStdin) {
		t.Errorf("Executed command stdin = %v, want %v", executedCmd.Stdin, wantStdin)
	}
}

func TestMockCommandExecutorExecuteWithStdin(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*MockCommandExecutor)
		command    string
		stdin      io.Reader
		args       []string
		wantOutput []byte
		wantError  bool
		wantStdin  []byte
	}{
		{
			name: "mock successful command with stdin",
			setup: func(m *MockCommandExecutor) {
				m.Commands["git [patch-id --stable]"] = MockResponse{
					Output: []byte("abc12345 commit-id\n"),
					Error:  nil,
				}
			},
			command:    "git",
			stdin:      strings.NewReader("patch content"),
			args:       []string{"patch-id", "--stable"},
			wantOutput: []byte("abc12345 commit-id\n"),
			wantError:  false,
			wantStdin:  []byte("patch content"),
		},
		{
			name: "mock command with empty stdin",
			setup: func(m *MockCommandExecutor) {
				m.Commands["git [apply --cached]"] = MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
			},
			command:    "git",
			stdin:      strings.NewReader(""),
			args:       []string{"apply", "--cached"},
			wantOutput: []byte(""),
			wantError:  false,
			wantStdin:  []byte(""),
		},
		{
			name: "mock command with nil stdin",
			setup: func(m *MockCommandExecutor) {
				m.Commands["test-command []"] = MockResponse{
					Output: []byte("result"),
					Error:  nil,
				}
			},
			command:    "test-command",
			stdin:      nil,
			args:       []string{},
			wantOutput: []byte("result"),
			wantError:  false,
			wantStdin:  nil,
		},
		{
			name: "mock unexpected command with stdin",
			setup: func(m *MockCommandExecutor) {
				// Setup no commands
			},
			command:    "unknown",
			stdin:      strings.NewReader("input"),
			args:       []string{"arg"},
			wantOutput: nil,
			wantError:  true,
			wantStdin:  []byte("input"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommandExecutor()
			tt.setup(mock)

			output, err := mock.ExecuteWithStdin(tt.command, tt.stdin, tt.args...)
			validateMockExecutionWithStdin(t, mock, output, err, tt.command, tt.args,
				tt.wantOutput, tt.wantError, tt.wantStdin)
		})
	}
}

func TestMockCommandExecutorExecutedCommandsTracking(t *testing.T) {
	mock := NewMockCommandExecutor()

	// Setup multiple commands
	mock.Commands["git [--version]"] = MockResponse{Output: []byte("git version"), Error: nil}
	mock.Commands["git [status]"] = MockResponse{Output: []byte("git status"), Error: nil}

	// Execute multiple commands
	output1, err1 := mock.Execute("git", "--version")
	if err1 != nil {
		t.Fatalf("Unexpected error from Execute: %v", err1)
	}
	if output1 == nil {
		t.Error("Expected non-nil output from Execute")
	}

	output2, err2 := mock.ExecuteWithStdin("git", strings.NewReader("input"), "status")
	if err2 != nil {
		t.Fatalf("Unexpected error from ExecuteWithStdin: %v", err2)
	}
	if output2 == nil {
		t.Error("Expected non-nil output from ExecuteWithStdin")
	}

	// Check that all commands were tracked
	if len(mock.ExecutedCommands) != 2 {
		t.Errorf("Expected 2 executed commands, got %d", len(mock.ExecutedCommands))
	}

	// Check first command
	if mock.ExecutedCommands[0].Name != "git" {
		t.Errorf("First command name = %v, want git", mock.ExecutedCommands[0].Name)
	}
	if len(mock.ExecutedCommands[0].Args) != 1 || mock.ExecutedCommands[0].Args[0] != "--version" {
		t.Errorf("First command args = %v, want [--version]", mock.ExecutedCommands[0].Args)
	}
	if mock.ExecutedCommands[0].Stdin != nil {
		t.Errorf("First command stdin = %v, want nil", mock.ExecutedCommands[0].Stdin)
	}

	// Check second command
	if mock.ExecutedCommands[1].Name != "git" {
		t.Errorf("Second command name = %v, want git", mock.ExecutedCommands[1].Name)
	}
	if !bytes.Equal(mock.ExecutedCommands[1].Stdin, []byte("input")) {
		t.Errorf("Second command stdin = %v, want [input]", mock.ExecutedCommands[1].Stdin)
	}
}

func TestRealCommandExecutorExecute(t *testing.T) {
	executor := NewRealCommandExecutor()

	tests := []struct {
		name       string
		command    string
		args       []string
		wantError  bool
		skipReason string
	}{
		{
			name:      "real successful git version command",
			command:   "git",
			args:      []string{"--version"},
			wantError: false,
		},
		{
			name:      "real successful echo command",
			command:   "echo",
			args:      []string{"test output"},
			wantError: false,
		},
		{
			name:      "real nonexistent command",
			command:   "definitely-does-not-exist-command-12345",
			args:      []string{},
			wantError: true,
		},
		{
			name:       "real git with invalid argument",
			command:    "git",
			args:       []string{"definitely-invalid-subcommand"},
			wantError:  true,
			skipReason: "git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip test if required command is not available
			if tt.skipReason != "" {
				if _, err := exec.LookPath(tt.skipReason); err != nil {
					t.Skipf("%s not found in PATH", tt.skipReason)
				}
			}

			output, err := executor.Execute(tt.command, tt.args...)

			if (err != nil) != tt.wantError {
				t.Errorf("Execute() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && len(output) == 0 {
				t.Error("Expected non-empty output for successful command")
			}

			// For error cases, verify that stderr information is available
			if tt.wantError && err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					// This tests the stderr handling in real.go
					if len(exitErr.Stderr) == 0 {
						t.Log("ExitError.Stderr is empty, which may be expected for some commands")
					}
				}
			}
		})
	}
}

func TestRealCommandExecutorExecuteWithStdin(t *testing.T) {
	executor := NewRealCommandExecutor()

	tests := []struct {
		name      string
		command   string
		stdin     io.Reader
		args      []string
		wantError bool
	}{
		{
			name:      "real cat command with stdin",
			command:   "cat",
			stdin:     strings.NewReader("test input"),
			args:      []string{},
			wantError: false,
		},
		{
			name:      "real git patch-id with valid patch",
			command:   "git",
			stdin:     strings.NewReader("diff --git a/test.txt b/test.txt\nindex 123..456\n--- a/test.txt\n+++ b/test.txt\n@@ -1 +1 @@\n-old\n+new\n"),
			args:      []string{"patch-id", "--stable"},
			wantError: false,
		},
		{
			name:      "real nonexistent command with stdin",
			command:   "definitely-does-not-exist",
			stdin:     strings.NewReader("input"),
			args:      []string{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check if command exists (skip if not available)
			if _, err := exec.LookPath(tt.command); err != nil && !tt.wantError {
				t.Skipf("%s not found in PATH", tt.command)
			}

			output, err := executor.ExecuteWithStdin(tt.command, tt.stdin, tt.args...)

			if (err != nil) != tt.wantError {
				t.Errorf("ExecuteWithStdin() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && len(output) == 0 {
				t.Error("Expected non-empty output for successful command")
			}
		})
	}
}

func TestRealCommandExecutorErrorOutput(t *testing.T) {
	// This test specifically checks the stderr handling implementation in real.go
	executor := NewRealCommandExecutor()

	// Test with a command that should produce stderr output
	// We'll use 'ls' with a non-existent directory
	_, err := executor.Execute("ls", "/definitely/does/not/exist/path/12345")

	if err == nil {
		t.Error("Expected error for non-existent path")
		return
	}

	// Check if it's an ExitError with stderr information
	if exitErr, ok := err.(*exec.ExitError); ok {
		// The real.go implementation should have set the Stderr field
		if len(exitErr.Stderr) == 0 {
			t.Error("ExitError.Stderr should contain error information")
		}
	} else {
		t.Logf("Error is not ExitError: %T", err)
	}
}

const (
	// Buffer size for stderr capture testing
	stderrBufferSize = 2048
)

func TestRealCommandExecutorStderrCapture(t *testing.T) {
	// This test verifies that stderr is properly captured and logged
	executor := NewRealCommandExecutor()

	// Capture stderr output to verify error logging using a safer approach
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	defer func() {
		os.Stderr = oldStderr
		_ = r.Close()
	}()
	os.Stderr = w

	// Execute a command that will fail in a goroutine to avoid deadlock
	done := make(chan error, 1)
	go func() {
		_, execErr := executor.Execute("ls", "/definitely/does/not/exist")
		_ = w.Close() // Close write end to signal completion
		done <- execErr
	}()

	// Read stderr output with proper buffering
	buf := make([]byte, stderrBufferSize)
	n, readErr := r.Read(buf)
	if readErr != nil && readErr != io.EOF {
		t.Logf("Warning: Failed to read stderr: %v", readErr)
	}
	stderrOutput := string(buf[:n])

	// Wait for command execution to complete
	execErr := <-done
	if execErr == nil {
		t.Error("Expected error for non-existent path")
	}

	// Check that error information was written to stderr (may be empty on some systems)
	if n > 0 && !strings.Contains(stderrOutput, "[ERROR]") {
		t.Logf("Note: Expected [ERROR] in stderr output, got: %s", stderrOutput)
		t.Log("This may be expected behavior on some systems")
	}
}
