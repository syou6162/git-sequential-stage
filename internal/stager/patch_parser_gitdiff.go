package stager

import (
	"fmt"
	"strings"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// FileOperation represents the type of file operation
type FileOperation int

const (
	FileOperationModified FileOperation = iota
	FileOperationAdded
	FileOperationDeleted
	FileOperationRenamed
	FileOperationCopied
)

// HunkInfoNew represents information about a single hunk using go-gitdiff
type HunkInfoNew struct {
	GlobalIndex   int           // Global hunk number in the patch file (1, 2, 3, ...)
	FilePath      string        // File path this hunk belongs to (new path for renames)
	OldFilePath   string        // Old file path (for renames)
	IndexInFile   int           // Hunk number within the file (1, 2, 3, ...)
	PatchID       string        // Unique patch ID calculated using git patch-id
	StartLine     int           // Line number where this hunk starts in the patch file
	EndLine       int           // Line number where this hunk ends in the patch file
	Operation     FileOperation // Type of file operation
	IsBinary      bool          // Whether this is a binary file
	Fragment      *gitdiff.TextFragment // Original fragment from go-gitdiff
}

// parsePatchFileWithGitDiff parses a patch file using go-gitdiff library
func parsePatchFileWithGitDiff(patchContent string) ([]HunkInfoNew, error) {
	var hunks []HunkInfoNew
	globalIndex := 0

	// Parse the patch using go-gitdiff
	files, _, err := gitdiff.Parse(strings.NewReader(patchContent))
	if err != nil {
		return nil, NewParsingError("patch with go-gitdiff", err)
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
		if file.IsBinary {
			globalIndex++
			hunks = append(hunks, HunkInfoNew{
				GlobalIndex: globalIndex,
				FilePath:    filePath,
				OldFilePath: oldFilePath,
				IndexInFile: 1, // Binary files have one "hunk"
				Operation:   operation,
				IsBinary:    true,
			})
			continue
		}

		// Process text fragments (hunks)
		for i, fragment := range file.TextFragments {
			globalIndex++
			
			// Calculate approximate line numbers in patch
			// This is an approximation since go-gitdiff doesn't provide original line numbers
			startLine := calculateStartLine(patchContent, file, i)
			endLine := calculateEndLine(patchContent, file, i, fragment)

			hunks = append(hunks, HunkInfoNew{
				GlobalIndex: globalIndex,
				FilePath:    filePath,
				OldFilePath: oldFilePath,
				IndexInFile: i + 1,
				Operation:   operation,
				IsBinary:    false,
				StartLine:   startLine,
				EndLine:     endLine,
				Fragment:    fragment,
			})
		}
	}

	return hunks, nil
}

// calculateStartLine estimates the start line of a hunk in the original patch
func calculateStartLine(patchContent string, file *gitdiff.File, fragmentIndex int) int {
	lines := strings.Split(patchContent, "\n")
	
	// Build possible file headers
	var fileHeaders []string
	
	// Standard format
	if file.OldName != "" && file.NewName != "" {
		fileHeaders = append(fileHeaders, fmt.Sprintf("diff --git a/%s b/%s", file.OldName, file.NewName))
	}
	if file.IsNew && file.NewName != "" {
		fileHeaders = append(fileHeaders, fmt.Sprintf("diff --git a/%s b/%s", file.NewName, file.NewName))
	}
	if file.IsDelete && file.OldName != "" {
		fileHeaders = append(fileHeaders, fmt.Sprintf("diff --git a/%s b/%s", file.OldName, file.OldName))
	}
	
	// Also check without 'a/' and 'b/' prefixes for compatibility
	if file.OldName != "" && file.NewName != "" {
		fileHeaders = append(fileHeaders, fmt.Sprintf("diff --git %s %s", file.OldName, file.NewName))
	}
	
	foundFile := false
	hunkCount := 0
	
	for i, line := range lines {
		// Check if this is our file
		for _, header := range fileHeaders {
			if strings.Contains(line, header) {
				foundFile = true
				break
			}
		}
		
		if foundFile && strings.HasPrefix(line, "@@") {
			if hunkCount == fragmentIndex {
				return i
			}
			hunkCount++
		}
		
		// Reset if we hit another file
		if foundFile && strings.HasPrefix(line, "diff --git") {
			isOurFile := false
			for _, header := range fileHeaders {
				if strings.Contains(line, header) {
					isOurFile = true
					break
				}
			}
			if !isOurFile {
				break
			}
		}
	}
	
	return 0
}

// calculateEndLine estimates the end line of a hunk in the original patch
func calculateEndLine(patchContent string, file *gitdiff.File, fragmentIndex int, fragment *gitdiff.TextFragment) int {
	startLine := calculateStartLine(patchContent, file, fragmentIndex)
	
	// Count the lines in the fragment
	lineCount := 1 // hunk header
	lineCount += len(fragment.Lines)
	
	return startLine + lineCount - 1
}

// convertToHunkInfo converts HunkInfoNew to the original HunkInfo format
// This is a temporary adapter function until we fully migrate
func convertToHunkInfo(newHunks []HunkInfoNew) []HunkInfo {
	oldHunks := make([]HunkInfo, 0, len(newHunks))
	
	for _, h := range newHunks {
		// Skip binary files in the old format
		if h.IsBinary {
			continue
		}
		
		// Use the appropriate file path based on operation
		filePath := h.FilePath
		if h.Operation == FileOperationDeleted {
			filePath = h.OldFilePath
		}
		
		oldHunks = append(oldHunks, HunkInfo{
			GlobalIndex: h.GlobalIndex,
			FilePath:    filePath,
			IndexInFile: h.IndexInFile,
			PatchID:     h.PatchID,
			StartLine:   h.StartLine,
			EndLine:     h.EndLine,
		})
	}
	
	return oldHunks
}

// extractHunkContentFromFragment extracts hunk content from a go-gitdiff fragment
func extractHunkContentFromFragment(file *gitdiff.File, fragment *gitdiff.TextFragment) (string, error) {
	var result strings.Builder
	
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
	
	// Write index line (simplified)
	result.WriteString("index 0000000..0000000 100644\n")
	result.WriteString(fmt.Sprintf("--- a/%s\n", file.OldName))
	result.WriteString(fmt.Sprintf("+++ b/%s\n", file.NewName))
	
	// Write hunk header
	result.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@", 
		fragment.OldPosition, fragment.OldLines,
		fragment.NewPosition, fragment.NewLines))
	// Note: go-gitdiff doesn't expose the function context header
	result.WriteString("\n")
	
	// Write hunk lines
	for _, line := range fragment.Lines {
		switch line.Op {
		case gitdiff.OpContext:
			result.WriteString(" " + line.Line)
		case gitdiff.OpDelete:
			result.WriteString("-" + line.Line)
		case gitdiff.OpAdd:
			result.WriteString("+" + line.Line)
		}
		if !strings.HasSuffix(line.Line, "\n") {
			result.WriteString("\n")
		}
	}
	
	return result.String(), nil
}