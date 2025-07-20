package stager

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/syou6162/git-sequential-stage/internal/executor"
	"github.com/syou6162/git-sequential-stage/testutils"
)

// TestStageHunks_E2E_AmbiguousFilename tests git-sequential-stage with files that have ambiguous names
func TestStageHunks_E2E_AmbiguousFilename(t *testing.T) {
	// Skip if dependencies are not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
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
			// Create test repository
			testRepo := testutils.NewTestRepo(t, "stager_e2e_test_*")
			defer testRepo.Cleanup()
			defer testRepo.Chdir()()

			// Create initial commit
			testRepo.CreateAndCommitFile("README.md", "initial", "Initial commit")

			// Create a file with ambiguous name and commit it
			initialContent := "line 1\nline 2\nline 3\n"
			testRepo.CreateAndCommitFile(tc.filename, initialContent, "Add file")

			// Modify the file to create a diff
			modifiedContent := "line 1\nline 2\nline 3\nline 4\n"
			testRepo.ModifyFile(tc.filename, modifiedContent)

			// Generate patch file
			patchFile := "changes.patch"
			// Note: git diff needs -- to avoid ambiguity when generating the patch
			output, err := testRepo.RunCommand("git", "diff", "HEAD", "--", tc.filename)
			if err != nil {
				t.Fatalf("Failed to generate diff: %v", err)
			}
			testRepo.CreateFile(patchFile, output)

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
			statusOutput, err := testRepo.RunCommand("git", "status", "--porcelain")
			if err != nil {
				t.Fatalf("Failed to get status: %v", err)
			}
			expectedStatus := "M  " + tc.filename
			if !strings.Contains(statusOutput, expectedStatus) {
				t.Errorf("File '%s' was not staged properly.\nExpected: %s\nGot: %s",
					tc.filename, expectedStatus, statusOutput)
			}

			// Verify the correct hunk was staged
			diffCachedOutput, err := testRepo.RunCommand("git", "diff", "--cached")
			if err != nil {
				t.Fatalf("Failed to get cached diff: %v", err)
			}
			if !strings.Contains(diffCachedOutput, "+line 4") {
				t.Errorf("The correct hunk was not staged for file '%s'", tc.filename)
			}
		})
	}
}

