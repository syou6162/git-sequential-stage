package stager

import (
	"testing"
)

func TestPatchAnalyzer_EmptyPatch(t *testing.T) {
	analyzer := NewPatchAnalyzer()
	result, err := analyzer.AnalyzePatch("")
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if len(result.AllFiles) != 0 {
		t.Errorf("Expected no files, got: %v", result.AllFiles)
	}
	
	if len(result.FilesByStatus) != 0 {
		t.Errorf("Expected empty FilesByStatus, got: %v", result.FilesByStatus)
	}
}

func TestPatchAnalyzer_SimpleModification(t *testing.T) {
	patchContent := `diff --git a/file.txt b/file.txt
index 257cc56..5716ca5 100644
--- a/file.txt
+++ b/file.txt
@@ -1 +1 @@
-foo
+bar
`
	
	analyzer := NewPatchAnalyzer()
	result, err := analyzer.AnalyzePatch(patchContent)
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if len(result.AllFiles) != 1 || result.AllFiles[0] != "file.txt" {
		t.Errorf("Expected file.txt, got: %v", result.AllFiles)
	}
	
	modifiedFiles := result.FilesByStatus[FileStatusModified]
	if len(modifiedFiles) != 1 || modifiedFiles[0] != "file.txt" {
		t.Errorf("Expected file.txt in modified files, got: %v", modifiedFiles)
	}
}

func TestPatchAnalyzer_NewFile(t *testing.T) {
	patchContent := `diff --git a/new.txt b/new.txt
new file mode 100644
index 0000000..3b18e51
--- /dev/null
+++ b/new.txt
@@ -0,0 +1 @@
+hello world
`
	
	analyzer := NewPatchAnalyzer()
	result, err := analyzer.AnalyzePatch(patchContent)
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	addedFiles := result.FilesByStatus[FileStatusAdded]
	if len(addedFiles) != 1 || addedFiles[0] != "new.txt" {
		t.Errorf("Expected new.txt in added files, got: %v", addedFiles)
	}
}

func TestPatchAnalyzer_IntentToAddFile(t *testing.T) {
	// Empty new file (intent-to-add)
	patchContent := `diff --git a/empty.txt b/empty.txt
new file mode 100644
index 0000000..e69de29
--- /dev/null
+++ b/empty.txt
`
	
	analyzer := NewPatchAnalyzer()
	result, err := analyzer.AnalyzePatch(patchContent)
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if len(result.IntentToAddFiles) != 1 || result.IntentToAddFiles[0] != "empty.txt" {
		t.Errorf("Expected empty.txt in intent-to-add files, got: %v", result.IntentToAddFiles)
	}
	
	addedFiles := result.FilesByStatus[FileStatusAdded]
	if len(addedFiles) != 1 || addedFiles[0] != "empty.txt" {
		t.Errorf("Expected empty.txt also in added files, got: %v", addedFiles)
	}
}

func TestPatchAnalyzer_DeletedFile(t *testing.T) {
	patchContent := `diff --git a/deleted.txt b/deleted.txt
deleted file mode 100644
index 257cc56..0000000
--- a/deleted.txt
+++ /dev/null
@@ -1 +0,0 @@
-content
`
	
	analyzer := NewPatchAnalyzer()
	result, err := analyzer.AnalyzePatch(patchContent)
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	deletedFiles := result.FilesByStatus[FileStatusDeleted]
	if len(deletedFiles) != 1 || deletedFiles[0] != "deleted.txt" {
		t.Errorf("Expected deleted.txt in deleted files, got: %v", deletedFiles)
	}
}

func TestPatchAnalyzer_RenamedFile(t *testing.T) {
	patchContent := `diff --git a/old.txt b/new.txt
similarity index 100%
rename from old.txt
rename to new.txt
`
	
	analyzer := NewPatchAnalyzer()
	result, err := analyzer.AnalyzePatch(patchContent)
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	renamedFiles := result.FilesByStatus[FileStatusRenamed]
	if len(renamedFiles) != 1 || renamedFiles[0] != "old.txt -> new.txt" {
		t.Errorf("Expected 'old.txt -> new.txt' in renamed files, got: %v", renamedFiles)
	}
	
	// All files should contain the new name
	if len(result.AllFiles) != 1 || result.AllFiles[0] != "new.txt" {
		t.Errorf("Expected new.txt in all files, got: %v", result.AllFiles)
	}
}

func TestPatchAnalyzer_BinaryFile(t *testing.T) {
	patchContent := `diff --git a/image.png b/image.png
new file mode 100644
index 0000000..abc123
Binary files /dev/null and b/image.png differ
`
	
	analyzer := NewPatchAnalyzer()
	result, err := analyzer.AnalyzePatch(patchContent)
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	binaryFiles := result.FilesByStatus[FileStatusBinary]
	if len(binaryFiles) != 1 || binaryFiles[0] != "image.png" {
		t.Errorf("Expected image.png in binary files, got: %v", binaryFiles)
	}
	
	// Should NOT be in intent-to-add (binary files are different)
	if len(result.IntentToAddFiles) != 0 {
		t.Errorf("Expected no intent-to-add files for binary, got: %v", result.IntentToAddFiles)
	}
}

func TestPatchAnalyzer_InvalidPatch(t *testing.T) {
	patchContent := `This is not a valid patch format`
	
	analyzer := NewPatchAnalyzer()
	_, err := analyzer.AnalyzePatch(patchContent)
	
	if err == nil {
		t.Fatal("Expected error for invalid patch format")
	}
	
	safetyErr, ok := err.(*SafetyError)
	if !ok {
		t.Fatalf("Expected SafetyError, got %T", err)
	}
	
	if safetyErr.Type != GitOperationFailed {
		t.Errorf("Expected GitOperationFailed, got %v", safetyErr.Type)
	}
}

func TestPatchAnalyzer_MultipleMixedFiles(t *testing.T) {
	patchContent := `diff --git a/modified.txt b/modified.txt
index 257cc56..5716ca5 100644
--- a/modified.txt
+++ b/modified.txt
@@ -1 +1 @@
-old
+new
diff --git a/added.txt b/added.txt
new file mode 100644
index 0000000..3b18e51
--- /dev/null
+++ b/added.txt
@@ -0,0 +1 @@
+content
diff --git a/deleted.txt b/deleted.txt
deleted file mode 100644
index 257cc56..0000000
--- a/deleted.txt
+++ /dev/null
@@ -1 +0,0 @@
-gone
`
	
	analyzer := NewPatchAnalyzer()
	result, err := analyzer.AnalyzePatch(patchContent)
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if len(result.AllFiles) != 3 {
		t.Errorf("Expected 3 files, got: %v", result.AllFiles)
	}
	
	if len(result.FilesByStatus[FileStatusModified]) != 1 {
		t.Errorf("Expected 1 modified file, got: %v", result.FilesByStatus[FileStatusModified])
	}
	
	if len(result.FilesByStatus[FileStatusAdded]) != 1 {
		t.Errorf("Expected 1 added file, got: %v", result.FilesByStatus[FileStatusAdded])
	}
	
	if len(result.FilesByStatus[FileStatusDeleted]) != 1 {
		t.Errorf("Expected 1 deleted file, got: %v", result.FilesByStatus[FileStatusDeleted])
	}
}