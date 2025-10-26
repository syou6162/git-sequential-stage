package stager

import (
	"testing"
)

// TestCountHunksInDiff_NoChanges tests counting hunks when diff is empty
func TestCountHunksInDiff_NoChanges(t *testing.T) {
	result, err := CountHunksInDiff("")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty map for empty diff, got %v", result)
	}
}

// TestCountHunksInDiff_SingleFileMultipleHunks tests counting multiple hunks in one file
func TestCountHunksInDiff_SingleFileMultipleHunks(t *testing.T) {
	diffOutput := `diff --git a/calculator.go b/calculator.go
index 1234567..abcdefg 100644
--- a/calculator.go
+++ b/calculator.go
@@ -1,3 +1,4 @@
 package main
+import "fmt"

 func add() {
@@ -10,2 +11,3 @@ func multiply() {
 	return 0
+	// comment
 }
`

	result, err := CountHunksInDiff(diffOutput)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result["calculator.go"] != 2 {
		t.Errorf("Expected calculator.go to have 2 hunks, got %d", result["calculator.go"])
	}
}

// TestCountHunksInDiff_MultipleFiles tests counting hunks across multiple files
func TestCountHunksInDiff_MultipleFiles(t *testing.T) {
	diffOutput := `diff --git a/file1.go b/file1.go
index 1234567..abcdefg 100644
--- a/file1.go
+++ b/file1.go
@@ -1,1 +1,2 @@ func test1() {
 	println("test1")
+	println("modified")
 }
diff --git a/file2.go b/file2.go
index 2234567..bbcdefg 100644
--- a/file2.go
+++ b/file2.go
@@ -1,1 +1,2 @@ func test2() {
 	println("test2")
+	println("modified")
 }
`

	result, err := CountHunksInDiff(diffOutput)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := map[string]int{
		"file1.go": 1,
		"file2.go": 1,
	}

	if len(result) != len(expected) {
		t.Errorf("Expected %d files, got %d", len(expected), len(result))
	}

	for file, expectedCount := range expected {
		if result[file] != expectedCount {
			t.Errorf("Expected %s to have %d hunks, got %d", file, expectedCount, result[file])
		}
	}
}

// TestCountHunksInDiff_ParseError tests error handling when diff parsing fails
// Note: ParsePatchFileWithGitDiff has fallback mechanism, so it rarely returns errors.
// This test uses completely invalid input to trigger a parse error.
func TestCountHunksInDiff_ParseError(t *testing.T) {
	diffOutput := `diff --git a/file.go b/file.go
--- a/file.go
+++ b/file.go
@@ invalid header format
corrupted content
`

	result, err := CountHunksInDiff(diffOutput)

	// If parser has robust fallback, it might succeed with 0 hunks
	// Either error or empty result is acceptable
	if err != nil {
		expectedMsg := "failed to parse diff"
		if !stringContains(err.Error(), expectedMsg) {
			t.Errorf("Expected error message to contain %q, got %q", expectedMsg, err.Error())
		}
	} else if len(result) != 0 {
		t.Errorf("Expected empty result for malformed diff, got %v", result)
	}
}

// Helper function to check if a string contains a substring
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
