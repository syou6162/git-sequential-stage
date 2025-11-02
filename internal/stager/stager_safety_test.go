package stager

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/syou6162/git-sequential-stage/internal/executor"
	"github.com/syou6162/git-sequential-stage/internal/logger"
)

func TestStager_performSafetyChecks_CleanStagingArea(t *testing.T) {
	mockExecutor := executor.NewMockCommandExecutor()
	stager := &Stager{
		executor: mockExecutor,
		logger:   logger.NewFromEnv(),
	}

	// Empty patch content = clean staging area
	err := stager.performSafetyChecks("", nil)

	if err != nil {
		t.Fatalf("Expected no error for clean staging area, got: %v", err)
	}
}

func TestStager_performSafetyChecks_ModifiedFiles(t *testing.T) {
	mockExecutor := executor.NewMockCommandExecutor()
	stager := &Stager{
		executor: mockExecutor,
		logger:   logger.NewFromEnv(),
	}

	// Patch with modified files
	patchContent := `diff --git a/file1.txt b/file1.txt
index 257cc56..5716ca5 100644
--- a/file1.txt
+++ b/file1.txt
@@ -1 +1 @@
-foo
+bar
`

	err := stager.performSafetyChecks(patchContent, nil)

	if err == nil {
		t.Fatal("Expected error for modified files in staging area")
	}

	safetyErr, ok := err.(*SafetyError)
	if !ok {
		t.Fatalf("Expected SafetyError, got %T", err)
	}

	if safetyErr.Type != StagingAreaNotClean {
		t.Errorf("Expected StagingAreaNotClean error type, got %v", safetyErr.Type)
	}

	// Check that error message contains helpful information
	if safetyErr.Message == "" {
		t.Error("Expected non-empty error message")
	}

	if safetyErr.Advice == "" {
		t.Error("Expected non-empty advice")
	}
}

func TestStager_performSafetyChecks_IntentToAddFiles(t *testing.T) {
	mockExecutor := executor.NewMockCommandExecutor()
	stager := &Stager{
		executor: mockExecutor,
		logger:   logger.NewFromEnv(),
	}

	// Patch with intent-to-add file
	patchContent := `diff --git a/intent_to_add.txt b/intent_to_add.txt
new file mode 100644
index 0000000..e69de29
`

	err := stager.performSafetyChecks(patchContent, nil)

	// Should not error for intent-to-add files
	if err != nil {
		t.Fatalf("Expected no error for intent-to-add files, got: %v", err)
	}
}

func TestStager_performSafetyChecks_MixedFiles(t *testing.T) {
	mockExecutor := executor.NewMockCommandExecutor()
	stager := &Stager{
		executor: mockExecutor,
		logger:   logger.NewFromEnv(),
	}

	// Patch with mixed files (modified + intent-to-add)
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

	err := stager.performSafetyChecks(patchContent, nil)

	// Should error because of modified file (not just intent-to-add)
	if err == nil {
		t.Fatal("Expected error for mixed files in staging area")
	}

	safetyErr, ok := err.(*SafetyError)
	if !ok {
		t.Fatalf("Expected SafetyError, got %T", err)
	}

	if safetyErr.Type != StagingAreaNotClean {
		t.Errorf("Expected StagingAreaNotClean error type, got %v", safetyErr.Type)
	}
}

