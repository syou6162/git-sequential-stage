package stager

import (
	"fmt"
	"strings"
)

// SafetyErrorType represents the type of safety error
type SafetyErrorType int

const (
	// ErrorTypeStagingAreaNotClean indicates the staging area contains files
	ErrorTypeStagingAreaNotClean SafetyErrorType = iota
	// ErrorTypeNewFileConflict indicates a new file already exists in index
	ErrorTypeNewFileConflict
	// ErrorTypeDeletedFileConflict indicates a deleted file does not exist in index
	ErrorTypeDeletedFileConflict
	// ErrorTypeRenamedFileConflict indicates a renamed file conflict
	ErrorTypeRenamedFileConflict
	// ErrorTypeGitOperationFailed indicates a git operation failed
	ErrorTypeGitOperationFailed
	// ErrorTypeIntentToAddProcessing indicates an error processing intent-to-add files
	ErrorTypeIntentToAddProcessing
)

// SafetyError represents a safety-related error with advice
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

// Error implements the error interface
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

// Unwrap returns the underlying error for errors.Is/As support
func (e *SafetyError) Unwrap() error {
	return e.Underlying
}

// Is implements error comparison for errors.Is
func (e *SafetyError) Is(target error) bool {
	if target == nil {
		return false
	}

	// Check if target is a SafetyError with same type
	if targetSafety, ok := target.(*SafetyError); ok {
		return e.Type == targetSafety.Type
	}

	// Check underlying error
	if e.Underlying != nil {
		return e.Underlying == target
	}

	return false
}

// WithContext adds context information to the error
func (e *SafetyError) WithContext(key string, value interface{}) *SafetyError {
	e.Context[key] = value
	return e
}

// GetContext retrieves context information from the error
func (e *SafetyError) GetContext(key string) (interface{}, bool) {
	val, exists := e.Context[key]
	return val, exists
}