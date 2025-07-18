package stager

import (
	"fmt"
	"strings"
	"testing"

	"github.com/syou6162/git-sequential-stage/internal/executor"
)

func TestStager_applyHunk(t *testing.T) {
	tests := []struct {
		name        string
		hunkContent string
		targetID    string
		mockSetup   func(mock *executor.MockCommandExecutor)
		expectErr   bool
		checkError  func(t *testing.T, err error)
	}{
		{
			name: "successful apply",
			hunkContent: `diff --git a/file.go b/file.go
index abc123..def456 100644
--- a/file.go
+++ b/file.go
@@ -1,3 +1,4 @@
 package main
 
+import "fmt"
 func main() {}`,
			targetID: "abc12345",
			mockSetup: func(mock *executor.MockCommandExecutor) {
				mock.Commands["git [apply --cached]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
			},
			expectErr: false,
		},
		{
			name: "apply failure",
			hunkContent: `diff --git a/file.go b/file.go
index abc123..def456 100644
--- a/file.go
+++ b/file.go
@@ -1,3 +1,4 @@
 invalid patch content`,
			targetID: "def67890",
			mockSetup: func(mock *executor.MockCommandExecutor) {
				mock.Commands["git [apply --cached]"] = executor.MockResponse{
					Output: nil,
					Error:  fmt.Errorf("error: patch failed: file.go:1: trailing whitespace"),
				}
			},
			expectErr: true,
			checkError: func(t *testing.T, err error) {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}
				stagerErr, ok := err.(*StagerError)
				if !ok {
					t.Errorf("Expected StagerError, got %T", err)
				}
				if stagerErr.Type != ErrorTypePatchApplication {
					t.Errorf("Expected ErrorTypePatchApplication, got %v", stagerErr.Type)
				}
				if !strings.Contains(err.Error(), "def67890") {
					t.Errorf("Error should contain target ID, got: %v", err)
				}
			},
		},
		{
			name: "apply failure with debug file",
			hunkContent: `diff --git a/file.go b/file.go
index abc123..def456 100644
--- a/file.go
+++ b/file.go
@@ -1,3 +1,4 @@
 invalid patch content`,
			targetID: "debug123",
			mockSetup: func(mock *executor.MockCommandExecutor) {
				mock.Commands["git [apply --cached]"] = executor.MockResponse{
					Output: nil,
					Error:  fmt.Errorf("error: patch failed"),
				}
			},
			expectErr: true,
			checkError: func(t *testing.T, err error) {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}
				stagerErr, ok := err.(*StagerError)
				if !ok {
					t.Errorf("Expected StagerError, got %T", err)
				}
				
				// Check that patch content is in context
				if patchContent, exists := stagerErr.Context["patch_content"]; exists {
					if patchStr, ok := patchContent.(string); ok {
						if !strings.Contains(patchStr, "invalid patch content") {
							t.Errorf("Expected patch content to contain 'invalid patch content', got %q", patchStr)
						}
					}
				} else {
					t.Error("Expected patch_content in error context")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := executor.NewMockCommandExecutor()
			if tt.mockSetup != nil {
				tt.mockSetup(mock)
			}
			
			stager := NewStager(mock)
			
			// Test without debug environment variable
			err := stager.applyHunk([]byte(tt.hunkContent), tt.targetID)
			
			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			if tt.checkError != nil && err != nil {
				tt.checkError(t, err)
			}
		})
	}
}

