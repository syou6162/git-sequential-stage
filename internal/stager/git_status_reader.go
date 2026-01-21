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

// addIntentToAddFile adds an intent-to-add file to all relevant lists
// This method handles the side effects of registering an intent-to-add file
func (info *GitStatusInfo) addIntentToAddFile(path string) {
	info.StagedFiles = append(info.StagedFiles, path)
	info.FilesByStatus[FileStatusAdded] = append(info.FilesByStatus[FileStatusAdded], path)
	info.IntentToAddFiles = append(info.IntentToAddFiles, path)
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
	// Open the repository with worktree support
	repo, err := git.PlainOpenWithOptions(r.repoPath, &git.PlainOpenOptions{
		EnableDotGitCommonDir: true,
	})
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
		isIntentToAdd, err := r.isIntentToAddCandidate(path, fileStatus)
		if err != nil {
			return nil, fmt.Errorf("failed to check intent-to-add for %s: %w", path, err)
		}
		if isIntentToAdd {
			info.addIntentToAddFile(path)
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

// isIntentToAddCandidate checks if a file should be handled as intent-to-add
// This is critical for LLM agent semantic commit workflows where bulk intent-to-add is used
func (r *DefaultGitStatusReader) isIntentToAddCandidate(path string, fileStatus *git.FileStatus) (bool, error) {
	// Only files with Staging=Added can be intent-to-add candidates
	if fileStatus.Staging != git.Added {
		return false, nil
	}

	// LLM agents use bulk intent-to-add (git add -N) for multiple files, then selectively stage
	// Check if the staged version is empty (intent-to-add marker) to allow coexistence
	return r.isIntentToAddFile(path)
}

// isIntentToAddFile checks if a staged file is an intent-to-add file using go-git
// This is critical for LLM agent workflows where agents use bulk intent-to-add operations
// (git ls-files --others | xargs git add -N) and need to stage specific files without conflicts
func (r *DefaultGitStatusReader) isIntentToAddFile(path string) (bool, error) {
	// Open the repository
	repo, err := git.PlainOpen(r.repoPath)
	if err != nil {
		return false, fmt.Errorf("failed to open repository for intent-to-add check: %w", err)
	}

	// Get the index from repository storer
	idx, err := repo.Storer.Index()
	if err != nil {
		return false, fmt.Errorf("failed to read git index: %w", err)
	}

	// Find the entry for the given path
	entry, err := idx.Entry(path)
	if err != nil {
		// Entry not found is not an error - file may not be in index
		return false, nil
	}

	// Check the IntentToAdd flag using go-git's native field instead of hardcoded hash values
	// This provides type-safe intent-to-add detection for LLM agent semantic commit workflows
	return entry.IntentToAdd, nil
}
