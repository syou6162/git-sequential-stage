package stager

import (
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
	
	// Read patch file content
	patchContent, err := s.readFile(patchFile)
	if err != nil {
		return fmt.Errorf("failed to read patch file: %v", err)
	}
	
	// Extract all hunks and build patch ID map
	allHunks, err := ExtractHunksFromPatch(patchContent)
	if err != nil {
		return fmt.Errorf("failed to parse patch file: %v", err)
	}
	
	// Create a map for quick lookup by hunk number
	hunkMap := make(map[int]Hunk)
	for _, hunk := range allHunks {
		hunkMap[hunk.Number] = hunk
	}
	
	// Stage each requested hunk
	for _, hunkNum := range hunkNumbers {
		hunk, exists := hunkMap[hunkNum]
		if !exists {
			return fmt.Errorf("hunk %d not found in patch file", hunkNum)
		}
		
		// Apply the hunk using its content
		_, err = s.executor.ExecuteWithStdin("git", strings.NewReader(hunk.Content), "apply", "--cached")
		if err != nil {
			stderr := s.getStderrFromError(err)
			return fmt.Errorf("failed to apply hunk %d (patch ID: %s, file: %s): %s\nNote: This often happens when the hunk has already been staged or when there are conflicts", 
				hunkNum, hunk.PatchID, hunk.FilePath, stderr)
		}
	}
	
	return nil
}

// StageHunksByPatchID stages hunks by their patch IDs
func (s *Stager) StageHunksByPatchID(patchIDs string, patchFile string) error {
	// Read patch file content
	patchContent, err := s.readFile(patchFile)
	if err != nil {
		return fmt.Errorf("failed to read patch file: %v", err)
	}
	
	// Extract all hunks from patch
	allHunks, err := ExtractHunksFromPatch(patchContent)
	if err != nil {
		return fmt.Errorf("failed to parse patch file: %v", err)
	}
	
	// Parse requested patch IDs
	requestedIDs := strings.Split(patchIDs, ",")
	for i, id := range requestedIDs {
		requestedIDs[i] = strings.TrimSpace(id)
	}
	
	// Find and apply requested hunks
	for _, requestedID := range requestedIDs {
		found := false
		for _, hunk := range allHunks {
			if hunk.PatchID == requestedID {
				// Apply this hunk
				_, err = s.executor.ExecuteWithStdin("git", strings.NewReader(hunk.Content), "apply", "--cached")
				if err != nil {
					stderr := s.getStderrFromError(err)
					return fmt.Errorf("failed to apply hunk with patch ID %s (hunk #%d from %s): %s\nNote: This often happens when the hunk has already been staged or when there are conflicts", 
						requestedID, hunk.Number, hunk.FilePath, stderr)
				}
				found = true
				break
			}
		}
		
		if !found {
			return fmt.Errorf("patch ID %s not found in patch file", requestedID)
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