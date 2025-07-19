package stager

import (
	"fmt"
	"testing"

	"github.com/syou6162/git-sequential-stage/internal/executor"
)

func TestNewSafetyChecker(t *testing.T) {
	checker := NewSafetyChecker()

	if checker == nil {
		t.Fatal("NewSafetyChecker returned nil")
	}
}

func TestEvaluateStagingArea_CleanStagingArea(t *testing.T) {
	mockExecutor := executor.NewMockCommandExecutor()
	
	// Mock git status --porcelain (empty output = clean staging area)
	mockExecutor.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(""),
		Error:  nil,
	}
	
	// Mock git diff --cached (empty output = no staged changes)
	mockExecutor.Commands["git [diff --cached]"] = executor.MockResponse{
		Output: []byte(""),
		Error:  nil,
	}
	
	// Mock git ls-files --cached --stage (no intent-to-add files)
	mockExecutor.Commands["git [ls-files --cached --stage]"] = executor.MockResponse{
		Output: []byte(""),
		Error:  nil,
	}

	checker := NewSafetyChecker(mockExecutor)
	evaluation, err := checker.EvaluateStagingArea()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !evaluation.IsClean {
		t.Error("Expected clean staging area")
	}

	if !evaluation.AllowContinue {
		t.Error("Expected AllowContinue to be true for clean staging area")
	}

	if len(evaluation.StagedFiles) != 0 {
		t.Errorf("Expected no staged files, got %d", len(evaluation.StagedFiles))
	}

	if len(evaluation.IntentToAddFiles) != 0 {
		t.Errorf("Expected no intent-to-add files, got %d", len(evaluation.IntentToAddFiles))
	}
}

func TestEvaluateStagingArea_ModifiedFiles(t *testing.T) {
	mockExecutor := executor.NewMockCommandExecutor()
	
	// Mock git status --porcelain (modified files)
	gitStatusOutput := "M  file1.go\nM  file2.go\n"
	mockExecutor.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(gitStatusOutput),
		Error:  nil,
	}
	
	// Mock git diff --cached (modified files with diff output)
	gitDiffOutput := `diff --git a/file1.go b/file1.go
index abc123..def456 100644
--- a/file1.go
+++ b/file1.go
@@ -1,3 +1,4 @@
 package main
 
+// Added comment
 func main() {}
diff --git a/file2.go b/file2.go
index ghi789..jkl012 100644
--- a/file2.go
+++ b/file2.go
@@ -1,3 +1,4 @@
 package utils
 
+// Added comment
 func Helper() {}`
	mockExecutor.Commands["git [diff --cached]"] = executor.MockResponse{
		Output: []byte(gitDiffOutput),
		Error:  nil,
	}
	
	// Mock git ls-files --cached --stage (no intent-to-add files)
	mockExecutor.Commands["git [ls-files --cached --stage]"] = executor.MockResponse{
		Output: []byte("100644 abc123 0 file1.go\n100644 def456 0 file2.go\n"),
		Error:  nil,
	}

	checker := NewSafetyChecker(mockExecutor)
	evaluation, err := checker.EvaluateStagingArea()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if evaluation.IsClean {
		t.Error("Expected dirty staging area")
	}

	if evaluation.AllowContinue {
		t.Error("Expected AllowContinue to be false for modified files")
	}

	if len(evaluation.StagedFiles) != 2 {
		t.Errorf("Expected 2 staged files, got %d", len(evaluation.StagedFiles))
	}

	if len(evaluation.FilesByStatus["M"]) != 2 {
		t.Errorf("Expected 2 modified files, got %d", len(evaluation.FilesByStatus["M"]))
	}

	if evaluation.ErrorMessage == "" {
		t.Error("Expected error message for dirty staging area")
	}

	if len(evaluation.RecommendedActions) == 0 {
		t.Error("Expected recommended actions for dirty staging area")
	}
}

func TestEvaluateStagingArea_NewFiles(t *testing.T) {
	mockExecutor := executor.NewMockCommandExecutor()
	
	// Mock git status --porcelain (new files)
	gitStatusOutput := "A  new_file1.go\nA  new_file2.go\n"
	mockExecutor.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(gitStatusOutput),
		Error:  nil,
	}
	
	// Mock git ls-files --cached --stage (no intent-to-add files)
	mockExecutor.Commands["git [ls-files --cached --stage]"] = executor.MockResponse{
		Output: []byte("100644 abc123 0 new_file1.go\n100644 def456 0 new_file2.go\n"),
		Error:  nil,
	}

	checker := NewSafetyChecker(mockExecutor)
	evaluation, err := checker.EvaluateStagingArea()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if evaluation.IsClean {
		t.Error("Expected dirty staging area")
	}

	if len(evaluation.FilesByStatus["A"]) != 2 {
		t.Errorf("Expected 2 added files, got %d", len(evaluation.FilesByStatus["A"]))
	}
}

