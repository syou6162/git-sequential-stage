package stager

import (
	"strings"
	"testing"
)

func TestNewSafetyChecker(t *testing.T) {
	checker := NewSafetyChecker()

	if checker == nil {
		t.Fatal("NewSafetyChecker returned nil")
	}
}

func TestEvaluatePatchContent_EmptyPatch(t *testing.T) {
	checker := NewSafetyChecker()

	evaluation, err := checker.EvaluatePatchContent("")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !evaluation.IsClean {
		t.Error("Expected clean staging area for empty patch")
	}

	if !evaluation.AllowContinue {
		t.Error("Expected AllowContinue to be true for empty patch")
	}

	if len(evaluation.StagedFiles) != 0 {
		t.Errorf("Expected no staged files, got %d", len(evaluation.StagedFiles))
	}

	if len(evaluation.IntentToAddFiles) != 0 {
		t.Errorf("Expected no intent-to-add files, got %d", len(evaluation.IntentToAddFiles))
	}
}

func TestEvaluatePatchContent_ModifiedFiles(t *testing.T) {
	checker := NewSafetyChecker()

	// Example patch with modified files
	patchContent := `diff --git a/file1.txt b/file1.txt
index 257cc56..5716ca5 100644
--- a/file1.txt
+++ b/file1.txt
@@ -1 +1 @@
-foo
+bar
diff --git a/file2.txt b/file2.txt
index 606c2a0..5716ca5 100644
--- a/file2.txt
+++ b/file2.txt
@@ -1 +1 @@
-baz
+bar
`

	evaluation, err := checker.EvaluatePatchContent(patchContent)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if evaluation.IsClean {
		t.Error("Expected staging area not to be clean")
	}

	if evaluation.AllowContinue {
		t.Error("Expected AllowContinue to be false for modified files")
	}

	if len(evaluation.StagedFiles) != 2 {
		t.Errorf("Expected 2 staged files, got %d", len(evaluation.StagedFiles))
	}

	expectedFiles := []string{"file1.txt", "file2.txt"}
	for _, expectedFile := range expectedFiles {
		found := false
		for _, file := range evaluation.StagedFiles {
			if file == expectedFile {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected file %s not found in staged files", expectedFile)
		}
	}

	if evaluation.ErrorMessage == "" {
		t.Error("Expected error message for non-clean staging area")
	}

	if len(evaluation.RecommendedActions) == 0 {
		t.Error("Expected recommended actions for non-clean staging area")
	}

	// Check FilesByStatus
	if modifiedFiles, ok := evaluation.FilesByStatus[FileStatusModified]; !ok || len(modifiedFiles) != 2 {
		t.Errorf("Expected 2 modified files in FilesByStatus, got %v", modifiedFiles)
	}
}

func TestEvaluatePatchContent_NewFiles(t *testing.T) {
	checker := NewSafetyChecker()

	// Example patch with new files
	patchContent := `diff --git a/new_file.txt b/new_file.txt
new file mode 100644
index 0000000..257cc56
--- /dev/null
+++ b/new_file.txt
@@ -0,0 +1 @@
+foo
`

	evaluation, err := checker.EvaluatePatchContent(patchContent)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if evaluation.IsClean {
		t.Error("Expected staging area not to be clean")
	}

	if evaluation.AllowContinue {
		t.Error("Expected AllowContinue to be false for new files")
	}

	// Check FilesByStatus
	if addedFiles, ok := evaluation.FilesByStatus[FileStatusAdded]; !ok || len(addedFiles) != 1 {
		t.Errorf("Expected 1 added file in FilesByStatus, got %v", addedFiles)
	}
}

func TestEvaluatePatchContent_DeletedFiles(t *testing.T) {
	checker := NewSafetyChecker()

	// Example patch with deleted files
	patchContent := `diff --git a/deleted_file.txt b/deleted_file.txt
deleted file mode 100644
index 257cc56..0000000
--- a/deleted_file.txt
+++ /dev/null
@@ -1 +0,0 @@
-foo
`

	evaluation, err := checker.EvaluatePatchContent(patchContent)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if evaluation.IsClean {
		t.Error("Expected staging area not to be clean")
	}

	if evaluation.AllowContinue {
		t.Error("Expected AllowContinue to be false for deleted files")
	}

	// Check FilesByStatus
	if deletedFiles, ok := evaluation.FilesByStatus[FileStatusDeleted]; !ok || len(deletedFiles) != 1 {
		t.Errorf("Expected 1 deleted file in FilesByStatus, got %v", deletedFiles)
	}
}

func TestEvaluatePatchContent_RenamedFiles(t *testing.T) {
	checker := NewSafetyChecker()

	// Example patch with renamed files
	patchContent := `diff --git a/old_name.txt b/new_name.txt
similarity index 100%
rename from old_name.txt
rename to new_name.txt
`

	evaluation, err := checker.EvaluatePatchContent(patchContent)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if evaluation.IsClean {
		t.Error("Expected staging area not to be clean")
	}

	if evaluation.AllowContinue {
		t.Error("Expected AllowContinue to be false for renamed files")
	}

	// Check FilesByStatus
	if renamedFiles, ok := evaluation.FilesByStatus[FileStatusRenamed]; !ok || len(renamedFiles) != 1 {
		t.Errorf("Expected 1 renamed file in FilesByStatus, got %v", renamedFiles)
	}
}

func TestEvaluatePatchContent_IntentToAddFiles(t *testing.T) {
	checker := NewSafetyChecker()

	// Example patch with intent-to-add files (new file with no content)
	patchContent := `diff --git a/intent_to_add.txt b/intent_to_add.txt
new file mode 100644
index 0000000..e69de29
`

	evaluation, err := checker.EvaluatePatchContent(patchContent)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if evaluation.IsClean {
		t.Error("Expected staging area not to be clean")
	}

	if !evaluation.AllowContinue {
		t.Error("Expected AllowContinue to be true for intent-to-add files")
	}

	if len(evaluation.IntentToAddFiles) != 1 {
		t.Errorf("Expected 1 intent-to-add file, got %d", len(evaluation.IntentToAddFiles))
	}

	if evaluation.IntentToAddFiles[0] != "intent_to_add.txt" {
		t.Errorf("Expected intent_to_add.txt, got %s", evaluation.IntentToAddFiles[0])
	}
}

func TestEvaluatePatchContent_MixedFiles(t *testing.T) {
	checker := NewSafetyChecker()

	// Example patch with mixed file types
	patchContent := `diff --git a/modified.txt b/modified.txt
index 257cc56..5716ca5 100644
--- a/modified.txt
+++ b/modified.txt
@@ -1 +1 @@
-foo
+bar
diff --git a/intent_to_add.txt b/intent_to_add.txt
new file mode 100644
index 0000000..e69de29
`

	evaluation, err := checker.EvaluatePatchContent(patchContent)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if evaluation.IsClean {
		t.Error("Expected staging area not to be clean")
	}

	// Should not allow continue because of modified files (not just intent-to-add)
	if evaluation.AllowContinue {
		t.Error("Expected AllowContinue to be false for mixed files")
	}

	if len(evaluation.IntentToAddFiles) != 1 {
		t.Errorf("Expected 1 intent-to-add file, got %d", len(evaluation.IntentToAddFiles))
	}

	// Check FilesByStatus
	if modifiedFiles, ok := evaluation.FilesByStatus[FileStatusModified]; !ok || len(modifiedFiles) != 1 {
		t.Errorf("Expected 1 modified file in FilesByStatus, got %v", modifiedFiles)
	}
}

func TestEvaluatePatchContent_InvalidPatch(t *testing.T) {
	checker := NewSafetyChecker()

	// Invalid patch content
	patchContent := `This is not a valid patch`

	evaluation, err := checker.EvaluatePatchContent(patchContent)

	if err == nil {
		t.Fatal("Expected error for invalid patch content")
	}

	if evaluation != nil {
		t.Error("Expected nil evaluation for invalid patch")
	}

	// Check that it's a SafetyError
	safetyError, ok := err.(*SafetyError)
	if !ok {
		t.Errorf("Expected SafetyError, got %T", err)
	} else {
		if safetyError.Type != GitOperationFailed {
			t.Errorf("Expected GitOperationFailed error type, got %v", safetyError.Type)
		}
	}
}

func TestEvaluatePatchContent_BinaryFiles(t *testing.T) {
	checker := NewSafetyChecker()

	// Example patch with binary files
	patchContent := `diff --git a/image.png b/image.png
new file mode 100644
index 0000000..abcd123
Binary files /dev/null and b/image.png differ
`

	evaluation, err := checker.EvaluatePatchContent(patchContent)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if evaluation.IsClean {
		t.Error("Expected staging area not to be clean")
	}

	if evaluation.AllowContinue {
		t.Error("Expected AllowContinue to be false for binary files")
	}

	// Check FilesByStatus
	if binaryFiles, ok := evaluation.FilesByStatus[FileStatusBinary]; !ok || len(binaryFiles) != 1 {
		t.Errorf("Expected 1 binary file in FilesByStatus, got %v", binaryFiles)
	}
}

func TestRecommendedActions_Prioritization(t *testing.T) {
	checker := NewSafetyChecker()

	// Patch with multiple file types to test action prioritization
	patchContent := `diff --git a/deleted.txt b/deleted.txt
deleted file mode 100644
index 257cc56..0000000
--- a/deleted.txt
+++ /dev/null
@@ -1 +0,0 @@
-foo
diff --git a/modified.txt b/modified.txt
index 257cc56..5716ca5 100644
--- a/modified.txt
+++ b/modified.txt
@@ -1 +1 @@
-foo
+bar
`

	evaluation, err := checker.EvaluatePatchContent(patchContent)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(evaluation.RecommendedActions) == 0 {
		t.Fatal("Expected recommended actions")
	}

	// Check that actions are prioritized (deleted files should come first)
	foundDeleteAction := false
	for _, action := range evaluation.RecommendedActions {
		if action.Category == ActionCategoryCommit && action.Priority == 1 {
			// Check if it's related to deleted file
			if len(action.Commands) > 0 && strings.Contains(action.Commands[0], "deleted.txt") {
				foundDeleteAction = true
				break
			}
		}
	}

	if !foundDeleteAction {
		t.Error("Expected high-priority action for deleted file")
	}
}
