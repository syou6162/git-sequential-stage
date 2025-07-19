package stager

import (
	"errors"
	"log"
	"reflect"
	"strings"
	"testing"

	"github.com/syou6162/git-sequential-stage/internal/executor"
)

func TestNewSafetyChecker(t *testing.T) {
	mockExec := executor.NewMockCommandExecutor()
	logger := log.New(log.Writer(), "[test] ", log.LstdFlags)
	
	checker := NewSafetyChecker(mockExec, logger)
	
	if checker == nil {
		t.Fatal("NewSafetyChecker returned nil")
	}
	
	if checker.executor != mockExec {
		t.Error("executor not set correctly")
	}
	
	if checker.logger != logger {
		t.Error("logger not set correctly")
	}
}

func TestSafetyChecker_ValidateGitState(t *testing.T) {
	tests := []struct {
		name      string
		mockSetup func(*executor.MockCommandExecutor)
		wantErr   bool
		errorType SafetyErrorType
	}{
		{
			name: "valid git repository",
			mockSetup: func(m *executor.MockCommandExecutor) {
				m.Commands["git [rev-parse --git-dir]"] = executor.MockResponse{
					Output: []byte(".git"),
					Error:  nil,
				}
			},
			wantErr: false,
		},
		{
			name: "not a git repository",
			mockSetup: func(m *executor.MockCommandExecutor) {
				m.Commands["git [rev-parse --git-dir]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  errors.New("not a git repository"),
				}
			},
			wantErr:   true,
			errorType: ErrorTypeGitOperationFailed,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := executor.NewMockCommandExecutor()
			tt.mockSetup(mockExec)
			
			checker := NewSafetyChecker(mockExec, log.New(log.Writer(), "", 0))
			err := checker.ValidateGitState()
			
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateGitState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if tt.wantErr && err != nil {
				var safetyErr *SafetyError
				if errors.As(err, &safetyErr) {
					if safetyErr.Type != tt.errorType {
						t.Errorf("Error type = %v, want %v", safetyErr.Type, tt.errorType)
					}
				} else {
					t.Errorf("Expected SafetyError, got %T", err)
				}
			}
		})
	}
}