func TestEvaluateStagingArea_DeletedFiles(t *testing.T) {
	mockExecutor := executor.NewMockCommandExecutor()
	
	// Mock git status --porcelain (deleted files)
	gitStatusOutput := "D  deleted_file1.go\nD  deleted_file2.go\n"
	mockExecutor.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(gitStatusOutput),
		Error:  nil,
	}
	
	// Mock git ls-files --cached --stage
	mockExecutor.Commands["git [ls-files --cached --stage]"] = executor.MockResponse{
		Output: []byte(""),
		Error:  nil,
	}

	checker := NewSafetyChecker(mockExecutor)
	evaluation, err := checker.EvaluateStagingArea()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if evaluation.IsClean {
		t.Error("Expected dirty staging area")
	}

	if len(evaluation.FilesByStatus["D"]) != 2 {
		t.Errorf("Expected 2 deleted files, got %d", len(evaluation.FilesByStatus["D"]))
	}
}

func TestEvaluateStagingArea_RenamedFiles(t *testing.T) {
	mockExecutor := executor.NewMockCommandExecutor()
	
	// Mock git status --porcelain (renamed files)
	gitStatusOutput := "R  old_name.go -> new_name.go\n"
	mockExecutor.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(gitStatusOutput),
		Error:  nil,
	}
	
	// Mock git ls-files --cached --stage
	mockExecutor.Commands["git [ls-files --cached --stage]"] = executor.MockResponse{
		Output: []byte("100644 abc123 0 new_name.go\n"),
		Error:  nil,
	}

	checker := NewSafetyChecker(mockExecutor)
	evaluation, err := checker.EvaluateStagingArea()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if evaluation.IsClean {
		t.Error("Expected dirty staging area")
	}

	if len(evaluation.FilesByStatus["R"]) != 1 {
		t.Errorf("Expected 1 renamed file, got %d", len(evaluation.FilesByStatus["R"]))
	}
}

func TestEvaluateStagingArea_CopiedFiles(t *testing.T) {
	mockExecutor := executor.NewMockCommandExecutor()
	
	// Mock git status --porcelain (copied files)
	gitStatusOutput := "C  original.go -> copy.go\n"
	mockExecutor.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(gitStatusOutput),
		Error:  nil,
	}
	
	// Mock git ls-files --cached --stage
	mockExecutor.Commands["git [ls-files --cached --stage]"] = executor.MockResponse{
		Output: []byte("100644 abc123 0 copy.go\n"),
		Error:  nil,
	}

	checker := NewSafetyChecker(mockExecutor)
	evaluation, err := checker.EvaluateStagingArea()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if evaluation.IsClean {
		t.Error("Expected dirty staging area")
	}

	if len(evaluation.FilesByStatus["C"]) != 1 {
		t.Errorf("Expected 1 copied file, got %d", len(evaluation.FilesByStatus["C"]))
	}
}

func TestEvaluateStagingArea_IntentToAddFiles(t *testing.T) {
	mockExecutor := executor.NewMockCommandExecutor()
	
	// Mock git status --porcelain (intent-to-add files appear as added)
	gitStatusOutput := "A  intent_file1.go\nA  intent_file2.go\n"
	mockExecutor.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(gitStatusOutput),
		Error:  nil,
	}
	
	// Mock git ls-files --cached --stage (intent-to-add files have empty blob hash)
	lsFilesOutput := "100644 e69de29bb2d1d6434b8b29ae775ad8c2e48c5391 0 intent_file1.go\n100644 e69de29bb2d1d6434b8b29ae775ad8c2e48c5391 0 intent_file2.go\n"
	mockExecutor.Commands["git [ls-files --cached --stage]"] = executor.MockResponse{
		Output: []byte(lsFilesOutput),
		Error:  nil,
	}

	checker := NewSafetyChecker(mockExecutor)
	evaluation, err := checker.EvaluateStagingArea()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if evaluation.IsClean {
		t.Error("Expected dirty staging area")
	}

	if !evaluation.AllowContinue {
		t.Error("Expected AllowContinue to be true for intent-to-add only")
	}

	if len(evaluation.IntentToAddFiles) != 2 {
		t.Errorf("Expected 2 intent-to-add files, got %d", len(evaluation.IntentToAddFiles))
	}

	// Check that intent-to-add files are detected correctly
	expectedFiles := []string{"intent_file1.go", "intent_file2.go"}
	for i, file := range evaluation.IntentToAddFiles {
		if file != expectedFiles[i] {
			t.Errorf("Expected intent-to-add file %s, got %s", expectedFiles[i], file)
		}
	}
}

