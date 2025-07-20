package stager

import (
	"testing"

	"github.com/syou6162/git-sequential-stage/internal/executor"
)

func TestCheckActualStagingArea_Clean(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	// Mock clean staging area
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(""),
		Error:  nil,
	}
	
	checker := NewSafetyChecker(mockExec)
	evaluation, err := checker.CheckActualStagingArea()
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if !evaluation.IsClean {
		t.Error("Expected clean staging area")
	}
	
	if len(evaluation.StagedFiles) != 0 {
		t.Errorf("Expected no staged files, got: %v", evaluation.StagedFiles)
	}
}

func TestCheckActualStagingArea_WithStagedFiles(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	// Mock staging area with modified and added files
	statusOutput := `M  file1.txt
A  file2.txt
 M file3.txt`
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(statusOutput),
		Error:  nil,
	}
	
	checker := NewSafetyChecker(mockExec)
	evaluation, err := checker.CheckActualStagingArea()
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if evaluation.IsClean {
		t.Error("Expected non-clean staging area")
	}
	
	// Should only include staged files (M  and A , not  M)
	if len(evaluation.StagedFiles) != 2 {
		t.Errorf("Expected 2 staged files, got: %v", evaluation.StagedFiles)
	}
	
	// Check file categorization
	modifiedFiles := evaluation.FilesByStatus[FileStatusModified]
	if len(modifiedFiles) != 1 || modifiedFiles[0] != "file1.txt" {
		t.Errorf("Expected modified file1.txt, got: %v", modifiedFiles)
	}
	
	addedFiles := evaluation.FilesByStatus[FileStatusAdded]
	if len(addedFiles) != 1 || addedFiles[0] != "file2.txt" {
		t.Errorf("Expected added file2.txt, got: %v", addedFiles)
	}
}

func TestCheckActualStagingArea_IntentToAdd(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	// Mock staging area with intent-to-add file
	// Note: ' A' (space + A) indicates intent-to-add
	statusOutput := " A intent_to_add.txt"
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(statusOutput),
		Error:  nil,
	}
	
	checker := NewSafetyChecker(mockExec)
	evaluation, err := checker.CheckActualStagingArea()
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if !evaluation.AllowContinue {
		t.Error("Expected AllowContinue=true for intent-to-add only")
	}
	
	if len(evaluation.IntentToAddFiles) != 1 || evaluation.IntentToAddFiles[0] != "intent_to_add.txt" {
		t.Errorf("Expected intent-to-add file, got: %v", evaluation.IntentToAddFiles)
	}
}

func TestCheckActualStagingArea_RenamedFiles(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	// Mock staging area with renamed file
	statusOutput := `R  old_name.txt -> new_name.txt`
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(statusOutput),
		Error:  nil,
	}
	
	checker := NewSafetyChecker(mockExec)
	evaluation, err := checker.CheckActualStagingArea()
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if evaluation.IsClean {
		t.Error("Expected non-clean staging area")
	}
	
	renamedFiles := evaluation.FilesByStatus[FileStatusRenamed]
	if len(renamedFiles) != 1 || renamedFiles[0] != "old_name.txt -> new_name.txt" {
		t.Errorf("Expected renamed file notation, got: %v", renamedFiles)
	}
	
	// Staged files should contain the new name
	if len(evaluation.StagedFiles) != 1 || evaluation.StagedFiles[0] != "new_name.txt" {
		t.Errorf("Expected new_name.txt in staged files, got: %v", evaluation.StagedFiles)
	}
}

func TestCheckActualStagingArea_NoExecutor(t *testing.T) {
	checker := NewSafetyChecker(nil)
	evaluation, err := checker.CheckActualStagingArea()
	
	if err == nil {
		t.Fatal("Expected error for nil executor")
	}
	
	if evaluation != nil {
		t.Error("Expected nil evaluation on error")
	}
	
	safetyErr, ok := err.(*SafetyError)
	if !ok {
		t.Fatalf("Expected SafetyError, got %T", err)
	}
	
	if safetyErr.Type != GitOperationFailed {
		t.Errorf("Expected GitOperationFailed, got %v", safetyErr.Type)
	}
}

