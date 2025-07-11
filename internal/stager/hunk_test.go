package stager

import (
	"testing"
)

func TestExtractHunksFromPatch(t *testing.T) {
	tests := []struct {
		name        string
		patchContent string
		wantHunks   int
		validateFunc func(*testing.T, []Hunk)
	}{
		{
			name: "single file single hunk",
			patchContent: `diff --git a/file.txt b/file.txt
index abc123..def456 100644
--- a/file.txt
+++ b/file.txt
@@ -1,3 +1,3 @@
 line1
-line2
+line2 modified
 line3
`,
			wantHunks: 1,
			validateFunc: func(t *testing.T, hunks []Hunk) {
				if hunks[0].Number != 1 {
					t.Errorf("Expected hunk number 1, got %d", hunks[0].Number)
				}
				if hunks[0].FilePath != "file.txt" {
					t.Errorf("Expected file path 'file.txt', got %s", hunks[0].FilePath)
				}
				if hunks[0].PatchID == "" {
					t.Error("Expected non-empty patch ID")
				}
			},
		},
		{
			name: "single file multiple hunks",
			patchContent: `diff --git a/file.txt b/file.txt
index abc123..def456 100644
--- a/file.txt
+++ b/file.txt
@@ -1,3 +1,3 @@
 line1
-line2
+line2 modified
 line3
@@ -10,3 +10,3 @@
 line10
-line11
+line11 modified
 line12
`,
			wantHunks: 2,
			validateFunc: func(t *testing.T, hunks []Hunk) {
				if hunks[0].Number != 1 || hunks[1].Number != 2 {
					t.Error("Expected hunk numbers 1 and 2")
				}
				if hunks[0].PatchID == hunks[1].PatchID {
					t.Error("Different hunks should have different patch IDs")
				}
			},
		},
		{
			name: "multiple files",
			patchContent: `diff --git a/file1.txt b/file1.txt
index abc123..def456 100644
--- a/file1.txt
+++ b/file1.txt
@@ -1,3 +1,3 @@
 line1
-line2
+line2 modified
 line3
diff --git a/file2.txt b/file2.txt
index 111222..333444 100644
--- a/file2.txt
+++ b/file2.txt
@@ -5,3 +5,3 @@
 foo
-bar
+bar modified
 baz
`,
			wantHunks: 2,
			validateFunc: func(t *testing.T, hunks []Hunk) {
				if hunks[0].FilePath != "file1.txt" {
					t.Errorf("Expected first hunk file path 'file1.txt', got %s", hunks[0].FilePath)
				}
				if hunks[1].FilePath != "file2.txt" {
					t.Errorf("Expected second hunk file path 'file2.txt', got %s", hunks[1].FilePath)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hunks, err := ExtractHunksFromPatch(tt.patchContent)
			if err != nil {
				t.Fatalf("ExtractHunksFromPatch() error = %v", err)
			}
			
			if len(hunks) != tt.wantHunks {
				t.Errorf("ExtractHunksFromPatch() got %d hunks, want %d", len(hunks), tt.wantHunks)
			}
			
			if tt.validateFunc != nil {
				tt.validateFunc(t, hunks)
			}
		})
	}
}

func TestCalculatePatchID(t *testing.T) {
	// Test that same content produces same ID
	content := "test content\n"
	id1 := calculatePatchID(content)
	id2 := calculatePatchID(content)
	
	if id1 != id2 {
		t.Errorf("Same content should produce same patch ID, got %s and %s", id1, id2)
	}
	
	// Test that different content produces different ID
	id3 := calculatePatchID("different content\n")
	if id1 == id3 {
		t.Error("Different content should produce different patch ID")
	}
	
	// Test ID format
	if len(id1) != 8 {
		t.Errorf("Patch ID should be 8 characters, got %d", len(id1))
	}
}