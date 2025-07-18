package stager

import (
	"fmt"
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

// StagerError represents a custom error with additional context
type StagerError struct {
	Type    ErrorType
	Message string
	Err     error
	Context map[string]interface{}
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
		Context: make(map[string]interface{}),
	}
}

// WithContext adds context to the error
func (e *StagerError) WithContext(key string, value interface{}) *StagerError {
	e.Context[key] = value
	return e
}

// Common error constructors

// NewFileNotFoundError creates a file not found error
func NewFileNotFoundError(filename string, err error) *StagerError {
	return NewStagerError(ErrorTypeFileNotFound, 
		fmt.Sprintf("file not found: %s", filename), err).
		WithContext("filename", filename)
}

// NewParsingError creates a parsing error
func NewParsingError(what string, err error) *StagerError {
	return NewStagerError(ErrorTypeParsing,
		fmt.Sprintf("failed to parse %s", what), err).
		WithContext("parsing", what)
}

// NewGitCommandError creates a git command error
func NewGitCommandError(command string, err error) *StagerError {
	return NewStagerError(ErrorTypeGitCommand,
		fmt.Sprintf("git command failed: %s", command), err).
		WithContext("command", command)
}

// NewHunkNotFoundError creates a hunk not found error
func NewHunkNotFoundError(description string, err error) *StagerError {
	return NewStagerError(ErrorTypeHunkNotFound,
		fmt.Sprintf("not found: %s", description), err).
		WithContext("description", description)
}

// NewInvalidArgumentError creates an invalid argument error
func NewInvalidArgumentError(description string, err error) *StagerError {
	return NewStagerError(ErrorTypeInvalidArgument,
		description, err).
		WithContext("description", description)
}

// NewDependencyMissingError creates a dependency missing error
func NewDependencyMissingError(dependency string) *StagerError {
	return NewStagerError(ErrorTypeDependencyMissing,
		fmt.Sprintf("%s command not found", dependency), nil).
		WithContext("dependency", dependency)
}

// NewIOError creates an I/O error
func NewIOError(operation string, err error) *StagerError {
	return NewStagerError(ErrorTypeIO,
		fmt.Sprintf("I/O error during %s", operation), err).
		WithContext("operation", operation)
}

// NewPatchApplicationError creates a patch application error
func NewPatchApplicationError(patchID string, err error) *StagerError {
	return NewStagerError(ErrorTypePatchApplication,
		fmt.Sprintf("failed to apply patch with ID %s", patchID), err).
		WithContext("patch_id", patchID)
}