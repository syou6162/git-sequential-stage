package stager

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/syou6162/git-sequential-stage/internal/executor"
)

func TestStager_StageHunks_ErrorCases(t *testing.T) {
	tests := []struct {
		name      string
		hunkSpecs []string
		patchFile string
		mockSetup func(t *testing.T, mock *executor.MockCommandExecutor) (string, error)
		expectErr bool
		errMsg    string
	}{
		{
			name:      "non-existent patch file",
			hunkSpecs: []string{"file.go:1"},
			patchFile: "/non/existent/file.patch",
			mockSetup: func(t *testing.T, mock *executor.MockCommandExecutor) (string, error) {
				// Mock safety checks
				mock.Commands["git [status --porcelain]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
				mock.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
				return "/non/existent/file.patch", nil
			},
			expectErr: true,
			errMsg:    "failed to read patch file",
		},
		{
			name:      "git diff command failure",
			hunkSpecs: []string{"file.go:1"},
			patchFile: "/tmp/test.patch",
			mockSetup: func(t *testing.T, mock *executor.MockCommandExecutor) (string, error) {
				// Setup valid patch file
				f, err := os.CreateTemp("", "test_*.patch")
				if err != nil {
					return "", fmt.Errorf("failed to create temp file: %w", err)
				}
				t.Cleanup(func() {
					f.Close()
					os.Remove(f.Name())
				})
				
				validPatch := `diff --git a/file.go b/file.go
index abc1234..def5678 100644
--- a/file.go
+++ b/file.go
@@ -1,3 +1,4 @@
 package main
 
+import "fmt"
 func main() {}`
				f.WriteString(validPatch)
				
				// Mock safety checks
				mock.Commands["git [status --porcelain]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
				mock.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
				
				// Mock patch extraction - not needed since we use go-gitdiff now
				
				// Mock git patch-id for initial calculation
				mock.Commands["git [patch-id --stable]"] = executor.MockResponse{
					Output: []byte("abc12345 def67890"),
					Error:  nil,
				}
				
				// Mock git diff to fail
				mock.Commands["git [diff HEAD -- file.go]"] = executor.MockResponse{
					Output: nil,
					Error:  fmt.Errorf("git diff failed"),
				}
				
				return f.Name(), nil
			},
			expectErr: true,
			errMsg:    "git command failed: git diff",
		},
		{
			name:      "hunk not found",
			hunkSpecs: []string{"file.go:999"},
			patchFile: "/tmp/test.patch",
			mockSetup: func(t *testing.T, mock *executor.MockCommandExecutor) (string, error) {
				// Setup valid patch file
				f, err := os.CreateTemp("", "test_*.patch")
				if err != nil {
					return "", fmt.Errorf("failed to create temp file: %w", err)
				}
				t.Cleanup(func() {
					f.Close()
					os.Remove(f.Name())
				})
				
				validPatch := `diff --git a/file.go b/file.go
index abc1234..def5678 100644
--- a/file.go
+++ b/file.go
@@ -1,3 +1,4 @@
 package main
 
+import "fmt"
 func main() {}`
				f.WriteString(validPatch)
				
				// Mock safety checks
				mock.Commands["git [status --porcelain]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
				mock.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
				
				return f.Name(), nil
			},
			expectErr: true,
			errMsg:    "not found: hunk 999 in file file.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := executor.NewMockCommandExecutor()
			patchFile, setupErr := tt.mockSetup(t, mock)
			if setupErr != nil {
				t.Fatalf("Failed to setup test: %v", setupErr)
			}
			
			stager := NewStager(mock)
			
			err := stager.StageHunks(tt.hunkSpecs, patchFile)
			
			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectErr && err != nil && tt.errMsg != "" {
				// Check if error contains expected text (flexible to handle both old and new error formats)
				errorStr := err.Error()
				if tt.name == "non-existent patch file" {
					// For file not found, check for either old or new error format
					if !strings.Contains(errorStr, "patch file") && !strings.Contains(errorStr, "file not found") {
						t.Errorf("Error message mismatch. Expected error about patch file, got %q", errorStr)
					}
				} else if !strings.Contains(errorStr, tt.errMsg) {
					t.Errorf("Error message mismatch. Expected to contain %q, got %q", tt.errMsg, errorStr)
				}
			}
		})
	}
}
