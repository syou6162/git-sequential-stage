package stager

import (
	"os/exec"
	"testing"
)

func TestStager_GetStderrFromError(t *testing.T) {
	stager := NewStager(nil) // executor not needed for this test

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "regular error",
			err:      exec.Command("false").Run(),
			expected: "exit status 1",
		},
		{
			name:     "exit error with stderr",
			err:      &exec.ExitError{Stderr: []byte("command failed with stderr")},
			expected: "command failed with stderr",
		},
		{
			name:     "exit error without stderr",
			err:      &exec.ExitError{},
			expected: "<nil>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stager.getStderrFromError(tt.err)
			if result != tt.expected {
				t.Errorf("getStderrFromError() = %q, expected %q", result, tt.expected)
			}
		})
	}
}