func TestStager_StageHunks_WithSafetyCheck_Clean(t *testing.T) {
	// Create a temporary directory for test
	tmpDir, err := os.MkdirTemp("", "stage_hunks_safety_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create an empty patch file (no changes = clean staging area)
	patchContent := ""
	patchFile := filepath.Join(tmpDir, "test.patch")
	if err := os.WriteFile(patchFile, []byte(patchContent), 0644); err != nil {
		t.Fatal(err)
	}

	mockExecutor := executor.NewMockCommandExecutor()
	stager := NewStager(mockExecutor)

	// This should succeed because empty patch = clean staging area
	err = stager.StageHunks(context.Background(), []string{}, patchFile)

	if err != nil {
		t.Fatalf("Expected successful with clean staging area (empty patch), got error: %v", err)
	}
}

func TestStager_StageHunks_WithSafetyCheck_Dirty(t *testing.T) {
	// Create a temporary directory for test
	tmpDir, err := os.MkdirTemp("", "stage_hunks_safety_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a patch file that simulates already staged changes
	patchContent := `diff --git a/already_staged.txt b/already_staged.txt
index 257cc56..5716ca5 100644
--- a/already_staged.txt
+++ b/already_staged.txt
@@ -1 +1 @@
-foo
+bar
`
	patchFile := filepath.Join(tmpDir, "test.patch")
	if err := os.WriteFile(patchFile, []byte(patchContent), 0644); err != nil {
		t.Fatal(err)
	}

	mockExecutor := executor.NewMockCommandExecutor()
	stager := NewStager(mockExecutor)

	// This should fail because the patch shows modified files (dirty staging area)
	err = stager.StageHunks(context.Background(), []string{"already_staged.txt:1"}, patchFile)

	if err == nil {
		t.Fatal("Expected error for dirty staging area")
	}

	safetyErr, ok := err.(*SafetyError)
	if !ok {
		t.Fatalf("Expected SafetyError, got %T", err)
	}

	if safetyErr.Type != StagingAreaNotClean {
		t.Errorf("Expected StagingAreaNotClean error type, got %v", safetyErr.Type)
	}
}

func TestStager_StageHunks_WithSafetyCheck_NewFile(t *testing.T) {
	// Create a temporary directory for test
	tmpDir, err := os.MkdirTemp("", "stage_hunks_safety_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a patch file with new file (not intent-to-add because it has content)
	patchContent := `diff --git a/new_file.txt b/new_file.txt
new file mode 100644
index 0000000..257cc56
--- /dev/null
+++ b/new_file.txt
@@ -0,0 +1 @@
+foo
`
	patchFile := filepath.Join(tmpDir, "test.patch")
	if err := os.WriteFile(patchFile, []byte(patchContent), 0644); err != nil {
		t.Fatal(err)
	}

	mockExecutor := executor.NewMockCommandExecutor()
	stager := NewStager(mockExecutor)

	// This should fail because new files are considered "dirty" staging area
	err = stager.StageHunks(context.Background(), []string{"new_file.txt:1"}, patchFile)

	if err == nil {
		t.Fatal("Expected error for new file in staging area")
	}

	safetyErr, ok := err.(*SafetyError)
	if !ok {
		t.Fatalf("Expected SafetyError, got %T", err)
	}

	if safetyErr.Type != StagingAreaNotClean {
		t.Errorf("Expected StagingAreaNotClean error type, got %v", safetyErr.Type)
	}
}

func TestStager_generateDetailedStagingError(t *testing.T) {
	mockExecutor := executor.NewMockCommandExecutor()
	stager := &Stager{
		executor: mockExecutor,
		logger:   logger.NewFromEnv(),
	}

	evaluation := &StagingAreaEvaluation{
		IsClean:      false,
		StagedFiles:  []string{"file1.txt", "file2.txt"},
		ErrorMessage: "Files are already staged",
		RecommendedActions: []RecommendedAction{
			{
				Description: "Commit all staged changes",
				Commands:    []string{"git commit -m \"Your message\""},
				Priority:    1,
				Category:    ActionCategoryCommit,
			},
			{
				Description: "Unstage all files",
				Commands:    []string{"git reset HEAD"},
				Priority:    2,
				Category:    ActionCategoryUnstage,
			},
		},
	}

	err := stager.generateDetailedStagingError(evaluation)

	if err == nil {
		t.Fatal("Expected error to be generated")
	}

	safetyErr, ok := err.(*SafetyError)
	if !ok {
		t.Fatalf("Expected SafetyError, got %T", err)
	}

	// Check error contains the evaluation message
	if !strings.Contains(safetyErr.Message, "Files are already staged") {
		t.Errorf("Error message should contain evaluation message, got: %s", safetyErr.Message)
	}

	// Check advice contains recommended actions
	if !strings.Contains(safetyErr.Advice, "git commit") {
		t.Errorf("Advice should contain git commit command, got: %s", safetyErr.Advice)
	}

	if !strings.Contains(safetyErr.Advice, "git reset HEAD") {
		t.Errorf("Advice should contain git reset command, got: %s", safetyErr.Advice)
	}
}
