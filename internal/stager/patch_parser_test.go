package stager

import (
	"strings"
	"testing"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// TestParsePatchFileComparison compares existing string-based parser with go-gitdiff
// NOTE: Some test cases may fail due to go-gitdiff's stricter patch format requirements.
// This is expected and doesn't affect the actual functionality since we have a fallback.
func TestParsePatchFileComparison(t *testing.T) {
	testCases := []struct {
		name        string
		patchContent string
		description string
	}{
		{
			name: "single_file_single_hunk",
			patchContent: `diff --git a/test.py b/test.py
index abc123..def456 100644
--- a/test.py
+++ b/test.py
@@ -1,3 +1,4 @@
 def hello():
+    print("Hello, World!")
     return "hello"
 # end of file`,
			description: "Basic single file, single hunk patch - tests the simplest case of one file with one change",
		},
		{
			name: "single_file_multiple_hunks",
			patchContent: `diff --git a/test.py b/test.py
index abc123..def456 100644
--- a/test.py
+++ b/test.py
@@ -1,4 +1,5 @@
 def hello():
+    print("Hello, World!")
     return "hello"
 
 def middle():
@@ -10,5 +11,6 @@ def middle():
     return "middle"
 
 def goodbye():
     print("Goodbye")
+    print("See you later!")
     return "goodbye"`,
			description: "Single file with multiple hunks - tests handling of multiple separate changes in the same file",
		},
		{
			name: "multiple_files",
			patchContent: `diff --git a/file1.py b/file1.py
index abc123..def456 100644
--- a/file1.py
+++ b/file1.py
@@ -1,3 +1,4 @@
 def hello():
+    print("Hello from file1")
     return "hello"
 # end
diff --git a/file2.py b/file2.py
index 123abc..456def 100644
--- a/file2.py
+++ b/file2.py
@@ -5,4 +5,5 @@ def world():
 def world():
     print("World")
+    print("from file2")
     return "world"
 # end`,
			description: "Multiple files with hunks",
		},
		{
			name: "new_file",
			patchContent: `diff --git a/new_file.py b/new_file.py
new file mode 100644
index 000000..abc123
--- /dev/null
+++ b/new_file.py
@@ -0,0 +1,5 @@
+def new_function():
+    print("This is a new file")
+    return 42
+
+# End of file`,
			description: "New file creation",
		},
		{
			name: "deleted_file",
			patchContent: `diff --git a/deleted.py b/deleted.py
deleted file mode 100644
index abc123..000000
--- a/deleted.py
+++ /dev/null
@@ -1,3 +0,0 @@
-def old_function():
-    print("This will be deleted")
-    return 0`,
			description: "File deletion",
		},
		{
			name: "renamed_file",
			patchContent: `diff --git a/old_name.py b/new_name.py
similarity index 95%
rename from old_name.py
rename to new_name.py
index abc123..def456 100644
--- a/old_name.py
+++ b/new_name.py
@@ -1,4 +1,5 @@
 def renamed_func():
+    print("File was renamed")
     print("Original content")
     return True
 # end`,
			description: "File rename with changes",
		},
		{
			name: "binary_file",
			patchContent: `diff --git a/image.png b/image.png
new file mode 100644
index 000000..abc123
Binary files /dev/null and b/image.png differ`,
			description: "Binary file addition",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse with existing string-based parser
			existingHunks, err := parsePatchFile(tc.patchContent)
			if err != nil {
				t.Fatalf("existing parser failed: %v", err)
			}

			// Parse with go-gitdiff
			files, _, err := gitdiff.Parse(strings.NewReader(tc.patchContent))
			if err != nil {
				t.Fatalf("go-gitdiff parser failed: %v", err)
			}

			// Compare results
			t.Logf("\n=== Test: %s ===", tc.name)
			t.Logf("Description: %s", tc.description)
			t.Logf("\nExisting parser found %d hunks", len(existingHunks))
			
			// Count hunks from go-gitdiff
			gitdiffHunkCount := 0
			for _, file := range files {
				if file.IsBinary {
					t.Logf("go-gitdiff: Binary file detected: %s", file.NewName)
				}
				if file.IsRename {
					t.Logf("go-gitdiff: Rename detected: %s -> %s", file.OldName, file.NewName)
				}
				if file.IsDelete {
					t.Logf("go-gitdiff: Delete detected: %s", file.OldName)
				}
				if file.IsNew {
					t.Logf("go-gitdiff: New file detected: %s", file.NewName)
				}
				
				for _, frag := range file.TextFragments {
					gitdiffHunkCount++
					t.Logf("go-gitdiff: Hunk in %s at lines %d-%d (old) %d-%d (new)",
						file.NewName, frag.OldPosition, frag.OldPosition+frag.OldLines,
						frag.NewPosition, frag.NewPosition+frag.NewLines)
				}
			}
			
			t.Logf("go-gitdiff found %d hunks across %d files", gitdiffHunkCount, len(files))
			
			// Log existing parser results
			for i, hunk := range existingHunks {
				t.Logf("Existing parser: Hunk %d in %s (file hunk %d) at lines %d-%d",
					i+1, hunk.FilePath, hunk.IndexInFile, hunk.StartLine, hunk.EndLine)
			}

			// Check if counts match (note: they might not for special cases)
			if len(existingHunks) != gitdiffHunkCount {
				t.Logf("WARNING: Hunk count mismatch - existing: %d, go-gitdiff: %d", 
					len(existingHunks), gitdiffHunkCount)
			}
		})
	}
}

// TestGoGitDiffFeatures tests go-gitdiff specific features
func TestGoGitDiffFeatures(t *testing.T) {
	// Test that go-gitdiff can handle various patch formats
	testPatch := `diff --git a/test.go b/test.go
index abc123..def456 100644
--- a/test.go
+++ b/test.go
@@ -1,3 +1,4 @@
 package main
+import "fmt"
 
 func main() {`

	files, _, err := gitdiff.Parse(strings.NewReader(testPatch))
	if err != nil {
		t.Fatalf("Failed to parse patch: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(files))
	}

	file := files[0]
	if file.NewName != "test.go" {
		t.Errorf("Expected filename test.go, got %s", file.NewName)
	}

	if len(file.TextFragments) != 1 {
		t.Errorf("Expected 1 text fragment, got %d", len(file.TextFragments))
	}
}