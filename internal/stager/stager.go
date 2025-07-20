package stager

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/syou6162/git-sequential-stage/internal/executor"
	"github.com/syou6162/git-sequential-stage/internal/logger"
)

// Stager handles the sequential staging of hunks from Git patch files.
// It provides functionality to selectively stage specific hunks identified by patch IDs,
// solving the "hunk number drift" problem that occurs with dependent changes.
type Stager struct {
	executor executor.CommandExecutor
	logger   *logger.Logger
}

// NewStager creates a new Stager instance with the provided command executor.
// The executor is used to run Git commands.
func NewStager(exec executor.CommandExecutor) *Stager {
	return &Stager{
		executor: exec,
		logger:   logger.NewFromEnv(),
	}
}

// extractHunkContent extracts the content for a specific hunk
func (s *Stager) extractHunkContent(hunk *HunkInfo, patchFile string) ([]byte, error) {
	// For new files or binary files, return the entire file diff
	if (hunk.File != nil && hunk.File.IsNew) || hunk.IsBinary {
		if hunk.File != nil {
			return []byte(hunk.File.String()), nil
		}
		return nil, fmt.Errorf("file object is nil for %s", hunk.FilePath)
	}

	// For single hunks with Fragment
	if hunk.Fragment != nil && hunk.File != nil {
		return s.generateHunkPatch(hunk)
	}

	return nil, fmt.Errorf("fragment or file object is nil for %s", hunk.FilePath)
}

// generateHunkPatch generates a patch for a single hunk using go-gitdiff objects
func (s *Stager) generateHunkPatch(hunk *HunkInfo) ([]byte, error) {
	var result strings.Builder

	file := hunk.File
	fragment := hunk.Fragment

	// Write file header
	result.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", file.OldName, file.NewName))
	if file.OldMode != file.NewMode && file.OldMode != 0 && file.NewMode != 0 {
		result.WriteString(fmt.Sprintf("old mode %o\n", file.OldMode))
		result.WriteString(fmt.Sprintf("new mode %o\n", file.NewMode))
	}
	if file.IsNew {
		result.WriteString(fmt.Sprintf("new file mode %o\n", file.NewMode))
	}
	if file.IsDelete {
		result.WriteString(fmt.Sprintf("deleted file mode %o\n", file.OldMode))
	}
	if file.IsRename {
		result.WriteString(fmt.Sprintf("rename from %s\n", file.OldName))
		result.WriteString(fmt.Sprintf("rename to %s\n", file.NewName))
	}

	// Write index line
	if file.OldOIDPrefix != "" && file.NewOIDPrefix != "" {
		result.WriteString(fmt.Sprintf("index %s..%s", file.OldOIDPrefix, file.NewOIDPrefix))
		if file.NewMode != 0 {
			result.WriteString(fmt.Sprintf(" %o", file.NewMode))
		}
		result.WriteString("\n")
	}

	// Write file paths
	result.WriteString(fmt.Sprintf("--- a/%s\n", file.OldName))
	result.WriteString(fmt.Sprintf("+++ b/%s\n", file.NewName))

	// Write the fragment
	result.WriteString(fragment.String())

	return []byte(result.String()), nil
}

// setFallbackPatchID sets a fallback patch ID for a hunk when calculation fails
func setFallbackPatchID(hunk *HunkInfo) {
	hunk.PatchID = fmt.Sprintf("unknown-%d", hunk.GlobalIndex)
}

// calculatePatchIDsForHunks calculates patch IDs for all hunks in the list
func (s *Stager) calculatePatchIDsForHunks(allHunks []HunkInfo, patchContent string, patchFile string) error {
	for i := range allHunks {
		hunkContent, err := s.extractHunkContent(&allHunks[i], patchFile)
		if err != nil {
			// Continue without this hunk
			s.logger.Debug("Failed to extract hunk content for hunk %d: %v", allHunks[i].GlobalIndex, err)
			setFallbackPatchID(&allHunks[i])
			continue
		}

		if len(hunkContent) == 0 {
			setFallbackPatchID(&allHunks[i])
			continue
		}

		patchID, err := s.calculatePatchIDStable(hunkContent)
		if err != nil {
			// Continue without patch ID
			setFallbackPatchID(&allHunks[i])
		} else {
			allHunks[i].PatchID = patchID
		}
	}

	return nil
}

// collectTargetFiles extracts unique file paths from hunk specifications
func collectTargetFiles(hunkSpecs []string) (map[string]bool, error) {
	targetFiles := make(map[string]bool)
	for _, spec := range hunkSpecs {
		filePath, _, err := ParseHunkSpec(spec)
		if err != nil {
			return nil, err
		}
		targetFiles[filePath] = true
	}
	return targetFiles, nil
}

