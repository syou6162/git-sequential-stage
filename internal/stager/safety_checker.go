package stager

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// FileStatus represents the modification status of a file
type FileStatus int

const (
	FileStatusModified FileStatus = iota
	FileStatusAdded
	FileStatusDeleted
	FileStatusRenamed
	FileStatusCopied
	FileStatusBinary
)

// String returns the string representation of FileStatus
func (fs FileStatus) String() string {
	switch fs {
	case FileStatusModified:
		return "MODIFIED"
	case FileStatusAdded:
		return "ADDED"
	case FileStatusDeleted:
		return "DELETED"
	case FileStatusRenamed:
		return "RENAMED"
	case FileStatusCopied:
		return "COPIED"
	case FileStatusBinary:
		return "BINARY"
	default:
		return "UNKNOWN"
	}
}

// ActionCategory represents the category of a recommended action
type ActionCategory int

const (
	ActionCategoryInfo ActionCategory = iota
	ActionCategoryCommit
	ActionCategoryUnstage
	ActionCategoryReset
)

// String returns the string representation of ActionCategory
func (ac ActionCategory) String() string {
	switch ac {
	case ActionCategoryInfo:
		return "info"
	case ActionCategoryCommit:
		return "commit"
	case ActionCategoryUnstage:
		return "unstage"
	case ActionCategoryReset:
		return "reset"
	default:
		return "unknown"
	}
}

// SafetyChecker provides functionality to check the safety of staging operations
// This is now a stateless utility that operates purely on patch content
type SafetyChecker struct {
}

// StagingAreaEvaluation contains the result of evaluating the staging area
type StagingAreaEvaluation struct {
	IsClean            bool
	StagedFiles        []string
	IntentToAddFiles   []string
	ErrorMessage       string
	AllowContinue      bool
	RecommendedActions []RecommendedAction
	FilesByStatus      map[FileStatus][]string
}

// RecommendedAction represents an action that can be taken to resolve a staging issue
type RecommendedAction struct {
	Description string         // アクションの説明
	Commands    []string       // 実行すべきコマンドのリスト
	Priority    int            // 優先度（1が最高）
	Category    ActionCategory // アクションのカテゴリ
}

// NewSafetyChecker creates a new SafetyChecker instance
// No longer requires an executor since all operations are patch-based
func NewSafetyChecker() *SafetyChecker {
	return &SafetyChecker{}
}

