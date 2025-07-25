package stager

import (
	"fmt"
	"os/exec"
	"strings"

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
		// Intent-to-add files have Staging=Added and may have various worktree states
		// We need to check if the staged version is empty (intent-to-add marker)
		if fileStatus.Staging == git.Added {
			// Check if this is an intent-to-add file by checking the staged blob
			if isIntentToAddFile := r.isIntentToAddFile(path); isIntentToAddFile {
				info.StagedFiles = append(info.StagedFiles, path)
				info.FilesByStatus[FileStatusAdded] = append(info.FilesByStatus[FileStatusAdded], path)
				info.IntentToAddFiles = append(info.IntentToAddFiles, path)
				continue
			}
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

// isIntentToAddFile checks if a staged file is an intent-to-add file
// Intent-to-add files have the empty file hash in the index
func (r *DefaultGitStatusReader) isIntentToAddFile(path string) bool {
	// Check if the file has the empty blob hash (intent-to-add marker)
	// The empty blob hash is e69de29bb2d1d6434b8b29ae775ad8c2e48c5391
	cmd := exec.Command("git", "ls-files", "-s", path)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Parse the output: "mode hash stage\tfilename"
	// Intent-to-add files have hash e69de29bb2d1d6434b8b29ae775ad8c2e48c5391
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			hash := parts[1]
			// Empty blob hash indicates intent-to-add
			if hash == "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391" {
				return true
			}
		}
	}

	return false
}
