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

// StageHunks stages the specified hunks sequentially using patch IDs internally
func (s *Stager) StageHunks(hunks string, patchFile string) error {
	hunkNumbers, err := s.parseHunks(hunks)
	if err != nil {
		return err
	}
	
	// Stage each requested hunk using filterdiff + git patch-id workflow
	for _, hunkNum := range hunkNumbers {
		// Extract single hunk using filterdiff
		hunkPatch, err := s.executor.Execute("filterdiff", fmt.Sprintf("--hunks=%d", hunkNum), patchFile)
		if err != nil {
			return fmt.Errorf("failed to extract hunk %d: %v", hunkNum, err)
		}
		
		// Check if filterdiff returned empty output (hunk not found)
		if len(hunkPatch) == 0 {
			return fmt.Errorf("failed to extract hunk %d: hunk not found in patch file", hunkNum)
		}
		
		// Calculate patch ID for this hunk
		patchID, err := s.calculatePatchIDForHunk(hunkPatch)
		if err != nil {
			// Continue without patch ID
			patchID = fmt.Sprintf("unknown-%d", hunkNum)
		}
		
		// Apply the hunk to staging area
		_, err = s.executor.ExecuteWithStdin("git", bytes.NewReader(hunkPatch), "apply", "--cached")
		if err != nil {
			stderr := s.getStderrFromError(err)
			
			// Try to provide more helpful error information
			_, checkErr := s.executor.ExecuteWithStdin("git", bytes.NewReader(hunkPatch), "apply", "--cached", "--check")
			var checkMsg string
			if checkErr != nil {
				checkMsg = s.getStderrFromError(checkErr)
			}
			
			// Extract file path from patch for error message
			filePath := extractFilePathFromPatch(string(hunkPatch))
			
			return fmt.Errorf("failed to apply hunk %d (patch ID: %s):\nFile: %s\nError: %s\n\nDetailed check: %s\n\nPossible causes:\n1. The file has been modified since the patch was created\n2. This hunk has already been staged\n3. There are conflicts with existing changes\n\nTry running 'git status' to check the current state", 
				hunkNum, patchID, filePath, stderr, checkMsg)
		}
	}
	
	return nil
}

// StageHunksNew stages the specified hunks using the new file:hunk format
func (s *Stager) StageHunksNew(hunkSpecs []string, patchFile string) error {
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
	for i := range allHunks {
		// Check if this is a new file by looking for @@ -0,0
		isNewFile := false
		patchLines := strings.Split(patchContent, "\n")
		if allHunks[i].StartLine < len(patchLines) {
			hunkHeader := patchLines[allHunks[i].StartLine]
			if strings.Contains(hunkHeader, "@@ -0,0") {
				isNewFile = true
			}
		}
		
		var hunkContent []byte
		if isNewFile {
			// For new files, we need to get the entire file diff including the header
			// First, get the file boundaries
			fileStartLine := allHunks[i].StartLine
			for j := allHunks[i].StartLine - 1; j >= 0; j-- {
				if strings.HasPrefix(patchLines[j], "diff --git") {
					fileStartLine = j
					break
				}
			}
			
			// Find the end of the file (next diff or end of patch)
			fileEndLine := len(patchLines)
			for j := allHunks[i].EndLine + 1; j < len(patchLines); j++ {
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
			hunkContent = []byte(strings.Join(fileDiff, "\n"))
		} else {
			// Use filterdiff to extract single hunk
			filterCmd := fmt.Sprintf("--hunks=%d", allHunks[i].IndexInFile)
			hunkContent, _ = s.executor.Execute("filterdiff", filterCmd, patchFile)
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
	
	// Parse hunk specifications and build target ID list
	var targetIDs []string
	for _, spec := range hunkSpecs {
		filePath, hunkNumbers, err := parseHunkSpec(spec)
		if err != nil {
			return err
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
				return fmt.Errorf("hunk %d not found in file %s", hunkNum, filePath)
			}
		}
	}
	
	// Execution phase: Sequential staging loop
	for len(targetIDs) > 0 {
		// a. Get latest diff (only for files in the target list)
		// Collect unique file paths from target hunks
		targetFiles := make(map[string]bool)
		for _, spec := range hunkSpecs {
			filePath, _, err := parseHunkSpec(spec)
			if err == nil {
				targetFiles[filePath] = true
			}
		}
		
		// Get diff for target files only
		diffArgs := []string{"diff", "-U0", "HEAD"}
		for file := range targetFiles {
			diffArgs = append(diffArgs, file)
		}
		diffOutput, err := s.executor.Execute("git", diffArgs...)
		if err != nil {
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
			isNewFile := false
			if currentHunk.StartLine < len(diffLines) {
				hunkHeader := diffLines[currentHunk.StartLine]
				if strings.Contains(hunkHeader, "@@ -0,0") {
					isNewFile = true
				}
			}
			
			var hunkContent []byte
			if isNewFile {
				// For new files, extract the entire file diff
				fileStartLine := currentHunk.StartLine
				for j := currentHunk.StartLine - 1; j >= 0; j-- {
					if strings.HasPrefix(diffLines[j], "diff --git") {
						fileStartLine = j
						break
					}
				}
				
				fileEndLine := len(diffLines)
				for j := currentHunk.EndLine + 1; j < len(diffLines); j++ {
					if strings.HasPrefix(diffLines[j], "diff --git") {
						fileEndLine = j
						break
					}
				}
				
				var fileDiff []string
				for j := fileStartLine; j < fileEndLine; j++ {
					fileDiff = append(fileDiff, diffLines[j])
				}
				hunkContent = []byte(strings.Join(fileDiff, "\n"))
			} else {
				// Use filterdiff to extract single hunk from current diff
				filterCmd := fmt.Sprintf("--hunks=%d", currentHunk.IndexInFile)
				hunkContent, _ = s.executor.Execute("filterdiff", filterCmd, tmpFile.Name())
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
					// c. Apply this hunk
					_, err = s.executor.ExecuteWithStdin("git", bytes.NewReader(hunkContent), "apply", "--cached")
					if err != nil {
						// Debug: save the failing patch
						debugFile := fmt.Sprintf("/tmp/failing_patch_%s.patch", targetID)
						os.WriteFile(debugFile, hunkContent, 0644)
						return fmt.Errorf("failed to apply hunk with patch ID %s: %v (saved to %s)", targetID, err, debugFile)
					}
					
					// d. Remove from target list
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
