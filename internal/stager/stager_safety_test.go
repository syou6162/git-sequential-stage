package stager

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	
	"github.com/syou6162/git-sequential-stage/internal/executor"
	"github.com/syou6162/git-sequential-stage/internal/logger"
)

func TestStageHunks_SafetyChecks_Clean(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	stager := NewStager(mockExec)
	
	// Mock clean staging area
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(""),
		Error:  nil,
	}
	
	// Mock intent-to-add detection (no files)
	mockExec.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
		Output: []byte(""),
		Error:  nil,
	}
	
	// Mock patch file read
	patchContent := `diff --git a/file.go b/file.go
--- a/file.go
+++ b/file.go
@@ -1,3 +1,4 @@
 line1
 line2
 line3
+line4
`
	mockExec.Commands["cat [patch.diff]"] = executor.MockResponse{
		Output: []byte(patchContent),
		Error:  nil,
	}
	
	// Mock other necessary commands for StageHunks
	mockExec.Commands["git [diff --no-index /dev/null /dev/stdin]"] = executor.MockResponse{
		Output: []byte(patchContent),
		Error:  nil,
	}
	
	// No error should occur with clean staging area
	err := stager.StageHunks([]string{"file.go:1"}, "patch.diff")
	if err == nil {
		// StageHunks might fail for other reasons in mock, but safety check should pass
		var safetyErr *SafetyError
		if errors.As(err, &safetyErr) {
			t.Errorf("Expected no safety error, got %v", safetyErr)
		}
	}
}

func TestStageHunks_SafetyChecks_DirtyStagingArea(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	stager := NewStager(mockExec)
	
	// Mock dirty staging area with modified files
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte("M  file1.go\nA  file2.go\n"),
		Error:  nil,
	}
	
	// Mock intent-to-add detection (none in this case)
	mockExec.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
		Output: []byte("file2.go\n"),
		Error:  nil,
	}
	
	// file2.go has actual content (not intent-to-add)
	mockExec.Commands["git [diff --cached -- file2.go]"] = executor.MockResponse{
		Output: []byte("diff --git a/file2.go b/file2.go\n@@ -0,0 +1,5 @@\n+package main\n"),
		Error:  nil,
	}
	
	err := stager.StageHunks([]string{"file.go:1"}, "patch.diff")
	if err == nil {
		t.Fatal("Expected error for dirty staging area")
	}
	
	// Verify it's a safety error
	var safetyErr *SafetyError
	if !errors.As(err, &safetyErr) {
		t.Fatalf("Expected SafetyError, got %T: %v", err, err)
	}
	
	if safetyErr.Type != ErrorTypeStagingAreaNotClean {
		t.Errorf("Expected ErrorTypeStagingAreaNotClean, got %v", safetyErr.Type)
	}
	
	// Check error message contains expected information
	errMsg := err.Error()
	if !strings.Contains(errMsg, "SAFETY_CHECK_FAILED") {
		t.Error("Error message should contain SAFETY_CHECK_FAILED")
	}
	
	if !strings.Contains(errMsg, "MODIFIED: file1.go") {
		t.Error("Error message should list modified files")
	}
	
	if !strings.Contains(errMsg, "NEW: file2.go") {
		t.Error("Error message should list new files")
	}
	
	if !strings.Contains(errMsg, "RECOMMENDED_ACTIONS") {
		t.Error("Error message should contain recommended actions")
	}
}

func TestStageHunks_SafetyChecks_IntentToAddFiles(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	stager := NewStager(mockExec)
	
	// Mock staging area with intent-to-add files
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte("A  new_file.go\n"),
		Error:  nil,
	}
	
	// Mock intent-to-add detection
	mockExec.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
		Output: []byte("new_file.go\n"),
		Error:  nil,
	}
	
	// new_file.go is intent-to-add (empty diff indicates intent-to-add)
	mockExec.Commands["git [diff --cached -- new_file.go]"] = executor.MockResponse{
		Output: []byte(""),
		Error:  nil,
	}
	
	// Mock ls-files to confirm file exists
	mockExec.Commands["git [ls-files -- new_file.go]"] = executor.MockResponse{
		Output: []byte("new_file.go\n"),
		Error:  nil,
	}
	
	// Mock patch file read
	patchContent := `diff --git a/new_file.go b/new_file.go
new file mode 100644
--- /dev/null
+++ b/new_file.go
@@ -0,0 +1,3 @@
+package main
+
+func main() {}
`
	mockExec.Commands["cat [patch.diff]"] = executor.MockResponse{
		Output: []byte(patchContent),
		Error:  nil,
	}
	
	// Mock other necessary commands
	mockExec.Commands["git [diff --no-index /dev/null /dev/stdin]"] = executor.MockResponse{
		Output: []byte(patchContent),
		Error:  nil,
	}
	
	// Should not fail with intent-to-add files only
	err := stager.StageHunks([]string{"new_file.go:1"}, "patch.diff")
	if err != nil {
		var safetyErr *SafetyError
		if errors.As(err, &safetyErr) && safetyErr.Type == ErrorTypeStagingAreaNotClean {
			t.Errorf("Should allow continuation with intent-to-add files, got %v", err)
		}
	}
}

