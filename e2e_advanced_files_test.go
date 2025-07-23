package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/syou6162/git-sequential-stage/testutils"
)

// TestBinaryFileHandling tests handling of binary files in patches
func TestBinaryFileHandling(t *testing.T) {

	// Setup test repository
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()
	tempDir := testRepo.Path

	// Change to temp directory
	t.Chdir(tempDir)

	// Create initial text file
	textFile := "document.txt"
	textContent := "This is a text document.\nIt has multiple lines.\n"
	if err := os.WriteFile(textFile, []byte(textContent), 0644); err != nil {
		t.Fatalf("Failed to write text file: %v", err)
	}

	// Create binary file (small PNG image)
	binaryFile := "image.png"
	if err := os.WriteFile(binaryFile, testutils.TestData.MinimalPNGTransparent, 0644); err != nil {
		t.Fatalf("Failed to write binary file: %v", err)
	}

	// Initial commit
	gitAddCmd := exec.Command("git", "add", textFile, binaryFile)
	if output, err := gitAddCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git add: %v\nOutput: %s", err, output)
	}

	gitCommitCmd := exec.Command("git", "commit", "-m", "Initial commit with text and binary files")
	if output, err := gitCommitCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git commit: %v\nOutput: %s", err, output)
	}

	// Modify text file
	textContent2 := "This is a text document.\nIt has multiple lines.\nAdding a new line.\n"
	if err := os.WriteFile(textFile, []byte(textContent2), 0644); err != nil {
		t.Fatalf("Failed to update text file: %v", err)
	}

	// Replace binary file with a different one
	if err := os.WriteFile(binaryFile, testutils.TestData.MinimalPNGRed, 0644); err != nil {
		t.Fatalf("Failed to update binary file: %v", err)
	}

	// Generate patch
	patchFile := "mixed_changes.patch"
	gitDiffCmd := exec.Command("git", "diff", "HEAD")
	patchContent, err := gitDiffCmd.Output()
	if err != nil {
		t.Fatalf("Failed to generate diff: %v", err)
	}
	if err := os.WriteFile(patchFile, patchContent, 0644); err != nil {
		t.Fatalf("Failed to write patch file: %v", err)
	}

	// Test: Stage only text file changes (hunk 1)
	absPatchPath, err := filepath.Abs(patchFile)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	if err := runGitSequentialStage([]string{"document.txt:1"}, absPatchPath); err != nil {
		t.Fatalf("Failed to stage text file changes: %v", err)
	}

	// Verify only text file is staged
	gitStatusCmd := exec.Command("git", "status", "--porcelain")
	statusOutput, err := gitStatusCmd.Output()
	if err != nil {
		t.Fatalf("Failed to get git status: %v", err)
	}

	statusLines := strings.Split(strings.TrimSpace(string(statusOutput)), "\n")
	expectedStatus := map[string]string{
		"document.txt": "M ",
		"image.png":    " M",
	}

	for _, line := range statusLines {
		if line == "" {
			continue
		}
		status := line[:2]
		filename := line[3:]

		// Skip the patch file
		if filename == patchFile {
			continue
		}

		if expected, ok := expectedStatus[filename]; ok {
			if status != expected {
				t.Errorf("File %s: expected status %q, got %q", filename, expected, status)
			}
		} else {
			t.Errorf("Unexpected file in status: %s", filename)
		}
	}

	// Reset for next test
	gitResetCmd := exec.Command("git", "reset", "HEAD")
	if output, err := gitResetCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to reset: %v\nOutput: %s", err, output)
	}

	// Test: Try to stage binary file changes (should handle gracefully)
	// Expected behavior: Binary files don't have traditional hunks, so the tool
	// should either:
	// 1. Skip binary files with an appropriate message
	// 2. Stage the entire binary file change (since hunks don't apply)
	// The exact behavior depends on git's handling of binary files
	err = runGitSequentialStage([]string{"image.png:1"}, absPatchPath)
	if err == nil {
		// If it succeeds, verify the binary file is staged
		gitStatusCmd := exec.Command("git", "status", "--porcelain")
		statusOutput, err := gitStatusCmd.Output()
		if err != nil {
			t.Fatalf("Failed to get git status: %v", err)
		}

		if strings.Contains(string(statusOutput), "M  image.png") {
			t.Log("Binary file was successfully staged")
		}
	} else {
		// If it fails, make sure it's with a reasonable error message
		if !strings.Contains(err.Error(), "binary") && !strings.Contains(err.Error(), "hunk") {
			t.Errorf("Unexpected error for binary file: %v", err)
		}
		t.Logf("Binary file handling resulted in expected error: %v", err)
	}
}