func TestCheckActualStagingArea_MixedAddedFiles(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	// Mock staging area with mix of regular and intent-to-add files
	// 'A ' = regular added, ' A' = intent-to-add
	statusOutput := `A  regular1.txt
 A intent1.txt
A  regular2.txt
 A intent2.txt`
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(statusOutput),
		Error:  nil,
	}
	
	checker := NewSafetyChecker(mockExec)
	evaluation, err := checker.CheckActualStagingArea()
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	// Should identify 2 intent-to-add files
	if len(evaluation.IntentToAddFiles) != 2 {
		t.Errorf("Expected 2 intent-to-add files, got: %v", evaluation.IntentToAddFiles)
	}
	
	// Check specific files
	intentMap := make(map[string]bool)
	for _, f := range evaluation.IntentToAddFiles {
		intentMap[f] = true
	}
	
	if !intentMap["intent1.txt"] || !intentMap["intent2.txt"] {
		t.Errorf("Expected intent1.txt and intent2.txt to be intent-to-add, got: %v", evaluation.IntentToAddFiles)
	}
	
	// Total staged files should be 4
	if len(evaluation.StagedFiles) != 4 {
		t.Errorf("Expected 4 staged files, got: %v", evaluation.StagedFiles)
	}
}

func TestEvaluateWithFallback_EmptyPatch(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	checker := NewSafetyChecker(mockExec)
	
	// Empty patch should not trigger git commands
	evaluation, err := checker.EvaluateWithFallback("")
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if !evaluation.IsClean {
		t.Error("Expected clean evaluation for empty patch")
	}
	
	// Verify no git commands were executed
	if len(mockExec.ExecutedCommands) != 0 {
		t.Error("Expected no git commands for empty patch")
	}
}

func TestEvaluateWithFallback_WithChanges(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	checker := NewSafetyChecker(mockExec)
	
	// Patch with changes
	patchContent := `diff --git a/file.txt b/file.txt
index 257cc56..5716ca5 100644
--- a/file.txt
+++ b/file.txt
@@ -1 +1 @@
-foo
+bar
`
	
	// Mock actual staging area check
	statusOutput := `M  file.txt
A  other.txt`
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(statusOutput),
		Error:  nil,
	}
	
	// Mock ls-files check for other.txt (added file)
	mockExec.Commands["git [ls-files --cached -- other.txt]"] = executor.MockResponse{
		Output: []byte("other.txt\n"),
		Error:  nil,
	}
	
	// Mock diff check for other.txt (not intent-to-add, has content)
	mockExec.Commands["git [diff --cached -- other.txt]"] = executor.MockResponse{
		Output: []byte("some diff content"),
		Error:  nil,
	}
	
	evaluation, err := checker.EvaluateWithFallback(patchContent)
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	// Should use actual staging area result
	if len(evaluation.StagedFiles) != 2 {
		t.Errorf("Expected 2 staged files from actual check, got: %v", evaluation.StagedFiles)
	}
	
	// Verify git commands were executed
	if len(mockExec.ExecutedCommands) < 1 {
		t.Errorf("Expected at least 1 command executed, got: %d", len(mockExec.ExecutedCommands))
	}
	// First command should be git status
	if mockExec.ExecutedCommands[0].Name != "git" || len(mockExec.ExecutedCommands[0].Args) < 1 || mockExec.ExecutedCommands[0].Args[0] != "status" {
		t.Errorf("Expected first command to be git status, got: %v %v", mockExec.ExecutedCommands[0].Name, mockExec.ExecutedCommands[0].Args)
	}
}

func TestEvaluateWithFallback_FallbackOnError(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	checker := NewSafetyChecker(mockExec)
	
	// Patch with changes
	patchContent := `diff --git a/file.txt b/file.txt
index 257cc56..5716ca5 100644
--- a/file.txt
+++ b/file.txt
@@ -1 +1 @@
-foo
+bar
`
	
	// Mock git status failure
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: nil,
		Error:  NewGitCommandError("git status", nil),
	}
	
	evaluation, err := checker.EvaluateWithFallback(patchContent)
	
	// Should not fail, but fall back to patch evaluation
	if err != nil {
		t.Fatalf("Expected no error (fallback to patch), got: %v", err)
	}
	
	// Should have patch-based results
	if len(evaluation.StagedFiles) != 1 || evaluation.StagedFiles[0] != "file.txt" {
		t.Errorf("Expected patch-based result, got: %v", evaluation.StagedFiles)
	}
}

func TestEvaluateWithFallback_NoExecutor(t *testing.T) {
	// No executor means patch-only mode
	checker := NewSafetyChecker(nil)
	
	// Patch with changes
	patchContent := `diff --git a/file.txt b/file.txt
index 257cc56..5716ca5 100644
--- a/file.txt
+++ b/file.txt
@@ -1 +1 @@
-foo
+bar
`
	
	evaluation, err := checker.EvaluateWithFallback(patchContent)
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	// Should have patch-based results only
	if len(evaluation.StagedFiles) != 1 || evaluation.StagedFiles[0] != "file.txt" {
		t.Errorf("Expected patch-based result, got: %v", evaluation.StagedFiles)
	}
}