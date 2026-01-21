package stager

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
)

func TestGitStatusReader_Worktree(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()

	// Initialize a main repository using git command for accurate worktree support
	mainRepoPath := filepath.Join(tmpDir, "main-repo")
	if err := os.MkdirAll(mainRepoPath, 0755); err != nil {
		t.Fatalf("Failed to create main repo dir: %v", err)
	}

	// Use real git commands to create a proper worktree setup
	cmd := exec.Command("git", "init")
	cmd.Dir = mainRepoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git init: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = mainRepoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to config user.name: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = mainRepoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to config user.email: %v", err)
	}

	// Create and commit a file
	testFile := filepath.Join(mainRepoPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content\n"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = mainRepoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "initial commit")
	cmd.Dir = mainRepoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}

	// Create a worktree using git worktree add
	worktreePath := filepath.Join(tmpDir, "worktree")
	cmd = exec.Command("git", "worktree", "add", worktreePath, "-b", "test-branch")
	cmd.Dir = mainRepoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git worktree add: %v, output: %s", err, output)
	}

	// Modify the file in worktree
	worktreeFile := filepath.Join(worktreePath, "test.txt")
	if err := os.WriteFile(worktreeFile, []byte("modified content\n"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Test without EnableDotGitCommonDir: This should demonstrate the problem
	t.Run("without EnableDotGitCommonDir", func(t *testing.T) {
		// Open repository without EnableDotGitCommonDir
		repo, err := git.PlainOpen(worktreePath)
		if err != nil {
			t.Fatalf("Failed to open worktree: %v", err)
		}

		// Try to get HEAD - this should fail without EnableDotGitCommonDir
		head, err := repo.Head()
		if err != nil {
			t.Logf("Expected failure: HEAD resolution failed without EnableDotGitCommonDir: %v", err)
			// This is expected to fail
		} else {
			t.Logf("HEAD resolved: %s", head.Hash())
		}

		// Try to get status - this will include all files as "Added" due to HEAD resolution failure
		wt, err := repo.Worktree()
		if err != nil {
			t.Fatalf("Failed to get worktree: %v", err)
		}

		status, err := wt.Status()
		if err != nil {
			t.Fatalf("Failed to get status: %v", err)
		}

		// Count files marked as staged
		stagedCount := 0
		for _, fs := range status {
			if fs.Staging != git.Unmodified && fs.Staging != git.Untracked {
				stagedCount++
			}
		}

		t.Logf("Without EnableDotGitCommonDir: %d files detected in status", len(status))
		t.Logf("Staged files: %d", stagedCount)

		// In a worktree without EnableDotGitCommonDir, this will incorrectly report many files
		// as staged because it can't resolve HEAD properly
	})

	// Test with EnableDotGitCommonDir: This should work correctly
	t.Run("with EnableDotGitCommonDir", func(t *testing.T) {
		// Open repository with EnableDotGitCommonDir
		repo, err := git.PlainOpenWithOptions(worktreePath, &git.PlainOpenOptions{
			EnableDotGitCommonDir: true,
		})
		if err != nil {
			t.Fatalf("Failed to open worktree: %v", err)
		}

		// Try to get HEAD - this should succeed with EnableDotGitCommonDir
		head, err := repo.Head()
		if err != nil {
			t.Fatalf("HEAD resolution failed with EnableDotGitCommonDir: %v", err)
		}
		t.Logf("HEAD resolved: %s", head.Hash())

		// Get status - this should correctly identify only the modified file
		wt, err := repo.Worktree()
		if err != nil {
			t.Fatalf("Failed to get worktree: %v", err)
		}

		status, err := wt.Status()
		if err != nil {
			t.Fatalf("Failed to get status: %v", err)
		}

		// Count files marked as staged
		stagedCount := 0
		for _, fs := range status {
			if fs.Staging != git.Unmodified && fs.Staging != git.Untracked {
				stagedCount++
			}
		}

		t.Logf("With EnableDotGitCommonDir: %d files detected in status", len(status))
		t.Logf("Staged files: %d", stagedCount)

		// With EnableDotGitCommonDir, status should be accurate
		if len(status) > 5 {
			t.Errorf("Too many files in status (%d), expected only modified file", len(status))
		}
	})

	// Test GitStatusReader with worktree
	t.Run("GitStatusReader on worktree", func(t *testing.T) {
		reader := NewGitStatusReader(worktreePath)
		statusInfo, err := reader.ReadStatus()
		if err != nil {
			t.Fatalf("ReadStatus failed on worktree: %v", err)
		}

		t.Logf("GitStatusReader: %d staged files", len(statusInfo.StagedFiles))
		t.Logf("Staged files: %v", statusInfo.StagedFiles)

		// Without EnableDotGitCommonDir in GitStatusReader, this will fail or report incorrect files
		// This test demonstrates the bug in the current implementation
	})
}

// TestGitStatusReader_WorktreeShouldNotReportAllFilesAsStaged tests that
// GitStatusReader correctly handles worktrees and doesn't report all files as staged
func TestGitStatusReader_WorktreeShouldNotReportAllFilesAsStaged(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()

	// Initialize a main repository
	mainRepoPath := filepath.Join(tmpDir, "main-repo")
	if err := os.MkdirAll(mainRepoPath, 0755); err != nil {
		t.Fatalf("Failed to create main repo dir: %v", err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = mainRepoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git init: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = mainRepoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to config: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = mainRepoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to config: %v", err)
	}

	// Create multiple files
	for i := 1; i <= 5; i++ {
		filename := filepath.Join(mainRepoPath, "file"+string(rune('0'+i))+".txt")
		if err := os.WriteFile(filename, []byte("content\n"), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = mainRepoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = mainRepoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}

	// Create worktree
	worktreePath := filepath.Join(tmpDir, "worktree")
	cmd = exec.Command("git", "worktree", "add", worktreePath, "-b", "feature")
	cmd.Dir = mainRepoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git worktree add: %v", err)
	}

	// Modify only one file
	if err := os.WriteFile(filepath.Join(worktreePath, "file1.txt"), []byte("modified\n"), 0644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	// Test GitStatusReader
	reader := NewGitStatusReader(worktreePath)
	statusInfo, err := reader.ReadStatus()
	if err != nil {
		t.Fatalf("ReadStatus failed: %v", err)
	}

	// BUG: Without EnableDotGitCommonDir, this will incorrectly report all 5 files as staged
	// EXPECTED: Should report 0 staged files (the modified file is not staged yet)
	if len(statusInfo.StagedFiles) > 0 {
		t.Errorf("BUG: Expected 0 staged files in worktree, but got %d: %v",
			len(statusInfo.StagedFiles), statusInfo.StagedFiles)
		t.Logf("This indicates that go-git is not properly handling worktrees without EnableDotGitCommonDir")
	}
}
