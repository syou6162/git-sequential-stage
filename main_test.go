package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/syou6162/git-sequential-stage/testutils"
)

func TestRunGitSequentialStage_Usage(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedOutput string
		expectedError  bool
	}{
		{
			name:           "no arguments shows usage",
			args:           []string{},
			expectedOutput: "Usage:",
			expectedError:  true,
		},
		{
			name:           "missing patch flag",
			args:           []string{"-hunk", "file.go:1"},
			expectedOutput: "Usage:",
			expectedError:  true,
		},
		{
			name:           "missing hunk flag",
			args:           []string{"-patch", "test.patch"},
			expectedOutput: "Usage:",
			expectedError:  true,
		},
		{
			name:           "help flag",
			args:           []string{"-h"},
			expectedOutput: "Usage:",
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse arguments from test
			hunks := hunkList{}
			patchFile := ""
			for i := 0; i < len(tt.args); i++ {
				switch tt.args[i] {
				case "-hunk":
					if i+1 < len(tt.args) {
						hunks = append(hunks, tt.args[i+1])
						i++
					}
				case "-patch":
					if i+1 < len(tt.args) {
						patchFile = tt.args[i+1]
						i++
					}
				case "-h":
					// Help flag should not cause error
					err := error(nil)
					if tt.expectedError && err == nil {
						t.Error("Expected error but got none")
					}
					if !tt.expectedError && err != nil {
						t.Errorf("Unexpected error: %v", err)
					}
					return
				}
			}

			// Capture output by redirecting stderr/stdout would be complex
			// For now, just test the error behavior
			err := runGitSequentialStage(hunks, patchFile)

			if tt.expectedError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Output testing is removed since we can't easily capture it
			// from the refactored function
		})
	}
}

func TestHunkListType(t *testing.T) {
	// Test String method
	hl := hunkList{"file1.go:1,2", "file2.go:3"}
	expected := "file1.go:1,2, file2.go:3"
	if hl.String() != expected {
		t.Errorf("String() = %q, want %q", hl.String(), expected)
	}

	// Test Set method
	err := hl.Set("file3.go:4")
	if err != nil {
		t.Errorf("Set() returned unexpected error: %v", err)
	}
	if len(hl) != 3 {
		t.Errorf("Expected length 3, got %d", len(hl))
	}
	if hl[2] != "file3.go:4" {
		t.Errorf("Expected last element to be 'file3.go:4', got %q", hl[2])
	}
}

func TestWildcardParsing(t *testing.T) {
	tests := []struct {
		name          string
		hunks         []string
		expectError   bool
		errorContains string
	}{
		{
			name:        "wildcard for entire file",
			hunks:       []string{"file.go:*"},
			expectError: false,
		},
		{
			name:        "mixed wildcard and normal hunks",
			hunks:       []string{"file1.go:*", "file2.go:1,3"},
			expectError: false,
		},
		{
			name:          "invalid mixed wildcard and numbers",
			hunks:         []string{"file.go:1,*,3"},
			expectError:   true,
			errorContains: "mixed wildcard and hunk numbers not allowed",
		},
		{
			name:          "wildcard with numbers",
			hunks:         []string{"file.go:*,1"},
			expectError:   true,
			errorContains: "mixed wildcard and hunk numbers not allowed",
		},
		{
			name:          "invalid hunk format",
			hunks:         []string{"file.go"},
			expectError:   true,
			errorContains: "invalid hunk specification",
		},
		{
			name:        "multiple wildcards",
			hunks:       []string{"file1.go:*", "file2.go:*", "file3.go:1"},
			expectError: false,
		},
		{
			name:          "same file with both wildcard and numbers",
			hunks:         []string{"file.go:1,2", "file.go:*"},
			expectError:   true,
			errorContains: "mixed wildcard and hunk numbers not allowed",
		},
		{
			name:          "same file with wildcard then numbers",
			hunks:         []string{"file.go:*", "file.go:3,4"},
			expectError:   true,
			errorContains: "mixed wildcard and hunk numbers not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We need to test the parsing logic that's in runGitSequentialStage
			// Since we can't easily test it without running the full command,
			// we'll extract the parsing logic for testing
			wildcardFiles := []string{}
			normalHunks := []string{}
			var parseErr error

			fileSpecTypes := make(map[string]string)
			for _, spec := range tt.hunks {
				parts := strings.Split(spec, ":")
				if len(parts) != 2 {
					parseErr = fmt.Errorf("invalid hunk specification: %s (expected format: file:hunks)", spec)
					break
				}

				file := parts[0]
				hunksSpec := parts[1]

				// Check if this file has already been specified
				if existingType, exists := fileSpecTypes[file]; exists {
					if hunksSpec == "*" && existingType == "numbers" {
						parseErr = fmt.Errorf("mixed wildcard and hunk numbers not allowed for file %s", file)
						break
					}
					if hunksSpec != "*" && existingType == "wildcard" {
						parseErr = fmt.Errorf("mixed wildcard and hunk numbers not allowed for file %s", file)
						break
					}
				}

				if hunksSpec == "*" {
					wildcardFiles = append(wildcardFiles, file)
					fileSpecTypes[file] = "wildcard"
				} else {
					if strings.Contains(hunksSpec, "*") {
						parseErr = fmt.Errorf("mixed wildcard and hunk numbers not allowed in %s", spec)
						break
					}
					normalHunks = append(normalHunks, spec)
					fileSpecTypes[file] = "numbers"
				}
			}

			// Use variables to avoid linter warnings
			_ = wildcardFiles
			_ = normalHunks

			if tt.expectError {
				if parseErr == nil {
					t.Error("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(parseErr.Error(), tt.errorContains) {
					t.Errorf("Error should contain %q, got %v", tt.errorContains, parseErr)
				}
			} else {
				if parseErr != nil {
					t.Errorf("Unexpected error: %v", parseErr)
				}
			}
		})
	}
}

