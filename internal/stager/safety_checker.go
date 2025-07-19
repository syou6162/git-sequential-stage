package stager

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/syou6162/git-sequential-stage/internal/executor"
)

// SafetyChecker performs safety checks before staging operations
type SafetyChecker struct {
	executor executor.CommandExecutor
	logger   *log.Logger
}

// RecommendedAction represents a recommended action for the user
type RecommendedAction struct {
	Description string   // Action description
	Commands    []string // Commands to execute
	Priority    int      // Priority (1 is highest)
	Category    string   // Category: "commit", "unstage", "reset", "info"
}

// StagingAreaEvaluation represents the evaluation of the staging area
type StagingAreaEvaluation struct {
	IsClean            bool                  // Whether staging area is clean
	StagedFiles        []string              // All staged files
	IntentToAddFiles   []string              // Files added with git add -N
	ErrorMessage       string                // Error message if any
	AllowContinue      bool                  // Whether to allow continuation (intent-to-add only)
	RecommendedActions []RecommendedAction   // Recommended actions for LLM agents
	FilesByStatus      map[string][]string   // Files grouped by status (M/A/D/R/C)
}

// NewSafetyChecker creates a new SafetyChecker
func NewSafetyChecker(executor executor.CommandExecutor, logger *log.Logger) *SafetyChecker {
	return &SafetyChecker{
		executor: executor,
		logger:   logger,
	}
}

// EvaluateStagingArea evaluates the current staging area for safety
func (sc *SafetyChecker) EvaluateStagingArea() (*StagingAreaEvaluation, error) {
	// Get staging area status
	output, err := sc.executor.Execute("git", "status", "--porcelain")
	if err != nil {
		return nil, NewSafetyError(
			ErrorTypeGitOperationFailed,
			"Failed to get git status",
			"Ensure you are in a valid Git repository",
			err,
		)
	}

	evaluation := &StagingAreaEvaluation{
		IsClean:            true,
		StagedFiles:        []string{},
		IntentToAddFiles:   []string{},
		FilesByStatus:      make(map[string][]string),
		RecommendedActions: []RecommendedAction{},
	}

	// Parse status output
	// Note: Do not use TrimSpace on the entire output as it removes important leading spaces
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		// Git status format: XY filename
		// X = status in index, Y = status in working tree
		if len(line) < 3 {
			continue
		}

		statusIndex := line[0:1]
		// statusWorkTree := line[1:2] // Currently unused, may be used in future
		filename := strings.TrimSpace(line[2:])

		// Check if file is staged (index status is not space or ?)
		if statusIndex != " " && statusIndex != "?" {
			evaluation.IsClean = false
			evaluation.StagedFiles = append(evaluation.StagedFiles, filename)
			
			// Debug logging removed after fixing the issue

			// Categorize by status
			switch statusIndex {
			case "M":
				evaluation.FilesByStatus["M"] = append(evaluation.FilesByStatus["M"], filename)
			case "A":
				evaluation.FilesByStatus["A"] = append(evaluation.FilesByStatus["A"], filename)
			case "D":
				evaluation.FilesByStatus["D"] = append(evaluation.FilesByStatus["D"], filename)
			case "R":
				// For renamed files, parse old and new names
				parts := strings.Split(filename, " -> ")
				if len(parts) == 2 {
					evaluation.FilesByStatus["R"] = append(evaluation.FilesByStatus["R"], filename)
				}
			case "C":
				// For copied files
				evaluation.FilesByStatus["C"] = append(evaluation.FilesByStatus["C"], filename)
			}
		}
	}

	// Detect intent-to-add files
	intentToAddFiles, err := sc.DetectIntentToAddFiles()
	if err != nil {
		// Log warning but don't fail the evaluation
		sc.logger.Printf("Warning: Failed to detect intent-to-add files: %v", err)
	} else {
		evaluation.IntentToAddFiles = intentToAddFiles
	}

	// Check if we should allow continuation
	if !evaluation.IsClean {
		// If all staged files are intent-to-add, allow continuation
		if len(evaluation.IntentToAddFiles) > 0 && sc.allFilesAreIntentToAdd(evaluation) {
			evaluation.AllowContinue = true
			evaluation.ErrorMessage = "Intent-to-add files detected (semantic_commit workflow)"
		} else {
			evaluation.ErrorMessage = "Staging area contains staged files"
		}
	}

	// Generate recommended actions
	evaluation.RecommendedActions = sc.generateRecommendedActions(evaluation)

	return evaluation, nil
}

