package stager

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/syou6162/git-sequential-stage/internal/executor"
)

func TestStager_StageHunksWithPatchID(t *testing.T) {
	// Create a temporary patch file
	tmpDir := t.TempDir()
	patchFile := filepath.Join(tmpDir, "test.patch")
	patchContent := `diff --git a/file.txt b/file.txt
index abc123..def456 100644
--- a/file.txt
+++ b/file.txt
@@ -1,3 +1,3 @@
 line1
-line2
+line2 modified
 line3
@@ -10,3 +10,3 @@
 line10
-line11
+line11 modified
 line12
`
	
	err := os.WriteFile(patchFile, []byte(patchContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test patch file: %v", err)
	}
	
	tests := []struct {
		name     string
		hunks    string
		setup    func(*executor.MockCommandExecutor)
		wantErr  bool
		errMsg   string
		validate func(*testing.T, *executor.MockCommandExecutor)
	}{
		{
			name:  "stage multiple hunks with patch ID",
			hunks: "1,2",
			setup: func(m *executor.MockCommandExecutor) {
				// filterdiff will be called twice (for hunk 1 and 2)
				m.Commands[fmt.Sprintf("filterdiff [--hunks=1 %s]", patchFile)] = executor.MockResponse{
					Output: []byte(`diff --git a/test.txt b/test.txt
index abc123..def456 100644
--- a/test.txt
+++ b/test.txt
@@ -1,3 +1,3 @@
 line1
-line2
+line2 modified
 line3
`),
					Error: nil,
				}
				m.Commands[fmt.Sprintf("filterdiff [--hunks=2 %s]", patchFile)] = executor.MockResponse{
					Output: []byte(`diff --git a/test.txt b/test.txt
index abc123..def456 100644
--- a/test.txt
+++ b/test.txt
@@ -10,3 +10,3 @@
 line10
-line11
+line11 modified
 line12
`),
					Error: nil,
				}
				// git patch-id will be called twice
				m.Commands["git [patch-id]"] = executor.MockResponse{
					Output: []byte("abcdef1234567890 0000000000000000000000000000000000000000\n"),
					Error:  nil,
				}
				// git apply will be called twice
				m.Commands["git [apply --cached]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
			},
			wantErr: false,
			validate: func(t *testing.T, m *executor.MockCommandExecutor) {
				// Should have: filterdiff 1, git patch-id 1, git apply 1, filterdiff 2, git patch-id 2, git apply 2
				if len(m.ExecutedCommands) != 6 {
					t.Errorf("Expected 6 commands, got %d", len(m.ExecutedCommands))
				}
			},
		},
		{
			name:  "hunk not found",
			hunks: "999",
			setup: func(m *executor.MockCommandExecutor) {
				// filterdiff returns empty output for non-existent hunks
				m.Commands[fmt.Sprintf("filterdiff [--hunks=999 %s]", patchFile)] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
			},
			wantErr: true,
			errMsg:  "failed to extract hunk 999: hunk not found in patch file",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := executor.NewMockCommandExecutor()
			tt.setup(mock)
			
			s := NewStager(mock)
			err := s.StageHunks(tt.hunks, patchFile)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("StageHunks() error = %v, wantErr %v", err, tt.wantErr)
			}
			
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("StageHunks() error message = %v, want %v", err.Error(), tt.errMsg)
			}
			
			if tt.validate != nil {
				tt.validate(t, mock)
			}
		})
	}
}