func TestSafetyChecker_EvaluateStagingArea(t *testing.T) {
	tests := []struct {
		name      string
		mockSetup func(*executor.MockCommandExecutor)
		want      *StagingAreaEvaluation
		wantErr   bool
	}{
		{
			name: "clean staging area",
			mockSetup: func(m *executor.MockCommandExecutor) {
				m.Commands["git [status --porcelain]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
				m.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
			},
			want: &StagingAreaEvaluation{
				IsClean:            true,
				StagedFiles:        []string{},
				IntentToAddFiles:   []string{},
				FilesByStatus:      map[string][]string{},
				AllowContinue:      false,
				RecommendedActions: []RecommendedAction{},
			},
			wantErr: false,
		},
		{
			name: "modified files staged",
			mockSetup: func(m *executor.MockCommandExecutor) {
				m.Commands["git [status --porcelain]"] = executor.MockResponse{
					Output: []byte("M  file1.go\nM  file2.go\n"),
					Error:  nil,
				}
				m.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
			},
			want: &StagingAreaEvaluation{
				IsClean:          false,
				StagedFiles:      []string{"file1.go", "file2.go"},
				IntentToAddFiles: []string{},
				FilesByStatus: map[string][]string{
					"M": {"file1.go", "file2.go"},
				},
				AllowContinue: false,
				ErrorMessage:  "Staging area contains staged files",
			},
			wantErr: false,
		},
		{
			name: "new files staged",
			mockSetup: func(m *executor.MockCommandExecutor) {
				m.Commands["git [status --porcelain]"] = executor.MockResponse{
					Output: []byte("A  new1.go\nA  new2.go\n"),
					Error:  nil,
				}
				m.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
					Output: []byte("new1.go\nnew2.go\n"),
					Error:  nil,
				}
				// For intent-to-add detection
				m.Commands["git [diff --cached -- new1.go]"] = executor.MockResponse{
					Output: []byte("diff --git a/new1.go b/new1.go\n+++ b/new1.go\n+content"),
					Error:  nil,
				}
				m.Commands["git [diff --cached -- new2.go]"] = executor.MockResponse{
					Output: []byte("diff --git a/new2.go b/new2.go\n+++ b/new2.go\n+content"),
					Error:  nil,
				}
			},
			want: &StagingAreaEvaluation{
				IsClean:          false,
				StagedFiles:      []string{"new1.go", "new2.go"},
				IntentToAddFiles: []string{},
				FilesByStatus: map[string][]string{
					"A": {"new1.go", "new2.go"},
				},
				AllowContinue: false,
				ErrorMessage:  "Staging area contains staged files",
			},
			wantErr: false,
		},
		{
			name: "deleted files staged",
			mockSetup: func(m *executor.MockCommandExecutor) {
				m.Commands["git [status --porcelain]"] = executor.MockResponse{
					Output: []byte("D  deleted.go\n"),
					Error:  nil,
				}
				m.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
			},
			want: &StagingAreaEvaluation{
				IsClean:          false,
				StagedFiles:      []string{"deleted.go"},
				IntentToAddFiles: []string{},
				FilesByStatus: map[string][]string{
					"D": {"deleted.go"},
				},
				AllowContinue: false,
				ErrorMessage:  "Staging area contains staged files",
			},
			wantErr: false,
		},
		{
			name: "renamed files staged",
			mockSetup: func(m *executor.MockCommandExecutor) {
				m.Commands["git [status --porcelain]"] = executor.MockResponse{
					Output: []byte("R  old.go -> new.go\n"),
					Error:  nil,
				}
				m.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
			},
			want: &StagingAreaEvaluation{
				IsClean:          false,
				StagedFiles:      []string{"old.go -> new.go"},
				IntentToAddFiles: []string{},
				FilesByStatus: map[string][]string{
					"R": {"old.go -> new.go"},
				},
				AllowContinue: false,
				ErrorMessage:  "Staging area contains staged files",
			},
			wantErr: false,
		},
		{
			name: "intent-to-add files only",
			mockSetup: func(m *executor.MockCommandExecutor) {
				m.Commands["git [status --porcelain]"] = executor.MockResponse{
					Output: []byte("A  intent.go\n"),
					Error:  nil,
				}
				m.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
					Output: []byte("intent.go\n"),
					Error:  nil,
				}
				// Intent-to-add file has empty diff
				m.Commands["git [diff --cached -- intent.go]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
				m.Commands["git [ls-files -- intent.go]"] = executor.MockResponse{
					Output: []byte("intent.go\n"),
					Error:  nil,
				}
			},
			want: &StagingAreaEvaluation{
				IsClean:          false,
				StagedFiles:      []string{"intent.go"},
				IntentToAddFiles: []string{"intent.go"},
				FilesByStatus: map[string][]string{
					"A": {"intent.go"},
				},
				AllowContinue: true,
				ErrorMessage:  "Intent-to-add files detected (semantic_commit workflow)",
			},
			wantErr: false,
		},
		{
			name: "mixed staged and intent-to-add",
			mockSetup: func(m *executor.MockCommandExecutor) {
				m.Commands["git [status --porcelain]"] = executor.MockResponse{
					Output: []byte("M  modified.go\nA  intent.go\n"),
					Error:  nil,
				}
				m.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
					Output: []byte("intent.go\n"),
					Error:  nil,
				}
				// Intent-to-add file has empty diff
				m.Commands["git [diff --cached -- intent.go]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
				m.Commands["git [ls-files -- intent.go]"] = executor.MockResponse{
					Output: []byte("intent.go\n"),
					Error:  nil,
				}
			},
			want: &StagingAreaEvaluation{
				IsClean:          false,
				StagedFiles:      []string{"modified.go", "intent.go"},
				IntentToAddFiles: []string{"intent.go"},
				FilesByStatus: map[string][]string{
					"M": {"modified.go"},
					"A": {"intent.go"},
				},
				AllowContinue: false,
				ErrorMessage:  "Staging area contains staged files",
			},
			wantErr: false,
		},
		{
			name: "git status fails",
			mockSetup: func(m *executor.MockCommandExecutor) {
				m.Commands["git [status --porcelain]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  errors.New("git error"),
				}
			},
			want:    nil,
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := executor.NewMockCommandExecutor()
			tt.mockSetup(mockExec)
			
			checker := NewSafetyChecker(mockExec, log.New(log.Writer(), "", 0))
			got, err := checker.EvaluateStagingArea()
			
			if (err != nil) != tt.wantErr {
				t.Errorf("EvaluateStagingArea() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if err != nil {
				return
			}
			
			// Compare basic fields
			if got.IsClean != tt.want.IsClean {
				t.Errorf("IsClean = %v, want %v", got.IsClean, tt.want.IsClean)
			}
			
			if got.AllowContinue != tt.want.AllowContinue {
				t.Errorf("AllowContinue = %v, want %v", got.AllowContinue, tt.want.AllowContinue)
			}
			
			if got.ErrorMessage != tt.want.ErrorMessage {
				t.Errorf("ErrorMessage = %q, want %q", got.ErrorMessage, tt.want.ErrorMessage)
			}
			
			// Compare slices
			if !reflect.DeepEqual(got.StagedFiles, tt.want.StagedFiles) {
				t.Errorf("StagedFiles = %#v (len=%d), want %#v (len=%d)", 
					got.StagedFiles, len(got.StagedFiles), 
					tt.want.StagedFiles, len(tt.want.StagedFiles))
			}
			
			if !reflect.DeepEqual(got.IntentToAddFiles, tt.want.IntentToAddFiles) {
				t.Errorf("IntentToAddFiles = %#v (len=%d), want %#v (len=%d)", 
					got.IntentToAddFiles, len(got.IntentToAddFiles),
					tt.want.IntentToAddFiles, len(tt.want.IntentToAddFiles))
			}
			
			// Compare maps
			if !reflect.DeepEqual(got.FilesByStatus, tt.want.FilesByStatus) {
				t.Errorf("FilesByStatus = %#v, want %#v", got.FilesByStatus, tt.want.FilesByStatus)
			}
		})
	}
}

