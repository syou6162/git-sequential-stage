package stager

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/syou6162/git-sequential-stage/internal/executor"
)

// TestStageHunks_E2E_AmbiguousFilename tests git-sequential-stage with files that have ambiguous names
func TestStageHunks_E2E_AmbiguousFilename(t *testing.T) {
	// Skip if dependencies are not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	if _, err := exec.LookPath("filterdiff"); err != nil {
		t.Skip("filterdiff not found in PATH")
	}

	testCases := []struct {
		name     string
		filename string
	}{
		{"file named master", "master"},
		{"file named main", "main"}, 
		{"file named HEAD", "HEAD"},
		{"file with tag-like name", "v1.0.0"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a temporary directory for the test
			tmpDir, err := os.MkdirTemp("", "stager_e2e_test_*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Change to temp directory
			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get current dir: %v", err)
			}
			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("Failed to change to temp dir: %v", err)
			}
			defer os.Chdir(originalDir)

			// Initialize git repo
			runCommand(t, "git", "init")
			runCommand(t, "git", "config", "user.email", "test@example.com")
			runCommand(t, "git", "config", "user.name", "Test User")

			// Create initial commit
			if err := os.WriteFile("README.md", []byte("initial"), 0644); err != nil {
				t.Fatalf("Failed to create README: %v", err)
			}
			runCommand(t, "git", "add", "README.md")
			runCommand(t, "git", "commit", "-m", "Initial commit")

			// Create a file with ambiguous name and commit it
			initialContent := "line 1\nline 2\nline 3\n"
			if err := os.WriteFile(tc.filename, []byte(initialContent), 0644); err != nil {
				t.Fatalf("Failed to create file: %v", err)
			}
			runCommand(t, "git", "add", tc.filename)
			runCommand(t, "git", "commit", "-m", "Add file")

			// Modify the file to create a diff
			modifiedContent := "line 1\nline 2\nline 3\nline 4\n"
			if err := os.WriteFile(tc.filename, []byte(modifiedContent), 0644); err != nil {
				t.Fatalf("Failed to modify file: %v", err)
			}

			// Generate patch file
			patchFile := "changes.patch"
			// Note: git diff needs -- to avoid ambiguity when generating the patch
			output := runCommand(t, "git", "diff", "HEAD", "--", tc.filename)
			if err := os.WriteFile(patchFile, output, 0644); err != nil {
				t.Fatalf("Failed to write patch file: %v", err)
			}

			// Use StageHunks directly to test the fix
			realExec := executor.NewRealCommandExecutor()
			s := NewStager(realExec)

			// This is the actual test - StageHunks should handle ambiguous filenames correctly
			err = s.StageHunks([]string{tc.filename + ":1"}, patchFile)
			if err != nil {
				// If error contains "ambiguous argument", our fix didn't work
				if strings.Contains(err.Error(), "ambiguous argument") {
					t.Fatalf("StageHunks failed with ambiguous argument error for file '%s': %v\nThis means the -- separator fix is not working", tc.filename, err)
				}
				t.Fatalf("StageHunks failed for file '%s': %v", tc.filename, err)
			}

			// Verify the file was staged correctly
			statusOutput := string(runCommand(t, "git", "status", "--porcelain"))
			expectedStatus := "M  " + tc.filename
			if !strings.Contains(statusOutput, expectedStatus) {
				t.Errorf("File '%s' was not staged properly.\nExpected: %s\nGot: %s", 
					tc.filename, expectedStatus, statusOutput)
			}

			// Verify the correct hunk was staged
			diffCachedOutput := string(runCommand(t, "git", "diff", "--cached"))
			if !strings.Contains(diffCachedOutput, "+line 4") {
				t.Errorf("The correct hunk was not staged for file '%s'", tc.filename)
			}
		})
	}
}

// runCommand executes a command and returns its output, failing the test if it errors
func runCommand(t *testing.T, name string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command failed: %s %s\nOutput: %s\nError: %v", 
			name, strings.Join(args, " "), output, err)
	}
	return output
}