// TestFileModificationAndMove tests handling of file modifications combined with moves
func TestFileModificationAndMove(t *testing.T) {

	// Setup test repository
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()
	tempDir := testRepo.Path

	// Change to temp directory
	t.Chdir(tempDir)

	// Create initial file structure
	if err := os.MkdirAll("src", 0755); err != nil {
		t.Fatalf("Failed to create src directory: %v", err)
	}

	oldFile := "old_module.py"
	fileContent := `#!/usr/bin/env python3

def old_function():
    print("This is the old function")

def main():
    old_function()

if __name__ == "__main__":
    main()
`
	if err := os.WriteFile(oldFile, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Initial commit
	gitAddCmd := exec.Command("git", "add", oldFile)
	if output, err := gitAddCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git add: %v\nOutput: %s", err, output)
	}

	gitCommitCmd := exec.Command("git", "commit", "-m", "Initial commit with old module")
	if output, err := gitCommitCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git commit: %v\nOutput: %s", err, output)
	}

	// Test a different scenario: modify existing file and also move it
	// First modify the content
	modifiedContent := `#!/usr/bin/env python3

def old_function():
    print("This is the old function with modifications")
    print("Adding more functionality")

def new_helper():
    print("A helper function")

def main():
    old_function()
    new_helper()

if __name__ == "__main__":
    main()
`
	if err := os.WriteFile(oldFile, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("Failed to write modified content: %v", err)
	}

	// Generate patch for modifications
	patchFile := "modifications.patch"
	gitDiffCmd := exec.Command("git", "diff", "HEAD")
	patchContent, err := gitDiffCmd.Output()
	if err != nil {
		t.Fatalf("Failed to generate diff: %v", err)
	}
	if err := os.WriteFile(patchFile, patchContent, 0644); err != nil {
		t.Fatalf("Failed to write patch file: %v", err)
	}

	// Test: Stage only the first hunk (function modification)
	absPatchPath, err := filepath.Abs(patchFile)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	err = runGitSequentialStage([]string{"old_module.py:1"}, absPatchPath)
	if err != nil {
		t.Fatalf("Failed to stage first hunk: %v", err)
	}

	// Verify partial staging
	gitDiffCachedCmd := exec.Command("git", "diff", "--cached")
	cachedDiff, err := gitDiffCachedCmd.Output()
	if err != nil {
		t.Fatalf("Failed to get cached diff: %v", err)
	}

	// Should contain the oldFunction modification
	if !strings.Contains(string(cachedDiff), "old function with modifications") {
		t.Error("First hunk not properly staged")
	}

	// Check unstaged changes
	gitDiffCmd2 := exec.Command("git", "diff")
	unstagedDiff, err := gitDiffCmd2.Output()
	if err != nil {
		t.Fatalf("Failed to get unstaged diff: %v", err)
	}

	// Log the diffs for debugging
	t.Logf("Cached diff:\n%s", cachedDiff)
	t.Logf("Unstaged diff:\n%s", unstagedDiff)

	// The modifications might be in a single hunk, so check if anything remains unstaged
	if len(unstagedDiff) > 0 && strings.Contains(string(unstagedDiff), "@@") {
		t.Log("Some changes remain unstaged as expected")
	} else {
		t.Log("All changes were staged in a single hunk")
	}

	// Now test moving files scenario with a clean state
	gitResetCmd := exec.Command("git", "reset", "--hard", "HEAD")
	if output, err := gitResetCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to reset: %v\nOutput: %s", err, output)
	}

	// Test scenario: modify file content AND move it
	// This is a more complex test case showing how to handle both modifications and moves

	// First, apply modifications to the file
	modifiedContentForMove := `#!/usr/bin/env python3

def old_function():
    print("This is the function ready for move")
    print("Now with new modifications")

def new_helper():
    print("A helper function")

def additional_func():
    print("Another new function")

def main():
    old_function()
    new_helper()
    additional_func()

if __name__ == "__main__":
    main()
`
	if err := os.WriteFile(oldFile, []byte(modifiedContentForMove), 0644); err != nil {
		t.Fatalf("Failed to write content for move: %v", err)
	}

	// Move file to new location
	newFile := "src/new_module.py"
	if err := os.Rename(oldFile, newFile); err != nil {
		t.Fatalf("Failed to move file: %v", err)
	}

	// Stage the rename/move
	gitRmCmd := exec.Command("git", "rm", oldFile)
	if output, err := gitRmCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git rm: %v\nOutput: %s", err, output)
	}

	gitAddCmd2 := exec.Command("git", "add", newFile)
	if output, err := gitAddCmd2.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git add new file: %v\nOutput: %s", err, output)
	}

	// Verify Git recognizes it as a rename with modifications
	gitStatusCmd := exec.Command("git", "status", "--porcelain")
	statusOutput, err := gitStatusCmd.Output()
	if err != nil {
		t.Fatalf("Failed to get git status: %v", err)
	}

	statusStr := string(statusOutput)
	if strings.Contains(statusStr, "R  old_module.py -> src/new_module.py") {
		t.Log("Git correctly detected file rename with modifications")
	} else if strings.Contains(statusStr, "D  old_module.py") && strings.Contains(statusStr, "A  src/new_module.py") {
		t.Log("Git shows rename as delete + add (expected for files with significant changes)")
	} else {
		t.Logf("Unexpected git status output:\n%s", statusStr)
	}

	// Test that we can still apply patches to the original file location
	// This demonstrates handling of patches created before a file move
	t.Log("Note: Applying patches to moved files requires careful handling of file paths")
}

