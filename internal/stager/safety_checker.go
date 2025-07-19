package stager

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// SafetyChecker provides functionality to check the safety of staging operations
// This is now a stateless utility that operates purely on patch content
type SafetyChecker struct {
}

// StagingAreaEvaluation contains the result of evaluating the staging area
type StagingAreaEvaluation struct {
	IsClean              bool
	StagedFiles          []string
	IntentToAddFiles     []string
	ErrorMessage         string
	AllowContinue        bool
	RecommendedActions   []RecommendedAction
	FilesByStatus        map[string][]string
}

// RecommendedAction represents an action that can be taken to resolve a staging issue
type RecommendedAction struct {
	Description string   // アクションの説明
	Commands    []string // 実行すべきコマンドのリスト
	Priority    int      // 優先度（1が最高）
	Category    string   // "commit", "unstage", "reset" など
}

// NewSafetyChecker creates a new SafetyChecker instance
// No longer requires an executor since all operations are patch-based
func NewSafetyChecker() *SafetyChecker {
	return &SafetyChecker{}
}

// EvaluateStagingArea evaluates the current staging area for safety
// DEPRECATED: This method requires git commands. Use EvaluatePatchContent instead for git-command-free operation.
func (s *SafetyChecker) EvaluateStagingArea() (*StagingAreaEvaluation, error) {
	return nil, NewSafetyError(DeprecatedMethod,
		"EvaluateStagingArea is deprecated",
		"Use EvaluatePatchContent with patch file content instead", nil)
}

// EvaluatePatchContent evaluates safety from patch content (git-command-free analysis)
func (s *SafetyChecker) EvaluatePatchContent(patchContent string) (*StagingAreaEvaluation, error) {
	filesByStatus := make(map[string][]string)
	var allStagedFiles []string
	var intentToAddFiles []string

	// If no patch content, staging area is clean
	if strings.TrimSpace(patchContent) == "" {
		return &StagingAreaEvaluation{
			IsClean:           true,
			StagedFiles:       []string{},
			IntentToAddFiles:  []string{},
			AllowContinue:     true,
			FilesByStatus:     filesByStatus,
		}, nil
	}

	// Parse the patch using go-gitdiff for comprehensive analysis
	files, _, err := gitdiff.Parse(strings.NewReader(patchContent))
	if err != nil {
		return nil, NewSafetyError(GitOperationFailed,
			"Failed to parse patch content",
			"Check if the patch content is valid", err)
	}

	// Extract file information from go-gitdiff analysis
	for _, file := range files {
		filename := file.NewName
		if file.IsDelete {
			filename = file.OldName
		}

		// Add to all staged files list
		allStagedFiles = append(allStagedFiles, filename)

		// Detect intent-to-add files (empty blobs in new files)
		if file.IsNew && len(file.TextFragments) == 0 {
			intentToAddFiles = append(intentToAddFiles, filename)
		}

		// Categorize files based on go-gitdiff detection
		switch {
		case file.IsBinary:
			// Handle binary files first (they can also be new/modified/etc)
			filesByStatus["BINARY"] = append(filesByStatus["BINARY"], filename)
		case file.IsNew:
			filesByStatus["A"] = append(filesByStatus["A"], filename)
		case file.IsDelete:
			filesByStatus["D"] = append(filesByStatus["D"], filename)
		case file.IsRename:
			// Store rename with proper notation
			renameNotation := file.OldName + " -> " + file.NewName
			filesByStatus["R"] = append(filesByStatus["R"], renameNotation)
		case file.IsCopy:
			// Store copy with proper notation
			copyNotation := file.OldName + " -> " + file.NewName
			filesByStatus["C"] = append(filesByStatus["C"], copyNotation)
		default:
			// Regular modifications
			filesByStatus["M"] = append(filesByStatus["M"], filename)
		}
	}

	// Determine if staging area is clean (only intent-to-add files allowed)
	nonIntentToAddStaged := s.filterNonIntentToAdd(allStagedFiles, intentToAddFiles)
	isClean := len(allStagedFiles) == 0
	allowContinue := len(nonIntentToAddStaged) == 0

	evaluation := &StagingAreaEvaluation{
		IsClean:           isClean,
		StagedFiles:       allStagedFiles,
		IntentToAddFiles:  intentToAddFiles,
		AllowContinue:     allowContinue,
		FilesByStatus:     filesByStatus,
	}

	// Generate error message and recommended actions if not clean
	if !isClean {
		evaluation.ErrorMessage = s.buildStagingErrorMessage(filesByStatus, intentToAddFiles)
		evaluation.RecommendedActions = s.buildRecommendedActions(filesByStatus, intentToAddFiles)
	}

	return evaluation, nil
}


// filterNonIntentToAdd removes intent-to-add files from the staged files list
func (s *SafetyChecker) filterNonIntentToAdd(stagedFiles, intentToAddFiles []string) []string {
	intentToAddMap := make(map[string]bool)
	for _, file := range intentToAddFiles {
		intentToAddMap[file] = true
	}

	var nonIntentToAdd []string
	for _, file := range stagedFiles {
		if !intentToAddMap[file] {
			nonIntentToAdd = append(nonIntentToAdd, file)
		}
	}

	return nonIntentToAdd
}