// buildTargetIDs builds a list of patch IDs from hunk specifications
func buildTargetIDs(hunkSpecs []string, allHunks []HunkInfo) ([]string, error) {
	var targetIDs []string
	for _, spec := range hunkSpecs {
		filePath, hunkNumbers, err := ParseHunkSpec(spec)
		if err != nil {
			return nil, err
		}

		// Find matching hunks in allHunks
		for _, hunkNum := range hunkNumbers {
			found := false
			for _, hunk := range allHunks {
				if hunk.FilePath == filePath && hunk.IndexInFile == hunkNum {
					targetIDs = append(targetIDs, hunk.PatchID)
					found = true
					break
				}
			}
			if !found {
				return nil, NewHunkNotFoundError(fmt.Sprintf("hunk %d in file %s", hunkNum, filePath), nil)
			}
		}
	}
	return targetIDs, nil
}

// performSafetyChecks checks the safety of the staging area using hybrid approach
func (s *Stager) performSafetyChecks(patchContent string) error {
	// Use hybrid approach: patch-first with git command fallback
	checker := NewSafetyChecker(".")
	evaluation, err := checker.EvaluateWithFallback(patchContent)
	if err != nil {
		return NewSafetyError(GitOperationFailed,
			"Failed to evaluate staging area safety",
			"Check if the patch content is valid and git commands are available", err)
	}

	// Intent-to-add files are allowed to continue
	if evaluation.AllowContinue {
		if len(evaluation.IntentToAddFiles) > 0 {
			s.logger.Info("Intent-to-add files detected (semantic_commit workflow). Continuing...")
			s.logger.Info("Intent-to-add files: %v", evaluation.IntentToAddFiles)
		}
		return nil
	}

	// Not clean and not allowed to continue
	if !evaluation.IsClean {
		return s.generateDetailedStagingError(evaluation)
	}

	return nil
}

// generateDetailedStagingError creates a detailed error message for staging area issues
func (s *Stager) generateDetailedStagingError(evaluation *StagingAreaEvaluation) error {
	// Build a comprehensive error message
	message := "Staging area is not clean. " + evaluation.ErrorMessage

	// Build advice from recommended actions
	var advice strings.Builder
	advice.WriteString("Please resolve the staging area issues first:\n")

	// Group actions by category for better organization
	actionsByCategory := make(map[ActionCategory][]RecommendedAction)
	for _, action := range evaluation.RecommendedActions {
		actionsByCategory[action.Category] = append(actionsByCategory[action.Category], action)
	}

	// Display actions in a logical order
	categories := []ActionCategory{ActionCategoryInfo, ActionCategoryCommit, ActionCategoryUnstage, ActionCategoryReset}
	for _, category := range categories {
		if actions, exists := actionsByCategory[category]; exists {
			for _, action := range actions {
				if len(action.Commands) > 0 {
					advice.WriteString(fmt.Sprintf("\n%s:\n", action.Description))
					for _, cmd := range action.Commands {
						advice.WriteString(fmt.Sprintf("  %s\n", cmd))
					}
				}
			}
		}
	}

	return NewSafetyError(StagingAreaNotClean, message, advice.String(), nil)
}

// StageHunks stages the specified hunks from a patch file to Git's staging area.
// hunkSpecs should be in the format "file:hunk_numbers" (e.g., "main.go:1,3").
// The function uses patch IDs to track hunks across changes, solving the drift problem.
func (s *Stager) StageHunks(hunkSpecs []string, patchFile string) error {
	// Phase 0: Safety checks (only if enabled)
	if os.Getenv("GIT_SEQUENTIAL_STAGE_SAFETY_CHECK") == "true" {
		patchContent, err := os.ReadFile(patchFile)
		if err != nil {
			return NewFileNotFoundError(patchFile, err)
		}

		if err := s.performSafetyChecks(string(patchContent)); err != nil {
			return err
		}
	}

	// Phase 1: Preparation
	allHunks, err := s.preparePatchData(patchFile)
	if err != nil {
		return err
	}

	// Build target ID list
	targetIDs, err := buildTargetIDs(hunkSpecs, allHunks)
	if err != nil {
		return err
	}

	// Phase 2: Execution - Sequential staging loop
	for len(targetIDs) > 0 {
		// Get target files
		targetFiles, err := collectTargetFiles(hunkSpecs)
		if err != nil {
			return NewInvalidArgumentError("failed to collect target files", err)
		}

		// Get current diff
		diffOutput, err := s.getCurrentDiff(targetFiles)
		if err != nil {
			return err
		}

		// Parse current diff
		currentHunks, err := parsePatchFileWithGitDiff(string(diffOutput))
		if err != nil {
			s.logger.Error("Failed to parse current diff: %v", err)
			return NewParsingError("current diff", err)
		}

		diffLines := strings.Split(string(diffOutput), "\n")

		// Create temp file for hunk processing
		tmpFileName, cleanup, err := s.createTempDiffFile(diffOutput)
		if err != nil {
			return err
		}
		defer cleanup()

		// Find and apply matching hunk
		newTargetIDs, applied, err := s.findAndApplyMatchingHunk(currentHunks, diffLines, tmpFileName, targetIDs)
		if err != nil {
			return err
		}

		if !applied {
			return NewHunkNotFoundError(fmt.Sprintf("hunks with patch IDs: %v", targetIDs), nil)
		}

		targetIDs = newTargetIDs
	}

	return nil
}

