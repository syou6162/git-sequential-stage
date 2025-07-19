package stager

import (
	"fmt"
	"sort"
	"strings"

	"github.com/syou6162/git-sequential-stage/internal/executor"
)

// SafetyChecker provides functionality to check the safety of staging operations
type SafetyChecker struct {
	executor executor.CommandExecutor
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
func NewSafetyChecker(executor executor.CommandExecutor) *SafetyChecker {
	return &SafetyChecker{
		executor: executor,
	}
}

// EvaluateStagingArea evaluates the current staging area for safety
func (s *SafetyChecker) EvaluateStagingArea() (*StagingAreaEvaluation, error) {
	// Execute git status --porcelain to get the current status
	output, err := s.executor.Execute("git", "status", "--porcelain")
	if err != nil {
		return nil, NewSafetyError(GitOperationFailed,
			"Failed to get git status",
			"Ensure you are in a valid Git repository", err)
	}

	// Parse the status output
	filesByStatus, allStagedFiles := s.parseGitStatus(string(output))

	// Detect intent-to-add files
	intentToAddFiles, err := s.detectIntentToAddFiles()
	if err != nil {
		return nil, NewSafetyError(GitOperationFailed,
			"Failed to detect intent-to-add files",
			"Check git repository state", err)
	}

	// Determine if staging area is clean
	// Remove intent-to-add files from staged files count for cleanliness check
	nonIntentToAddStaged := s.filterNonIntentToAdd(allStagedFiles, intentToAddFiles)
	isClean := len(allStagedFiles) == 0

	// Allow continue if only intent-to-add files are present
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

// parseGitStatus parses the output of git status --porcelain
func (s *SafetyChecker) parseGitStatus(output string) (map[string][]string, []string) {
	filesByStatus := make(map[string][]string)
	var allStagedFiles []string

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}

		indexStatus := line[0]
		workingStatus := line[1]
		filename := line[3:]

		// Only consider files that are staged (index status is not space)
		if indexStatus != ' ' && indexStatus != '?' {
			allStagedFiles = append(allStagedFiles, filename)

			// Categorize by status
			switch indexStatus {
			case 'M':
				filesByStatus["M"] = append(filesByStatus["M"], filename)
			case 'A':
				filesByStatus["A"] = append(filesByStatus["A"], filename)
			case 'D':
				filesByStatus["D"] = append(filesByStatus["D"], filename)
			case 'R':
				filesByStatus["R"] = append(filesByStatus["R"], filename)
			case 'C':
				filesByStatus["C"] = append(filesByStatus["C"], filename)
			}
		}

		// Also track working directory changes for completeness
		if workingStatus != ' ' && workingStatus != '?' {
			switch workingStatus {
			case 'M':
				filesByStatus["WM"] = append(filesByStatus["WM"], filename)
			case 'D':
				filesByStatus["WD"] = append(filesByStatus["WD"], filename)
			}
		}
	}

	return filesByStatus, allStagedFiles
}

// detectIntentToAddFiles detects files added with git add -N
func (s *SafetyChecker) detectIntentToAddFiles() ([]string, error) {
	// Use git ls-files to detect intent-to-add files
	// Intent-to-add files appear as empty blobs in the index
	output, err := s.executor.Execute("git", "ls-files", "--cached", "--stage")
	if err != nil {
		return nil, err
	}

	var intentToAddFiles []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		
		// Parse ls-files --stage output: mode hash stage filename
		parts := strings.Fields(line)
		if len(parts) >= 4 {
			hash := parts[1]
			filename := strings.Join(parts[3:], " ")
			
			// Empty hash (all zeros) indicates intent-to-add
			if hash == "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391" || strings.HasPrefix(hash, "00000") {
				intentToAddFiles = append(intentToAddFiles, filename)
			}
		}
	}

	return intentToAddFiles, nil
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