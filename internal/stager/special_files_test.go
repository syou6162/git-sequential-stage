package stager

import (
	"testing"
)

// TestParsePatchFile_SpecialFiles tests parsing of special file operations including renamed, deleted, and binary files.
// This test ensures our legacy parser can handle these edge cases that commonly appear in Git patches.
func TestParsePatchFile_SpecialFiles(t *testing.T) {
	testCases := []struct {
		name         string
		patchContent string
		wantHunks    int
		checkFunc    func(t *testing.T, hunks []HunkInfo)
	}{
		{
			// Tests file rename with content changes - ensures the parser correctly identifies the new filename
			name: "renamed_file_with_changes",
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
			wantHunks: 1,
			checkFunc: func(t *testing.T, hunks []HunkInfo) {
				if len(hunks) != 1 {
					t.Errorf("Expected 1 hunk, got %d", len(hunks))
					return
				}
				// Note: With the current implementation, renamed files appear with their new name
				if hunks[0].FilePath != "new_name.py" {
					t.Errorf("Expected file path 'new_name.py', got '%s'", hunks[0].FilePath)
				}
			},
		},
		{
			// Tests file deletion - ensures the parser can handle patches that remove entire files
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
			wantHunks: 1,
			checkFunc: func(t *testing.T, hunks []HunkInfo) {
				if len(hunks) != 1 {
					t.Errorf("Expected 1 hunk, got %d", len(hunks))
					return
				}
				if hunks[0].FilePath != "deleted.py" {
					t.Errorf("Expected file path 'deleted.py', got '%s'", hunks[0].FilePath)
				}
			},
		},
		{
			name: "binary_file_added",
			patchContent: `diff --git a/image.png b/image.png
new file mode 100644
index 000000..abc123
Binary files /dev/null and b/image.png differ`,
			wantHunks: 0, // Binary files don't have text hunks
			checkFunc: func(t *testing.T, hunks []HunkInfo) {
				if len(hunks) != 0 {
					t.Errorf("Expected 0 hunks for binary file, got %d", len(hunks))
				}
			},
		},
		{
			name: "binary_file_modified",
			patchContent: `diff --git a/image.png b/image.png
index abc123..def456 100644
Binary files a/image.png and b/image.png differ`,
			wantHunks: 0, // Binary files don't have text hunks
			checkFunc: func(t *testing.T, hunks []HunkInfo) {
				if len(hunks) != 0 {
					t.Errorf("Expected 0 hunks for binary file, got %d", len(hunks))
				}
			},
		},
		{
			name: "renamed_file_no_changes",
			patchContent: `diff --git a/old_name.py b/new_name.py
similarity index 100%
rename from old_name.py
rename to new_name.py`,
			wantHunks: 0, // No content changes, only rename
			checkFunc: func(t *testing.T, hunks []HunkInfo) {
				if len(hunks) != 0 {
					t.Errorf("Expected 0 hunks for rename-only, got %d", len(hunks))
				}
			},
		},
		{
			name: "new_binary_file",
			patchContent: `diff --git a/new_binary.bin b/new_binary.bin
new file mode 100644
index 0000000..abc123
Binary files /dev/null and b/new_binary.bin differ`,
			wantHunks: 0,
			checkFunc: func(t *testing.T, hunks []HunkInfo) {
				if len(hunks) != 0 {
					t.Errorf("Expected 0 hunks for new binary file, got %d", len(hunks))
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hunks, err := parsePatchFile(tc.patchContent)
			if err != nil {
				t.Fatalf("Failed to parse patch: %v", err)
			}

			if len(hunks) != tc.wantHunks {
				t.Errorf("Expected %d hunks, got %d", tc.wantHunks, len(hunks))
			}

			if tc.checkFunc != nil {
				tc.checkFunc(t, hunks)
			}
		})
	}
}

// TestParsePatchFileWithGitDiff_SpecialFiles tests the new parser directly
func TestParsePatchFileWithGitDiff_SpecialFiles(t *testing.T) {
	testCases := []struct {
		name         string
		patchContent string
		checkFunc    func(t *testing.T, hunks []HunkInfoNew)
	}{
		{
			name: "renamed_file_detection",
			patchContent: `diff --git a/old_name.py b/new_name.py
similarity index 95%
rename from old_name.py
rename to new_name.py
index abc123..def456 100644
--- a/old_name.py
+++ b/new_name.py
@@ -1,3 +1,4 @@
 def renamed_func():
+    print("File was renamed")
     print("Original content")
     return True`,
			checkFunc: func(t *testing.T, hunks []HunkInfoNew) {
				if len(hunks) != 1 {
					t.Errorf("Expected 1 hunk, got %d", len(hunks))
					return
				}
				hunk := hunks[0]
				if hunk.Operation != FileOperationRenamed {
					t.Errorf("Expected FileOperationRenamed, got %v", hunk.Operation)
				}
				if hunk.FilePath != "new_name.py" {
					t.Errorf("Expected new file path 'new_name.py', got '%s'", hunk.FilePath)
				}
				if hunk.OldFilePath != "old_name.py" {
					t.Errorf("Expected old file path 'old_name.py', got '%s'", hunk.OldFilePath)
				}
			},
		},
		{
			name: "deleted_file_detection",
			patchContent: `diff --git a/deleted.py b/deleted.py
deleted file mode 100644
index abc123..000000
--- a/deleted.py
+++ /dev/null
@@ -1,3 +0,0 @@
-def old_function():
-    print("This will be deleted")
-    return 0`,
			checkFunc: func(t *testing.T, hunks []HunkInfoNew) {
				if len(hunks) != 1 {
					t.Errorf("Expected 1 hunk, got %d", len(hunks))
					return
				}
				hunk := hunks[0]
				if hunk.Operation != FileOperationDeleted {
					t.Errorf("Expected FileOperationDeleted, got %v", hunk.Operation)
				}
				if hunk.FilePath != "deleted.py" {
					t.Errorf("Expected file path 'deleted.py', got '%s'", hunk.FilePath)
				}
			},
		},
		{
			name: "binary_file_detection",
			patchContent: `diff --git a/image.png b/image.png
new file mode 100644
index 000000..abc123
Binary files /dev/null and b/image.png differ`,
			checkFunc: func(t *testing.T, hunks []HunkInfoNew) {
				// Binary files are represented as a single hunk in our implementation
				if len(hunks) != 1 {
					t.Errorf("Expected 1 hunk for binary file, got %d", len(hunks))
					return
				}
				if !hunks[0].IsBinary {
					t.Error("Expected IsBinary to be true")
				}
				if hunks[0].Operation != FileOperationAdded {
					t.Errorf("Expected FileOperationAdded, got %v", hunks[0].Operation)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hunks, err := parsePatchFileWithGitDiff(tc.patchContent)
			if err != nil {
				t.Fatalf("Failed to parse patch with go-gitdiff: %v", err)
			}

			if tc.checkFunc != nil {
				tc.checkFunc(t, hunks)
			}
		})
	}
}