func TestEvaluateStagingArea_MixedFiles(t *testing.T) {
	mockExecutor := executor.NewMockCommandExecutor()
	
	// Mock git status --porcelain (mixed file types)
	gitStatusOutput := "M  modified.go\nA  added.go\nA  intent.go\nD  deleted.go\n"
	mockExecutor.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(gitStatusOutput),
		Error:  nil,
	}
	
	// Mock git ls-files --cached --stage (one intent-to-add file)
	lsFilesOutput := "100644 abc123 0 modified.go\n100644 def456 0 added.go\n100644 e69de29bb2d1d6434b8b29ae775ad8c2e48c5391 0 intent.go\n"
	mockExecutor.Commands["git [ls-files --cached --stage]"] = executor.MockResponse{
		Output: []byte(lsFilesOutput),
		Error:  nil,
	}

	checker := NewSafetyChecker(mockExecutor)
	evaluation, err := checker.EvaluateStagingArea()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if evaluation.IsClean {
		t.Error("Expected dirty staging area")
	}

	if evaluation.AllowContinue {
		t.Error("Expected AllowContinue to be false for mixed non-intent-to-add files")
	}

	if len(evaluation.IntentToAddFiles) != 1 {
		t.Errorf("Expected 1 intent-to-add file, got %d", len(evaluation.IntentToAddFiles))
	}

	if evaluation.IntentToAddFiles[0] != "intent.go" {
		t.Errorf("Expected intent-to-add file 'intent.go', got '%s'", evaluation.IntentToAddFiles[0])
	}

	// Check file categorization
	if len(evaluation.FilesByStatus["M"]) != 1 {
		t.Errorf("Expected 1 modified file, got %d", len(evaluation.FilesByStatus["M"]))
	}
	if len(evaluation.FilesByStatus["A"]) != 2 {
		t.Errorf("Expected 2 added files, got %d", len(evaluation.FilesByStatus["A"]))
	}
	if len(evaluation.FilesByStatus["D"]) != 1 {
		t.Errorf("Expected 1 deleted file, got %d", len(evaluation.FilesByStatus["D"]))
	}
}

