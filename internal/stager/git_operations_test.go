package stager

import (
	"fmt"
	"strings"
	"testing"

	"github.com/syou6162/git-sequential-stage/internal/executor"
)

// TestIsDeletedFile tests the isDeletedFile function
func TestIsDeletedFile(t *testing.T) {
	tests := []struct {
		name         string
		patchContent string
		hunkIndex    int
		expected     bool
	}{
		{
			name: "deleted file",
			patchContent: `diff --git a/deleted_file.py b/deleted_file.py
deleted file mode 100644
index 1234567..0000000
--- a/deleted_file.py
+++ /dev/null
@@ -1,5 +0,0 @@
-def deleted_function():
-    print("This will be deleted")
-
-if __name__ == "__main__":
-    deleted_function()`,
			hunkIndex: 0,
			expected:  true,
		},
		{
			name: "regular file modification",
			patchContent: `diff --git a/regular_file.py b/regular_file.py
index abc1234..def5678 100644
--- a/regular_file.py
+++ b/regular_file.py
@@ -1,3 +1,4 @@
 def regular_function():
     print("This is regular")
+    print("Added line")`,
			hunkIndex: 0,
			expected:  false,
		},
		{
			name: "new file",
			patchContent: `diff --git a/new_file.py b/new_file.py
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/new_file.py
@@ -0,0 +1,3 @@
+def new_function():
+    print("This is new")`,
			hunkIndex: 0,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hunks, err := parsePatchFile(tt.patchContent)
			if err != nil {
				t.Fatalf("Failed to parse patch: %v", err)
			}

			if tt.hunkIndex >= len(hunks) {
				t.Fatalf("Hunk index %d out of range, only %d hunks found", tt.hunkIndex, len(hunks))
			}

			patchLines := strings.Split(tt.patchContent, "\n")
			result := isDeletedFile(patchLines, &hunks[tt.hunkIndex])

			if result != tt.expected {
				t.Errorf("isDeletedFile() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestIsRenamedFile tests the isRenamedFile function
func TestIsRenamedFile(t *testing.T) {
	tests := []struct {
		name         string
		patchContent string
		hunkIndex    int
		expected     bool
	}{
		{
			name: "renamed file with similarity index",
			patchContent: `diff --git a/old_name.py b/new_name.py
similarity index 85%
rename from old_name.py
rename to new_name.py
index abc1234..def5678 100644
--- a/old_name.py
+++ b/new_name.py
@@ -1,3 +1,4 @@
 def renamed_function():
     print("This was renamed")
+    print("And modified")`,
			hunkIndex: 0,
			expected:  true,
		},
		{
			name: "renamed file with rename from marker",
			patchContent: `diff --git a/old_file.py b/new_file.py
rename from old_file.py
rename to new_file.py
index abc1234..def5678 100644
--- a/old_file.py
+++ b/new_file.py
@@ -1,2 +1,3 @@
 def function():
     print("Renamed")
+    print("Added")`,
			hunkIndex: 0,
			expected:  true,
		},
		{
			name: "regular file modification",
			patchContent: `diff --git a/regular_file.py b/regular_file.py
index abc1234..def5678 100644
--- a/regular_file.py
+++ b/regular_file.py
@@ -1,3 +1,4 @@
 def regular_function():
     print("This is regular")
+    print("Just modified")`,
			hunkIndex: 0,
			expected:  false,
		},
		{
			name: "new file",
			patchContent: `diff --git a/new_file.py b/new_file.py
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/new_file.py
@@ -0,0 +1,3 @@
+def new_function():
+    print("This is new")`,
			hunkIndex: 0,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hunks, err := parsePatchFile(tt.patchContent)
			if err != nil {
				t.Fatalf("Failed to parse patch: %v", err)
			}

			if tt.hunkIndex >= len(hunks) {
				t.Fatalf("Hunk index %d out of range, only %d hunks found", tt.hunkIndex, len(hunks))
			}

			patchLines := strings.Split(tt.patchContent, "\n")
			result := isRenamedFile(patchLines, &hunks[tt.hunkIndex])

			if result != tt.expected {
				t.Errorf("isRenamedFile() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestGetRenamedFileInfo tests the getRenamedFileInfo function
func TestGetRenamedFileInfo(t *testing.T) {
	tests := []struct {
		name         string
		patchContent string
		hunkIndex    int
		expectedOld  string
		expectedNew  string
	}{
		{
			name: "simple rename",
			patchContent: `diff --git a/old_name.py b/new_name.py
similarity index 100%
rename from old_name.py
rename to new_name.py
index abc1234..def5678 100644
--- a/old_name.py
+++ b/new_name.py
@@ -1,3 +1,3 @@
 def function():
     print("Renamed")`,
			hunkIndex:   0,
			expectedOld: "old_name.py",
			expectedNew: "new_name.py",
		},
		{
			name: "rename with path",
			patchContent: `diff --git a/src/old_module.py b/src/new_module.py
similarity index 90%
rename from src/old_module.py
rename to src/new_module.py
index abc1234..def5678 100644
--- a/src/old_module.py
+++ b/src/new_module.py
@@ -1,3 +1,4 @@
 def module_function():
     print("In module")
+    print("Modified")`,
			hunkIndex:   0,
			expectedOld: "src/old_module.py",
			expectedNew: "src/new_module.py",
		},
		{
			name: "file move to different directory",
			patchContent: `diff --git a/old_dir/file.py b/new_dir/file.py
similarity index 100%
rename from old_dir/file.py
rename to new_dir/file.py
index abc1234..def5678 100644
--- a/old_dir/file.py
+++ b/new_dir/file.py
@@ -1,2 +1,2 @@
 def moved_function():
     print("Moved")`,
			hunkIndex:   0,
			expectedOld: "old_dir/file.py",
			expectedNew: "new_dir/file.py",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hunks, err := parsePatchFile(tt.patchContent)
			if err != nil {
				t.Fatalf("Failed to parse patch: %v", err)
			}

			if tt.hunkIndex >= len(hunks) {
				t.Fatalf("Hunk index %d out of range, only %d hunks found", tt.hunkIndex, len(hunks))
			}

			patchLines := strings.Split(tt.patchContent, "\n")
			oldFile, newFile := getRenamedFileInfo(patchLines, &hunks[tt.hunkIndex])

			if oldFile != tt.expectedOld {
				t.Errorf("getRenamedFileInfo() oldFile = %q, expected %q", oldFile, tt.expectedOld)
			}

			if newFile != tt.expectedNew {
				t.Errorf("getRenamedFileInfo() newFile = %q, expected %q", newFile, tt.expectedNew)
			}
		})
	}
}

// TestCheckStagingArea tests the checkStagingArea function
func TestCheckStagingArea(t *testing.T) {
	tests := []struct {
		name        string
		stagedFiles string
		expectError bool
		errorContains []string
	}{
		{
			name:        "clean staging area",
			stagedFiles: "",
			expectError: false,
		},
		{
			name:        "single staged file",
			stagedFiles: "file1.py",
			expectError: true,
			errorContains: []string{
				"staging area is not clean",
				"1 staged file(s)",
				"file1.py",
				"git commit",
				"git reset HEAD",
			},
		},
		{
			name:        "multiple staged files",
			stagedFiles: "file1.py\nfile2.py\nfile3.py",
			expectError: true,
			errorContains: []string{
				"staging area is not clean",
				"3 staged file(s)",
				"file1.py, file2.py, file3.py",
				"git commit",
				"git reset HEAD",
			},
		},
		{
			name:        "staged files with whitespace",
			stagedFiles: "  file1.py  \n  file2.py  \n",
			expectError: true,
			errorContains: []string{
				"staging area is not clean",
				"staged file(s)",
				"git commit",
				"git reset HEAD",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock executor
			mock := executor.NewMockCommandExecutor()
			mock.Commands["git [diff --cached --name-only]"] = executor.MockResponse{
				Output: []byte(tt.stagedFiles),
				Error:  nil,
			}

			stager := NewStager(mock)
			err := stager.checkStagingArea()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}

				errorMessage := err.Error()
				for _, expectedPattern := range tt.errorContains {
					if !strings.Contains(errorMessage, expectedPattern) {
						t.Errorf("Expected error message to contain '%s', but it didn't.\nActual error: %v", expectedPattern, err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestCheckStagingAreaWithCommandError tests error handling in checkStagingArea
func TestCheckStagingAreaWithCommandError(t *testing.T) {
	mock := executor.NewMockCommandExecutor()
	mock.Commands["git [diff --cached --name-only]"] = executor.MockResponse{
		Output: nil,
		Error:  fmt.Errorf("git command failed"),
	}

	stager := NewStager(mock)
	err := stager.checkStagingArea()

	if err == nil {
		t.Error("Expected error but got none")
		return
	}

	if !strings.Contains(err.Error(), "failed to check staging area") {
		t.Errorf("Expected error message to contain 'failed to check staging area', got: %v", err)
	}
}

// TestBuildTargetIDsWithRenamedFiles tests the enhanced buildTargetIDs function
func TestBuildTargetIDsWithRenamedFiles(t *testing.T) {
	tests := []struct {
		name      string
		hunkSpecs []string
		hunks     []HunkInfo
		expected  []string
		shouldErr bool
	}{
		{
			name:      "direct file match",
			hunkSpecs: []string{"file1.py:1", "file2.py:2"},
			hunks: []HunkInfo{
				{FilePath: "file1.py", IndexInFile: 1, PatchID: "patch1"},
				{FilePath: "file2.py", IndexInFile: 2, PatchID: "patch2"},
			},
			expected:  []string{"patch1", "patch2"},
			shouldErr: false,
		},
		{
			name:      "renamed file partial match",
			hunkSpecs: []string{"name.py:1"},
			hunks: []HunkInfo{
				{FilePath: "new_name.py", IndexInFile: 1, PatchID: "patch1"},
			},
			expected:  []string{"patch1"},
			shouldErr: false,
		},
		{
			name:      "file not found",
			hunkSpecs: []string{"nonexistent.py:1"},
			hunks: []HunkInfo{
				{FilePath: "file1.py", IndexInFile: 1, PatchID: "patch1"},
			},
			expected:  nil,
			shouldErr: true,
		},
		{
			name:      "hunk not found",
			hunkSpecs: []string{"file1.py:2"},
			hunks: []HunkInfo{
				{FilePath: "file1.py", IndexInFile: 1, PatchID: "patch1"},
			},
			expected:  nil,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildTargetIDs(tt.hunkSpecs, tt.hunks)

			if tt.shouldErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d target IDs, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Expected target ID[%d] = %q, got %q", i, expected, result[i])
				}
			}
		})
	}
}