// EvaluatePatchContent evaluates safety from patch content (git-command-free analysis)
func (s *SafetyChecker) EvaluatePatchContent(patchContent string) (*StagingAreaEvaluation, error) {
	filesByStatus := make(map[FileStatus][]string)
	var allStagedFiles []string
	var intentToAddFiles []string

	// If no patch content, staging area is clean
	if strings.TrimSpace(patchContent) == "" {
		return &StagingAreaEvaluation{
			IsClean:          true,
			StagedFiles:      []string{},
			IntentToAddFiles: []string{},
			AllowContinue:    true,
			FilesByStatus:    filesByStatus,
		}, nil
	}

	// Parse the patch using go-gitdiff for comprehensive analysis
	files, _, err := gitdiff.Parse(strings.NewReader(patchContent))
	if err != nil {
		return nil, NewSafetyError(GitOperationFailed,
			"Failed to parse patch content",
			"Check if the patch content is valid", err)
	}

	// Check if we have a valid patch with actual file changes
	if len(files) == 0 && strings.TrimSpace(patchContent) != "" {
		// Non-empty content but no files parsed - likely invalid patch format
		return nil, NewSafetyError(GitOperationFailed,
			"Invalid patch format - no file changes detected",
			"Ensure the patch content is in valid git diff format", nil)
	}

	// Extract file information from go-gitdiff analysis
	for _, file := range files {
		filename := file.NewName
		if file.IsDelete {
			filename = file.OldName
		}

		// Add to all staged files list
		allStagedFiles = append(allStagedFiles, filename)

		// Detect intent-to-add files (empty blobs in new files, but not binary)
		if file.IsNew && len(file.TextFragments) == 0 && !file.IsBinary {
			intentToAddFiles = append(intentToAddFiles, filename)
		}

		// Categorize files based on go-gitdiff detection
		switch {
		case file.IsBinary:
			// Handle binary files first (they can also be new/modified/etc)
			filesByStatus[FileStatusBinary] = append(filesByStatus[FileStatusBinary], filename)
		case file.IsNew:
			filesByStatus[FileStatusAdded] = append(filesByStatus[FileStatusAdded], filename)
		case file.IsDelete:
			filesByStatus[FileStatusDeleted] = append(filesByStatus[FileStatusDeleted], filename)
		case file.IsRename:
			// Store rename with proper notation
			renameNotation := file.OldName + " -> " + file.NewName
			filesByStatus[FileStatusRenamed] = append(filesByStatus[FileStatusRenamed], renameNotation)
		case file.IsCopy:
			// Store copy with proper notation
			copyNotation := file.OldName + " -> " + file.NewName
			filesByStatus[FileStatusCopied] = append(filesByStatus[FileStatusCopied], copyNotation)
		default:
			// Regular modifications
			filesByStatus[FileStatusModified] = append(filesByStatus[FileStatusModified], filename)
		}
	}

	// Determine if staging area is clean (only intent-to-add files allowed)
	nonIntentToAddStaged := s.filterNonIntentToAdd(allStagedFiles, intentToAddFiles)
	isClean := len(allStagedFiles) == 0
	allowContinue := len(nonIntentToAddStaged) == 0

	evaluation := &StagingAreaEvaluation{
		IsClean:          isClean,
		StagedFiles:      allStagedFiles,
		IntentToAddFiles: intentToAddFiles,
		AllowContinue:    allowContinue,
		FilesByStatus:    filesByStatus,
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
func (s *SafetyChecker) buildStagingErrorMessage(filesByStatus map[FileStatus][]string, intentToAddFiles []string) string {
	var message strings.Builder
	message.WriteString("SAFETY_CHECK_FAILED: staging_area_not_clean\n\n")
	message.WriteString("STAGED_FILES:\n")

	if files, exists := filesByStatus[FileStatusModified]; exists && len(files) > 0 {
		message.WriteString(fmt.Sprintf("  MODIFIED: %s\n", strings.Join(files, ",")))
	}
	if files, exists := filesByStatus[FileStatusAdded]; exists && len(files) > 0 {
		// Filter out intent-to-add files from newly added files
		nonIntentToAdd := s.filterNonIntentToAdd(files, intentToAddFiles)
		if len(nonIntentToAdd) > 0 {
			message.WriteString(fmt.Sprintf("  NEW: %s\n", strings.Join(nonIntentToAdd, ",")))
		}
	}
	if len(intentToAddFiles) > 0 {
		message.WriteString(fmt.Sprintf("  INTENT_TO_ADD: %s\n", strings.Join(intentToAddFiles, ",")))
	}
	if files, exists := filesByStatus[FileStatusDeleted]; exists && len(files) > 0 {
		message.WriteString(fmt.Sprintf("  DELETED: %s\n", strings.Join(files, ",")))
	}
	if files, exists := filesByStatus[FileStatusRenamed]; exists && len(files) > 0 {
		message.WriteString(fmt.Sprintf("  RENAMED: %s\n", strings.Join(files, ",")))
	}
	if files, exists := filesByStatus[FileStatusCopied]; exists && len(files) > 0 {
		message.WriteString(fmt.Sprintf("  COPIED: %s\n", strings.Join(files, ",")))
	}
	if files, exists := filesByStatus[FileStatusBinary]; exists && len(files) > 0 {
		message.WriteString(fmt.Sprintf("  BINARY: %s\n", strings.Join(files, ",")))
	}

	return message.String()
}

// buildRecommendedActions builds recommended actions for resolving staging issues
func (s *SafetyChecker) buildRecommendedActions(filesByStatus map[FileStatus][]string, intentToAddFiles []string) []RecommendedAction {
	var actions []RecommendedAction

	// Intent-to-add files information
	if len(intentToAddFiles) > 0 {
		actions = append(actions, RecommendedAction{
			Description: "Intent-to-add files detected (semantic_commit workflow)",
			Commands:    []string{"# These files will be processed normally"},
			Priority:    1,
			Category:    ActionCategoryInfo,
		})
	}

	// Handle deletions first (highest priority)
	if files, exists := filesByStatus[FileStatusDeleted]; exists && len(files) > 0 {
		for _, file := range files {
			actions = append(actions, RecommendedAction{
				Description: fmt.Sprintf("Commit deletion of %s", file),
				Commands:    []string{fmt.Sprintf("git commit -m \"Remove %s\"", file)},
				Priority:    1,
				Category:    ActionCategoryCommit,
			})
		}
	}

	// Handle renames and copies
	if files, exists := filesByStatus[FileStatusRenamed]; exists && len(files) > 0 {
		for _, file := range files {
			actions = append(actions, RecommendedAction{
				Description: fmt.Sprintf("Commit rename of %s", file),
				Commands:    []string{fmt.Sprintf("git commit -m \"Rename %s\"", file)},
				Priority:    1,
				Category:    ActionCategoryCommit,
			})
		}
	}

	if files, exists := filesByStatus[FileStatusCopied]; exists && len(files) > 0 {
		for _, file := range files {
			actions = append(actions, RecommendedAction{
				Description: fmt.Sprintf("Commit copy of %s", file),
				Commands:    []string{fmt.Sprintf("git commit -m \"Copy %s\"", file)},
				Priority:    1,
				Category:    ActionCategoryCommit,
			})
		}
	}

	// Handle modifications and non-intent-to-add new files
	var problematicFiles []string
	if files, exists := filesByStatus[FileStatusModified]; exists {
		problematicFiles = append(problematicFiles, files...)
	}
	if files, exists := filesByStatus[FileStatusAdded]; exists {
		nonIntentToAdd := s.filterNonIntentToAdd(files, intentToAddFiles)
		problematicFiles = append(problematicFiles, nonIntentToAdd...)
	}
	if files, exists := filesByStatus[FileStatusBinary]; exists {
		problematicFiles = append(problematicFiles, files...)
	}

	if len(problematicFiles) > 0 {
		// Option 1: Commit all changes
		actions = append(actions, RecommendedAction{
			Description: "Commit all staged changes",
			Commands:    []string{"git commit -m \"Your commit message\""},
			Priority:    2,
			Category:    ActionCategoryCommit,
		})

		// Option 2: Unstage all changes
		actions = append(actions, RecommendedAction{
			Description: "Unstage all changes",
			Commands:    []string{"git reset HEAD"},
			Priority:    3,
			Category:    ActionCategoryUnstage,
		})

		// Option 3: Unstage specific files
		for _, file := range problematicFiles {
			actions = append(actions, RecommendedAction{
				Description: fmt.Sprintf("Unstage specific file %s", file),
				Commands:    []string{fmt.Sprintf("git reset HEAD %s", file)},
				Priority:    4,
				Category:    ActionCategoryUnstage,
			})
		}
	}

	// Sort by priority
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].Priority < actions[j].Priority
	})

	return actions
}
