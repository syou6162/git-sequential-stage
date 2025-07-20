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

		// Check for intent-to-add files
		// Intent-to-add files have Staging=Added and Worktree=Untracked
		if fileStatus.Staging == git.Added && fileStatus.Worktree == git.Untracked {
			info.StagedFiles = append(info.StagedFiles, path)
			info.FilesByStatus[FileStatusAdded] = append(info.FilesByStatus[FileStatusAdded], path)
			info.IntentToAddFiles = append(info.IntentToAddFiles, path)
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