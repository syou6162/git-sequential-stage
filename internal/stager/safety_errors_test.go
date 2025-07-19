package stager

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestNewSafetyError(t *testing.T) {
	tests := []struct {
		name       string
		errorType  SafetyErrorType
		message    string
		advice     string
		underlying error
	}{
		{
			name:       "staging area not clean",
			errorType:  ErrorTypeStagingAreaNotClean,
			message:    "Staging area contains modified files",
			advice:     "Commit or reset staged files first",
			underlying: nil,
		},
		{
			name:       "new file conflict",
			errorType:  ErrorTypeNewFileConflict,
			message:    "New file already exists in index",
			advice:     "Reset the file first",
			underlying: errors.New("git error"),
		},
		{
			name:       "deleted file conflict",
			errorType:  ErrorTypeDeletedFileConflict,
			message:    "File does not exist in index",
			advice:     "File already deleted",
			underlying: nil,
		},
		{
			name:       "renamed file conflict",
			errorType:  ErrorTypeRenamedFileConflict,
			message:    "Renamed file conflict",
			advice:     "Check file status",
			underlying: nil,
		},
		{
			name:       "git operation failed",
			errorType:  ErrorTypeGitOperationFailed,
			message:    "Git command failed",
			advice:     "Check git status",
			underlying: errors.New("command failed"),
		},
		{
			name:       "intent-to-add processing",
			errorType:  ErrorTypeIntentToAddProcessing,
			message:    "Failed to process intent-to-add file",
			advice:     "Check file permissions",
			underlying: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewSafetyError(tt.errorType, tt.message, tt.advice, tt.underlying)

			if err == nil {
				t.Fatal("NewSafetyError returned nil")
			}

			if err.Type != tt.errorType {
				t.Errorf("Type = %v, want %v", err.Type, tt.errorType)
			}

			if err.Message != tt.message {
				t.Errorf("Message = %v, want %v", err.Message, tt.message)
			}

			if err.Advice != tt.advice {
				t.Errorf("Advice = %v, want %v", err.Advice, tt.advice)
			}

			if err.Underlying != tt.underlying {
				t.Errorf("Underlying = %v, want %v", err.Underlying, tt.underlying)
			}

			if err.Context == nil {
				t.Error("Context is nil, want initialized map")
			}
		})
	}
}

func TestSafetyError_Error(t *testing.T) {
	tests := []struct {
		name           string
		err            *SafetyError
		expectedParts  []string
		unexpectedParts []string
	}{
		{
			name: "basic error",
			err:  NewSafetyError(ErrorTypeStagingAreaNotClean, "Test message", "", nil),
			expectedParts: []string{
				"Safety Error: Test message",
			},
			unexpectedParts: []string{
				"Advice:",
				"Underlying error:",
			},
		},
		{
			name: "error with advice",
			err:  NewSafetyError(ErrorTypeStagingAreaNotClean, "Test message", "Test advice", nil),
			expectedParts: []string{
				"Safety Error: Test message",
				"Advice: Test advice",
			},
			unexpectedParts: []string{
				"Underlying error:",
			},
		},
		{
			name: "error with underlying",
			err:  NewSafetyError(ErrorTypeStagingAreaNotClean, "Test message", "", errors.New("underlying")),
			expectedParts: []string{
				"Safety Error: Test message",
				"Underlying error: underlying",
			},
			unexpectedParts: []string{
				"Advice:",
			},
		},
		{
			name: "error with all fields",
			err:  NewSafetyError(ErrorTypeStagingAreaNotClean, "Test message", "Test advice", errors.New("underlying")),
			expectedParts: []string{
				"Safety Error: Test message",
				"Advice: Test advice",
				"Underlying error: underlying",
			},
			unexpectedParts: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errStr := tt.err.Error()

			for _, part := range tt.expectedParts {
				if !strings.Contains(errStr, part) {
					t.Errorf("Error string missing expected part: %q\nGot: %q", part, errStr)
				}
			}

			for _, part := range tt.unexpectedParts {
				if strings.Contains(errStr, part) {
					t.Errorf("Error string contains unexpected part: %q\nGot: %q", part, errStr)
				}
			}
		})
	}
}

func TestSafetyError_Unwrap(t *testing.T) {
	underlying := errors.New("underlying error")
	err := NewSafetyError(ErrorTypeStagingAreaNotClean, "Test", "", underlying)

	unwrapped := err.Unwrap()
	if unwrapped != underlying {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlying)
	}

	errNoUnderlying := NewSafetyError(ErrorTypeStagingAreaNotClean, "Test", "", nil)
	if errNoUnderlying.Unwrap() != nil {
		t.Errorf("Unwrap() = %v, want nil", errNoUnderlying.Unwrap())
	}
}

