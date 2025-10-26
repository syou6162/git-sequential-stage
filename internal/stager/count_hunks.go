package stager

import (
	"fmt"
	"strconv"
)

// CountHunksInDiff counts the number of hunks per file in the given diff output.
// It parses the diff using ParsePatchFileWithGitDiff and returns a map of file paths to hunk counts.
// For binary files, returns "*" to indicate wildcard staging is required.
// For text files, returns the hunk count as a string.
// Returns an empty map if diffOutput is empty.
func CountHunksInDiff(diffOutput string) (map[string]string, error) {
	if len(diffOutput) == 0 {
		return make(map[string]string), nil
	}

	// Parse the diff using existing parser
	hunks, err := ParsePatchFileWithGitDiff(diffOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse diff: %w", err)
	}

	// Count hunks per file, tracking binary status
	hunkCounts := make(map[string]int)
	binaryFiles := make(map[string]bool)

	for _, hunk := range hunks {
		if hunk.IsBinary {
			binaryFiles[hunk.FilePath] = true
		} else {
			hunkCounts[hunk.FilePath]++
		}
	}

	// Convert to string format
	result := make(map[string]string)
	for file := range binaryFiles {
		result[file] = "*"
	}
	for file, count := range hunkCounts {
		result[file] = strconv.Itoa(count)
	}

	return result, nil
}