func TestStageHunks_SafetyChecks_MixedStagingArea(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	stager := NewStager(mockExec)
	
	// Mock staging area with mixed changes
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte("M  modified.go\nA  intent_file.go\nD  deleted.go\n"),
		Error:  nil,
	}
	
	// Mock intent-to-add detection
	mockExec.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
		Output: []byte("intent_file.go\n"),
		Error:  nil,
	}
	
	// intent_file.go is intent-to-add (empty diff)
	mockExec.Commands["git [diff --cached -- intent_file.go]"] = executor.MockResponse{
		Output: []byte(""),
		Error:  nil,
	}
	
	// Mock ls-files to confirm file exists
	mockExec.Commands["git [ls-files -- intent_file.go]"] = executor.MockResponse{
		Output: []byte("intent_file.go\n"),
		Error:  nil,
	}
	
	err := stager.StageHunks([]string{"file.go:1"}, "patch.diff")
	if err == nil {
		t.Fatal("Expected error for mixed staging area")
	}
	
	// Should fail because there are non-intent-to-add files
	var safetyErr *SafetyError
	if !errors.As(err, &safetyErr) {
		t.Fatalf("Expected SafetyError, got %T", err)
	}
	
	// Check that intent-to-add files are mentioned
	errMsg := err.Error()
	if !strings.Contains(errMsg, "INTENT_TO_ADD: intent_file.go") {
		t.Error("Error message should mention intent-to-add files")
	}
}

func TestStageHunks_SafetyChecks_GitNotRepository(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	stager := NewStager(mockExec)
	
	// Mock git status failure (not a repository)
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(""),
		Error:  errors.New("fatal: not a git repository"),
	}
	
	err := stager.StageHunks([]string{"file.go:1"}, "patch.diff")
	if err == nil {
		t.Fatal("Expected error when not in git repository")
	}
	
	var safetyErr *SafetyError
	if !errors.As(err, &safetyErr) {
		t.Fatalf("Expected SafetyError, got %T", err)
	}
	
	if safetyErr.Type != ErrorTypeGitOperationFailed {
		t.Errorf("Expected ErrorTypeGitOperationFailed, got %v", safetyErr.Type)
	}
}

func TestPerformSafetyChecks_DirectCall(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	stager := NewStager(mockExec)
	
	tests := []struct {
		name          string
		statusOutput  string
		intentToAdd   map[string]string // file -> diff output
		expectError   bool
		errorType     SafetyErrorType
		allowContinue bool
	}{
		{
			name:         "clean staging area",
			statusOutput: "",
			expectError:  false,
		},
		{
			name:         "modified files",
			statusOutput: "M  file.go\n",
			expectError:  true,
			errorType:    ErrorTypeStagingAreaNotClean,
		},
		{
			name:         "deleted files",
			statusOutput: "D  old.go\n",
			expectError:  true,
			errorType:    ErrorTypeStagingAreaNotClean,
		},
		{
			name:         "renamed files",
			statusOutput: "R  old.go -> new.go\n",
			expectError:  true,
			errorType:    ErrorTypeStagingAreaNotClean,
		},
		{
			name:         "intent-to-add only",
			statusOutput: "A  new.go\n",
			intentToAdd: map[string]string{
				"new.go": "", // Empty diff indicates intent-to-add
			},
			expectError:   false,
			allowContinue: true,
		},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Reset mock commands
			mockExec.Commands = make(map[string]executor.MockResponse)
			
			// Mock git status
			mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
				Output: []byte(tc.statusOutput),
				Error:  nil,
			}
			
			// Mock intent-to-add detection if needed
			if strings.Contains(tc.statusOutput, "A ") {
				files := []string{}
				for file := range tc.intentToAdd {
					files = append(files, file)
				}
				
				mockExec.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
					Output: []byte(strings.Join(files, "\n")),
					Error:  nil,
				}
				
				for file, diff := range tc.intentToAdd {
					key := fmt.Sprintf("git [diff --cached -- %s]", file)
					mockExec.Commands[key] = executor.MockResponse{
						Output: []byte(diff),
						Error:  nil,
					}
					
					// Mock ls-files to confirm file exists (for intent-to-add)
					lsKey := fmt.Sprintf("git [ls-files -- %s]", file)
					mockExec.Commands[lsKey] = executor.MockResponse{
						Output: []byte(file + "\n"),
						Error:  nil,
					}
				}
			} else {
				mockExec.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
			}
			
			err := stager.performSafetyChecks()
			
			if tc.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				
				var safetyErr *SafetyError
				if !errors.As(err, &safetyErr) {
					t.Fatalf("Expected SafetyError, got %T", err)
				}
				
				if safetyErr.Type != tc.errorType {
					t.Errorf("Expected error type %v, got %v", tc.errorType, safetyErr.Type)
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error, got %v", err)
				}
			}
		})
	}
}

