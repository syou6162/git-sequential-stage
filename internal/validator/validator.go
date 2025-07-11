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
	
	// Check filterdiff
	if _, err := v.executor.Execute("filterdiff", "--version"); err != nil {
		return errors.New("filterdiff command not found (install patchutils)")
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
}	return nil
}

// ValidateArgsNew validates command line arguments for the new format
func (v *Validator) ValidateArgsNew(hunkSpecs []string, patchFile string) error {
	if len(hunkSpecs) == 0 {
		return errors.New("at least one hunk specification is required")
	}
	
	if patchFile == "" {
		return errors.New("patch file cannot be empty")
	}
	
	// Validate each hunk specification
	for _, spec := range hunkSpecs {
		if !strings.Contains(spec, ":") {
			return fmt.Errorf("invalid hunk specification format: %s (expected file:numbers)", spec)
		}
		
		parts := strings.SplitN(spec, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("invalid hunk specification: %s", spec)
		}
		
		// Validate hunk numbers
		for _, numStr := range strings.Split(parts[1], ",") {
			numStr = strings.TrimSpace(numStr)
			num, err := strconv.Atoi(numStr)
			if err != nil {
				return fmt.Errorf("invalid hunk number in %s: %s", spec, numStr)
			}
			if num <= 0 {
				return fmt.Errorf("hunk number must be positive in %s: %d", spec, num)
			}
		}
	}
	