// preparePatchData prepares patch data by reading and parsing the patch file
func (s *Stager) preparePatchData(patchFile string) ([]HunkInfo, error) {
	content, err := os.ReadFile(patchFile)
	if err != nil {
		return nil, NewFileNotFoundError(patchFile, err)
	}
	patchContent := string(content)

	allHunks, err := parsePatchFileWithGitDiff(patchContent)
	if err != nil {
		return nil, NewParsingError("patch file", err)
	}

	// Calculate patch IDs for all hunks
	if err := s.calculatePatchIDsForHunks(allHunks, patchContent, patchFile); err != nil {
		s.logger.Error("Failed to calculate patch IDs: %v", err)
		return nil, NewGitCommandError("patch-id calculation", err)
	}

	return allHunks, nil
}

// getCurrentDiff gets the current diff for target files
func (s *Stager) getCurrentDiff(targetFiles map[string]bool) ([]byte, error) {
	// Build diff command with specific files
	diffArgs := []string{"diff", "HEAD", "--"}
	for file := range targetFiles {
		diffArgs = append(diffArgs, file)
	}

	diffOutput, err := s.executor.Execute("git", diffArgs...)
	if err != nil {
		return nil, NewGitCommandError("git diff", err)
	}

	return diffOutput, nil
}

// createTempDiffFile creates a temporary file with diff content
func (s *Stager) createTempDiffFile(diffOutput []byte) (string, func(), error) {
	tmpFile, err := os.CreateTemp("", "current_diff_*.patch")
	if err != nil {
		return "", nil, NewIOError("create temp file", err)
	}

	cleanup := func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}

	if _, err := tmpFile.Write(diffOutput); err != nil {
		cleanup()
		return "", nil, NewIOError("write temp file", err)
	}

	tmpFile.Close()
	return tmpFile.Name(), cleanup, nil
}

// findAndApplyMatchingHunk finds a matching hunk and applies it
func (s *Stager) findAndApplyMatchingHunk(currentHunks []HunkInfo, diffLines []string, tmpFileName string, targetIDs []string) ([]string, bool, error) {
	for i := range currentHunks {
		hunkContent, err := s.extractHunkContent(&currentHunks[i], tmpFileName)
		if err != nil || len(hunkContent) == 0 {
			continue
		}

		currentPatchID, err := s.calculatePatchIDStable(hunkContent)
		if err != nil {
			continue
		}

		// Check if this hunk matches any target
		for i, targetID := range targetIDs {
			if currentPatchID == targetID {
				// Apply the hunk
				s.logger.Info("Applying hunk with patch ID: %s", targetID)
				if err := s.applyHunk(hunkContent, targetID); err != nil {
					return nil, false, err
				}

				// Remove from target list
				targetIDs = append(targetIDs[:i], targetIDs[i+1:]...)
				return targetIDs, true, nil
			}
		}
	}

	return targetIDs, false, nil
}

// applyHunk applies a single hunk to the staging area
func (s *Stager) applyHunk(hunkContent []byte, targetID string) error {
	_, err := s.executor.ExecuteWithStdin("git", bytes.NewReader(hunkContent), "apply", "--cached")
	if err != nil {
		// Debug output for troubleshooting
		s.logger.Debug("Failed patch content for %s:\n%s", targetID, string(hunkContent))
		return NewPatchApplicationError(targetID, err)
	}
	return nil
}

// calculatePatchIDStable calculates patch ID using git patch-id --stable
func (s *Stager) calculatePatchIDStable(hunkPatch []byte) (string, error) {
	output, err := s.executor.ExecuteWithStdin("git", bytes.NewReader(hunkPatch), "patch-id", "--stable")
	if err != nil {
		return "", err
	}

	// git patch-id output format: "patch-id commit-id"
	parts := strings.Fields(string(output))
	if len(parts) > 0 {
		// Return first 8 chars for brevity
		if len(parts[0]) >= 8 {
			return parts[0][:8], nil
		}
		return parts[0], nil
	}

	return "", NewGitCommandError("git patch-id", fmt.Errorf("unexpected output"))
}
