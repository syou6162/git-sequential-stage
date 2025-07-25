package stager

import (
	"strings"
	"testing"

	"github.com/syou6162/git-sequential-stage/internal/executor"
)

// Test helper functions

// parseAndValidateHunk is a helper to parse patch file and validate hunk extraction
func parseAndValidateHunk(t *testing.T, patchContent string, hunkIndex int) HunkInfo {
	t.Helper()

	hunks, err := ParsePatchFileWithGitDiff(patchContent)
	if err != nil {
		t.Fatalf("Failed to parse patch: %v", err)
	}

	if hunkIndex >= len(hunks) {
		t.Fatalf("Hunk index %d out of range, only %d hunks found", hunkIndex, len(hunks))
	}

	return hunks[hunkIndex]
}

// assertHunkProperties validates common hunk properties
func assertHunkProperties(t *testing.T, hunk HunkInfo, expectedFile string, expectedGlobalIndex, expectedIndexInFile int) {
	t.Helper()

	if hunk.FilePath != expectedFile {
		t.Errorf("Expected file path %q, got %q", expectedFile, hunk.FilePath)
	}
	if hunk.GlobalIndex != expectedGlobalIndex {
		t.Errorf("Expected GlobalIndex %d, got %d", expectedGlobalIndex, hunk.GlobalIndex)
	}
	if hunk.IndexInFile != expectedIndexInFile {
		t.Errorf("Expected IndexInFile %d, got %d", expectedIndexInFile, hunk.IndexInFile)
	}
}

// TestExtractFileDiff tests extracting the entire file diff using File.String().
// This is crucial for handling new file creation where the entire file content
// must be staged as one unit.
func TestExtractFileDiff(t *testing.T) {
	tests := []struct {
		name           string
		patchContent   string
		hunkIndex      int // which hunk to test (0-based)
		expectedOutput string
	}{
		{
			name: "single new file",
			patchContent: `diff --git a/new_file.go b/new_file.go
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/new_file.go
@@ -0,0 +1,3 @@
+package main
+
+func main() {}`,
			hunkIndex: 0,
			expectedOutput: `diff --git a/new_file.go b/new_file.go
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/new_file.go
@@ -0,0 +1,3 @@
+package main
+
+func main() {}`,
		},
		{
			name: "multiple new files - extract first",
			patchContent: `diff --git a/file1.go b/file1.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/file1.go
@@ -0,0 +1,2 @@
+package main
+func main() {}
diff --git a/file2.go b/file2.go
new file mode 100644
index 0000000..def5678
--- /dev/null
+++ b/file2.go
@@ -0,0 +1,1 @@
+package test`,
			hunkIndex: 0, // first file
			expectedOutput: `diff --git a/file1.go b/file1.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/file1.go
@@ -0,0 +1,2 @@
+package main
+func main() {}`,
		},
		{
			name: "multiple new files - extract second",
			patchContent: `diff --git a/file1.go b/file1.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/file1.go
@@ -0,0 +1,2 @@
+package main
+func main() {}
diff --git a/file2.go b/file2.go
new file mode 100644
index 0000000..def5678
--- /dev/null
+++ b/file2.go
@@ -0,0 +1,1 @@
+package test`,
			hunkIndex: 1, // second file
			expectedOutput: `diff --git a/file2.go b/file2.go
new file mode 100644
index 0000000..def5678
--- /dev/null
+++ b/file2.go
@@ -0,0 +1,1 @@
+package test`,
		},
		{
			name: "new file with mixed patch",
			patchContent: `diff --git a/existing.go b/existing.go
index abc1234..def5678 100644
--- a/existing.go
+++ b/existing.go
@@ -1,3 +1,4 @@
 package main

+import "fmt"
 func main() {}
diff --git a/new_file.go b/new_file.go
new file mode 100644
index 0000000..999888
--- /dev/null
+++ b/new_file.go
@@ -0,0 +1,1 @@
+package new`,
			hunkIndex: 1, // new file (second hunk)
			expectedOutput: `diff --git a/new_file.go b/new_file.go
new file mode 100644
index 0000000..999888
--- /dev/null
+++ b/new_file.go
@@ -0,0 +1,1 @@
+package new`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use helper function to parse and validate
			hunk := parseAndValidateHunk(t, tt.patchContent, tt.hunkIndex)

			// Call the function under test
			var result []byte
			if hunk.File != nil {
				result = []byte(hunk.File.String())
			}

			// Compare results
			resultStr := string(result)
			// Normalize the output for comparison (handle trailing newlines and "\ No newline at end of file")
			expectedNorm := strings.TrimSpace(tt.expectedOutput)
			resultNorm := strings.TrimSpace(resultStr)
			// Remove "\ No newline at end of file" markers for comparison
			resultNorm = strings.ReplaceAll(resultNorm, "\n\\ No newline at end of file", "")

			if expectedNorm != resultNorm {
				t.Errorf("File.String() result mismatch\nExpected:\n%s\n\nGot:\n%s", tt.expectedOutput, resultStr)
			}
		})
	}
}