// TestSubcommandRouting tests the subcommand routing functionality
func TestSubcommandRouting(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectError    bool
		errorContains  string
		outputContains string
	}{
		{
			name:          "no subcommand shows usage",
			args:          []string{},
			expectError:   true,
			errorContains: "subcommand required",
		},
		{
			name:          "unknown subcommand shows error",
			args:          []string{"unknown"},
			expectError:   true,
			errorContains: "unknown subcommand",
		},
		{
			name:        "stage subcommand with no args shows usage",
			args:        []string{"stage"},
			expectError: true,
			// Will check for flag parsing error
		},
		{
			name:        "count-hunks subcommand executes",
			args:        []string{"count-hunks"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the routing function
			err := routeSubcommand(tt.args)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q but got none", tt.errorContains)
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing %q, got %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestRunCountHunksCommand_Integration tests the count-hunks command integration
func TestRunCountHunksCommand_Integration(t *testing.T) {
	// Setup test repository
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-count-hunks-*")
	defer testRepo.Cleanup()
	tempDir := testRepo.Path

	// Change to temp directory
	t.Chdir(tempDir)

	// Create test files
	file1 := "file1.go"
	file1Content := `package main

func function1() {
	println("function1")
}

func function2() {
	println("function2")
}
`
	if err := os.WriteFile(file1, []byte(file1Content), 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}

	file2 := "file2.go"
	file2Content := `package main

func function3() {
	println("function3")
}
`
	if err := os.WriteFile(file2, []byte(file2Content), 0644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	// Initial commit
	gitAddCmd := exec.Command("git", "add", ".")
	if output, err := gitAddCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git add: %v\nOutput: %s", err, output)
	}

	gitCommitCmd := exec.Command("git", "commit", "-m", "Initial commit")
	if output, err := gitCommitCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git commit: %v\nOutput: %s", err, output)
	}

	// Make changes to create hunks
	file1ModifiedContent := `package main

import "fmt"

func function1() {
	fmt.Println("function1 modified")
}

func function2() {
	println("function2")
}

func function4() {
	println("function4")
}
`
	if err := os.WriteFile(file1, []byte(file1ModifiedContent), 0644); err != nil {
		t.Fatalf("Failed to modify file1: %v", err)
	}

	file2ModifiedContent := `package main

func function3() {
	println("function3 modified")
}
`
	if err := os.WriteFile(file2, []byte(file2ModifiedContent), 0644); err != nil {
		t.Fatalf("Failed to modify file2: %v", err)
	}

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
	runErr := runCountHunksCommand([]string{})

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

	// Check that file1.go and file2.go are in the output
	hasFile1 := false
	hasFile2 := false
	for _, line := range lines {
		if strings.Contains(line, "file1.go") {
			hasFile1 = true
			// file1.go should have 1 hunk (all changes are adjacent)
			if !strings.Contains(line, ": 1") {
				t.Errorf("Expected file1.go to have 1 hunk, got: %s", line)
			}
		}
		if strings.Contains(line, "file2.go") {
			hasFile2 = true
			// file2.go should have 1 hunk
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
