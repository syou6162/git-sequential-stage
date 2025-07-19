package stager

import (
	"fmt"
	"strings"
)

// SafetyErrorType represents the type of safety-related error
type SafetyErrorType int

const (
	// ErrorTypeStagingAreaNotClean indicates the staging area has existing staged files
	ErrorTypeStagingAreaNotClean SafetyErrorType = iota
	// ErrorTypeNewFileConflict indicates a new file is already staged
	ErrorTypeNewFileConflict
	// ErrorTypeDeletedFileConflict indicates a deleted file conflict
	ErrorTypeDeletedFileConflict
	// ErrorTypeRenamedFileConflict indicates a renamed file conflict
	ErrorTypeRenamedFileConflict
	// ErrorTypeGitOperationFailed indicates a git operation failed
	ErrorTypeGitOperationFailed
	// ErrorTypeIntentToAddProcessing indicates an error during intent-to-add file processing
	ErrorTypeIntentToAddProcessing
)

// SafetyError represents a safety-related error with detailed context
type SafetyError struct {
	Type       SafetyErrorType
	Message    string
	Advice     string
	Underlying error
	Context    map[string]interface{}
}

// NewSafetyError creates a new SafetyError
func NewSafetyError(errorType SafetyErrorType, message, advice string, underlying error) *SafetyError {
	return &SafetyError{
		Type:       errorType,
		Message:    message,
		Advice:     advice,
		Underlying: underlying,
		Context:    make(map[string]interface{}),
	}
}

// Error returns the formatted error message
func (e *SafetyError) Error() string {
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Safety Error: %s", e.Message))

	if e.Advice != "" {
		result.WriteString(fmt.Sprintf("\nAdvice: %s", e.Advice))
	}

	if e.Underlying != nil {
		result.WriteString(fmt.Sprintf("\nUnderlying error: %v", e.Underlying))
	}

	return result.String()
}

// Is implements error comparison for errors.Is
func (e *SafetyError) Is(target error) bool {
	t, ok := target.(*SafetyError)
	if !ok {
		return false
	}
	return e.Type == t.Type
}

// Unwrap returns the underlying error
func (e *SafetyError) Unwrap() error {
	return e.Underlying
}

// WithContext adds context information to the error
func (e *SafetyError) WithContext(key string, value interface{}) *SafetyError {
	e.Context[key] = value
	return e
}

// GetContext retrieves context information from the error
func (e *SafetyError) GetContext(key string) (interface{}, bool) {
	value, exists := e.Context[key]
	return value, exists
}

// String returns a string representation of the error type
func (t SafetyErrorType) String() string {
	switch t {
	case ErrorTypeStagingAreaNotClean:
		return "StagingAreaNotClean"
	case ErrorTypeNewFileConflict:
		return "NewFileConflict"
	case ErrorTypeDeletedFileConflict:
		return "DeletedFileConflict"
	case ErrorTypeRenamedFileConflict:
		return "RenamedFileConflict"
	case ErrorTypeGitOperationFailed:
		return "GitOperationFailed"
	case ErrorTypeIntentToAddProcessing:
		return "IntentToAddProcessing"
	default:
		return "Unknown"
	}
}