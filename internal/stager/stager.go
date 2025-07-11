package stager

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/syou6162/git-sequential-stage/internal/executor"
)

// Stager handles the sequential staging of hunks
type Stager struct {
	executor executor.CommandExecutor
}

// NewStager creates a new stager
func NewStager(exec executor.CommandExecutor) *Stager {
	return &Stager{
		executor: exec,
	}
}

// StageHunks stages the specified hunks sequentially using patch IDs internally
func (s *Stager) StageHunks(hunks string, patchFile string) error {
	hunkNumbers, err := s.parseHunks(hunks)
	if err != nil {
		return err
	}
	
	// Stage each requested hunk using filterdiff + git patch-id workflow
	for _, hunkNum := range hunkNumbers {
		// Extract single hunk using filterdiff
		hunkPatch, err := s.executor.Execute("filterdiff", fmt.Sprintf("--hunks=%d", hunkNum), patchFile)
		if err != nil {
			return fmt.Errorf("failed to extract hunk %d: %v", hunkNum, err)
		}
		
		// Check if filterdiff returned empty output (hunk not found)
		if len(hunkPatch) == 0 {
			return fmt.Errorf("failed to extract hunk %d: hunk not found in patch file", hunkNum)
		}
		
		// Calculate patch ID for this hunk
		patchID, err := s.calculatePatchIDForHunk(hunkPatch)
		if err != nil {
			// Continue without patch ID
			patchID = fmt.Sprintf("unknown-%d", hunkNum)
		}
		
		// Apply the hunk to staging area
		_, err = s.executor.ExecuteWithStdin("git", bytes.NewReader(hunkPatch), "apply", "--cached")
		if err != nil {
			stderr := s.getStderrFromError(err)
			
			// Try to provide more helpful error information
			_, checkErr := s.executor.ExecuteWithStdin("git", bytes.NewReader(hunkPatch), "apply", "--cached", "--check")
			var checkMsg string
			if checkErr != nil {
				checkMsg = s.getStderrFromError(checkErr)
			}
			
			// Extract file path from patch for error message
			filePath := extractFilePathFromPatch(string(hunkPatch))
			
			return fmt.Errorf("failed to apply hunk %d (patch ID: %s):\nFile: %s\nError: %s\n\nDetailed check: %s\n\nPossible causes:\n1. The file has been modified since the patch was created\n2. This hunk has already been staged\n3. There are conflicts with existing changes\n\nTry running 'git status' to check the current state", 
				hunkNum, patchID, filePath, stderr, checkMsg)
		}
	}
	
	return nil
}


// parseHunks parses the comma-separated hunk numbers
func (s *Stager) parseHunks(hunks string) ([]int, error) {
	parts := strings.Split(hunks, ",")
	result := make([]int, 0, len(parts))
	
	for _, part := range parts {
		part = strings.TrimSpace(part)
		num, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid hunk number: %s", part)
		}
		result = append(result, num)
	}
	
	return result, nil
}

// getStderrFromError extracts stderr from exec.ExitError
func (s *Stager) getStderrFromError(err error) string {
	if err == nil {
		return ""
	}
	
	if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
		return string(exitErr.Stderr)
	}
	
	return err.Error()
}

// readFile reads the content of a file
func (s *Stager) readFile(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// calculatePatchIDForHunk calculates patch ID using git patch-id
func (s *Stager) calculatePatchIDForHunk(hunkPatch []byte) (string, error) {
	output, err := s.executor.ExecuteWithStdin("git", bytes.NewReader(hunkPatch), "patch-id")
	if err != nil {
		return "", err
	}
	
	// git patch-id output format: "patch-id commit-id"
	parts := strings.Fields(string(output))
	if len(parts) > 0 {
		// Return first 8 chars for brevity
		if len(parts[0]) >= 8 {
			return parts[0][:8], nil
		}
		return parts[0], nil
	}
	
	return "", fmt.Errorf("unexpected git patch-id output")
}

// extractFilePathFromPatch extracts the file path from a patch
func extractFilePathFromPatch(patch string) string {
	lines := strings.Split(patch, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+++ b/") {
			return strings.TrimPrefix(line, "+++ b/")
		}
		if strings.HasPrefix(line, "diff --git") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				return strings.TrimPrefix(parts[3], "b/")
			}
		}
	}
	return "unknown"
}