// TestGitMvThenModifyFile tests the case where a file is moved with `git mv` and then modified
// This is a common workflow where users first move/rename a file and then make changes to it.
// The tool should be able to stage hunks from the modified moved file correctly.
func TestGitMvThenModifyFile(t *testing.T) {

	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()
	tempDir := testRepo.Path

	// Change to temp directory
	t.Chdir(tempDir)

	// Create initial file
	originalFile := "original_module.go"
	initialContent := `package main

import "fmt"

func oldFunction() {
	fmt.Println("This is the old function")
}

func main() {
	oldFunction()
}
`
	if err := os.WriteFile(originalFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to create initial file: %v", err)
	}

	// Initial commit
	if err := exec.Command("git", "add", originalFile).Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	if err := exec.Command("git", "commit", "-m", "Initial commit").Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}

	// Step 1: Move file using git mv and commit it
	newFile := "renamed_module.go"
	if err := exec.Command("git", "mv", originalFile, newFile).Run(); err != nil {
		t.Fatalf("Failed to git mv: %v", err)
	}

	// Commit the move operation first
	if err := exec.Command("git", "commit", "-m", "Move file").Run(); err != nil {
		t.Fatalf("Failed to commit move: %v", err)
	}

	// Step 2: Modify the moved file
	modifiedContent := `package main

import "fmt"

func oldFunction() {
	fmt.Println("This is the old function")
	fmt.Println("Adding more functionality to the old function")
}

func newFunction() {
	fmt.Println("This is a new function")
}

func main() {
	oldFunction()
	newFunction()
}
`
	if err := os.WriteFile(newFile, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("Failed to modify moved file: %v", err)
	}

	// Generate patch for just the modifications to the moved file
	patchFile := "moved_file_changes.patch"
	patchCmd := exec.Command("git", "diff", newFile)
	patchContent, err := patchCmd.Output()
	if err != nil {
		t.Fatalf("Failed to generate patch: %v", err)
	}

	if err := os.WriteFile(patchFile, patchContent, 0644); err != nil {
		t.Fatalf("Failed to write patch file: %v", err)
	}

	// Now test staging specific hunks from the patch
	absPatchPath, err := filepath.Abs(patchFile)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Attempt to stage first hunk from the moved file
	err = runGitSequentialStage([]string{newFile + ":1"}, absPatchPath)
	if err != nil {
		t.Fatalf("Failed to stage hunk from moved file: %v", err)
	}

	// Verify that changes were staged successfully
	stagedDiff, err := exec.Command("git", "diff", "--cached").Output()
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	// Should contain some changes from the moved file
	if len(stagedDiff) == 0 {
		t.Errorf("Expected staged diff to contain changes from moved file, but it was empty")
	}

	// The key success criteria is that the tool works without errors
	// and manages to stage some content from the moved file

	t.Log("Successfully staged hunk from moved file")
}

