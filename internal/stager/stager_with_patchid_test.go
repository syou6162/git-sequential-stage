package stager

import (
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
				// Git apply will be called twice
				m.Commands["git [apply --cached]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
			},
			wantErr: false,
			validate: func(t *testing.T, m *executor.MockCommandExecutor) {
				if len(m.ExecutedCommands) != 2 {
					t.Errorf("Expected 2 commands, got %d", len(m.ExecutedCommands))
				}
				// Check that both commands are git apply
				for i, cmd := range m.ExecutedCommands {
					if cmd.Name != "git" || len(cmd.Args) != 2 || cmd.Args[0] != "apply" || cmd.Args[1] != "--cached" {
						t.Errorf("Command %d: expected 'git apply --cached', got '%s %v'", i, cmd.Name, cmd.Args)
					}
				}
			},
		},
		{
			name:  "hunk not found",
			hunks: "999",
			setup: func(m *executor.MockCommandExecutor) {
				// No commands should be executed
			},
			wantErr: true,
			errMsg:  "hunk 999 not found in patch file",
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