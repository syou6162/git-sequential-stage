package stager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestGitStatusReader_Clean(t *testing.T) {
	// Create a temporary directory for the test repository
	tmpDir, err := os.MkdirTemp("", "git_status_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Initialize a git repository
	repo, err := git.PlainInit(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create an initial commit to avoid empty repository issues
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	// Create and commit a file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if _, err := worktree.Add("test.txt"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	if _, err := worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
		},
	}); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	reader := NewGitStatusReader(tmpDir)
	info, err := reader.ReadStatus()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(info.StagedFiles) != 0 {
		t.Errorf("Expected no staged files, got: %v", info.StagedFiles)
	}

	if len(info.FilesByStatus) != 0 {
		t.Errorf("Expected empty FilesByStatus, got: %v", info.FilesByStatus)
	}
}

func TestGitStatusReader_StagedFiles(t *testing.T) {
	// Create a temporary directory for the test repository
	tmpDir, err := os.MkdirTemp("", "git_status_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Initialize a git repository
	repo, err := git.PlainInit(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	// Create initial files and commit them
	files := []string{"file1.txt", "file3.txt"}
	for _, file := range files {
		testFile := filepath.Join(tmpDir, file)
		if err := os.WriteFile(testFile, []byte("initial content"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
		if _, err := worktree.Add(file); err != nil {
			t.Fatalf("Failed to add file %s: %v", file, err)
		}
	}

	if _, err := worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
		},
	}); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Modify file1.txt and stage it
	if err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("modified content"), 0644); err != nil {
		t.Fatalf("Failed to modify file1.txt: %v", err)
	}
	if _, err := worktree.Add("file1.txt"); err != nil {
		t.Fatalf("Failed to stage file1.txt: %v", err)
	}

	// Add file2.txt and stage it
	if err := os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("new content"), 0644); err != nil {
		t.Fatalf("Failed to create file2.txt: %v", err)
	}
	if _, err := worktree.Add("file2.txt"); err != nil {
		t.Fatalf("Failed to stage file2.txt: %v", err)
	}

	// Remove file3.txt and stage the deletion
	if err := os.Remove(filepath.Join(tmpDir, "file3.txt")); err != nil {
		t.Fatalf("Failed to remove file3.txt: %v", err)
	}
	if _, err := worktree.Add("file3.txt"); err != nil {
		t.Fatalf("Failed to stage file3.txt deletion: %v", err)
	}

	// Create file4.txt but don't stage it (worktree only change)
	if err := os.WriteFile(filepath.Join(tmpDir, "file4.txt"), []byte("unstaged content"), 0644); err != nil {
		t.Fatalf("Failed to create file4.txt: %v", err)
	}

	reader := NewGitStatusReader(tmpDir)
	info, err := reader.ReadStatus()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should only include staged files (M , A , D , not unstaged files)
	if len(info.StagedFiles) != 3 {
		t.Errorf("Expected 3 staged files, got: %v", info.StagedFiles)
	}

	// Check file categorization
	modifiedFiles := info.FilesByStatus[FileStatusModified]
	if len(modifiedFiles) != 1 || modifiedFiles[0] != "file1.txt" {
		t.Errorf("Expected modified file1.txt, got: %v", modifiedFiles)
	}

	addedFiles := info.FilesByStatus[FileStatusAdded]
	if len(addedFiles) != 1 || addedFiles[0] != "file2.txt" {
		t.Errorf("Expected added file2.txt, got: %v", addedFiles)
	}

	deletedFiles := info.FilesByStatus[FileStatusDeleted]
	if len(deletedFiles) != 1 || deletedFiles[0] != "file3.txt" {
		t.Errorf("Expected deleted file3.txt, got: %v", deletedFiles)
	}
}

func TestGitStatusReader_InvalidRepo(t *testing.T) {
	// Test with invalid repository path
	reader := NewGitStatusReader("/nonexistent/path")
	info, err := reader.ReadStatus()

	if err == nil {
		t.Fatal("Expected error for invalid repository path")
	}

	if info != nil {
		t.Error("Expected nil info on error")
	}
}

func TestGitStatusReader_NotGitRepo(t *testing.T) {
	// Create a temporary directory that's not a git repository
	tmpDir, err := os.MkdirTemp("", "not_git_repo_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	reader := NewGitStatusReader(tmpDir)
	info, err := reader.ReadStatus()

	if err == nil {
		t.Fatal("Expected error when directory is not a git repository")
	}

	if info != nil {
		t.Error("Expected nil info on error")
	}
}
