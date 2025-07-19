package stager

import (
	"errors"
	"testing"
)

func TestNewSafetyError(t *testing.T) {
	tests := []struct {
		name           string
		errorType      SafetyErrorType
		message        string
		advice         string
		underlying     error
		expectedError  string
	}{
		{
			name:          "staging area not clean error",
			errorType:     StagingAreaNotClean,
			message:       "staging area contains already staged files",
			advice:        "commit or reset staged changes first",
			underlying:    nil,
			expectedError: "Safety Error: staging area contains already staged files\nAdvice: commit or reset staged changes first",
		},
		{
			name:          "new file conflict with underlying error",
			errorType:     NewFileConflict,
			message:       "new file already exists in index",
			advice:        "run 'git reset HEAD file.txt' to unstage",
			underlying:    errors.New("already exists in index"),
			expectedError: "Safety Error: new file already exists in index\nAdvice: run 'git reset HEAD file.txt' to unstage\nUnderlying error: already exists in index",
		},
		{
			name:          "git operation failed",
			errorType:     GitOperationFailed,
			message:       "failed to check staging area",
			advice:        "",
			underlying:    errors.New("git status failed"),
			expectedError: "Safety Error: failed to check staging area\nUnderlying error: git status failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewSafetyError(tt.errorType, tt.message, tt.advice, tt.underlying)
			
			if err.Type != tt.errorType {
				t.Errorf("expected error type %v, got %v", tt.errorType, err.Type)
			}
			
			if err.Message != tt.message {
				t.Errorf("expected message %q, got %q", tt.message, err.Message)
			}
			
			if err.Advice != tt.advice {
				t.Errorf("expected advice %q, got %q", tt.advice, err.Advice)
			}
			
			if err.Error() != tt.expectedError {
				t.Errorf("expected error string:\n%q\ngot:\n%q", tt.expectedError, err.Error())
			}
		})
	}
}

func TestSafetyError_Is(t *testing.T) {
	err1 := NewSafetyError(StagingAreaNotClean, "test", "", nil)
	err2 := NewSafetyError(StagingAreaNotClean, "different message", "", nil)
	err3 := NewSafetyError(NewFileConflict, "test", "", nil)
	regularErr := errors.New("regular error")

	if !errors.Is(err1, err2) {
		t.Error("expected errors with same type to match")
	}

	if errors.Is(err1, err3) {
		t.Error("expected errors with different types not to match")
	}

	if errors.Is(err1, regularErr) {
		t.Error("expected SafetyError not to match regular error")
	}
}

func TestSafetyError_Unwrap(t *testing.T) {
	underlying := errors.New("underlying error")
	err := NewSafetyError(ErrorTypeGitOperationFailed, "test", "", underlying)

	if errors.Unwrap(err) != underlying {
		t.Error("expected Unwrap to return underlying error")
	}

	errNoUnderlying := NewSafetyError(ErrorTypeStagingAreaNotClean, "test", "", nil)
	if errors.Unwrap(errNoUnderlying) != nil {
		t.Error("expected Unwrap to return nil when no underlying error")
	}
}

func TestSafetyError_Context(t *testing.T) {
	err := NewSafetyError(ErrorTypeStagingAreaNotClean, "test", "", nil)

	// Test adding context
	err.WithContext("files", []string{"file1.txt", "file2.txt"})
	err.WithContext("count", 42)

	// Test retrieving context
	files, exists := err.GetContext("files")
	if !exists {
		t.Error("expected 'files' context to exist")
	}
	if fileList, ok := files.([]string); !ok || len(fileList) != 2 {
		t.Error("expected 'files' context to contain correct value")
	}

	count, exists := err.GetContext("count")
	if !exists {
		t.Error("expected 'count' context to exist")
	}
	if countVal, ok := count.(int); !ok || countVal != 42 {
		t.Error("expected 'count' context to contain correct value")
	}

	// Test non-existent context
	_, exists = err.GetContext("nonexistent")
	if exists {
		t.Error("expected non-existent context to return false")
	}
}

func TestSafetyErrorType_String(t *testing.T) {
	tests := []struct {
		errorType SafetyErrorType
		expected  string
	}{
		{ErrorTypeStagingAreaNotClean, "StagingAreaNotClean"},
		{ErrorTypeNewFileConflict, "NewFileConflict"},
		{ErrorTypeDeletedFileConflict, "DeletedFileConflict"},
		{ErrorTypeRenamedFileConflict, "RenamedFileConflict"},
		{ErrorTypeGitOperationFailed, "GitOperationFailed"},
		{ErrorTypeIntentToAddProcessing, "IntentToAddProcessing"},
		{SafetyErrorType(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if result := tt.errorType.String(); result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSafetyError_MethodChaining(t *testing.T) {
	err := NewSafetyError(ErrorTypeStagingAreaNotClean, "test", "", nil).
		WithContext("file", "test.txt").
		WithContext("line", 42)

	file, _ := err.GetContext("file")
	if file != "test.txt" {
		t.Error("expected method chaining to work correctly")
	}

	line, _ := err.GetContext("line")
	if line != 42 {
		t.Error("expected multiple context values to be stored")
	}
}