package stager

import (
	"fmt"
	"strings"
)

// ErrorType represents the type of error that occurred
type ErrorType int

const (
	// ErrorTypeUnknown is for unknown errors
	ErrorTypeUnknown ErrorType = iota
	// ErrorTypeFileNotFound is when a file cannot be found
	ErrorTypeFileNotFound
	// ErrorTypeParsing is for parsing errors
	ErrorTypeParsing
	// ErrorTypeGitCommand is for git command failures
	ErrorTypeGitCommand
	// ErrorTypeHunkNotFound is when a hunk cannot be found
	ErrorTypeHunkNotFound
	// ErrorTypeInvalidArgument is for invalid arguments
	ErrorTypeInvalidArgument
	// ErrorTypeDependencyMissing is when a required dependency is missing
	ErrorTypeDependencyMissing
	// ErrorTypeIO is for I/O errors
	ErrorTypeIO
	// ErrorTypePatchApplication is when applying a patch fails
	ErrorTypePatchApplication
)

// StagerError represents a custom error with type classification
type StagerError struct {
	Type    ErrorType
	Message string
	Err     error
}

// Error implements the error interface
func (e *StagerError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap allows errors.Is and errors.As to work
func (e *StagerError) Unwrap() error {
	return e.Err
}

// Is allows comparison with error types
func (e *StagerError) Is(target error) bool {
	t, ok := target.(*StagerError)
	if !ok {
		return false
	}
	return e.Type == t.Type
}

// NewStagerError creates a new StagerError
func NewStagerError(errType ErrorType, message string, err error) *StagerError {
	return &StagerError{
		Type:    errType,
		Message: message,
		Err:     err,
	}
}


// Common error constructors

// NewFileNotFoundError creates a file not found error
func NewFileNotFoundError(filename string, err error) *StagerError {
	return NewStagerError(ErrorTypeFileNotFound,
		fmt.Sprintf("file not found: %s", filename), err)
}

// NewParsingError creates a parsing error
func NewParsingError(what string, err error) *StagerError {
	return NewStagerError(ErrorTypeParsing,
		fmt.Sprintf("failed to parse %s", what), err)
}

// NewGitCommandError creates a git command error
func NewGitCommandError(command string, err error) *StagerError {
	return NewStagerError(ErrorTypeGitCommand,
		fmt.Sprintf("git command failed: %s", command), err)
}

// NewHunkNotFoundError creates a hunk not found error
func NewHunkNotFoundError(description string, err error) *StagerError {
	return NewStagerError(ErrorTypeHunkNotFound,
		fmt.Sprintf("not found: %s", description), err)
}

// NewInvalidArgumentError creates an invalid argument error
func NewInvalidArgumentError(description string, err error) *StagerError {
	return NewStagerError(ErrorTypeInvalidArgument,
		description, err)
}

// NewDependencyMissingError creates a dependency missing error
func NewDependencyMissingError(dependency string) *StagerError {
	return NewStagerError(ErrorTypeDependencyMissing,
		fmt.Sprintf("%s command not found", dependency), nil)
}

// NewIOError creates an I/O error
func NewIOError(operation string, err error) *StagerError {
	return NewStagerError(ErrorTypeIO,
		fmt.Sprintf("I/O error during %s", operation), err)
}

// NewPatchApplicationError creates a patch application error
func NewPatchApplicationError(patchID string, err error) *StagerError {
	return NewStagerError(ErrorTypePatchApplication,
		fmt.Sprintf("failed to apply patch with ID %s", patchID), err)
}

// SafetyErrorType represents the type of safety-related error
type SafetyErrorType int

const (
	// StagingAreaNotClean indicates the staging area has existing staged files
	StagingAreaNotClean SafetyErrorType = iota
	// NewFileConflict indicates a new file is already staged
	NewFileConflict
	// DeletedFileConflict indicates a deleted file conflict
	DeletedFileConflict
	// RenamedFileConflict indicates a renamed file conflict
	RenamedFileConflict
	// GitOperationFailed indicates a git operation failed
	GitOperationFailed
	// IntentToAddProcessing indicates an error during intent-to-add file processing
	IntentToAddProcessing
	// DeprecatedMethod indicates a deprecated method was called
	DeprecatedMethod
)

// SafetyError represents a safety-related error with detailed context
type SafetyError struct {
	Type       SafetyErrorType
	Message    string
	Advice     string
	Underlying error
}

// NewSafetyError creates a new SafetyError
func NewSafetyError(errorType SafetyErrorType, message, advice string, underlying error) *SafetyError {
	return &SafetyError{
		Type:       errorType,
		Message:    message,
		Advice:     advice,
		Underlying: underlying,
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


// String returns a string representation of the error type
func (t SafetyErrorType) String() string {
	switch t {
	case StagingAreaNotClean:
		return "StagingAreaNotClean"
	case NewFileConflict:
		return "NewFileConflict"
	case DeletedFileConflict:
		return "DeletedFileConflict"
	case RenamedFileConflict:
		return "RenamedFileConflict"
	case GitOperationFailed:
		return "GitOperationFailed"
	case IntentToAddProcessing:
		return "IntentToAddProcessing"
	default:
		return "Unknown"
	}
}