// TestIsNewFile tests that go-gitdiff correctly identifies new files.
// This distinction is important because new files require different handling
// than modifications to existing files.
func TestIsNewFile(t *testing.T) {
	tests := []struct {
		name         string
		patchContent string
		hunkIndex    int
		expected     bool
	}{
		{
			name: "new file hunk with @@ -0,0",
			patchContent: `diff --git a/new_file.go b/new_file.go
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/new_file.go
@@ -0,0 +1,3 @@
+package main
+
+func main() {}`,
			hunkIndex: 0,
			expected:  true,
		},
		{
			name: "regular file modification hunk",
			patchContent: `diff --git a/existing.go b/existing.go
index abc1234..def5678 100644
--- a/existing.go
+++ b/existing.go
@@ -1,3 +1,4 @@
 package main

+import "fmt"
 func main() {}`,
			hunkIndex: 0,
			expected:  false,
		},
		{
			name: "new file hunk with different format",
			patchContent: `diff --git a/test.txt b/test.txt
new file mode 100644
index 0000000..e69de29
--- /dev/null
+++ b/test.txt
@@ -0,0 +1 @@
+Hello world`,
			hunkIndex: 0,
			expected:  true,
		},
		{
			name: "file deletion hunk",
			patchContent: `diff --git a/deleted.go b/deleted.go
deleted file mode 100644
index 1234567..0000000
--- a/deleted.go
+++ /dev/null
@@ -1,3 +0,0 @@
-package main
-
-func main() {}`,
			hunkIndex: 0,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use helper function to parse and validate
			hunk := parseAndValidateHunk(t, tt.patchContent, tt.hunkIndex)

			result := hunk.File != nil && hunk.File.IsNew

			if result != tt.expected {
				t.Errorf("File.IsNew = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestParsePatchFile_NewFiles tests the parsePatchFile function specifically for
// new file creation scenarios. It verifies that the parser correctly identifies
// new files, assigns proper hunk indices, and maintains the correct file-to-hunk
// relationships in both single and multi-file patches.
func TestParsePatchFile_NewFiles(t *testing.T) {
	tests := []struct {
		name          string
		patchContent  string
		expectedHunks int
		expectedFiles []string
		validateHunks func(t *testing.T, hunks []HunkInfo)
	}{
		{
			name: "single new file",
			patchContent: `diff --git a/new_file.go b/new_file.go
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/new_file.go
@@ -0,0 +1,3 @@
+package main
+
+func main() {}`,
			expectedHunks: 1,
			expectedFiles: []string{"new_file.go"},
			validateHunks: func(t *testing.T, hunks []HunkInfo) {
				assertHunkProperties(t, hunks[0], "new_file.go", 1, 1)
			},
		},
		{
			name: "multiple new files",
			patchContent: `diff --git a/file1.go b/file1.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/file1.go
@@ -0,0 +1,2 @@
+package main
+func main() {}
diff --git a/file2.go b/file2.go
new file mode 100644
index 0000000..def5678
--- /dev/null
+++ b/file2.go
@@ -0,0 +1,1 @@
+package test`,
			expectedHunks: 2,
			expectedFiles: []string{"file1.go", "file2.go"},
			validateHunks: func(t *testing.T, hunks []HunkInfo) {
				// Validate both hunks using helper function
				assertHunkProperties(t, hunks[0], "file1.go", 1, 1)
				assertHunkProperties(t, hunks[1], "file2.go", 2, 1)
			},
		},
		{
			name: "mixed new and existing files",
			patchContent: `diff --git a/existing.go b/existing.go
index abc1234..def5678 100644
--- a/existing.go
+++ b/existing.go
@@ -1,3 +1,4 @@
 package main

+import "fmt"
 func main() {}
diff --git a/new_file.go b/new_file.go
new file mode 100644
index 0000000..999888
--- /dev/null
+++ b/new_file.go
@@ -0,0 +1,1 @@
+package new`,
			expectedHunks: 2,
			expectedFiles: []string{"existing.go", "new_file.go"},
			validateHunks: func(t *testing.T, hunks []HunkInfo) {
				// Existing file hunk
				if hunks[0].FilePath != "existing.go" {
					t.Errorf("First hunk file path: expected 'existing.go', got '%s'", hunks[0].FilePath)
				}
				// New file hunk
				if hunks[1].FilePath != "new_file.go" {
					t.Errorf("Second hunk file path: expected 'new_file.go', got '%s'", hunks[1].FilePath)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hunks, err := ParsePatchFileWithGitDiff(tt.patchContent)
			if err != nil {
				t.Fatalf("Failed to parse patch: %v", err)
			}

			if len(hunks) != tt.expectedHunks {
				t.Errorf("Expected %d hunks, got %d", tt.expectedHunks, len(hunks))
			}

			// Check file paths
			for i, expectedFile := range tt.expectedFiles {
				if i >= len(hunks) {
					t.Errorf("Expected file[%d] = %s, but only %d hunks found", i, expectedFile, len(hunks))
					continue
				}
				if hunks[i].FilePath != expectedFile {
					t.Errorf("Expected file[%d] = %s, got %s", i, expectedFile, hunks[i].FilePath)
				}
			}

			// Run custom validation with helper assertions
			if tt.validateHunks != nil {
				tt.validateHunks(t, hunks)
			}
		})
	}
}

// TestStager_ExtractHunkContent_NewFile tests the extractHunkContent method of Stager
// for both new file creation and existing file modification scenarios.
// Since we now use go-gitdiff for all parsing.
func TestStager_ExtractHunkContent_NewFile(t *testing.T) {
	tests := []struct {
		name         string
		patchContent string
		hunkIndex    int
		expectError  bool
		expectedLen  int // expected minimum length of result
	}{
		{
			name: "new file extraction",
			patchContent: `diff --git a/new_file.go b/new_file.go
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/new_file.go
@@ -0,0 +1,3 @@
+package main
+
+func main() {}`,
			hunkIndex:   0,
			expectError: false,
			expectedLen: 100, // Approximate minimum length for a new file patch
		},
		{
			name: "existing file modification",
			patchContent: `diff --git a/existing.go b/existing.go
index abc1234..def5678 100644
--- a/existing.go
+++ b/existing.go
@@ -1,3 +1,4 @@
 package main

+import "fmt"
 func main() {}`,
			hunkIndex:   0,
			expectError: false,
			expectedLen: 50, // Approximate minimum length for a hunk patch
		},
		{
			name: "binary file",
			patchContent: `diff --git a/image.png b/image.png
new file mode 100644
index 0000000..abc123
Binary files /dev/null and b/image.png differ`,
			hunkIndex:   0,
			expectError: false,
			expectedLen: 50, // Binary files return the full diff
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// No mocking needed since we use go-gitdiff parsing
			stager := NewStager(executor.NewRealCommandExecutor())

			// Use helper to parse and validate
			hunk := parseAndValidateHunk(t, tt.patchContent, tt.hunkIndex)

			// Call the method under test
			result, err := stager.extractHunkContent(&hunk, "/tmp/test.patch")

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError && len(result) < tt.expectedLen {
				t.Errorf("Result too short: expected at least %d bytes, got %d", tt.expectedLen, len(result))
			}
		})
	}
}