func TestStageHunks_SafetyChecks_E2E(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "safety_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	
	// Save current directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current dir: %v", err)
	}
	defer os.Chdir(originalDir)
	
	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}
	
	// Initialize git repo
	if err := exec.Command("git", "init").Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}
	
	// Configure git
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()
	
	// Create and commit initial file
	if err := os.WriteFile("file.go", []byte("line1\nline2\nline3\n"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	
	if err := exec.Command("git", "add", "file.go").Run(); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}
	
	if err := exec.Command("git", "commit", "-m", "initial").Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}
	
	// Modify file
	if err := os.WriteFile("file.go", []byte("line1\nline2\nline3\nline4\n"), 0644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}
	
	// Stage the modification
	if err := exec.Command("git", "add", "file.go").Run(); err != nil {
		t.Fatalf("Failed to stage file: %v", err)
	}
	
	// Make another change
	if err := os.WriteFile("file.go", []byte("line1\nline2\nline3\nline4\nline5\n"), 0644); err != nil {
		t.Fatalf("Failed to modify file again: %v", err)
	}
	
	// Generate patch
	patchOutput, err := exec.Command("git", "diff", "HEAD").Output()
	if err != nil {
		t.Fatalf("Failed to generate patch: %v", err)
	}
	
	patchFile := "changes.patch"
	if err := os.WriteFile(patchFile, patchOutput, 0644); err != nil {
		t.Fatalf("Failed to write patch: %v", err)
	}
	
	// Try to use git-sequential-stage with dirty staging area
	stager := NewStager(executor.NewRealCommandExecutor())
	err = stager.StageHunks([]string{"file.go:1"}, patchFile)
	
	if err == nil {
		t.Fatal("Expected error for dirty staging area")
	}
	
	var safetyErr *SafetyError
	if !errors.As(err, &safetyErr) {
		t.Fatalf("Expected SafetyError, got %T: %v", err, err)
	}
	
	if safetyErr.Type != ErrorTypeStagingAreaNotClean {
		t.Errorf("Expected ErrorTypeStagingAreaNotClean, got %v", safetyErr.Type)
	}
}

// TestSafetyChecksLogging verifies that appropriate log messages are generated
func TestSafetyChecksLogging(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	
	// Create a logger that captures output
	var logOutput strings.Builder
	logger := logger.New(logger.InfoLevel)
	logger.SetOutput(&logOutput)
	
	stager := &Stager{
		executor: mockExec,
		logger:   logger,
	}
	
	// Mock intent-to-add files scenario
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte("A  file1.go\nA  file2.go\n"),
		Error:  nil,
	}
	
	mockExec.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
		Output: []byte("file1.go\nfile2.go\n"),
		Error:  nil,
	}
	
	mockExec.Commands["git [diff --cached -- file1.go]"] = executor.MockResponse{
		Output: []byte(""), // Empty diff indicates intent-to-add
		Error:  nil,
	}
	
	mockExec.Commands["git [diff --cached -- file2.go]"] = executor.MockResponse{
		Output: []byte(""), // Empty diff indicates intent-to-add
		Error:  nil,
	}
	
	// Mock ls-files for intent-to-add files
	mockExec.Commands["git [ls-files -- file1.go]"] = executor.MockResponse{
		Output: []byte("file1.go\n"),
		Error:  nil,
	}
	
	mockExec.Commands["git [ls-files -- file2.go]"] = executor.MockResponse{
		Output: []byte("file2.go\n"),
		Error:  nil,
	}
	
	err := stager.performSafetyChecks()
	if err != nil {
		t.Fatalf("Expected no error for intent-to-add files, got %v", err)
	}
	
	// Check log output
	logs := logOutput.String()
	if !strings.Contains(logs, "Intent-to-add files detected") {
		t.Error("Expected log message about intent-to-add files")
	}
	
	if !strings.Contains(logs, "semantic_commit workflow") {
		t.Error("Expected mention of semantic_commit workflow")
	}
}