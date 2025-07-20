package testutils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// TestData contains common test data like binary files
var TestData = struct {
	// MinimalPNGTransparent is a 1x1 transparent PNG image
	MinimalPNGTransparent []byte
	// MinimalPNGRed is a 1x1 red PNG image
	MinimalPNGRed []byte
}{
	MinimalPNGTransparent: []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
		0x89, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x44, 0x41, // IDAT chunk
		0x54, 0x78, 0x9C, 0x62, 0x00, 0x00, 0x00, 0x02,
		0x00, 0x01, 0xE5, 0x27, 0xDE, 0xFC, 0x00, 0x00, // IEND chunk
		0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42,
		0x60, 0x82,
	},
	MinimalPNGRed: []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, // IDAT chunk
		0x54, 0x08, 0x99, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xDD, 0x8D, // IEND chunk
		0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
		0x44, 0xAE, 0x42, 0x60, 0x82,
	},
}

// TestRepo provides a unified interface for test repositories
type TestRepo struct {
	t       *testing.T
	Path    string
	Repo    *git.Repository
	cleanup func()
}

// NewTestRepo creates a new test repository with proper initialization
func NewTestRepo(t *testing.T, prefix string) *TestRepo {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", prefix)
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Initialize git repository
	repo, err := git.PlainInit(tmpDir, false)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to initialize git repository: %v", err)
	}

	// Configure git user
	cfg, err := repo.Config()
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to get config: %v", err)
	}

	cfg.User.Name = "Test User"
	cfg.User.Email = "test@example.com"
	err = repo.SetConfig(cfg)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to set config: %v", err)
	}

	testRepo := &TestRepo{
		t:    t,
		Path: tmpDir,
		Repo: repo,
		cleanup: func() {
			os.RemoveAll(tmpDir)
		},
	}

	return testRepo
}

// Cleanup removes the test repository
func (tr *TestRepo) Cleanup() {
	if tr.cleanup != nil {
		tr.cleanup()
	}
}

// Chdir changes to the repository directory and returns a cleanup function
func (tr *TestRepo) Chdir() func() {
	tr.t.Helper()

	originalDir, err := os.Getwd()
	if err != nil {
		tr.t.Fatalf("Failed to get current dir: %v", err)
	}

	if err := os.Chdir(tr.Path); err != nil {
		tr.t.Fatalf("Failed to change to temp dir: %v", err)
	}

	return func() {
		os.Chdir(originalDir)
	}
}

// RunCommand executes a command in the repository directory
func (tr *TestRepo) RunCommand(command string, args ...string) (string, error) {
	tr.t.Helper()
	cmd := exec.Command(command, args...)
	cmd.Dir = tr.Path
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// RunCommandOrFail executes a command and fails the test if it errors
func (tr *TestRepo) RunCommandOrFail(command string, args ...string) string {
	tr.t.Helper()
	output, err := tr.RunCommand(command, args...)
	if err != nil {
		tr.t.Fatalf("Command failed: %s %s\nOutput: %s\nError: %v",
			command, strings.Join(args, " "), output, err)
	}
	return output
}

// CreateFile creates a file with the given content
func (tr *TestRepo) CreateFile(filename, content string) {
	tr.t.Helper()
	filepath := filepath.Join(tr.Path, filename)
	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		tr.t.Fatalf("Failed to create file %s: %v", filename, err)
	}
}

// CreateBinaryFile creates a binary file with the given content
func (tr *TestRepo) CreateBinaryFile(filename string, content []byte) {
	tr.t.Helper()
	filepath := filepath.Join(tr.Path, filename)
	if err := os.WriteFile(filepath, content, 0644); err != nil {
		tr.t.Fatalf("Failed to create binary file %s: %v", filename, err)
	}
}

// ModifyFile modifies an existing file with new content
func (tr *TestRepo) ModifyFile(filename, newContent string) {
	tr.t.Helper()
	filepath := filepath.Join(tr.Path, filename)
	if err := os.WriteFile(filepath, []byte(newContent), 0644); err != nil {
		tr.t.Fatalf("Failed to modify file %s: %v", filename, err)
	}
}

// CommitChanges commits all changes with the given message
func (tr *TestRepo) CommitChanges(message string) {
	tr.t.Helper()
	w, err := tr.Repo.Worktree()
	if err != nil {
		tr.t.Fatalf("Failed to get worktree: %v", err)
	}

	_, err = w.Add(".")
	if err != nil {
		tr.t.Fatalf("Failed to add files: %v", err)
	}

	_, err = w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})

	if err != nil {
		tr.t.Fatalf("Failed to commit: %v", err)
	}
}

// CreateAndCommitFile creates a file and commits it in one operation
func (tr *TestRepo) CreateAndCommitFile(filename, content, message string) {
	tr.t.Helper()
	tr.CreateFile(filename, content)
	tr.CommitChanges(message)
}

// GetStagedFiles returns a list of staged files
func (tr *TestRepo) GetStagedFiles() []string {
	tr.t.Helper()
	output, err := tr.RunCommand("git", "diff", "--cached", "--name-only")
	if err != nil {
		tr.t.Fatalf("Failed to get staged files: %v", err)
	}

	files := strings.Split(strings.TrimSpace(output), "\n")
	if len(files) == 1 && files[0] == "" {
		return []string{}
	}

	sort.Strings(files)
	return files
}

