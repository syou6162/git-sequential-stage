package stager

import (
	"fmt"
	"strings"
)

// HunkInfo represents information about a single hunk
type HunkInfo struct {
	GlobalIndex int    // Global hunk number in the patch file (1, 2, 3, ...)
	FilePath    string // File path this hunk belongs to
	IndexInFile int    // Hunk number within the file (1, 2, 3, ...)
	PatchID     string // Unique patch ID calculated using git patch-id
	StartLine   int    // Line number where this hunk starts in the patch file
	EndLine     int    // Line number where this hunk ends in the patch file
}

// parsePatchFile parses a patch file and returns a list of HunkInfo
func parsePatchFile(patchContent string) ([]HunkInfo, error) {
	var hunks []HunkInfo
	globalIndex := 0
	currentFile := ""
	fileHunkIndex := make(map[string]int)
	isNewFile := false
	
	lines := strings.Split(patchContent, "\n")
	lineNum := 0
	
	for lineNum < len(lines) {
		line := lines[lineNum]
		
		// Check for file header
		if strings.HasPrefix(line, "diff --git") {
			// Extract file path from diff line
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				// Handle both regular diffs and --no-index diffs
				filePath := parts[3]
				if strings.HasPrefix(filePath, "b/") {
					currentFile = strings.TrimPrefix(filePath, "b/")
				} else {
					// For --no-index diffs, use the path as-is
					currentFile = filePath
				}
				fileHunkIndex[currentFile] = 0
				isNewFile = false
			}
		} else if strings.HasPrefix(line, "new file mode") {
			isNewFile = true
		} else if strings.HasPrefix(line, "@@") && currentFile != "" {
			// Found a hunk header
			globalIndex++
			fileHunkIndex[currentFile]++
			
			hunkStartLine := lineNum
			
			// Find the end of this hunk
			hunkEndLine := lineNum + 1
			for hunkEndLine < len(lines) {
				nextLine := lines[hunkEndLine]
				// Stop at next hunk header or file header
				if strings.HasPrefix(nextLine, "@@") || strings.HasPrefix(nextLine, "diff --git") {
					break
				}
				hunkEndLine++
			}
			
			hunks = append(hunks, HunkInfo{
				GlobalIndex: globalIndex,
				FilePath:    currentFile,
				IndexInFile: fileHunkIndex[currentFile],
				StartLine:   hunkStartLine,
				EndLine:     hunkEndLine - 1,
			})
			
			// Skip to the end of this hunk
			lineNum = hunkEndLine - 1
		} else if isNewFile && currentFile != "" && strings.HasPrefix(line, "@@ -0,0") {
			// Special case for new files with @@ -0,0 format
			globalIndex++
			fileHunkIndex[currentFile]++
			
			hunkStartLine := lineNum
			
			// For new files, the entire content is one hunk
			hunkEndLine := lineNum + 1
			for hunkEndLine < len(lines) {
				nextLine := lines[hunkEndLine]
				if strings.HasPrefix(nextLine, "diff --git") {
					break
				}
				hunkEndLine++
			}
			
			hunks = append(hunks, HunkInfo{
				GlobalIndex: globalIndex,
				FilePath:    currentFile,
				IndexInFile: fileHunkIndex[currentFile],
				StartLine:   hunkStartLine,
				EndLine:     hunkEndLine - 1,
			})
			
			// Skip to the end of this hunk
			lineNum = hunkEndLine - 1
		}
		
		lineNum++
	}
	
	return hunks, nil
}

// extractHunkContent extracts the content of a hunk including its file header
func extractHunkContent(patchLines []string, hunk HunkInfo) string {
	// Find the file header for this hunk
	fileHeaderStart := hunk.StartLine
	for i := hunk.StartLine - 1; i >= 0; i-- {
		if strings.HasPrefix(patchLines[i], "diff --git") {
			fileHeaderStart = i
			break
		}
	}
	
	// Extract file header up to the hunk header
	var result []string
	for i := fileHeaderStart; i < len(patchLines); i++ {
		line := patchLines[i]
		result = append(result, line)
		// Stop after the hunk header
		if strings.HasPrefix(line, "@@") && i >= hunk.StartLine {
			break
		}
	}
	
	// Extract hunk content (lines after the hunk header)
	for i := hunk.StartLine + 1; i <= hunk.EndLine && i < len(patchLines); i++ {
		result = append(result, patchLines[i])
	}
	
	return strings.Join(result, "\n")
}

// parseHunkSpec parses a hunk specification like "file.go:1,3"
func parseHunkSpec(spec string) (filePath string, hunkNumbers []int, err error) {
	parts := strings.SplitN(spec, ":", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid hunk spec format: %s (expected file:numbers)", spec)
	}
	
	filePath = parts[0]
	numbersPart := parts[1]
	
	// Parse comma-separated numbers
	for _, numStr := range strings.Split(numbersPart, ",") {
		numStr = strings.TrimSpace(numStr)
		var num int
		if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
			return "", nil, fmt.Errorf("invalid hunk number: %s", numStr)
		}
		if num <= 0 {
			return "", nil, fmt.Errorf("hunk number must be positive: %d", num)
		}
		hunkNumbers = append(hunkNumbers, num)
	}
	
	return filePath, hunkNumbers, nil
}