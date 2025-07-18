package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunGitSequentialStage_Usage(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedOutput string
		expectedError  bool
	}{
		{
			name:           "no arguments shows usage",
			args:           []string{},
			expectedOutput: "Usage:",
			expectedError:  true,
		},
		{
			name:           "missing patch flag",
			args:           []string{"-hunk", "file.go:1"},
			expectedOutput: "Usage:",
			expectedError:  true,
		},
		{
			name:           "missing hunk flag",
			args:           []string{"-patch", "test.patch"},
			expectedOutput: "Usage:",
			expectedError:  true,
		},
		{
			name:           "help flag",
			args:           []string{"-h"},
			expectedOutput: "Usage:",
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			err := runGitSequentialStage(tt.args, &output)

			if tt.expectedError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			outputStr := output.String()
			if !strings.Contains(outputStr, tt.expectedOutput) {
				t.Errorf("Expected output to contain %q, got %q", tt.expectedOutput, outputStr)
			}
		})
	}
}

func TestHunkListType(t *testing.T) {
	var hl hunkList

	// Test String method
	hl = hunkList{"file1.go:1,2", "file2.go:3"}
	expected := "file1.go:1,2, file2.go:3"
	if hl.String() != expected {
		t.Errorf("String() = %q, want %q", hl.String(), expected)
	}

	// Test Set method
	err := hl.Set("file3.go:4")
	if err != nil {
		t.Errorf("Set() returned unexpected error: %v", err)
	}
	if len(hl) != 3 {
		t.Errorf("Expected length 3, got %d", len(hl))
	}
	if hl[2] != "file3.go:4" {
		t.Errorf("Expected last element to be 'file3.go:4', got %q", hl[2])
	}
}