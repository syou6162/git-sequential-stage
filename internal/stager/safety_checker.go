package stager

import (
	"fmt"
	"sort"
	"strings"
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

// Git status codes from git status --porcelain
// These are the character codes used in git status output
const (
	GitStatusCodeModified  = 'M'
	GitStatusCodeAdded     = 'A'
	GitStatusCodeDeleted   = 'D'
	GitStatusCodeRenamed   = 'R'
	GitStatusCodeCopied    = 'C'
	GitStatusCodeUnmerged  = 'U'
	GitStatusCodeUntracked = '?'
	GitStatusCodeIgnored   = '!'
	GitStatusCodeSpace     = ' '
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
// Uses a hybrid approach: patch-based analysis with git command fallback when necessary
type SafetyChecker struct {
	statusReader  GitStatusReader
	patchAnalyzer PatchAnalyzer
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
// Accepts an optional repoPath for hybrid approach ("" = patch-only mode)
func NewSafetyChecker(repoPath string) *SafetyChecker {
	var statusReader GitStatusReader
	if repoPath != "" {
		statusReader = NewGitStatusReader(repoPath)
	}
	return &SafetyChecker{
		statusReader:  statusReader,
		patchAnalyzer: NewPatchAnalyzer(),
	}
}

// EvaluatePatchContent evaluates safety from patch content (git-command-free analysis)
func (s *SafetyChecker) EvaluatePatchContent(patchContent string) (*StagingAreaEvaluation, error) {
	// Use patch analyzer to analyze the patch
	analysisResult, err := s.patchAnalyzer.AnalyzePatch(patchContent)
	if err != nil {
		return nil, err
	}

	// If no files in the analysis, staging area is clean
	if len(analysisResult.AllFiles) == 0 {
		return &StagingAreaEvaluation{
			IsClean:          true,
			StagedFiles:      []string{},
			IntentToAddFiles: []string{},
			AllowContinue:    true,
			FilesByStatus:    analysisResult.FilesByStatus,
		}, nil
	}

	// Check for file move operations using patch analysis
	moveOperations := s.detectFileMoveOperationsFromPatch(patchContent)

	// Determine if staging area is clean (only intent-to-add files allowed, or file moves)
	nonIntentToAddStaged := s.filterNonIntentToAdd(analysisResult.AllFiles, analysisResult.IntentToAddFiles)
	isClean := len(analysisResult.AllFiles) == 0
	allowContinue := len(nonIntentToAddStaged) == 0 || len(moveOperations) > 0

	evaluation := &StagingAreaEvaluation{
		IsClean:          isClean,
		StagedFiles:      analysisResult.AllFiles,
		IntentToAddFiles: analysisResult.IntentToAddFiles,
		AllowContinue:    allowContinue,
		FilesByStatus:    analysisResult.FilesByStatus,
	}

	// Generate error message and recommended actions if not clean
	if !isClean {
		evaluation.ErrorMessage = s.buildStagingErrorMessage(analysisResult.FilesByStatus, analysisResult.IntentToAddFiles)
		evaluation.RecommendedActions = s.buildRecommendedActions(analysisResult.FilesByStatus, analysisResult.IntentToAddFiles)
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

// fileStatusOrder defines the order in which file statuses should be processed
var fileStatusOrder = []FileStatus{
	FileStatusModified,
	FileStatusAdded,
	FileStatusDeleted,
	FileStatusRenamed,
	FileStatusCopied,
	FileStatusBinary,
}

// getFilesForStatus returns files for a given status, with special handling for intent-to-add
func (s *SafetyChecker) getFilesForStatus(status FileStatus, filesByStatus map[FileStatus][]string, intentToAddFiles []string) []string {
	files, exists := filesByStatus[status]
	if !exists || len(files) == 0 {
		return nil
	}

	// Special handling for added files - filter out intent-to-add
	if status == FileStatusAdded {
		return s.filterNonIntentToAdd(files, intentToAddFiles)
	}

	return files
}

// FileMove represents a detected file move operation
type FileMove struct {
	From string
	To   string
}

// detectFileMoveOperationsFromPatch detects file move operations using go-gitdiff patch analysis
func (s *SafetyChecker) detectFileMoveOperationsFromPatch(patchContent string) []FileMove {
	var moves []FileMove

	// Use existing patch analysis to detect renames
	hunks, err := ParsePatchFileWithGitDiff(patchContent)
	if err != nil {
		// If patch parsing fails, return empty moves (fallback to original behavior)
		return moves
	}

	// Extract rename operations from parsed hunks
	renamedFiles := make(map[string]FileMove)
	for _, hunk := range hunks {
		if hunk.File != nil && hunk.File.IsRename {
			move := FileMove{
				From: hunk.File.OldName,
				To:   hunk.File.NewName,
			}
			// Avoid duplicates (same file might have multiple hunks)
			renamedFiles[move.From+">"+move.To] = move
		}
	}

	// Convert map to slice
	for _, move := range renamedFiles {
		moves = append(moves, move)
	}

	return moves
}

// detectFileMoveOperationsFromStatus detects file move operations from git status (fallback method)
// This is a simplified version used when patch analysis is not available
func (s *SafetyChecker) detectFileMoveOperationsFromStatus(filesByStatus map[FileStatus][]string) []FileMove {
	var moves []FileMove

	// Look for explicit rename operations in status
	if renamedFiles, exists := filesByStatus[FileStatusRenamed]; exists {
		for _, renamed := range renamedFiles {
			// Handle formats like "old.txt -> new.txt" from git status
			if strings.Contains(renamed, " -> ") {
				parts := strings.Split(renamed, " -> ")
				if len(parts) == 2 {
					moves = append(moves, FileMove{
						From: strings.TrimSpace(parts[0]),
						To:   strings.TrimSpace(parts[1]),
					})
				}
			}
		}
	}

	return moves
}

// buildStagingErrorMessage builds a detailed error message about staging area state
func (s *SafetyChecker) buildStagingErrorMessage(filesByStatus map[FileStatus][]string, intentToAddFiles []string) string {
	var message strings.Builder
	message.WriteString("SAFETY_CHECK_FAILED: staging_area_not_clean\n\n")
	message.WriteString("STAGED_FILES:\n")

	// Process file statuses in defined order
	for _, status := range fileStatusOrder {
		files := s.getFilesForStatus(status, filesByStatus, intentToAddFiles)
		if len(files) > 0 {
			label := s.getStatusLabel(status)
			message.WriteString(fmt.Sprintf("  %s: %s\n", label, strings.Join(files, ",")))
		}
	}

	// Handle intent-to-add files separately
	if len(intentToAddFiles) > 0 {
		message.WriteString(fmt.Sprintf("  INTENT_TO_ADD: %s\n", strings.Join(intentToAddFiles, ",")))
	}

	return message.String()
}

// getStatusLabel returns the display label for a file status
func (s *SafetyChecker) getStatusLabel(status FileStatus) string {
	switch status {
	case FileStatusModified:
		return "MODIFIED"
	case FileStatusAdded:
		return "NEW"
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

	// Handle specific file statuses with individual commit recommendations
	specificStatuses := []FileStatus{FileStatusDeleted, FileStatusRenamed, FileStatusCopied}
	for _, status := range specificStatuses {
		files := s.getFilesForStatus(status, filesByStatus, intentToAddFiles)
		for _, file := range files {
			action := s.createCommitAction(status, file)
			if action != nil {
				actions = append(actions, *action)
			}
		}
	}

	// Collect all problematic files (modifications, non-intent-to-add new files, and binary files)
	var problematicFiles []string
	problematicStatuses := []FileStatus{FileStatusModified, FileStatusAdded, FileStatusBinary}
	for _, status := range problematicStatuses {
		files := s.getFilesForStatus(status, filesByStatus, intentToAddFiles)
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

// createCommitAction creates a commit action for a specific file status
func (s *SafetyChecker) createCommitAction(status FileStatus, file string) *RecommendedAction {
	var description, commitMsg string

	switch status {
	case FileStatusDeleted:
		description = fmt.Sprintf("Commit deletion of %s", file)
		commitMsg = fmt.Sprintf("Remove %s", file)
	case FileStatusRenamed:
		description = fmt.Sprintf("Commit rename of %s", file)
		commitMsg = fmt.Sprintf("Rename %s", file)
	case FileStatusCopied:
		description = fmt.Sprintf("Commit copy of %s", file)
		commitMsg = fmt.Sprintf("Copy %s", file)
	default:
		return nil
	}

	return &RecommendedAction{
		Description: description,
		Commands:    []string{fmt.Sprintf("git commit -m \"%s\"", commitMsg)},
		Priority:    1,
		Category:    ActionCategoryCommit,
	}
}

// CheckActualStagingArea checks the actual staging area using git commands
// This method is more accurate but requires git command execution
func (s *SafetyChecker) CheckActualStagingArea() (*StagingAreaEvaluation, error) {
	if s.statusReader == nil {
		return nil, NewSafetyError(GitOperationFailed,
			"GitStatusReader not available for git command execution",
			"Initialize SafetyChecker with an executor for git commands", nil)
	}

	// Use status reader to get git status information
	statusInfo, err := s.statusReader.ReadStatus()
	if err != nil {
		return nil, err
	}

	// Check for file move operations using basic status analysis (fallback)
	moveOperations := s.detectFileMoveOperationsFromStatus(statusInfo.FilesByStatus)

	// Determine if staging area is clean
	nonIntentToAddStaged := s.filterNonIntentToAdd(statusInfo.StagedFiles, statusInfo.IntentToAddFiles)
	isClean := len(statusInfo.StagedFiles) == 0
	allowContinue := len(nonIntentToAddStaged) == 0 || len(moveOperations) > 0

	evaluation := &StagingAreaEvaluation{
		IsClean:          isClean,
		StagedFiles:      statusInfo.StagedFiles,
		IntentToAddFiles: statusInfo.IntentToAddFiles,
		AllowContinue:    allowContinue,
		FilesByStatus:    statusInfo.FilesByStatus,
	}

	// Generate error message and recommended actions if not clean
	if !isClean {
		evaluation.ErrorMessage = s.buildStagingErrorMessage(statusInfo.FilesByStatus, statusInfo.IntentToAddFiles)
		evaluation.RecommendedActions = s.buildRecommendedActions(statusInfo.FilesByStatus, statusInfo.IntentToAddFiles)
	}

	return evaluation, nil
}

// EvaluateWithFallback performs hybrid evaluation: patch-first with git command fallback
// This is the recommended API for safety checking
func (s *SafetyChecker) EvaluateWithFallback(patchContent string) (*StagingAreaEvaluation, error) {
	return s.EvaluateWithFallbackAndTargets(patchContent, nil)
}

// EvaluateWithFallbackAndTargets performs hybrid evaluation with target files consideration
// When targetFiles is provided, intent-to-add files in the target list are allowed to have staged hunks
func (s *SafetyChecker) EvaluateWithFallbackAndTargets(patchContent string, targetFiles map[string]bool) (*StagingAreaEvaluation, error) {
	// First, try patch-based evaluation
	patchEval, err := s.EvaluatePatchContent(patchContent)
	if err != nil {
		return nil, err
	}

	// If patch shows changes and we have a status reader, verify with actual staging area
	if len(patchEval.StagedFiles) > 0 && s.statusReader != nil {
		// Get actual staging area state
		actualEval, err := s.CheckActualStagingArea()
		if err != nil {
			// If we can't check actual state, fall back to patch evaluation
			return patchEval, nil
		}

		// Apply target files logic if provided
		if targetFiles != nil {
			s.adjustEvaluationForTargetFiles(actualEval, targetFiles)
		}

		// Use actual evaluation as it's more accurate
		return actualEval, nil
	}

	// Apply target files logic if provided
	if targetFiles != nil {
		s.adjustEvaluationForTargetFiles(patchEval, targetFiles)
	}

	// For empty patches or no executor, patch evaluation is sufficient
	return patchEval, nil
}

// adjustEvaluationForTargetFiles adjusts the evaluation based on target files
// If staged files are in the target files list, they are allowed regardless of intent-to-add status
func (s *SafetyChecker) adjustEvaluationForTargetFiles(evaluation *StagingAreaEvaluation, targetFiles map[string]bool) {
	if evaluation.AllowContinue {
		return
	}

	if s.shouldAllowTargetFileStaging(evaluation, targetFiles) {
		s.updateEvaluationToAllow(evaluation, targetFiles)
	}
}

// shouldAllowTargetFileStaging determines if staging should be allowed based on target files
func (s *SafetyChecker) shouldAllowTargetFileStaging(evaluation *StagingAreaEvaluation, targetFiles map[string]bool) bool {
	// First check: all staged files are either intent-to-add or target files
	if s.allStagedFilesAreValid(evaluation, targetFiles) {
		return true
	}

	// Special case: If we couldn't detect intent-to-add files properly,
	// but all staged files are in the target list, allow continuation
	return s.handleIntentToAddDetectionFallback(evaluation, targetFiles)
}

// allStagedFilesAreValid checks if all staged files are either intent-to-add or target files
func (s *SafetyChecker) allStagedFilesAreValid(evaluation *StagingAreaEvaluation, targetFiles map[string]bool) bool {
	intentToAddMap := s.createIntentToAddMap(evaluation.IntentToAddFiles)

	for _, stagedFile := range evaluation.StagedFiles {
		// If it's an intent-to-add file, it's allowed
		if intentToAddMap[stagedFile] {
			continue
		}

		// Check if this staged file is in the target files
		if !targetFiles[stagedFile] {
			// Found a staged file that is neither intent-to-add nor a target file
			return false
		}
	}

	return true
}

// createIntentToAddMap creates a map for quick lookup of intent-to-add files
func (s *SafetyChecker) createIntentToAddMap(intentToAddFiles []string) map[string]bool {
	intentToAddMap := make(map[string]bool)
	for _, file := range intentToAddFiles {
		intentToAddMap[file] = true
	}
	return intentToAddMap
}

// handleIntentToAddDetectionFallback handles the case where intent-to-add detection might have failed
func (s *SafetyChecker) handleIntentToAddDetectionFallback(evaluation *StagingAreaEvaluation, targetFiles map[string]bool) bool {
	// Only apply fallback if no intent-to-add files were detected
	if len(evaluation.IntentToAddFiles) > 0 {
		return false
	}

	// Re-check if all staged files are in target files
	for _, stagedFile := range evaluation.StagedFiles {
		if !targetFiles[stagedFile] {
			return false
		}
	}

	return true
}

// updateEvaluationToAllow updates the evaluation to allow continuation
func (s *SafetyChecker) updateEvaluationToAllow(evaluation *StagingAreaEvaluation, targetFiles map[string]bool) {
	evaluation.AllowContinue = true

	// Check if there are non-intent-to-add staged files to update messaging
	nonIntentToAddStaged := s.filterNonIntentToAdd(evaluation.StagedFiles, evaluation.IntentToAddFiles)
	if len(nonIntentToAddStaged) > 0 {
		evaluation.ErrorMessage = ""
		evaluation.RecommendedActions = []RecommendedAction{
			{
				Description: "Staging hunks from target files (semantic_commit workflow)",
				Commands:    []string{"# These changes will be processed normally"},
				Priority:    1,
				Category:    ActionCategoryInfo,
			},
		}
	}
}
