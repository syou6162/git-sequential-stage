package logger

import (
	"fmt"
	"io"
	"os"
)

// Level represents the logging level
type Level int

const (
	// ErrorLevel logs only errors
	ErrorLevel Level = iota
	// InfoLevel logs errors and info messages
	InfoLevel
	// DebugLevel logs everything including debug messages
	DebugLevel
)

// Logger provides structured logging functionality
type Logger struct {
	level  Level
	output io.Writer
}

// New creates a new logger with the specified level
func New(level Level) *Logger {
	return &Logger{
		level:  level,
		output: os.Stderr,
	}
}

// NewFromEnv creates a logger based on environment variable
func NewFromEnv() *Logger {
	level := ErrorLevel
	if os.Getenv("GIT_SEQUENTIAL_STAGE_VERBOSE") != "" {
		level = DebugLevel
	}
	return New(level)
}

// SetOutput sets the output writer for the logger
func (l *Logger) SetOutput(w io.Writer) {
	l.output = w
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	if l.level >= ErrorLevel {
		_, _ = fmt.Fprintf(l.output, "[ERROR] "+format+"\n", args...)
	}
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	if l.level >= InfoLevel {
		_, _ = fmt.Fprintf(l.output, "[INFO] "+format+"\n", args...)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level >= DebugLevel {
		_, _ = fmt.Fprintf(l.output, "[DEBUG] "+format+"\n", args...)
	}
}

// Printf provides compatibility with existing code
func (l *Logger) Printf(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(l.output, format, args...)
}
