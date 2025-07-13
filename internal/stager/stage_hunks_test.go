package stager

import (
	"fmt"
	"os"
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
				
				// Mock filterdiff to return valid patch ID calculation
				mock.Commands["filterdiff [-i *file.go --hunks=1 "+f.Name()+"]"] = executor.MockResponse{
					Output: []byte(validPatch),
					Error:  nil,
				}
				
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
			errMsg:    "failed to get current diff",
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
				
				return f.Name(), nil
			},
			expectErr: true,
			errMsg:    "hunk 999 not found in file file.go",
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
				if len(err.Error()) < len(tt.errMsg) || err.Error()[:len(tt.errMsg)] != tt.errMsg {
					t.Errorf("Error message mismatch. Expected to start with %q, got %q", tt.errMsg, err.Error())
				}
			}
		})
	}
}
