package validator

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/syou6162/git-sequential-stage/internal/executor"
)

// Validator handles dependency checks and argument validation
type Validator struct {
	executor executor.CommandExecutor
}

// NewValidator creates a new validator
func NewValidator(exec executor.CommandExecutor) *Validator {
	return &Validator{
		executor: exec,
	}
}

// CheckDependencies checks if required commands are available
func (v *Validator) CheckDependencies() error {
	// Check git
	if _, err := v.executor.Execute("git", "--version"); err != nil {
		return errors.New("git command not found")
	}
	
	return nil
}

// ValidateArgs validates command line arguments
func (v *Validator) ValidateArgs(hunks, patchFile string) error {
	if hunks == "" {
		return errors.New("hunks cannot be empty")
	}
	
	if patchFile == "" {
		return errors.New("patch file cannot be empty")
	}
	
	// Validate hunk numbers
	hunkList := strings.Split(hunks, ",")
	for _, h := range hunkList {
		h = strings.TrimSpace(h)
		num, err := strconv.Atoi(h)
		if err != nil {
			return fmt.Errorf("invalid hunk number: %s", h)
		}
		if num <= 0 {
			return fmt.Errorf("hunk number must be positive: %d", num)
		}
	}
	
	return nil
}