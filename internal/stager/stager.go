package stager

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/syou6162/git-sequential-stage/internal/executor"
)

// Stager handles the sequential staging of hunks
type Stager struct {
	executor executor.CommandExecutor
}

// NewStager creates a new stager
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
				return nil, fmt.Errorf("hunk %d not found in file %s", hunkNum, filePath)
			}
		}
	}
	return targetIDs, nil
}

// StageHunks stages the specified hunks using the file:hunk format
func (s *Stager) StageHunks(hunkSpecs []string, patchFile string) error {
	// Preparation phase: Parse master patch and build HunkInfo list
	patchContent, err := s.readFile(patchFile)
	if err != nil {
		return fmt.Errorf("failed to read patch file: %v", err)
	}
	
	allHunks, err := parsePatchFile(patchContent)
	if err != nil {
		return fmt.Errorf("failed to parse patch file: %v", err)
	}
	
	// Calculate patch IDs for all hunks using filterdiff
	if err := s.calculatePatchIDsForHunks(allHunks, patchContent, patchFile); err != nil {
		return fmt.Errorf("failed to calculate patch IDs: %v", err)
	}
	
	// Parse hunk specifications and build target ID list
	targetIDs, err := buildTargetIDs(hunkSpecs, allHunks)
	if err != nil {
		return err
	}
	
	// Execution phase: Sequential staging loop
	for len(targetIDs) > 0 {
		// a. Get latest diff for target files only
		targetFiles, err := collectTargetFiles(hunkSpecs)
		if err != nil {
			return fmt.Errorf("failed to collect target files: %v", err)
		}
		
		// Build diff command with specific files
		diffArgs := []string{"diff", "HEAD", "--"}
		for file := range targetFiles {
			diffArgs = append(diffArgs, file)
		}
		
		diffOutput, err := s.executor.Execute("git", diffArgs...)
		
		if err != nil {
			errorMsg := s.getStderrFromError(err)
			if errorMsg != "" {
				return fmt.Errorf("failed to get current diff: exit status %v - %s", err, errorMsg)
			}
			return fmt.Errorf("failed to get current diff: %v", err)
		}
		
		// b. Parse current diff and find matching hunks
		currentHunks, err := parsePatchFile(string(diffOutput))
		if err != nil {
			return fmt.Errorf("failed to parse current diff: %v", err)
		}
		
		diffLines := strings.Split(string(diffOutput), "\n")
		
		// Find and apply matching hunk
		applied := false
		
		// Write current diff to temp file for filterdiff
		tmpFile, err := os.CreateTemp("", "current_diff_*.patch")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		
		if _, err := tmpFile.Write(diffOutput); err != nil {
			return fmt.Errorf("failed to write temp file: %v", err)
		}
		tmpFile.Close()
		
		for _, currentHunk := range currentHunks {
			// Check if this is a new file
			isNewFile := isNewFileHunk(diffLines, &currentHunk)
			
			hunkContent, err := s.extractHunkContentFromTempFile(diffLines, &currentHunk, tmpFile.Name(), isNewFile)
			if err != nil {
				continue
			}
			
			if len(hunkContent) == 0 {
				continue
			}
			
			currentPatchID, err := s.calculatePatchIDStable(hunkContent)
			if err != nil {
				continue
			}
			
			// Check if this hunk matches any target
			for i, targetID := range targetIDs {
				if currentPatchID == targetID {
					// Always apply hunks one by one (simpler and more reliable)
					_, err = s.executor.ExecuteWithStdin("git", bytes.NewReader(hunkContent), "apply", "--cached")
					if err != nil {
						// Debug: save the failing patch
						debugFile := fmt.Sprintf("/tmp/failing_patch_%s.patch", targetID)
						os.WriteFile(debugFile, hunkContent, 0644)
						return fmt.Errorf("failed to apply hunk with patch ID %s: %v (saved to %s)", targetID, err, debugFile)
					}
					
					// Remove from target list
					targetIDs = append(targetIDs[:i], targetIDs[i+1:]...)
					applied = true
					break
				}
			}
			if applied {
				break
			}
		}
		
		if !applied {
			// No matching hunk found
			return fmt.Errorf("unable to find hunks with patch IDs: %v", targetIDs)
		}
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
	
	return "", fmt.Errorf("unexpected git patch-id output")
}

// parseHunks parses the comma-separated hunk numbers
func (s *Stager) parseHunks(hunks string) ([]int, error) {
	parts := strings.Split(hunks, ",")
	result := make([]int, 0, len(parts))
	
	for _, part := range parts {
		part = strings.TrimSpace(part)
		num, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid hunk number: %s", part)
		}
		result = append(result, num)
	}
	
	return result, nil
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

// calculatePatchIDForHunk calculates patch ID using git patch-id
func (s *Stager) calculatePatchIDForHunk(hunkPatch []byte) (string, error) {
	output, err := s.executor.ExecuteWithStdin("git", bytes.NewReader(hunkPatch), "patch-id")
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
	
	return "", fmt.Errorf("unexpected git patch-id output")
}

// extractFilePathFromPatch extracts the file path from a patch
func extractFilePathFromPatch(patch string) string {
	lines := strings.Split(patch, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+++ b/") {
			return strings.TrimPrefix(line, "+++ b/")
		}
		if strings.HasPrefix(line, "diff --git") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				return strings.TrimPrefix(parts[3], "b/")
			}
		}
	}
	return "unknown"
}