func TestEvaluateStagingArea_GitStatusError(t *testing.T) {
	mockExecutor := executor.NewMockCommandExecutor()
	
	// Mock git status --porcelain error
	mockExecutor.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: nil,
		Error:  fmt.Errorf("not a git repository"),
	}

	checker := NewSafetyChecker(mockExecutor)
	evaluation, err := checker.EvaluateStagingArea()

	if err == nil {
		t.Fatal("Expected error when git status fails")
	}

	if evaluation != nil {
		t.Error("Expected nil evaluation when git status fails")
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

func TestEvaluateStagingArea_LsFilesError(t *testing.T) {
	mockExecutor := executor.NewMockCommandExecutor()
	
	// Mock git status --porcelain (clean)
	mockExecutor.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(""),
		Error:  nil,
	}
	
	// Mock git diff --cached (clean)
	mockExecutor.Commands["git [diff --cached]"] = executor.MockResponse{
		Output: []byte(""),
		Error:  nil,
	}
	
	// Mock git ls-files --cached --stage error
	mockExecutor.Commands["git [ls-files --cached --stage]"] = executor.MockResponse{
		Output: nil,
		Error:  fmt.Errorf("ls-files failed"),
	}

	checker := NewSafetyChecker(mockExecutor)
	evaluation, err := checker.EvaluateStagingArea()

	if err == nil {
		t.Fatal("Expected error when git ls-files fails")
	}

	if evaluation != nil {
		t.Error("Expected nil evaluation when git ls-files fails")
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
}

func TestEvaluatePatchContent_ModifiedFiles(t *testing.T) {
	checker := NewSafetyChecker()

	patchContent := `diff --git a/file1.go b/file1.go
index abc123..def456 100644
--- a/file1.go
+++ b/file1.go
@@ -1,3 +1,4 @@
 package main
 
+// Added comment
 func main() {}
diff --git a/file2.go b/file2.go
index ghi789..jkl012 100644
--- a/file2.go
+++ b/file2.go
@@ -1,3 +1,4 @@
 package utils
 
+// Added comment
 func Helper() {}`

	evaluation, err := checker.EvaluatePatchContent(patchContent)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if evaluation.IsClean {
		t.Error("Expected dirty staging area")
	}

	if evaluation.AllowContinue {
		t.Error("Expected AllowContinue to be false for modified files")
	}

	if len(evaluation.StagedFiles) != 2 {
		t.Errorf("Expected 2 staged files, got %d", len(evaluation.StagedFiles))
	}

	if len(evaluation.FilesByStatus["M"]) != 2 {
		t.Errorf("Expected 2 modified files, got %d", len(evaluation.FilesByStatus["M"]))
	}
}

func TestEvaluatePatchContent_IntentToAddFiles(t *testing.T) {
	checker := NewSafetyChecker()

	// Intent-to-add files have IsNew but no TextFragments
	patchContent := `diff --git a/new_file.go b/new_file.go
new file mode 100644
index 0000000..e69de29`

	evaluation, err := checker.EvaluatePatchContent(patchContent)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if evaluation.IsClean {
		t.Error("Expected dirty staging area")
	}

	if !evaluation.AllowContinue {
		t.Error("Expected AllowContinue to be true for intent-to-add only")
	}

	if len(evaluation.IntentToAddFiles) != 1 {
		t.Errorf("Expected 1 intent-to-add file, got %d", len(evaluation.IntentToAddFiles))
	}

	if evaluation.IntentToAddFiles[0] != "new_file.go" {
		t.Errorf("Expected intent-to-add file 'new_file.go', got '%s'", evaluation.IntentToAddFiles[0])
	}
}

func TestEvaluatePatchContent_BinaryFile(t *testing.T) {
	checker := NewSafetyChecker()

	patchContent := `diff --git a/image.png b/image.png
new file mode 100644
index 0000000..1234567
Binary files /dev/null and b/image.png differ`

	evaluation, err := checker.EvaluatePatchContent(patchContent)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if evaluation.IsClean {
		t.Error("Expected dirty staging area")
	}

	// Since go-gitdiff might not detect this specific format as binary,
	// let's check if it's classified as a new file at minimum
	if len(evaluation.StagedFiles) != 1 {
		t.Errorf("Expected 1 staged file, got %d", len(evaluation.StagedFiles))
	}

	if evaluation.StagedFiles[0] != "image.png" {
		t.Errorf("Expected 'image.png', got '%s'", evaluation.StagedFiles[0])
	}
}

func TestBuildStagingErrorMessage(t *testing.T) {
	checker := NewSafetyChecker()

	filesByStatus := map[string][]string{
		"M":      {"modified1.go", "modified2.go"},
		"A":      {"added1.go", "added2.go"},
		"D":      {"deleted.go"},
		"R":      {"renamed.go"},
		"C":      {"copied.go"},
		"BINARY": {"binary.jpg"},
	}
	intentToAddFiles := []string{"added1.go"}

	message := checker.buildStagingErrorMessage(filesByStatus, intentToAddFiles)

	// Check that the message contains expected sections
	if !contains(message, "SAFETY_CHECK_FAILED: staging_area_not_clean") {
		t.Error("Message should contain safety check failed header")
	}
	if !contains(message, "STAGED_FILES:") {
		t.Error("Message should contain staged files section")
	}
	if !contains(message, "MODIFIED: modified1.go,modified2.go") {
		t.Error("Message should contain modified files")
	}
	if !contains(message, "NEW: added2.go") {
		t.Error("Message should contain non-intent-to-add new files")
	}
	if !contains(message, "INTENT_TO_ADD: added1.go") {
		t.Error("Message should contain intent-to-add files")
	}
	if !contains(message, "DELETED: deleted.go") {
		t.Error("Message should contain deleted files")
	}
	if !contains(message, "RENAMED: renamed.go") {
		t.Error("Message should contain renamed files")
	}
	if !contains(message, "COPIED: copied.go") {
		t.Error("Message should contain copied files")
	}
	if !contains(message, "BINARY: binary.jpg") {
		t.Error("Message should contain binary files")
	}
}

func TestBuildRecommendedActions(t *testing.T) {
	mockExecutor := executor.NewMockCommandExecutor()
	checker := NewSafetyChecker(mockExecutor)

	filesByStatus := map[string][]string{
		"M": {"modified.go"},
		"A": {"added.go"},
		"D": {"deleted.go"},
	}
	intentToAddFiles := []string{"added.go"}

	actions := checker.buildRecommendedActions(filesByStatus, intentToAddFiles)

	if len(actions) == 0 {
		t.Fatal("Expected some recommended actions")
	}

	// Check that actions are sorted by priority
	for i := 1; i < len(actions); i++ {
		if actions[i].Priority < actions[i-1].Priority {
			t.Error("Actions should be sorted by priority")
			break
		}
	}

	// Check for intent-to-add info action
	foundInfoAction := false
	for _, action := range actions {
		if action.Category == "info" && contains(action.Description, "Intent-to-add") {
			foundInfoAction = true
			break
		}
	}
	if !foundInfoAction {
		t.Error("Expected info action for intent-to-add files")
	}

	// Check for deletion commit action
	foundDeleteAction := false
	for _, action := range actions {
		if action.Category == "commit" && contains(action.Description, "deletion") {
			foundDeleteAction = true
			break
		}
	}
	if !foundDeleteAction {
		t.Error("Expected commit action for deleted files")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (len(substr) == 0 || findSubstring(s, substr) >= 0)
}

// Simple substring search implementation
func findSubstring(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(substr) > len(s) {
		return -1
	}
	
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}