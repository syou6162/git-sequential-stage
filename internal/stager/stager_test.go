package stager

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/yasuhisa-yoshida/git-sequential-stage/internal/executor"
)

func TestStager_StageHunks(t *testing.T) {
	tests := []struct {
		name      string
		hunks     string
		patchFile string
		setup     func(*executor.MockCommandExecutor)
		wantErr   bool
		errMsg    string
		validate  func(*testing.T, *executor.MockCommandExecutor)
	}{
		{
			name:      "single hunk success",
			hunks:     "1",
			patchFile: "test.patch",
			setup: func(m *executor.MockCommandExecutor) {
				m.Commands["filterdiff [--hunks=1 test.patch]"] = executor.MockResponse{
					Output: []byte("diff --git a/file.txt b/file.txt\n--- a/file.txt\n+++ b/file.txt\n@@ -1,3 +1,3 @@\n line1\n-line2\n+line2 modified\n line3\n"),
					Error:  nil,
				}
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
			},
		},
		{
			name:      "multiple hunks success",
			hunks:     "1,3,5",
			patchFile: "test.patch",
			setup: func(m *executor.MockCommandExecutor) {
				for _, hunk := range []string{"1", "3", "5"} {
					m.Commands["filterdiff [--hunks="+hunk+" test.patch]"] = executor.MockResponse{
						Output: []byte("diff --git a/file.txt b/file.txt\n--- a/file.txt\n+++ b/file.txt\n@@ -1,3 +1,3 @@\n content\n"),
						Error:  nil,
					}
				}
				m.Commands["git [apply --cached]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
			},
			wantErr: false,
			validate: func(t *testing.T, m *executor.MockCommandExecutor) {
				if len(m.ExecutedCommands) != 6 { // 3 filterdiff + 3 git apply
					t.Errorf("Expected 6 commands, got %d", len(m.ExecutedCommands))
				}
			},
		},
		{
			name:      "filterdiff fails",
			hunks:     "1",
			patchFile: "test.patch",
			setup: func(m *executor.MockCommandExecutor) {
				m.Commands["filterdiff [--hunks=1 test.patch]"] = executor.MockResponse{
					Output: nil,
					Error:  errors.New("failed to extract hunk"),
				}
			},
			wantErr: true,
			errMsg:  "failed to extract hunk 1: failed to extract hunk",
		},
		{
			name:      "git apply fails with stderr",
			hunks:     "1",
			patchFile: "test.patch",
			setup: func(m *executor.MockCommandExecutor) {
				m.Commands["filterdiff [--hunks=1 test.patch]"] = executor.MockResponse{
					Output: []byte("diff content"),
					Error:  nil,
				}
				exitErr := &exec.ExitError{
					Stderr: []byte("error: patch does not apply"),
				}
				m.Commands["git [apply --cached]"] = executor.MockResponse{
					Output: nil,
					Error:  exitErr,
				}
			},
			wantErr: true,
			errMsg:  "failed to apply hunk 1: error: patch does not apply",
		},
		{
			name:      "second hunk fails",
			hunks:     "1,2",
			patchFile: "test.patch",
			setup: func(m *executor.MockCommandExecutor) {
				m.Commands["filterdiff [--hunks=1 test.patch]"] = executor.MockResponse{
					Output: []byte("diff content"),
					Error:  nil,
				}
				m.Commands["filterdiff [--hunks=2 test.patch]"] = executor.MockResponse{
					Output: []byte("diff content"),
					Error:  nil,
				}
				// First git apply succeeds
				m.Commands["git [apply --cached]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
			},
			wantErr: false,
			validate: func(t *testing.T, m *executor.MockCommandExecutor) {
				// Should have executed: filterdiff 1, git apply, filterdiff 2, git apply
				if len(m.ExecutedCommands) != 4 {
					t.Errorf("Expected 4 commands, got %d", len(m.ExecutedCommands))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := executor.NewMockCommandExecutor()
			tt.setup(mock)
			
			s := NewStager(mock)
			err := s.StageHunks(tt.hunks, tt.patchFile)
			
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

func TestStager_parseHunks(t *testing.T) {
	tests := []struct {
		name    string
		hunks   string
		want    []int
		wantErr bool
	}{
		{
			name:    "single hunk",
			hunks:   "1",
			want:    []int{1},
			wantErr: false,
		},
		{
			name:    "multiple hunks",
			hunks:   "1,3,5",
			want:    []int{1, 3, 5},
			wantErr: false,
		},
		{
			name:    "hunks with spaces",
			hunks:   " 1 , 3 , 5 ",
			want:    []int{1, 3, 5},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStager(nil)
			got, err := s.parseHunks(tt.hunks)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("parseHunks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if len(got) != len(tt.want) {
				t.Errorf("parseHunks() got %v, want %v", got, tt.want)
				return
			}
			
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseHunks() got[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestStager_getStderrFromError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "exec.ExitError with stderr",
			err: &exec.ExitError{
				Stderr: []byte("error: patch does not apply"),
			},
			want: "error: patch does not apply",
		},
		{
			name: "regular error",
			err:  errors.New("some error"),
			want: "some error",
		},
		{
			name: "nil error",
			err:  nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStager(nil)
			got := s.getStderrFromError(tt.err)
			
			if got != tt.want {
				t.Errorf("getStderrFromError() = %v, want %v", got, tt.want)
			}
		})
	}
}