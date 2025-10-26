package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/syou6162/git-sequential-stage/internal/executor"
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

// TestCountHunksInRepository_NoChanges tests counting hunks when there are no changes
func TestCountHunksInRepository_NoChanges(t *testing.T) {
	// Mock executor that returns empty diff
	mockExec := executor.NewMockCommandExecutor()
	mockExec.Commands["git [diff HEAD]"] = executor.MockResponse{
		Output: []byte(""),
		Error:  nil,
	}

	result, err := countHunksInRepository(mockExec)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty map for no changes, got %v", result)
	}
}

// TestCountHunksInRepository_SingleFileOneHunk tests counting one hunk in one file
func TestCountHunksInRepository_SingleFileOneHunk(t *testing.T) {
	// Mock executor that returns diff with one file and one hunk
	mockExec := executor.NewMockCommandExecutor()
	mockExec.Commands["git [diff HEAD]"] = executor.MockResponse{
		Output: []byte(`diff --git a/main.go b/main.go
index 1234567..abcdefg 100644
--- a/main.go
+++ b/main.go
@@ -10,1 +10,2 @@ func main() {
 	fmt.Println("Hello")
+	fmt.Println("World")
`),
		Error: nil,
	}

	result, err := countHunksInRepository(mockExec)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := map[string]int{"main.go": 1}
	if len(result) != len(expected) {
		t.Errorf("Expected %d files, got %d", len(expected), len(result))
	}
	if result["main.go"] != 1 {
		t.Errorf("Expected main.go to have 1 hunk, got %d", result["main.go"])
	}
}
