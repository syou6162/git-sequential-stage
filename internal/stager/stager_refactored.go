package stager

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

// preparePatchData prepares patch data by reading and parsing the patch file
func (s *Stager) preparePatchData(patchFile string) ([]HunkInfo, error) {
	patchContent, err := s.readFile(patchFile)
	if err != nil {
		return nil, NewFileNotFoundError(patchFile, err)
	}
	
	allHunks, err := parsePatchFile(patchContent)
	if err != nil {
		return nil, NewParsingError("patch file", err)
	}
	
	// Calculate patch IDs for all hunks
	if err := s.calculatePatchIDsForHunks(allHunks, patchContent, patchFile); err != nil {
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
		errorMsg := s.getStderrFromError(err)
		if errorMsg != "" {
			return nil, fmt.Errorf("failed to get current diff: exit status %v - %s", err, errorMsg)
		}
		return nil, fmt.Errorf("failed to get current diff: %v", err)
	}
	
	return diffOutput, nil
}

// createTempDiffFile creates a temporary file with diff content
func (s *Stager) createTempDiffFile(diffOutput []byte) (string, func(), error) {
	tmpFile, err := os.CreateTemp("", "current_diff_*.patch")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp file: %v", err)
	}
	
	cleanup := func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}
	
	if _, err := tmpFile.Write(diffOutput); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to write temp file: %v", err)
	}
	
	tmpFile.Close()
	return tmpFile.Name(), cleanup, nil
}

// findAndApplyMatchingHunk finds a matching hunk and applies it
func (s *Stager) findAndApplyMatchingHunk(currentHunks []HunkInfo, diffLines []string, tmpFileName string, targetIDs []string) ([]string, bool, error) {
	for _, currentHunk := range currentHunks {
		// Check if this is a new file
		isNewFile := isNewFileHunk(diffLines, &currentHunk)
		
		hunkContent, err := s.extractHunkContentFromTempFile(diffLines, &currentHunk, tmpFileName, isNewFile)
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
		// Debug: save the failing patch
		debugFile := fmt.Sprintf("/tmp/failing_patch_%s.patch", targetID)
		os.WriteFile(debugFile, hunkContent, 0644)
		return NewPatchApplicationError(targetID, err).
			WithContext("debug_file", debugFile)
	}
	return nil
}

// StageHunksRefactored is the refactored version of StageHunks with better separation of concerns
func (s *Stager) StageHunksRefactored(hunkSpecs []string, patchFile string) error {
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
			return fmt.Errorf("failed to collect target files: %v", err)
		}
		
		// Get current diff
		diffOutput, err := s.getCurrentDiff(targetFiles)
		if err != nil {
			return err
		}
		
		// Parse current diff
		currentHunks, err := parsePatchFile(string(diffOutput))
		if err != nil {
			return fmt.Errorf("failed to parse current diff: %v", err)
		}
		
		diffLines := strings.Split(string(diffOutput), "\n")
		
		// Create temp file for filterdiff
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
			return fmt.Errorf("unable to find hunks with patch IDs: %v", targetIDs)
		}
		
		targetIDs = newTargetIDs
	}
	
	return nil
}