func TestSafetyChecker_DetectIntentToAddFiles(t *testing.T) {
	tests := []struct {
		name      string
		mockSetup func(*executor.MockCommandExecutor)
		want      []string
		wantErr   bool
	}{
		{
			name: "no intent-to-add files",
			mockSetup: func(m *executor.MockCommandExecutor) {
				m.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
			},
			want:    []string{},
			wantErr: false,
		},
		{
			name: "single intent-to-add file",
			mockSetup: func(m *executor.MockCommandExecutor) {
				m.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
					Output: []byte("new.go\n"),
					Error:  nil,
				}
				m.Commands["git [diff --cached -- new.go]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				} // Empty diff indicates intent-to-add
				m.Commands["git [ls-files -- new.go]"] = executor.MockResponse{
					Output: []byte("new.go\n"),
					Error:  nil,
				}
			},
			want:    []string{"new.go"},
			wantErr: false,
		},
		{
			name: "multiple intent-to-add files",
			mockSetup: func(m *executor.MockCommandExecutor) {
				m.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
					Output: []byte("file1.go\nfile2.go\n"),
					Error:  nil,
				}
				m.Commands["git [diff --cached -- file1.go]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
				m.Commands["git [ls-files -- file1.go]"] = executor.MockResponse{
					Output: []byte("file1.go\n"),
					Error:  nil,
				}
				m.Commands["git [diff --cached -- file2.go]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
				m.Commands["git [ls-files -- file2.go]"] = executor.MockResponse{
					Output: []byte("file2.go\n"),
					Error:  nil,
				}
			},
			want:    []string{"file1.go", "file2.go"},
			wantErr: false,
		},
		{
			name: "mixed regular and intent-to-add",
			mockSetup: func(m *executor.MockCommandExecutor) {
				m.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
					Output: []byte("regular.go\nintent.go\n"),
					Error:  nil,
				}
				m.Commands["git [diff --cached -- regular.go]"] = executor.MockResponse{
					Output: []byte("diff --git a/regular.go\n+++ b/regular.go\n+content"),
					Error:  nil,
				}
				m.Commands["git [diff --cached -- intent.go]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  nil,
				}
				m.Commands["git [ls-files -- intent.go]"] = executor.MockResponse{
					Output: []byte("intent.go\n"),
					Error:  nil,
				}
			},
			want:    []string{"intent.go"},
			wantErr: false,
		},
		{
			name: "git diff fails",
			mockSetup: func(m *executor.MockCommandExecutor) {
				m.Commands["git [diff --name-only --diff-filter=A --cached]"] = executor.MockResponse{
					Output: []byte(""),
					Error:  errors.New("git error"),
				}
			},
			want:    nil,
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := executor.NewMockCommandExecutor()
			tt.mockSetup(mockExec)
			
			checker := NewSafetyChecker(mockExec, log.New(log.Writer(), "", 0))
			got, err := checker.DetectIntentToAddFiles()
			
			if (err != nil) != tt.wantErr {
				t.Errorf("DetectIntentToAddFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DetectIntentToAddFiles() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSafetyChecker_generateRecommendedActions(t *testing.T) {
	tests := []struct {
		name       string
		evaluation *StagingAreaEvaluation
		wantTypes  []string // Expected action categories in order
	}{
		{
			name: "clean staging area",
			evaluation: &StagingAreaEvaluation{
				IsClean: true,
			},
			wantTypes: []string{},
		},
		{
			name: "intent-to-add files only",
			evaluation: &StagingAreaEvaluation{
				IsClean:          false,
				StagedFiles:      []string{"new.go"},
				IntentToAddFiles: []string{"new.go"},
				FilesByStatus:    map[string][]string{"A": {"new.go"}},
			},
			wantTypes: []string{"info"},
		},
		{
			name: "deleted files",
			evaluation: &StagingAreaEvaluation{
				IsClean:          false,
				StagedFiles:      []string{"deleted.go"},
				IntentToAddFiles: []string{},
				FilesByStatus:    map[string][]string{"D": {"deleted.go"}},
			},
			wantTypes: []string{"commit", "commit", "unstage"},
		},
		{
			name: "renamed files",
			evaluation: &StagingAreaEvaluation{
				IsClean:          false,
				StagedFiles:      []string{"old.go -> new.go"},
				IntentToAddFiles: []string{},
				FilesByStatus:    map[string][]string{"R": {"old.go -> new.go"}},
			},
			wantTypes: []string{"commit", "commit", "unstage"},
		},
		{
			name: "modified files",
			evaluation: &StagingAreaEvaluation{
				IsClean:          false,
				StagedFiles:      []string{"file1.go", "file2.go"},
				IntentToAddFiles: []string{},
				FilesByStatus:    map[string][]string{"M": {"file1.go", "file2.go"}},
			},
			wantTypes: []string{"commit", "unstage", "unstage", "unstage"},
		},
		{
			name: "mixed types",
			evaluation: &StagingAreaEvaluation{
				IsClean:          false,
				StagedFiles:      []string{"modified.go", "deleted.go", "intent.go"},
				IntentToAddFiles: []string{"intent.go"},
				FilesByStatus: map[string][]string{
					"M": {"modified.go"},
					"D": {"deleted.go"},
					"A": {"intent.go"},
				},
			},
			wantTypes: []string{"info", "commit", "commit", "unstage", "unstage"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := &executor.MockCommandExecutor{}
			checker := NewSafetyChecker(mockExec, log.New(log.Writer(), "", 0))
			
			actions := checker.generateRecommendedActions(tt.evaluation)
			
			// Extract categories
			gotTypes := []string{}
			for _, action := range actions {
				gotTypes = append(gotTypes, action.Category)
			}
			
			if !reflect.DeepEqual(gotTypes, tt.wantTypes) {
				t.Errorf("Action categories = %#v (len=%d), want %#v (len=%d)", 
					gotTypes, len(gotTypes), tt.wantTypes, len(tt.wantTypes))
			}
			
			// Verify actions are sorted by priority
			for i := 1; i < len(actions); i++ {
				if actions[i].Priority < actions[i-1].Priority {
					t.Errorf("Actions not sorted by priority: %d < %d at index %d", 
						actions[i].Priority, actions[i-1].Priority, i)
				}
			}
			
			// Verify each action has commands
			for i, action := range actions {
				if len(action.Commands) == 0 {
					t.Errorf("Action %d has no commands", i)
				}
				if action.Description == "" {
					t.Errorf("Action %d has no description", i)
				}
			}
		})
	}
}

func TestSafetyChecker_allFilesAreIntentToAdd(t *testing.T) {
	tests := []struct {
		name       string
		evaluation *StagingAreaEvaluation
		want       bool
	}{
		{
			name: "all files are intent-to-add",
			evaluation: &StagingAreaEvaluation{
				StagedFiles:      []string{"file1.go", "file2.go"},
				IntentToAddFiles: []string{"file1.go", "file2.go"},
			},
			want: true,
		},
		{
			name: "some files are intent-to-add",
			evaluation: &StagingAreaEvaluation{
				StagedFiles:      []string{"file1.go", "file2.go", "file3.go"},
				IntentToAddFiles: []string{"file1.go", "file2.go"},
			},
			want: false,
		},
		{
			name: "no intent-to-add files",
			evaluation: &StagingAreaEvaluation{
				StagedFiles:      []string{"file1.go", "file2.go"},
				IntentToAddFiles: []string{},
			},
			want: false,
		},
		{
			name: "no staged files",
			evaluation: &StagingAreaEvaluation{
				StagedFiles:      []string{},
				IntentToAddFiles: []string{},
			},
			want: true, // Vacuously true
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := &executor.MockCommandExecutor{}
			checker := NewSafetyChecker(mockExec, log.New(log.Writer(), "", 0))
			
			got := checker.allFilesAreIntentToAdd(tt.evaluation)
			if got != tt.want {
				t.Errorf("allFilesAreIntentToAdd() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test helper to verify recommended actions contain expected git commands
func TestSafetyChecker_recommendedActionsCommands(t *testing.T) {
	evaluation := &StagingAreaEvaluation{
		IsClean:          false,
		StagedFiles:      []string{"file.go", "deleted.go"},
		IntentToAddFiles: []string{},
		FilesByStatus: map[string][]string{
			"M": {"file.go"},
			"D": {"deleted.go"},
		},
	}
	
	mockExec := executor.NewMockCommandExecutor()
	checker := NewSafetyChecker(mockExec, log.New(log.Writer(), "", 0))
	
	actions := checker.generateRecommendedActions(evaluation)
	
	// Check for expected commands
	expectedCommands := []string{
		`git commit -m "Remove deleted.go"`,
		`git commit -m "Your commit message"`,
		`git reset HEAD`,
		`git reset HEAD file.go`,
	}
	
	var foundCommands []string
	for _, action := range actions {
		foundCommands = append(foundCommands, action.Commands...)
	}
	
	for _, expected := range expectedCommands {
		found := false
		for _, cmd := range foundCommands {
			if strings.Contains(cmd, expected) || cmd == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected command not found: %q", expected)
		}
	}
}