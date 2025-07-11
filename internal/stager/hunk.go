package stager

import (
	"crypto/sha1"
	"fmt"
	"strings"
)

// Hunk represents a single hunk from a patch file
type Hunk struct {
	Number   int
	PatchID  string
	Content  string
	FilePath string
	Header   string
}

// ExtractHunksFromPatch parses a patch file and returns all hunks with their patch IDs
func ExtractHunksFromPatch(patchContent string) ([]Hunk, error) {
	var hunks []Hunk
	lines := strings.Split(patchContent, "\n")
	
	var currentHunk *Hunk
	var hunkContent strings.Builder
	var filePath string
	hunkNumber := 0
	
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		
		// New file header
		if strings.HasPrefix(line, "diff --git") {
			// Save previous hunk if exists
			if currentHunk != nil {
				currentHunk.Content = hunkContent.String()
				currentHunk.PatchID = calculatePatchID(currentHunk.Content)
				hunks = append(hunks, *currentHunk)
			}
			
			// Extract file path
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				filePath = strings.TrimPrefix(parts[3], "b/")
			}
			
			hunkContent.Reset()
			currentHunk = nil
		} else if strings.HasPrefix(line, "@@") {
			// New hunk header
			if currentHunk != nil {
				currentHunk.Content = hunkContent.String()
				currentHunk.PatchID = calculatePatchID(currentHunk.Content)
				hunks = append(hunks, *currentHunk)
			}
			
			hunkNumber++
			hunkContent.Reset()
			
			// Include the file header in hunk content
			if i >= 2 {
				// Go back to find the file header
				for j := i - 1; j >= 0; j-- {
					if strings.HasPrefix(lines[j], "diff --git") {
						// Include diff --git, index, ---, and +++ lines
						for k := j; k < i && k < len(lines); k++ {
							hunkContent.WriteString(lines[k])
							hunkContent.WriteString("\n")
						}
						break
					}
				}
			}
			
			currentHunk = &Hunk{
				Number:   hunkNumber,
				FilePath: filePath,
				Header:   line,
			}
			hunkContent.WriteString(line)
			hunkContent.WriteString("\n")
		} else if currentHunk != nil {
			// Part of current hunk
			hunkContent.WriteString(line)
			// Only add newline if not the last line or if the line is not empty
			if i < len(lines)-1 || line != "" {
				hunkContent.WriteString("\n")
			}
		}
	}
	
	// Save last hunk
	if currentHunk != nil {
		currentHunk.Content = hunkContent.String()
		currentHunk.PatchID = calculatePatchID(currentHunk.Content)
		hunks = append(hunks, *currentHunk)
	}
	
	return hunks, nil
}

// calculatePatchID generates a unique ID for a hunk using SHA1
func calculatePatchID(content string) string {
	h := sha1.New()
	h.Write([]byte(content))
	return fmt.Sprintf("%x", h.Sum(nil))[:8] // Use first 8 chars for brevity
}