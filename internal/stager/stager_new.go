package stager

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// StageHunksNew stages the specified hunks using the file:hunk format with go-gitdiff
func (s *Stager) StageHunksNew(hunkSpecs []string, patchFile string) error {
	// Preparation phase: Parse master patch and build HunkInfo list
	patchContent, err := s.readFile(patchFile)
	if err != nil {
		return fmt.Errorf("failed to read patch file: %v", err)
	}
	
	allHunks, err := parsePatchFileWithGitDiff(patchContent)
	if err != nil {
		return fmt.Errorf("failed to parse patch file: %v", err)
	}
	
	// Calculate patch IDs for all hunks
	if err := s.calculatePatchIDsForHunksNew(allHunks, patchContent); err != nil {
		return fmt.Errorf("failed to calculate patch IDs: %v", err)
	}
	
	// Parse hunk specifications and build target ID list
	targetIDs, err := buildTargetIDsNew(hunkSpecs, allHunks)
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
		currentHunks, err := parsePatchFileWithGitDiff(string(diffOutput))
		if err != nil {
			return fmt.Errorf("failed to parse current diff: %v", err)
		}
		
		// Find and apply matching hunk
		applied := false
		
		for _, currentHunk := range currentHunks {
			// Skip binary files
			if currentHunk.IsBinary {
				continue
			}
			
			hunkContent, err := s.extractHunkContentNew(currentHunk)
			if err != nil {
				continue
			}
			
			if len(hunkContent) == 0 {
				continue
			}
			
			currentPatchID, err := s.calculatePatchIDStable([]byte(hunkContent))
			if err != nil {
				continue
			}
			
			// Check if this hunk matches any target
			for i, targetID := range targetIDs {
				if currentPatchID == targetID {
					// Always apply hunks one by one (simpler and more reliable)
					_, err = s.executor.ExecuteWithStdin("git", bytes.NewReader([]byte(hunkContent)), "apply", "--cached")
					if err != nil {
						// Debug: save the failing patch
						debugFile := fmt.Sprintf("/tmp/failing_patch_%s.patch", targetID)
						os.WriteFile(debugFile, []byte(hunkContent), 0644)
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

// calculatePatchIDsForHunksNew calculates patch IDs for all hunks in the list
func (s *Stager) calculatePatchIDsForHunksNew(allHunks []HunkInfoNew, patchContent string) error {
	for i := range allHunks {
		// Skip binary files
		if allHunks[i].IsBinary {
			allHunks[i].PatchID = fmt.Sprintf("binary-%d", allHunks[i].GlobalIndex)
			continue
		}
		
		hunkContent, err := s.extractHunkContentNew(allHunks[i])
		if err != nil {
			// Continue without this hunk
			allHunks[i].PatchID = fmt.Sprintf("unknown-%d", allHunks[i].GlobalIndex)
			continue
		}
		
		if len(hunkContent) == 0 {
			allHunks[i].PatchID = fmt.Sprintf("unknown-%d", allHunks[i].GlobalIndex)
			continue
		}
		
		patchID, err := s.calculatePatchIDStable([]byte(hunkContent))
		if err != nil {
			// Continue without patch ID
			allHunks[i].PatchID = fmt.Sprintf("unknown-%d", allHunks[i].GlobalIndex)
		} else {
			allHunks[i].PatchID = patchID
		}
	}
	
	return nil
}

// extractHunkContentNew extracts the content for a specific hunk using go-gitdiff
func (s *Stager) extractHunkContentNew(hunk HunkInfoNew) (string, error) {
	// For binary files, return a placeholder
	if hunk.IsBinary {
		return "", fmt.Errorf("cannot extract content from binary file")
	}
	
	// Reconstruct the file for this hunk
	file := &gitdiff.File{
		OldName: hunk.OldFilePath,
		NewName: hunk.FilePath,
	}
	
	// Set file operation flags
	switch hunk.Operation {
	case FileOperationAdded:
		file.IsNew = true
	case FileOperationDeleted:
		file.IsDelete = true
	case FileOperationRenamed:
		file.IsRename = true
	case FileOperationCopied:
		file.IsCopy = true
	}
	
	// Use the fragment if available
	if hunk.Fragment != nil {
		return extractHunkContentFromFragment(file, hunk.Fragment)
	}
	
	return "", fmt.Errorf("no fragment available for hunk")
}

// buildTargetIDsNew builds a list of patch IDs from hunk specifications
func buildTargetIDsNew(hunkSpecs []string, allHunks []HunkInfoNew) ([]string, error) {
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
				// Match by file path (considering renames)
				fileMatches := hunk.FilePath == filePath || 
					(hunk.Operation == FileOperationRenamed && hunk.OldFilePath == filePath)
				
				if fileMatches && hunk.IndexInFile == hunkNum {
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

// parseGitDiffFiles parses a git diff output and returns gitdiff.File structures
func parseGitDiffFiles(diffContent string) ([]*gitdiff.File, error) {
	files, _, err := gitdiff.Parse(strings.NewReader(diffContent))
	if err != nil {
		return nil, err
	}
	return files, nil
}