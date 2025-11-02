package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/syou6162/git-sequential-stage/testutils"
)

// formatRegex is used to validate output format: "filename: count"
var formatRegex = regexp.MustCompile(`^[^:]+: (\d+|\*)$`)

// TestE2E_CountHunks_NoChanges tests count-hunks with no working tree changes
func TestE2E_CountHunks_NoChanges(t *testing.T) {
	testRepo := testutils.NewTestRepo(t, "count-hunks-nochanges-*")
	defer testRepo.Cleanup()

	// Create initial commit
	testRepo.CreateFile("file1.go", "package main\n")
	testRepo.CommitChanges("Initial commit")

	// Change to test repo directory
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(testRepo.Path)

	// Run count-hunks command
	err := runCountHunksCommand(context.Background(), []string{})
	if err != nil {
		t.Fatalf("count-hunks failed: %v", err)
	}

	// With no changes, should output nothing (empty result is valid)
}

// TestE2E_CountHunks_BasicIntegration tests count-hunks with real git changes
func TestE2E_CountHunks_BasicIntegration(t *testing.T) {
	testRepo := testutils.NewTestRepo(t, "count-hunks-basic-*")
	defer testRepo.Cleanup()

	// Create initial files
	testRepo.CreateFile("file1.go", `package main

func main() {
}
`)
	testRepo.CreateFile("file2.go", `package main

func test() {
}
`)
	testRepo.CommitChanges("Initial commit")

	// Make changes to create hunks
	testRepo.CreateFile("file1.go", `package main

import "fmt"

func main() {
	fmt.Println("Hello")
}
`)
	testRepo.CreateFile("file2.go", `package main

import "fmt"

func test() {
	fmt.Println("Test")
}
`)

	// Change to test repo directory
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(testRepo.Path)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = w

	outCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, r); err != nil {
			t.Errorf("Failed to copy output: %v", err)
		}
		outCh <- buf.String()
	}()

	// Run count-hunks command
	runErr := runCountHunksCommand(context.Background(), []string{})

	// Close write end and restore stdout
	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close pipe: %v", err)
	}
	os.Stdout = oldStdout

	// Get captured output
	output := <-outCh

	// Check for errors
	if runErr != nil {
		t.Fatalf("runCountHunksCommand failed: %v", runErr)
	}

	// Verify output
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines of output, got %d:\n%s", len(lines), output)
	}

	// Verify sort order (alphabetically)
	if len(lines) >= 2 {
		if !strings.HasPrefix(lines[0], "file1.go:") {
			t.Errorf("Expected first line to start with 'file1.go:', got: %s", lines[0])
		}
		if !strings.HasPrefix(lines[1], "file2.go:") {
			t.Errorf("Expected second line to start with 'file2.go:', got: %s", lines[1])
		}
	}

	// Verify format: "filename: count"
	for i, line := range lines {
		if !formatRegex.MatchString(line) {
			t.Errorf("Line %d does not match expected format 'filename: count', got: %s", i+1, line)
		}
	}

	// Check specific content - both files should have 1 hunk
	hasFile1 := false
	hasFile2 := false
	for _, line := range lines {
		if strings.Contains(line, "file1.go") {
			hasFile1 = true
			if !strings.Contains(line, ": 1") {
				t.Errorf("Expected file1.go to have 1 hunk, got: %s", line)
			}
		}
		if strings.Contains(line, "file2.go") {
			hasFile2 = true
			if !strings.Contains(line, ": 1") {
				t.Errorf("Expected file2.go to have 1 hunk, got: %s", line)
			}
		}
	}

	if !hasFile1 {
		t.Errorf("Output does not contain file1.go:\n%s", output)
	}
	if !hasFile2 {
		t.Errorf("Output does not contain file2.go:\n%s", output)
	}
}

// TestE2E_CountHunks_BinaryFiles tests count-hunks with binary files
func TestE2E_CountHunks_BinaryFiles(t *testing.T) {
	testRepo := testutils.NewTestRepo(t, "count-hunks-binary-*")
	defer testRepo.Cleanup()

	// Create initial files
	textFile := "text.go"
	textContent := `package main

func main() {
}
`
	testRepo.CreateFile(textFile, textContent)

	// Create binary file
	binaryFile := "image.png"
	binaryContent := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}
	if err := os.WriteFile(filepath.Join(testRepo.Path, binaryFile), binaryContent, 0644); err != nil {
		t.Fatalf("Failed to write binary file: %v", err)
	}

	testRepo.CommitChanges("Initial commit")

	// Make changes
	textModified := `package main

import "fmt"

func main() {
	fmt.Println("Hello")
}
`
	testRepo.CreateFile(textFile, textModified)

	// Modify binary file
	binaryModified := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0xff, 0xff, 0xff, 0xff}
	if err := os.WriteFile(filepath.Join(testRepo.Path, binaryFile), binaryModified, 0644); err != nil {
		t.Fatalf("Failed to modify binary file: %v", err)
	}

	// Change to test repo directory
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(testRepo.Path)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = w

	outCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, r); err != nil {
			t.Errorf("Failed to copy output: %v", err)
		}
		outCh <- buf.String()
	}()

	// Run count-hunks command
	runErr := runCountHunksCommand(context.Background(), []string{})

	// Close write end and restore stdout
	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close pipe: %v", err)
	}
	os.Stdout = oldStdout

	// Get captured output
	output := <-outCh

	// Check for errors
	if runErr != nil {
		t.Fatalf("runCountHunksCommand failed: %v", runErr)
	}

	// Verify output
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines of output, got %d:\n%s", len(lines), output)
	}

	// Verify sort order (alphabetically: image.png, text.go)
	if len(lines) >= 2 {
		if !strings.HasPrefix(lines[0], "image.png:") {
			t.Errorf("Expected first line to start with 'image.png:', got: %s", lines[0])
		}
		if !strings.HasPrefix(lines[1], "text.go:") {
			t.Errorf("Expected second line to start with 'text.go:', got: %s", lines[1])
		}
	}

	// Verify binary file shows "*" and text file shows count
	hasBinary := false
	hasText := false
	for _, line := range lines {
		if strings.Contains(line, "image.png") {
			hasBinary = true
			// Binary file should show "*"
			if !strings.Contains(line, ": *") {
				t.Errorf("Expected image.png to show '*', got: %s", line)
			}
		}
		if strings.Contains(line, "text.go") {
			hasText = true
			// Text file should show number
			if !strings.Contains(line, ": 1") {
				t.Errorf("Expected text.go to show '1', got: %s", line)
			}
		}
	}

	if !hasBinary {
		t.Errorf("Output does not contain image.png:\n%s", output)
	}
	if !hasText {
		t.Errorf("Output does not contain text.go:\n%s", output)
	}
}
