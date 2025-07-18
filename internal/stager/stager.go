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
				return nil, fmt.Errorf("hunk %d not found in file %s", hunkNum, filePath)
			}
		}
	}
	return targetIDs, nil
}

// StageHunks stages the specified hunks from a patch file to Git's staging area.
// hunkSpecs should be in the format "file:hunk_numbers" (e.g., "main.go:1,3").
// The function uses patch IDs to track hunks across changes, solving the drift problem.
func (s *Stager) StageHunks(hunkSpecs []string, patchFile string) error {
	// Use the refactored implementation
	return s.StageHunksRefactored(hunkSpecs, patchFile)
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