// buildStagingErrorMessage builds a detailed error message about staging area state
func (s *SafetyChecker) buildStagingErrorMessage(filesByStatus map[string][]string, intentToAddFiles []string) string {
	var message strings.Builder
	message.WriteString("SAFETY_CHECK_FAILED: staging_area_not_clean\n\n")
	message.WriteString("STAGED_FILES:\n")

	if files, exists := filesByStatus["M"]; exists && len(files) > 0 {
		message.WriteString(fmt.Sprintf("  MODIFIED: %s\n", strings.Join(files, ",")))
	}
	if files, exists := filesByStatus["A"]; exists && len(files) > 0 {
		// Filter out intent-to-add files from newly added files
		nonIntentToAdd := s.filterNonIntentToAdd(files, intentToAddFiles)
		if len(nonIntentToAdd) > 0 {
			message.WriteString(fmt.Sprintf("  NEW: %s\n", strings.Join(nonIntentToAdd, ",")))
		}
	}
	if len(intentToAddFiles) > 0 {
		message.WriteString(fmt.Sprintf("  INTENT_TO_ADD: %s\n", strings.Join(intentToAddFiles, ",")))
	}
	if files, exists := filesByStatus["D"]; exists && len(files) > 0 {
		message.WriteString(fmt.Sprintf("  DELETED: %s\n", strings.Join(files, ",")))
	}
	if files, exists := filesByStatus["R"]; exists && len(files) > 0 {
		message.WriteString(fmt.Sprintf("  RENAMED: %s\n", strings.Join(files, ",")))
	}
	if files, exists := filesByStatus["C"]; exists && len(files) > 0 {
		message.WriteString(fmt.Sprintf("  COPIED: %s\n", strings.Join(files, ",")))
	}
	if files, exists := filesByStatus["BINARY"]; exists && len(files) > 0 {
		message.WriteString(fmt.Sprintf("  BINARY: %s\n", strings.Join(files, ",")))
	}

	return message.String()
}

// buildRecommendedActions builds recommended actions for resolving staging issues
func (s *SafetyChecker) buildRecommendedActions(filesByStatus map[string][]string, intentToAddFiles []string) []RecommendedAction {
	var actions []RecommendedAction

	// Intent-to-add files information
	if len(intentToAddFiles) > 0 {
		actions = append(actions, RecommendedAction{
			Description: "Intent-to-add files detected (semantic_commit workflow)",
			Commands:    []string{"# These files will be processed normally"},
			Priority:    1,
			Category:    "info",
		})
	}

	// Handle deletions first (highest priority)
	if files, exists := filesByStatus["D"]; exists && len(files) > 0 {
		for _, file := range files {
			actions = append(actions, RecommendedAction{
				Description: fmt.Sprintf("Commit deletion of %s", file),
				Commands:    []string{fmt.Sprintf("git commit -m \"Remove %s\"", file)},
				Priority:    1,
				Category:    "commit",
			})
		}
	}

	// Handle renames and copies
	if files, exists := filesByStatus["R"]; exists && len(files) > 0 {
		for _, file := range files {
			actions = append(actions, RecommendedAction{
				Description: fmt.Sprintf("Commit rename of %s", file),
				Commands:    []string{fmt.Sprintf("git commit -m \"Rename %s\"", file)},
				Priority:    1,
				Category:    "commit",
			})
		}
	}

	if files, exists := filesByStatus["C"]; exists && len(files) > 0 {
		for _, file := range files {
			actions = append(actions, RecommendedAction{
				Description: fmt.Sprintf("Commit copy of %s", file),
				Commands:    []string{fmt.Sprintf("git commit -m \"Copy %s\"", file)},
				Priority:    1,
				Category:    "commit",
			})
		}
	}

	// Handle modifications and non-intent-to-add new files
	var problematicFiles []string
	if files, exists := filesByStatus["M"]; exists {
		problematicFiles = append(problematicFiles, files...)
	}
	if files, exists := filesByStatus["A"]; exists {
		nonIntentToAdd := s.filterNonIntentToAdd(files, intentToAddFiles)
		problematicFiles = append(problematicFiles, nonIntentToAdd...)
	}
	if files, exists := filesByStatus["BINARY"]; exists {
		problematicFiles = append(problematicFiles, files...)
	}

	if len(problematicFiles) > 0 {
		// Option 1: Commit all changes
		actions = append(actions, RecommendedAction{
			Description: "Commit all staged changes",
			Commands:    []string{"git commit -m \"Your commit message\""},
			Priority:    2,
			Category:    "commit",
		})

		// Option 2: Unstage all changes
		actions = append(actions, RecommendedAction{
			Description: "Unstage all changes",
			Commands:    []string{"git reset HEAD"},
			Priority:    3,
			Category:    "unstage",
		})

		// Option 3: Unstage specific files
		for _, file := range problematicFiles {
			actions = append(actions, RecommendedAction{
				Description: fmt.Sprintf("Unstage specific file %s", file),
				Commands:    []string{fmt.Sprintf("git reset HEAD %s", file)},
				Priority:    4,
				Category:    "unstage",
			})
		}
	}

	// Sort by priority
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].Priority < actions[j].Priority
	})

	return actions
}