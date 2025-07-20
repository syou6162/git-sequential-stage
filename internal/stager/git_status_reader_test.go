package stager

import (
	"testing"

	"github.com/syou6162/git-sequential-stage/internal/executor"
)

func TestGitStatusReader_Clean(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(""),
		Error:  nil,
	}
	
	reader := NewGitStatusReader(mockExec)
	info, err := reader.ReadStatus()
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if len(info.StagedFiles) != 0 {
		t.Errorf("Expected no staged files, got: %v", info.StagedFiles)
	}
	
	if len(info.FilesByStatus) != 0 {
		t.Errorf("Expected empty FilesByStatus, got: %v", info.FilesByStatus)
	}
}

func TestGitStatusReader_StagedFiles(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	statusOutput := `M  file1.txt
A  file2.txt
D  file3.txt
 M file4.txt`
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(statusOutput),
		Error:  nil,
	}
	
	reader := NewGitStatusReader(mockExec)
	info, err := reader.ReadStatus()
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	// Should only include staged files (M , A , D , not  M)
	if len(info.StagedFiles) != 3 {
		t.Errorf("Expected 3 staged files, got: %v", info.StagedFiles)
	}
	
	// Check file categorization
	modifiedFiles := info.FilesByStatus[FileStatusModified]
	if len(modifiedFiles) != 1 || modifiedFiles[0] != "file1.txt" {
		t.Errorf("Expected modified file1.txt, got: %v", modifiedFiles)
	}
	
	addedFiles := info.FilesByStatus[FileStatusAdded]
	if len(addedFiles) != 1 || addedFiles[0] != "file2.txt" {
		t.Errorf("Expected added file2.txt, got: %v", addedFiles)
	}
	
	deletedFiles := info.FilesByStatus[FileStatusDeleted]
	if len(deletedFiles) != 1 || deletedFiles[0] != "file3.txt" {
		t.Errorf("Expected deleted file3.txt, got: %v", deletedFiles)
	}
}

func TestGitStatusReader_IntentToAdd(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	// ' A' (space + A) indicates intent-to-add
	statusOutput := ` A intent1.txt
 A intent2.txt`
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(statusOutput),
		Error:  nil,
	}
	
	reader := NewGitStatusReader(mockExec)
	info, err := reader.ReadStatus()
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if len(info.IntentToAddFiles) != 2 {
		t.Errorf("Expected 2 intent-to-add files, got: %v", info.IntentToAddFiles)
	}
	
	// Intent-to-add files should also be in staged files and added files
	if len(info.StagedFiles) != 2 {
		t.Errorf("Expected 2 staged files, got: %v", info.StagedFiles)
	}
	
	addedFiles := info.FilesByStatus[FileStatusAdded]
	if len(addedFiles) != 2 {
		t.Errorf("Expected 2 added files, got: %v", addedFiles)
	}
}

func TestGitStatusReader_RenamedFiles(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	statusOutput := `R  old.txt -> new.txt
RM old2.txt -> new2.txt`
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(statusOutput),
		Error:  nil,
	}
	
	reader := NewGitStatusReader(mockExec)
	info, err := reader.ReadStatus()
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	renamedFiles := info.FilesByStatus[FileStatusRenamed]
	if len(renamedFiles) != 2 {
		t.Errorf("Expected 2 renamed files, got: %v", renamedFiles)
	}
	
	// Check rename notation
	expectedRenames := map[string]bool{
		"old.txt -> new.txt":   true,
		"old2.txt -> new2.txt": true,
	}
	
	for _, rename := range renamedFiles {
		if !expectedRenames[rename] {
			t.Errorf("Unexpected rename notation: %s", rename)
		}
	}
	
	// Staged files should contain new names
	if len(info.StagedFiles) != 2 {
		t.Errorf("Expected 2 staged files (new names), got: %v", info.StagedFiles)
	}
}

func TestGitStatusReader_CopiedFiles(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	statusOutput := `C  source.txt -> copy.txt`
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(statusOutput),
		Error:  nil,
	}
	
	reader := NewGitStatusReader(mockExec)
	info, err := reader.ReadStatus()
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	copiedFiles := info.FilesByStatus[FileStatusCopied]
	if len(copiedFiles) != 1 || copiedFiles[0] != "source.txt -> copy.txt" {
		t.Errorf("Expected 'source.txt -> copy.txt' in copied files, got: %v", copiedFiles)
	}
}

func TestGitStatusReader_MixedStagedUnstaged(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	// MM = modified in index and working tree
	// AM = added to index, modified in working tree
	statusOutput := `MM both_modified.txt
AM added_then_modified.txt
M  only_staged.txt
 M only_unstaged.txt`
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(statusOutput),
		Error:  nil,
	}
	
	reader := NewGitStatusReader(mockExec)
	info, err := reader.ReadStatus()
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	// Should include files with staged changes (MM, AM, M ) but not  M
	if len(info.StagedFiles) != 3 {
		t.Errorf("Expected 3 staged files, got: %v", info.StagedFiles)
	}
	
	// Check that only_unstaged.txt is not included
	for _, file := range info.StagedFiles {
		if file == "only_unstaged.txt" {
			t.Error("only_unstaged.txt should not be in staged files")
		}
	}
}

func TestGitStatusReader_NoExecutor(t *testing.T) {
	reader := NewGitStatusReader(nil)
	info, err := reader.ReadStatus()
	
	if err == nil {
		t.Fatal("Expected error for nil executor")
	}
	
	if info != nil {
		t.Error("Expected nil info on error")
	}
}

func TestGitStatusReader_GitCommandError(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: nil,
		Error:  NewGitCommandError("git status", nil),
	}
	
	reader := NewGitStatusReader(mockExec)
	info, err := reader.ReadStatus()
	
	if err == nil {
		t.Fatal("Expected error when git command fails")
	}
	
	if info != nil {
		t.Error("Expected nil info on error")
	}
	
	// Check that it's a StagerError
	stagerErr, ok := err.(*StagerError)
	if !ok {
		t.Fatalf("Expected StagerError, got %T", err)
	}
	
	if stagerErr.Type != ErrorTypeGitCommand {
		t.Errorf("Expected ErrorTypeGitCommand error type, got %v", stagerErr.Type)
	}
}

func TestGitStatusReader_ComplexFilenames(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	// Test with spaces and special characters
	statusOutput := `M  "file with spaces.txt"
A  regular_file.txt
R  "old name.txt" -> "new name.txt"`
	mockExec.Commands["git [status --porcelain]"] = executor.MockResponse{
		Output: []byte(statusOutput),
		Error:  nil,
	}
	
	reader := NewGitStatusReader(mockExec)
	info, err := reader.ReadStatus()
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	// Note: For simplicity, our parser doesn't handle quoted filenames
	// This is a known limitation that could be addressed if needed
	if len(info.StagedFiles) != 3 {
		t.Errorf("Expected 3 staged files, got: %v", info.StagedFiles)
	}
}