// DetectIntentToAddFiles detects files added with git add -N
func (sc *SafetyChecker) DetectIntentToAddFiles() ([]string, error) {
	// Get diff of intent-to-add files
	// Intent-to-add files show up as new files in diff but are tracked
	output, err := sc.executor.Execute("git", "diff", "--name-only", "--diff-filter=A", "--cached")
	if err != nil {
		return nil, err
	}

	intentToAddFiles := []string{}
	
	if len(output) == 0 {
		return intentToAddFiles, nil
	}

	// Check each new file to see if it's intent-to-add
	candidateFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, file := range candidateFiles {
		if file == "" {
			continue
		}

		// Intent-to-add files have special characteristics:
		// They appear in git diff --cached but have no actual content staged
		diffOutput, err := sc.executor.Execute("git", "diff", "--cached", "--", file)
		if err != nil {
			continue
		}

		// If the file has an empty diff in --cached, it's likely intent-to-add
		if len(diffOutput) == 0 || !strings.Contains(string(diffOutput), "+++") {
			// Double-check with git ls-files
			lsOutput, err := sc.executor.Execute("git", "ls-files", "--", file)
			if err == nil && len(lsOutput) > 0 {
				intentToAddFiles = append(intentToAddFiles, file)
			}
		}
	}

	return intentToAddFiles, nil
}

// allFilesAreIntentToAdd checks if all staged files are intent-to-add
func (sc *SafetyChecker) allFilesAreIntentToAdd(evaluation *StagingAreaEvaluation) bool {
	// Create a map for quick lookup
	intentToAddMap := make(map[string]bool)
	for _, file := range evaluation.IntentToAddFiles {
		intentToAddMap[file] = true
	}

	// Check if all staged files are in the intent-to-add list
	for _, file := range evaluation.StagedFiles {
		if !intentToAddMap[file] {
			return false
		}
	}

	return true
}

// generateRecommendedActions generates recommended actions based on the evaluation
func (sc *SafetyChecker) generateRecommendedActions(evaluation *StagingAreaEvaluation) []RecommendedAction {
	var actions []RecommendedAction

	if evaluation.IsClean {
		return actions
	}

	// Intent-to-add files information
	if len(evaluation.IntentToAddFiles) > 0 {
		actions = append(actions, RecommendedAction{
			Description: "Intent-to-add files detected (semantic_commit workflow)",
			Commands:    []string{"# These files will be processed normally"},
			Priority:    1,
			Category:    "info",
		})
	}

	// Handle different file types
	if deleted := evaluation.FilesByStatus["D"]; len(deleted) > 0 {
		for _, file := range deleted {
			actions = append(actions, RecommendedAction{
				Description: fmt.Sprintf("Commit deletion of %s", file),
				Commands:    []string{fmt.Sprintf("git commit -m \"Remove %s\"", file)},
				Priority:    1,
				Category:    "commit",
			})
		}
	}

	if renamed := evaluation.FilesByStatus["R"]; len(renamed) > 0 {
		for _, file := range renamed {
			actions = append(actions, RecommendedAction{
				Description: fmt.Sprintf("Commit rename: %s", file),
				Commands:    []string{fmt.Sprintf("git commit -m \"Rename %s\"", file)},
				Priority:    1,
				Category:    "commit",
			})
		}
	}

	// General actions for modified/added files
	hasNonIntentToAddFiles := false
	for _, file := range evaluation.StagedFiles {
		isIntentToAdd := false
		for _, itaFile := range evaluation.IntentToAddFiles {
			if file == itaFile {
				isIntentToAdd = true
				break
			}
		}
		if !isIntentToAdd {
			hasNonIntentToAddFiles = true
			break
		}
	}

	if hasNonIntentToAddFiles {
		// Commit all changes
		actions = append(actions, RecommendedAction{
			Description: "Commit all staged changes",
			Commands:    []string{"git commit -m \"Your commit message\""},
			Priority:    2,
			Category:    "commit",
		})

		// Reset all changes
		actions = append(actions, RecommendedAction{
			Description: "Unstage all changes",
			Commands:    []string{"git reset HEAD"},
			Priority:    3,
			Category:    "unstage",
		})

		// Reset specific files
		for status, files := range evaluation.FilesByStatus {
			if status == "D" || status == "R" {
				continue // Already handled above
			}
			for _, file := range files {
				// Skip intent-to-add files
				isIntentToAdd := false
				for _, itaFile := range evaluation.IntentToAddFiles {
					if file == itaFile {
						isIntentToAdd = true
						break
					}
				}
				if !isIntentToAdd {
					actions = append(actions, RecommendedAction{
						Description: fmt.Sprintf("Unstage %s", file),
						Commands:    []string{fmt.Sprintf("git reset HEAD %s", file)},
						Priority:    4,
						Category:    "unstage",
					})
				}
			}
		}
	}

	// Sort actions by priority
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].Priority < actions[j].Priority
	})

	return actions
}

// ValidateGitState validates that we're in a valid git repository
func (sc *SafetyChecker) ValidateGitState() error {
	_, err := sc.executor.Execute("git", "rev-parse", "--git-dir")
	if err != nil {
		return NewSafetyError(
			ErrorTypeGitOperationFailed,
			"Not in a git repository",
			"Run this command from within a git repository",
			err,
		)
	}
	return nil
}