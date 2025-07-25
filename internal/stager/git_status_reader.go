package stager

import (
	"fmt"

	"github.com/go-git/go-git/v5"
)

// GitStatusReader is responsible for reading and parsing git status information
type GitStatusReader interface {
	// ReadStatus reads the current git status and returns parsed file information
	ReadStatus() (*GitStatusInfo, error)
}

// GitStatusInfo contains parsed git status information
type GitStatusInfo struct {
	FilesByStatus    map[FileStatus][]string
	StagedFiles      []string
	IntentToAddFiles []string
}

// DefaultGitStatusReader implements GitStatusReader using go-git
type DefaultGitStatusReader struct {
	repoPath string
}

// NewGitStatusReader creates a new GitStatusReader instance
func NewGitStatusReader(repoPath string) GitStatusReader {
	if repoPath == "" {
		repoPath = "."
	}
	return &DefaultGitStatusReader{
		repoPath: repoPath,
	}
}

// ReadStatus implements GitStatusReader.ReadStatus
func (r *DefaultGitStatusReader) ReadStatus() (*GitStatusInfo, error) {
	// Open the repository
	repo, err := git.PlainOpen(r.repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	// Get the worktree
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	// Get the status
	status, err := worktree.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	return r.parseGitStatus(status)
}

// parseGitStatus parses go-git status information
func (r *DefaultGitStatusReader) parseGitStatus(status git.Status) (*GitStatusInfo, error) {
	info := &GitStatusInfo{
		FilesByStatus:    make(map[FileStatus][]string),
		StagedFiles:      []string{},
		IntentToAddFiles: []string{},
	}

	// Process each file status
	for path, fileStatus := range status {
		// Skip if no staging changes (only worktree changes are not staged)
		// Also skip untracked files as they are not staged
		if fileStatus.Staging == git.Unmodified || fileStatus.Staging == git.Untracked {
			continue
		}

		// Check for intent-to-add files - critical for LLM agent semantic commit workflows
		if r.processIntentToAddFile(path, fileStatus, info) {
			continue
		}

		// Add to staged files list
		info.StagedFiles = append(info.StagedFiles, path)

		// Categorize based on staging status
		switch fileStatus.Staging {
		case git.Modified:
			info.FilesByStatus[FileStatusModified] = append(info.FilesByStatus[FileStatusModified], path)
		case git.Added:
			info.FilesByStatus[FileStatusAdded] = append(info.FilesByStatus[FileStatusAdded], path)
		case git.Deleted:
			info.FilesByStatus[FileStatusDeleted] = append(info.FilesByStatus[FileStatusDeleted], path)
		case git.Renamed:
			// For renamed files, we need to get both old and new names
			// go-git provides this information in the Extra field
			if fileStatus.Extra != "" {
				renameInfo := fmt.Sprintf("%s -> %s", fileStatus.Extra, path)
				info.FilesByStatus[FileStatusRenamed] = append(info.FilesByStatus[FileStatusRenamed], renameInfo)
			} else {
				// Fallback if Extra is not set
				info.FilesByStatus[FileStatusRenamed] = append(info.FilesByStatus[FileStatusRenamed], path)
			}
		case git.Copied:
			// For copied files, similar to renamed
			if fileStatus.Extra != "" {
				copyInfo := fmt.Sprintf("%s -> %s", fileStatus.Extra, path)
				info.FilesByStatus[FileStatusCopied] = append(info.FilesByStatus[FileStatusCopied], copyInfo)
			} else {
				info.FilesByStatus[FileStatusCopied] = append(info.FilesByStatus[FileStatusCopied], path)
			}
		}
	}

	return info, nil
}

// processIntentToAddFile processes intent-to-add files for LLM agent workflows
// Returns true if the file was processed as intent-to-add, false otherwise
func (r *DefaultGitStatusReader) processIntentToAddFile(path string, fileStatus *git.FileStatus, info *GitStatusInfo) bool {
	// Only check files with Staging=Added as potential intent-to-add candidates
	if fileStatus.Staging != git.Added {
		return false
	}

	// LLM agents use bulk intent-to-add (git add -N) for multiple files, then selectively stage
	// We need to check if the staged version is empty (intent-to-add marker) to allow coexistence
	if isIntentToAddFile := r.isIntentToAddFile(path); isIntentToAddFile {
		info.StagedFiles = append(info.StagedFiles, path)
		info.FilesByStatus[FileStatusAdded] = append(info.FilesByStatus[FileStatusAdded], path)
		info.IntentToAddFiles = append(info.IntentToAddFiles, path)
		return true
	}

	return false
}

// isIntentToAddFile checks if a staged file is an intent-to-add file using go-git
// This is critical for LLM agent workflows where agents use bulk intent-to-add operations
// (git ls-files --others | xargs git add -N) and need to stage specific files without conflicts
func (r *DefaultGitStatusReader) isIntentToAddFile(path string) bool {
	// Open the repository
	repo, err := git.PlainOpen(r.repoPath)
	if err != nil {
		return false
	}

	// Get the index from repository storer
	idx, err := repo.Storer.Index()
	if err != nil {
		return false
	}

	// Find the entry for the given path
	entry, err := idx.Entry(path)
	if err != nil {
		return false
	}

	// Check the IntentToAdd flag using go-git's native field instead of hardcoded hash values
	// This provides type-safe intent-to-add detection for LLM agent semantic commit workflows
	return entry.IntentToAdd
}
