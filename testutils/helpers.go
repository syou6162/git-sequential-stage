package testutils

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// RunCommand executes a command in the specified directory
func RunCommand(t *testing.T, dir string, command string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// CreateAndCommitFile creates a file with the given content and commits it
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

// CreateTestRepo creates a temporary directory with an initialized git repository
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

// SetupTestDir changes to the test directory and returns a cleanup function
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

// CreateLargeFileWithManyHunks creates a file with many functions for performance testing
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
