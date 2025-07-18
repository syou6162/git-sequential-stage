package stager

import (
	"strings"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)


// parsePatchFileWithGitDiff parses a patch file using go-gitdiff library
func parsePatchFileWithGitDiff(patchContent string) ([]HunkInfo, error) {
	var hunks []HunkInfo
	globalIndex := 0

	// Parse the patch using go-gitdiff
	files, _, err := gitdiff.Parse(strings.NewReader(patchContent))
	if err != nil {
		return nil, NewParsingError("patch with go-gitdiff", err)
	}
	
	// Check if the patch contains binary file markers
	// go-gitdiff doesn't automatically detect "Binary files ... differ" as binary
	binaryFiles := make(map[string]bool)
	lines := strings.Split(patchContent, "\n")
	currentFile := ""
	for _, line := range lines {
		// Track current file being processed
		if strings.HasPrefix(line, "diff --git ") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				// Extract filename from "diff --git a/file b/file"
				currentFile = strings.TrimPrefix(parts[3], "b/")
			}
		}
		
		// Check for binary file marker
		if strings.HasPrefix(line, "Binary files") && strings.HasSuffix(line, "differ") && currentFile != "" {
			binaryFiles[currentFile] = true
		}
	}

	// Process each file in the patch
	for _, file := range files {
		// Determine file operation
		var operation FileOperation
		var filePath, oldFilePath string

		switch {
		case file.IsDelete:
			operation = FileOperationDeleted
			filePath = file.OldName
			oldFilePath = file.OldName
		case file.IsNew:
			operation = FileOperationAdded
			filePath = file.NewName
			oldFilePath = ""
		case file.IsRename:
			operation = FileOperationRenamed
			filePath = file.NewName
			oldFilePath = file.OldName
		case file.IsCopy:
			operation = FileOperationCopied
			filePath = file.NewName
			oldFilePath = file.OldName
		default:
			operation = FileOperationModified
			filePath = file.NewName
			oldFilePath = file.OldName
		}


		// Handle binary files
		// Check both go-gitdiff detection and our manual detection
		if file.IsBinary || binaryFiles[filePath] {
			globalIndex++
			hunks = append(hunks, HunkInfo{
				GlobalIndex: globalIndex,
				FilePath:    filePath,
				OldFilePath: oldFilePath,
				IndexInFile: 1, // Binary files have one "hunk"
				Operation:   operation,
				IsBinary:    true,
				File:        file,
			})
			continue
		}

		// Process text fragments (hunks)
		for i, fragment := range file.TextFragments {
			globalIndex++

			hunks = append(hunks, HunkInfo{
				GlobalIndex: globalIndex,
				FilePath:    filePath,
				OldFilePath: oldFilePath,
				IndexInFile: i + 1,
				Operation:   operation,
				IsBinary:    false,
				Fragment:    fragment,
				File:        file,
			})
		}
	}

	return hunks, nil
}


