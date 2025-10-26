package stager

import "fmt"

// CountHunksInDiff counts the number of hunks per file in the given diff output.
// It parses the diff using ParsePatchFileWithGitDiff and returns a map of file paths to hunk counts.
// Returns an empty map if diffOutput is empty.
func CountHunksInDiff(diffOutput string) (map[string]int, error) {
	if len(diffOutput) == 0 {
		return make(map[string]int), nil
	}

	// Parse the diff using existing parser
	hunks, err := ParsePatchFileWithGitDiff(diffOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse diff: %w", err)
	}

	// Count hunks per file
	hunkCounts := make(map[string]int)
	for _, hunk := range hunks {
		hunkCounts[hunk.FilePath]++
	}

	return hunkCounts, nil
}
