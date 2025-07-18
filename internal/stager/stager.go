package stager

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/syou6162/git-sequential-stage/internal/executor"
)

// Stager handles the sequential staging of hunks from Git patch files.
// It provides functionality to selectively stage specific hunks identified by patch IDs,
// solving the "hunk number drift" problem that occurs with dependent changes.
type Stager struct {
	executor executor.CommandExecutor
}

// NewStager creates a new Stager instance with the provided command executor.
// The executor is used to run Git and filterdiff commands.
func NewStager(exec executor.CommandExecutor) *Stager {
	return &Stager{
		executor: exec,
	}
}


// isNewFileHunk checks if a hunk represents a new file by looking for @@ -0,0 in the hunk header
func isNewFileHunk(patchLines []string, hunk *HunkInfo) bool {
	if hunk.StartLine < len(patchLines) {
		hunkHeader := patchLines[hunk.StartLine]
		return strings.Contains(hunkHeader, "@@ -0,0")
	}
	return false
}

// extractFileDiff extracts the entire file diff including headers for a given hunk
func extractFileDiff(patchLines []string, hunk *HunkInfo) []byte {
	// Find the start of the file (diff --git line)
	fileStartLine := hunk.StartLine
	for j := hunk.StartLine - 1; j >= 0; j-- {
		if strings.HasPrefix(patchLines[j], "diff --git") {
			fileStartLine = j
			break
		}
	}
	
	// Find the end of the file (next diff or end of patch)
	fileEndLine := len(patchLines)
	for j := hunk.EndLine + 1; j < len(patchLines); j++ {
		if strings.HasPrefix(patchLines[j], "diff --git") {
			fileEndLine = j
			break
		}
	}
	
	// Extract the entire file diff
	var fileDiff []string
	for j := fileStartLine; j < fileEndLine; j++ {
		fileDiff = append(fileDiff, patchLines[j])
	}
	return []byte(strings.Join(fileDiff, "\n"))
}

// extractHunkContent extracts the content for a specific hunk
// For new files, it returns the entire file diff
// For regular files, it uses filterdiff to extract the specific hunk
func (s *Stager) extractHunkContent(patchLines []string, hunk *HunkInfo, patchFile string, isNewFile bool) ([]byte, error) {
	if isNewFile {
		return extractFileDiff(patchLines, hunk), nil
	}
	
	// Use filterdiff to extract single hunk with file filter
	filterCmd := fmt.Sprintf("--hunks=%d", hunk.IndexInFile)
	filePattern := fmt.Sprintf("*%s", hunk.FilePath)
	return s.executor.Execute("filterdiff", "-i", filePattern, filterCmd, patchFile)
}

// extractHunkContentFromTempFile extracts hunk content using a temporary file
// This is used in the staging loop where we work with current diffs
func (s *Stager) extractHunkContentFromTempFile(diffLines []string, hunk *HunkInfo, tmpFileName string, isNewFile bool) ([]byte, error) {
	if isNewFile {
		return extractFileDiff(diffLines, hunk), nil
	}
	
	// Use filterdiff to extract single hunk from current diff with file filter
	filterCmd := fmt.Sprintf("--hunks=%d", hunk.IndexInFile)
	filePattern := fmt.Sprintf("*%s", hunk.FilePath)
	return s.executor.Execute("filterdiff", "-i", filePattern, filterCmd, tmpFileName)
}

// calculatePatchIDsForHunks calculates patch IDs for all hunks in the list
func (s *Stager) calculatePatchIDsForHunks(allHunks []HunkInfo, patchContent string, patchFile string) error {
	patchLines := strings.Split(patchContent, "\n")
	
	for i := range allHunks {
		// Check if this is a new file by looking for @@ -0,0
		isNewFile := isNewFileHunk(patchLines, &allHunks[i])
		
		hunkContent, err := s.extractHunkContent(patchLines, &allHunks[i], patchFile, isNewFile)
		if err != nil {
			// Continue without this hunk
			allHunks[i].PatchID = fmt.Sprintf("unknown-%d", allHunks[i].GlobalIndex)
			continue
		}
		
		if len(hunkContent) == 0 {
			allHunks[i].PatchID = fmt.Sprintf("unknown-%d", allHunks[i].GlobalIndex)
			continue
		}
		
		patchID, err := s.calculatePatchIDStable(hunkContent)
		if err != nil {
			// Continue without patch ID
			allHunks[i].PatchID = fmt.Sprintf("unknown-%d", allHunks[i].GlobalIndex)
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
		filePath, _, err := parseHunkSpec(spec)
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
		filePath, hunkNumbers, err := parseHunkSpec(spec)
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

// StageHunks stages the specified hunks from a patch file to Git's staging area.
// hunkSpecs should be in the format "file:hunk_numbers" (e.g., "main.go:1,3").
// The function uses patch IDs to track hunks across changes, solving the drift problem.
func (s *Stager) StageHunks(hunkSpecs []string, patchFile string) error {
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
		currentHunks, err := parsePatchFile(string(diffOutput))
		if err != nil {
			return NewParsingError("current diff", err)
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
			return NewHunkNotFoundError(fmt.Sprintf("hunks with patch IDs: %v", targetIDs), nil)
		}
		
		targetIDs = newTargetIDs
	}
	
	return nil
}

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
			return nil, NewGitCommandError("git diff", err).WithContext("stderr", errorMsg)
		}
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
		return NewPatchApplicationError(targetID, err).
			WithContext("patch_content", string(hunkContent))
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


// getStderrFromError extracts stderr from exec.ExitError
func (s *Stager) getStderrFromError(err error) string {
	if err == nil {
		return ""
	}
	
	if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
		return string(exitErr.Stderr)
	}
	
	return err.Error()
}

// readFile reads the content of a file
func (s *Stager) readFile(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

