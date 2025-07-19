package stager

import "testing"

func TestFileStatus_String(t *testing.T) {
	tests := []struct {
		status   FileStatus
		expected string
	}{
		{FileStatusModified, "MODIFIED"},
		{FileStatusAdded, "ADDED"},
		{FileStatusDeleted, "DELETED"},
		{FileStatusRenamed, "RENAMED"},
		{FileStatusCopied, "COPIED"},
		{FileStatusBinary, "BINARY"},
		{FileStatus(999), "UNKNOWN"}, // Test unknown status
	}

	for _, test := range tests {
		if test.status.String() != test.expected {
			t.Errorf("FileStatus(%d).String() = %s, expected %s", 
				test.status, test.status.String(), test.expected)
		}
	}
}

func TestActionCategory_String(t *testing.T) {
	tests := []struct {
		category ActionCategory
		expected string
	}{
		{ActionCategoryInfo, "info"},
		{ActionCategoryCommit, "commit"},
		{ActionCategoryUnstage, "unstage"},
		{ActionCategoryReset, "reset"},
		{ActionCategory(999), "unknown"}, // Test unknown category
	}

	for _, test := range tests {
		if test.category.String() != test.expected {
			t.Errorf("ActionCategory(%d).String() = %s, expected %s", 
				test.category, test.category.String(), test.expected)
		}
	}
}

func TestEnumTypeSafety(t *testing.T) {
	// Test that we can create maps with enum keys
	filesByStatus := make(map[FileStatus][]string)
	filesByStatus[FileStatusModified] = []string{"test.go"}
	filesByStatus[FileStatusAdded] = []string{"new.go"}
	
	// Test accessing enum map
	if len(filesByStatus[FileStatusModified]) != 1 {
		t.Error("Expected 1 modified file")
	}
	
	if filesByStatus[FileStatusModified][0] != "test.go" {
		t.Error("Expected test.go in modified files")
	}
	
	// Test RecommendedAction with enum category
	action := RecommendedAction{
		Description: "Test action",
		Commands:    []string{"git commit"},
		Priority:    1,
		Category:    ActionCategoryCommit,
	}
	
	if action.Category != ActionCategoryCommit {
		t.Error("Expected ActionCategoryCommit")
	}
	
	if action.Category.String() != "commit" {
		t.Errorf("Expected 'commit', got %s", action.Category.String())
	}
}