// GetCommitCount returns the number of commits in the repository
func (tr *TestRepo) GetCommitCount() int {
	tr.t.Helper()
	output, err := tr.RunCommand("git", "rev-list", "--count", "HEAD")
	if err != nil {
		tr.t.Fatalf("Failed to get commit count: %v", err)
	}

	count := 0
	if _, err := fmt.Sscanf(strings.TrimSpace(output), "%d", &count); err != nil {
		tr.t.Fatalf("Failed to parse commit count: %v", err)
	}

	return count
}

// GeneratePatch generates a patch file for the current changes
func (tr *TestRepo) GeneratePatch(filename string) {
	tr.t.Helper()
	output, err := tr.RunCommand("git", "diff", "HEAD")
	if err != nil {
		tr.t.Fatalf("Failed to generate patch: %v", err)
	}

	patchPath := filepath.Join(tr.Path, filename)
	if err := os.WriteFile(patchPath, []byte(output), 0644); err != nil {
		tr.t.Fatalf("Failed to write patch file: %v", err)
	}
}

// CreateLargeFileWithManyHunks creates a file with many functions for performance testing
func (tr *TestRepo) CreateLargeFileWithManyHunks() {
	tr.t.Helper()

	// Create initial version
	var initialContent strings.Builder
	initialContent.WriteString("#!/usr/bin/env python3\n\n")

	for i := 0; i < 20; i++ {
		initialContent.WriteString(GenerateFunction(i, "initial"))
	}

	filename := "large_module.py"
	tr.CreateFile(filename, initialContent.String())
	tr.CommitChanges("Initial large file")

	// Create modified version with changes in multiple hunks
	var modifiedContent strings.Builder
	modifiedContent.WriteString("#!/usr/bin/env python3\n\n")

	for i := 0; i < 20; i++ {
		if i%2 == 0 {
			// Modify even-numbered functions
			modifiedContent.WriteString(GenerateFunction(i, "modified"))
		} else {
			// Keep odd-numbered functions unchanged
			modifiedContent.WriteString(GenerateFunction(i, "initial"))
		}
	}

	tr.ModifyFile(filename, modifiedContent.String())
}

// RunCommand executes a command in the specified directory (legacy function for backward compatibility)
func RunCommand(t *testing.T, dir string, command string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// CreateAndCommitFile creates a file with the given content and commits it (legacy function for backward compatibility)
func CreateAndCommitFile(t *testing.T, dir string, repo *git.Repository, filename, content, message string) {
	t.Helper()
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	w, _ := repo.Worktree()
	w.Add(filename)

	_, err := w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})

	if err != nil {
		t.Fatal(err)
	}
}

// CreateTestRepo creates a temporary directory with an initialized git repository (legacy function for backward compatibility)
func CreateTestRepo(t *testing.T, prefix string) (string, *git.Repository, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", prefix)
	if err != nil {
		t.Fatal(err)
	}

	repo, err := git.PlainInit(tmpDir, false)
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, repo, cleanup
}

// SetupTestDir changes to the test directory and returns a cleanup function (legacy function for backward compatibility)
func SetupTestDir(t *testing.T, dir string) func() {
	t.Helper()

	originalDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	return func() {
		os.Chdir(originalDir)
	}
}

// CreateLargeFileWithManyHunks creates a file with many functions for performance testing (legacy function for backward compatibility)
func CreateLargeFileWithManyHunks(t *testing.T, tmpDir string, repo *git.Repository) {
	t.Helper()

	// Create initial version
	var initialContent strings.Builder
	initialContent.WriteString("#!/usr/bin/env python3\n\n")

	for i := 0; i < 20; i++ {
		initialContent.WriteString(GenerateFunction(i, "initial"))
	}

	filename := "large_module.py"
	if err := os.WriteFile(filename, []byte(initialContent.String()), 0644); err != nil {
		t.Fatal(err)
	}

	// Commit initial version
	w, _ := repo.Worktree()
	w.Add(filename)

	commitOptions := &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	}

	if _, err := w.Commit("Initial large file", commitOptions); err != nil {
		t.Fatal(err)
	}

	// Create modified version with changes in multiple hunks
	var modifiedContent strings.Builder
	modifiedContent.WriteString("#!/usr/bin/env python3\n\n")

	for i := 0; i < 20; i++ {
		if i%2 == 0 {
			// Modify even-numbered functions
			modifiedContent.WriteString(GenerateFunction(i, "modified"))
		} else {
			// Keep odd-numbered functions unchanged
			modifiedContent.WriteString(GenerateFunction(i, "initial"))
		}
	}

	if err := os.WriteFile(filename, []byte(modifiedContent.String()), 0644); err != nil {
		t.Fatal(err)
	}
}

// GenerateFunction generates a function with the given index and version
func GenerateFunction(index int, version string) string {
	return strings.ReplaceAll(strings.ReplaceAll(`def function_{INDEX}():
    """Function {INDEX} - {VERSION} version"""
    result = 0
    for i in range(10):
        result += i * {INDEX}
    print(f"Function {INDEX} result: {result}")
    return result

`, "{INDEX}", string(rune('0'+index))), "{VERSION}", version)
}
