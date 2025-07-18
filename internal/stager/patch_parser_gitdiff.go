package stager

import (
	"fmt"
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
		// Note: go-gitdiff may not always detect binary files correctly from the patch format
		// We also check if the file is empty (no text fragments) and patch contains "Binary files"
		isBinary := file.IsBinary || (len(file.TextFragments) == 0 && containsBinaryMarker(patchContent, filePath))
		if isBinary {
			globalIndex++
			hunks = append(hunks, HunkInfo{
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

			hunks = append(hunks, HunkInfo{
				GlobalIndex: globalIndex,
				FilePath:    filePath,
				OldFilePath: oldFilePath,
				IndexInFile: i + 1,
				Operation:   operation,
				IsBinary:    false,
				StartLine:   0, // Line numbers not used in go-gitdiff mode
				EndLine:     0, // Line numbers not used in go-gitdiff mode
				Fragment:    fragment,
			})
		}
	}

	return hunks, nil
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

// containsBinaryMarker checks if the patch content contains a binary file marker for the given file
func containsBinaryMarker(patchContent, filePath string) bool {
	// Look for "Binary files" line that mentions this file
	lines := strings.Split(patchContent, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Binary files") && strings.Contains(line, filePath) {
			return true
		}
	}
	return false
}