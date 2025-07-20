package stager

import (
	"fmt"
	"strings"

	"github.com/syou6162/git-sequential-stage/internal/executor"
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

// DefaultGitStatusReader implements GitStatusReader using git commands
type DefaultGitStatusReader struct {
	executor executor.CommandExecutor
}

// NewGitStatusReader creates a new GitStatusReader instance
func NewGitStatusReader(executor executor.CommandExecutor) GitStatusReader {
	return &DefaultGitStatusReader{
		executor: executor,
	}
}

// ReadStatus implements GitStatusReader.ReadStatus
func (r *DefaultGitStatusReader) ReadStatus() (*GitStatusInfo, error) {
	if r.executor == nil {
		return nil, fmt.Errorf("executor is required for git status reading")
	}

	// Get the actual staging area status using git status
	output, err := r.executor.Execute("git", "status", "--porcelain")
	if err != nil {
		return nil, NewGitCommandError("git status", err)
	}

	return r.parseGitStatus(string(output))
}

// parseGitStatus parses git status --porcelain output
func (r *DefaultGitStatusReader) parseGitStatus(output string) (*GitStatusInfo, error) {
	info := &GitStatusInfo{
		FilesByStatus:    make(map[FileStatus][]string),
		StagedFiles:      []string{},
		IntentToAddFiles: []string{},
	}

	// Empty output means clean staging area
	if strings.TrimSpace(output) == "" {
		return info, nil
	}

	// Process each line of git status output
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Skip empty lines
		if line == "" {
			continue
		}
		if len(line) < 3 {
			continue
		}

		statusCode := line[0:2]
		filename := strings.TrimSpace(line[2:])

		// Only process staged changes (first character is not space)
		if statusCode[0] == GitStatusCodeSpace {
			// Special case: ' A' indicates intent-to-add file
			if statusCode[1] == GitStatusCodeAdded {
				info.StagedFiles = append(info.StagedFiles, filename)
				info.FilesByStatus[FileStatusAdded] = append(info.FilesByStatus[FileStatusAdded], filename)
				info.IntentToAddFiles = append(info.IntentToAddFiles, filename)
			}
			continue
		}

		// Handle rename/copy specially
		if strings.Contains(line, " -> ") {
			parts := strings.Split(filename, " -> ")
			if len(parts) == 2 {
				oldName := parts[0]
				newName := parts[1]
				if statusCode[0] == GitStatusCodeRenamed {
					info.FilesByStatus[FileStatusRenamed] = append(info.FilesByStatus[FileStatusRenamed], oldName+" -> "+newName)
					info.StagedFiles = append(info.StagedFiles, newName)
				} else if statusCode[0] == GitStatusCodeCopied {
					info.FilesByStatus[FileStatusCopied] = append(info.FilesByStatus[FileStatusCopied], oldName+" -> "+newName)
					info.StagedFiles = append(info.StagedFiles, newName)
				}
				continue
			}
		}

		// Add to staged files list
		info.StagedFiles = append(info.StagedFiles, filename)

		// Categorize based on status code
		switch statusCode[0] {
		case GitStatusCodeModified:
			info.FilesByStatus[FileStatusModified] = append(info.FilesByStatus[FileStatusModified], filename)
		case GitStatusCodeAdded:
			info.FilesByStatus[FileStatusAdded] = append(info.FilesByStatus[FileStatusAdded], filename)
			// Regular added files (not intent-to-add) are handled here
		case GitStatusCodeDeleted:
			info.FilesByStatus[FileStatusDeleted] = append(info.FilesByStatus[FileStatusDeleted], filename)
		}
	}

	return info, nil
}