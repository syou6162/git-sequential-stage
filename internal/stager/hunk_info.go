package stager

import (
	"fmt"
	"strings"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// HunkInfo represents information about a single hunk
type HunkInfo struct {
	GlobalIndex   int                   // Global hunk number in the patch file (1, 2, 3, ...)
	FilePath      string                // File path this hunk belongs to (new path for renames)
	OldFilePath   string                // Old file path (for renames)
	IndexInFile   int                   // Hunk number within the file (1, 2, 3, ...)
	PatchID       string                // Unique patch ID calculated using git patch-id
	Operation     FileOperation         // Type of file operation
	IsBinary      bool                  // Whether this is a binary file
	Fragment      *gitdiff.TextFragment // Original fragment from go-gitdiff
	File          *gitdiff.File         // Original file from go-gitdiff
}

// parsePatchFile parses a patch file and returns a list of HunkInfo
// This function uses go-gitdiff for robust patch parsing
func parsePatchFile(patchContent string) ([]HunkInfo, error) {
	return parsePatchFileWithGitDiff(patchContent)
}



// parseHunkSpec parses a hunk specification like "file.go:1,3"
func parseHunkSpec(spec string) (filePath string, hunkNumbers []int, err error) {
	parts := strings.SplitN(spec, ":", 2)
	if len(parts) != 2 {
		return "", nil, NewInvalidArgumentError(fmt.Sprintf("invalid hunk spec format: %s (expected file:numbers)", spec), nil)
	}
	
	filePath = parts[0]
	numbersPart := parts[1]
	
	// Parse comma-separated numbers
	for _, numStr := range strings.Split(numbersPart, ",") {
		numStr = strings.TrimSpace(numStr)
		var num int
		if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
			return "", nil, NewInvalidArgumentError(fmt.Sprintf("invalid hunk number: %s", numStr), err)
		}
		if num <= 0 {
			return "", nil, NewInvalidArgumentError(fmt.Sprintf("hunk number must be positive: %d", num), nil)
		}
		hunkNumbers = append(hunkNumbers, num)
	}
	
	return filePath, hunkNumbers, nil
}