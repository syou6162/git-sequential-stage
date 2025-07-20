package stager

import (
	"strings"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// PatchAnalyzer is responsible for analyzing patch content and extracting file information
type PatchAnalyzer interface {
	// AnalyzePatch analyzes patch content and returns file status information
	AnalyzePatch(patchContent string) (*PatchAnalysisResult, error)
}

// PatchAnalysisResult contains the result of patch analysis
type PatchAnalysisResult struct {
	FilesByStatus    map[FileStatus][]string
	AllFiles         []string
	IntentToAddFiles []string
}

// DefaultPatchAnalyzer implements PatchAnalyzer using go-gitdiff
type DefaultPatchAnalyzer struct{}

// NewPatchAnalyzer creates a new PatchAnalyzer instance
func NewPatchAnalyzer() PatchAnalyzer {
	return &DefaultPatchAnalyzer{}
}

// AnalyzePatch implements PatchAnalyzer.AnalyzePatch
func (a *DefaultPatchAnalyzer) AnalyzePatch(patchContent string) (*PatchAnalysisResult, error) {
	result := &PatchAnalysisResult{
		FilesByStatus:    make(map[FileStatus][]string),
		AllFiles:         []string{},
		IntentToAddFiles: []string{},
	}

	// If no patch content, return empty result
	if strings.TrimSpace(patchContent) == "" {
		return result, nil
	}

	// Parse the patch using go-gitdiff for comprehensive analysis
	files, _, err := gitdiff.Parse(strings.NewReader(patchContent))
	if err != nil {
		return nil, NewSafetyError(GitOperationFailed,
			"Failed to parse patch content",
			"Check if the patch content is valid", err)
	}

	// Check if we have a valid patch with actual file changes
	if len(files) == 0 && strings.TrimSpace(patchContent) != "" {
		// Non-empty content but no files parsed - likely invalid patch format
		return nil, NewSafetyError(GitOperationFailed,
			"Invalid patch format - no file changes detected",
			"Ensure the patch content is in valid git diff format", nil)
	}

	// Extract file information from go-gitdiff analysis
	for _, file := range files {
		filename := file.NewName
		if file.IsDelete {
			filename = file.OldName
		}

		// Add to all files list
		result.AllFiles = append(result.AllFiles, filename)

		// Detect intent-to-add files (empty blobs in new files, but not binary)
		if file.IsNew && len(file.TextFragments) == 0 && !file.IsBinary {
			result.IntentToAddFiles = append(result.IntentToAddFiles, filename)
		}

		// Categorize files based on go-gitdiff detection
		switch {
		case file.IsBinary:
			// Handle binary files first (they can also be new/modified/etc)
			result.FilesByStatus[FileStatusBinary] = append(result.FilesByStatus[FileStatusBinary], filename)
		case file.IsNew:
			result.FilesByStatus[FileStatusAdded] = append(result.FilesByStatus[FileStatusAdded], filename)
		case file.IsDelete:
			result.FilesByStatus[FileStatusDeleted] = append(result.FilesByStatus[FileStatusDeleted], filename)
		case file.IsRename:
			// Store rename with proper notation
			renameNotation := file.OldName + " -> " + file.NewName
			result.FilesByStatus[FileStatusRenamed] = append(result.FilesByStatus[FileStatusRenamed], renameNotation)
		case file.IsCopy:
			// Store copy with proper notation
			copyNotation := file.OldName + " -> " + file.NewName
			result.FilesByStatus[FileStatusCopied] = append(result.FilesByStatus[FileStatusCopied], copyNotation)
		default:
			// Regular modifications
			result.FilesByStatus[FileStatusModified] = append(result.FilesByStatus[FileStatusModified], filename)
		}
	}

	return result, nil
}