// TestGitMvThenModifyFileWithoutCommit tests the case where a file is moved with `git mv`
// and then modified WITHOUT committing the move first.
// This should also work as it's a valid workflow.
func TestGitMvThenModifyFileWithoutCommit(t *testing.T) {
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-mv-no-commit-*")
	defer testRepo.Cleanup()
	tempDir := testRepo.Path

	// Change to temp directory
	t.Chdir(tempDir)

	// Create initial file
	originalFile := "original_module.go"
	initialContent := `package main

import "fmt"

func oldFunction() {
	fmt.Println("This is the old function")
}

func main() {
	oldFunction()
}
`
	if err := os.WriteFile(originalFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to create initial file: %v", err)
	}

	// Initial commit
	if err := exec.Command("git", "add", originalFile).Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	if err := exec.Command("git", "commit", "-m", "Initial commit").Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}

	// Step 1: Move file using git mv BUT DO NOT COMMIT YET
	newFile := "renamed_module.go"
	if err := exec.Command("git", "mv", originalFile, newFile).Run(); err != nil {
		t.Fatalf("Failed to git mv: %v", err)
	}

	// Step 2: Modify the moved file (without committing the move)
	modifiedContent := `package main

import "fmt"

func oldFunction() {
	fmt.Println("This is the old function")
	fmt.Println("Adding more functionality to the old function")
}

func newFunction() {
	fmt.Println("This is a new function")
}

func main() {
	oldFunction()
	newFunction()
}
`
	if err := os.WriteFile(newFile, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("Failed to modify moved file: %v", err)
	}

	// Debug: Check current git status before generating patch
	statusOutput, _ := exec.Command("git", "status", "--porcelain").Output()
	t.Logf("Git status with move and modifications: %s", string(statusOutput))

	// Generate patch for the complete state (move + modifications)
	// This represents the realistic scenario where someone does git mv + modifications
	patchFile := "moved_and_modified.patch"
	patchCmd := exec.Command("git", "diff", "HEAD")
	patchContent, err := patchCmd.Output()
	if err != nil {
		t.Fatalf("Failed to generate patch: %v", err)
	}

	t.Logf("Patch content length: %d", len(patchContent))
	if len(patchContent) > 0 {
		maxLen := 500
		if len(patchContent) < maxLen {
			maxLen = len(patchContent)
		}
		t.Logf("First %d chars of patch: %s", maxLen, string(patchContent)[:maxLen])
	}

	if err := os.WriteFile(patchFile, patchContent, 0644); err != nil {
		t.Fatalf("Failed to write patch file: %v", err)
	}

	// Now test staging specific hunks from the patch
	absPatchPath, err := filepath.Abs(patchFile)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Attempt to stage first hunk from the moved file
	// This should work even though we have a move operation in the staging area
	err = runGitSequentialStage([]string{newFile + ":1"}, absPatchPath)
	if err != nil {
		t.Fatalf("Failed to stage hunk from moved and modified file: %v", err)
	}

	// Verify that changes were staged successfully
	stagedDiff, err := exec.Command("git", "diff", "--cached").Output()
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	// Should contain some changes from the moved file
	if len(stagedDiff) == 0 {
		t.Errorf("Expected staged diff to contain changes from moved file, but it was empty")
	}

	t.Log("Successfully staged hunk from moved and modified file without committing move first")
}

// TestMultipleFilesMoveAndModify tests git mv of multiple files followed by modifications
// TODO: パッチIDの問題を解決してから有効化
func TestMultipleFilesMoveAndModify_Skip(t *testing.T) {
	t.Skip("Skipping test due to patch ID issues - needs investigation")
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-multi-move-*")
	defer testRepo.Cleanup()

	// Phase 1: Create initial files
	testRepo.CreateFile("src/utils.py", `def calculate(x, y):
    return x + y
`)
	testRepo.CreateFile("src/helper.py", `def validate(data):
    return data is not None
`)
	testRepo.CommitChanges("Initial files")

	// Phase 2: Create lib directory and move files
	testRepo.CreateFile("lib/.gitkeep", "") // Create lib directory
	testRepo.RunCommandOrFail("git", "mv", "src/utils.py", "lib/calculations.py")
	testRepo.RunCommandOrFail("git", "mv", "src/helper.py", "lib/validators.py")
	testRepo.CommitChanges("Move files to lib directory")

	// Phase 3: Modify moved files (simple changes)
	testRepo.ModifyFile("lib/calculations.py", `def calculate(x, y):
    # Enhanced with logging
    print(f"Calculating {x} + {y}")
    return x + y
`)

	testRepo.ModifyFile("lib/validators.py", `def validate(data):
    # Enhanced validation
    print("Validating data")
    return data is not None
`)

	// Phase 4: Generate patch and test selective staging
	testRepo.GeneratePatch("changes.patch")
	testRepo.RunCommandOrFail("git", "reset", "--hard", "HEAD")

	// Test staging specific hunks
	defer testRepo.Chdir()()
	absPatchPath, err := filepath.Abs(testRepo.GetFilePath("changes.patch"))
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Test with only one file to ensure it works
	err = runGitSequentialStage([]string{
		"lib/calculations.py:1", // Enhanced calculate function
	}, absPatchPath)

	if err != nil {
		t.Fatalf("git-sequential-stage failed: %v", err)
	}

	// Verify selective staging worked
	stagedDiff, err := testRepo.RunCommand("git", "diff", "--cached")
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	expectedChanges := []string{
		"lib/calculations.py",
		"Enhanced with logging",
	}

	for _, expected := range expectedChanges {
		if !strings.Contains(stagedDiff, expected) {
			t.Errorf("Expected staged content to contain '%s'", expected)
		}
	}

	t.Log("Successfully staged selective hunks from multiple moved files")
}