func TestSafetyError_Is(t *testing.T) {
	underlying := errors.New("underlying")
	
	tests := []struct {
		name   string
		err    *SafetyError
		target error
		want   bool
	}{
		{
			name:   "same type SafetyError",
			err:    NewSafetyError(ErrorTypeStagingAreaNotClean, "Test", "", nil),
			target: NewSafetyError(ErrorTypeStagingAreaNotClean, "Different", "", nil),
			want:   true,
		},
		{
			name:   "different type SafetyError",
			err:    NewSafetyError(ErrorTypeStagingAreaNotClean, "Test", "", nil),
			target: NewSafetyError(ErrorTypeNewFileConflict, "Test", "", nil),
			want:   false,
		},
		{
			name:   "nil target",
			err:    NewSafetyError(ErrorTypeStagingAreaNotClean, "Test", "", nil),
			target: nil,
			want:   false,
		},
		{
			name:   "underlying error match",
			err:    NewSafetyError(ErrorTypeStagingAreaNotClean, "Test", "", underlying),
			target: underlying,
			want:   true,
		},
		{
			name:   "non-SafetyError target without underlying",
			err:    NewSafetyError(ErrorTypeStagingAreaNotClean, "Test", "", nil),
			target: errors.New("different"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Is(tt.target); got != tt.want {
				t.Errorf("Is() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSafetyError_Context(t *testing.T) {
	err := NewSafetyError(ErrorTypeStagingAreaNotClean, "Test", "", nil)

	// Test WithContext
	err.WithContext("file", "test.go")
	err.WithContext("line", 42)

	// Test GetContext
	file, exists := err.GetContext("file")
	if !exists {
		t.Error("GetContext(\"file\") returned exists=false, want true")
	}
	if file != "test.go" {
		t.Errorf("GetContext(\"file\") = %v, want \"test.go\"", file)
	}

	line, exists := err.GetContext("line")
	if !exists {
		t.Error("GetContext(\"line\") returned exists=false, want true")
	}
	if line != 42 {
		t.Errorf("GetContext(\"line\") = %v, want 42", line)
	}

	// Test non-existent key
	_, exists = err.GetContext("nonexistent")
	if exists {
		t.Error("GetContext(\"nonexistent\") returned exists=true, want false")
	}
}

func TestSafetyError_ChainedContext(t *testing.T) {
	err := NewSafetyError(ErrorTypeStagingAreaNotClean, "Test", "", nil)
	
	// Test chained WithContext calls
	err.WithContext("key1", "value1").WithContext("key2", "value2")
	
	val1, _ := err.GetContext("key1")
	if val1 != "value1" {
		t.Errorf("GetContext(\"key1\") = %v, want \"value1\"", val1)
	}
	
	val2, _ := err.GetContext("key2") 
	if val2 != "value2" {
		t.Errorf("GetContext(\"key2\") = %v, want \"value2\"", val2)
	}
}

func TestSafetyError_ErrorsIs(t *testing.T) {
	// Test with errors.Is from standard library
	underlying := errors.New("underlying")
	err := NewSafetyError(ErrorTypeStagingAreaNotClean, "Test", "", underlying)
	
	// Should match underlying error
	if !errors.Is(err, underlying) {
		t.Error("errors.Is(err, underlying) = false, want true")
	}
	
	// Should match same type SafetyError
	targetErr := NewSafetyError(ErrorTypeStagingAreaNotClean, "Different", "", nil)
	if !errors.Is(err, targetErr) {
		t.Error("errors.Is(err, targetErr) = false, want true")
	}
	
	// Should not match different type SafetyError
	differentErr := NewSafetyError(ErrorTypeNewFileConflict, "Test", "", nil)
	if errors.Is(err, differentErr) {
		t.Error("errors.Is(err, differentErr) = true, want false")
	}
}

func TestSafetyError_ErrorsAs(t *testing.T) {
	// Test with errors.As from standard library
	err := NewSafetyError(ErrorTypeStagingAreaNotClean, "Test", "Advice", nil)
	
	var safetyErr *SafetyError
	if !errors.As(err, &safetyErr) {
		t.Fatal("errors.As(err, &safetyErr) = false, want true")
	}
	
	if safetyErr.Type != ErrorTypeStagingAreaNotClean {
		t.Errorf("Type = %v, want %v", safetyErr.Type, ErrorTypeStagingAreaNotClean)
	}
	
	if safetyErr.Message != "Test" {
		t.Errorf("Message = %v, want \"Test\"", safetyErr.Message)
	}
}

func TestSafetyErrorType_String(t *testing.T) {
	// Ensure error types can be used in fmt.Sprintf
	tests := []struct {
		errorType SafetyErrorType
		expected  string
	}{
		{ErrorTypeStagingAreaNotClean, "0"},
		{ErrorTypeNewFileConflict, "1"},
		{ErrorTypeDeletedFileConflict, "2"},
		{ErrorTypeRenamedFileConflict, "3"},
		{ErrorTypeGitOperationFailed, "4"},
		{ErrorTypeIntentToAddProcessing, "5"},
	}
	
	for _, tt := range tests {
		t.Run(fmt.Sprintf("ErrorType_%s", tt.expected), func(t *testing.T) {
			result := fmt.Sprintf("%d", tt.errorType)
			if result != tt.expected {
				t.Errorf("fmt.Sprintf(\"%%d\", %v) = %v, want %v", tt.errorType, result, tt.expected)
			}
		})
	}
}