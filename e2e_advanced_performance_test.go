package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/syou6162/git-sequential-stage/testutils"
)

// Test data constants
const (
	// performanceTargetSeconds is the performance target for large hunk operations
	performanceTargetSeconds = 5
)

// TestLargeFileWithManyHunks tests handling of large files with many hunks
func TestLargeFileWithManyHunks(t *testing.T) {

	// Setup test repository
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()
	tempDir := testRepo.Path

	// Change to temp directory
	t.Chdir(tempDir)

	// Create a large file with many functions
	largeFile := "large_module.py"
	var content strings.Builder
	content.WriteString("#!/usr/bin/env python3\n\n")

	// Create 20 functions
	for i := 1; i <= 20; i++ {
		content.WriteString(fmt.Sprintf(`def function_%d():
    print("This is function %d")

`, i, i))
	}

	content.WriteString(`def main():
`)
	for i := 1; i <= 20; i++ {
		content.WriteString(fmt.Sprintf("    function_%d()\n", i))
	}
	content.WriteString(`
if __name__ == "__main__":
    main()
`)

	if err := os.WriteFile(largeFile, []byte(content.String()), 0644); err != nil {
		t.Fatalf("Failed to write large file: %v", err)
	}

	// Initial commit
	gitAddCmd := exec.Command("git", "add", largeFile)
	if output, err := gitAddCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git add: %v\nOutput: %s", err, output)
	}

	gitCommitCmd := exec.Command("git", "commit", "-m", "Initial commit with large file")
	if output, err := gitCommitCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git commit: %v\nOutput: %s", err, output)
	}

	// Modify multiple functions throughout the file
	var modifiedContent strings.Builder
	modifiedContent.WriteString("#!/usr/bin/env python3\n\n")

	for i := 1; i <= 20; i++ {
		if i == 1 || i == 5 || i == 10 || i == 15 || i == 20 {
			// Modify these functions
			modifiedContent.WriteString(fmt.Sprintf(`def function_%d():
    print("This is function %d - MODIFIED")
    print("Additional line in function %d")

`, i, i, i))
		} else {
			// Keep original
			modifiedContent.WriteString(fmt.Sprintf(`def function_%d():
    print("This is function %d")

`, i, i))
		}
	}

	modifiedContent.WriteString(`def main():
`)
	for i := 1; i <= 20; i++ {
		modifiedContent.WriteString(fmt.Sprintf("    function_%d()\n", i))
	}
	modifiedContent.WriteString(`    print("All functions called")

if __name__ == "__main__":
    main()
`)

	if err := os.WriteFile(largeFile, []byte(modifiedContent.String()), 0644); err != nil {
		t.Fatalf("Failed to write modified file: %v", err)
	}

	// Generate patch
	patchFile := "large_file_changes.patch"
	gitDiffCmd := exec.Command("git", "diff", "HEAD")
	patchContent, err := gitDiffCmd.Output()
	if err != nil {
		t.Fatalf("Failed to generate diff: %v", err)
	}
	if err := os.WriteFile(patchFile, patchContent, 0644); err != nil {
		t.Fatalf("Failed to write patch file: %v", err)
	}

	// Count the number of hunks
	hunkCount := strings.Count(string(patchContent), "@@")
	t.Logf("Generated patch with %d hunks", hunkCount)

	// Test: Stage specific hunks (1, 3, and 5) with performance measurement
	absPatchPath, err := filepath.Abs(patchFile)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Stage hunks 1, 3, and 5 (if available)
	var selectedHunks []string
	selectedHunks = append(selectedHunks, "1", "3")
	if hunkCount >= 5 {
		selectedHunks = append(selectedHunks, "5")
	}

	hunkSpec := fmt.Sprintf("%s:%s", largeFile, strings.Join(selectedHunks, ","))

	// Measure performance
	startTime := time.Now()
	err = runGitSequentialStage([]string{hunkSpec}, absPatchPath)
	elapsed := time.Since(startTime)

	if err != nil {
		t.Fatalf("Failed to stage selected hunks: %v", err)
	}

	// Log performance metrics
	t.Logf("Performance: Staged %d hunks in %v", len(selectedHunks), elapsed)

	// Check if performance meets target
	targetDuration := time.Duration(performanceTargetSeconds) * time.Second
	if elapsed > targetDuration {
		t.Errorf("Performance issue: operation took %v, expected < %v", elapsed, targetDuration)
	} else {
		t.Logf("Performance is acceptable: %v < %v target", elapsed, targetDuration)
	}

	// Verify partial staging
	gitDiffCachedCmd := exec.Command("git", "diff", "--cached")
	cachedDiff, err := gitDiffCachedCmd.Output()
	if err != nil {
		t.Fatalf("Failed to get cached diff: %v", err)
	}

	// Count staged hunks
	stagedHunkCount := strings.Count(string(cachedDiff), "@@")
	t.Logf("Staged %d hunks out of %d", stagedHunkCount, hunkCount)

	// Verify we have both staged and unstaged changes
	gitDiffCmd2 := exec.Command("git", "diff")
	unstagedDiff, err := gitDiffCmd2.Output()
	if err != nil {
		t.Fatalf("Failed to get unstaged diff: %v", err)
	}

	unstagedHunkCount := strings.Count(string(unstagedDiff), "@@")
	t.Logf("Remaining unstaged hunks: %d", unstagedHunkCount)

	// Basic validation
	if stagedHunkCount == 0 {
		t.Error("No hunks were staged")
	}
	if stagedHunkCount == hunkCount {
		t.Error("All hunks were staged, expected partial staging")
	}
	if unstagedHunkCount == 0 && hunkCount > 3 {
		t.Error("No hunks remain unstaged, expected some unstaged changes")
	}

	// Test performance with many hunk selections
	// Reset for performance test
	gitResetCmd := exec.Command("git", "reset", "HEAD")
	if output, err := gitResetCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to reset: %v\nOutput: %s", err, output)
	}

	// Stage many hunks individually
	var manyHunks []string
	maxHunks := 10
	if hunkCount < maxHunks {
		maxHunks = hunkCount
	}
	for i := 1; i <= maxHunks; i++ {
		if i%2 == 1 { // Stage odd-numbered hunks
			manyHunks = append(manyHunks, fmt.Sprintf("%d", i))
		}
	}

	if len(manyHunks) > 0 {
		hunkSpec := fmt.Sprintf("%s:%s", largeFile, strings.Join(manyHunks, ","))
		err = runGitSequentialStage([]string{hunkSpec}, absPatchPath)
		if err != nil {
			t.Logf("Failed to stage many hunks: %v", err)
		} else {
			elapsed := time.Since(startTime)
			t.Logf("Staged %d hunks in %v", len(manyHunks), elapsed)

			// Warn if it takes too long
			if elapsed > 5*time.Second {
				t.Logf("Warning: Staging %d hunks took %v, which might be slow", len(manyHunks), elapsed)
			}
		}
	}
}
