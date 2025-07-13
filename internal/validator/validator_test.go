package validator

import (
	"errors"
	"testing"

	"github.com/syou6162/git-sequential-stage/internal/executor"
)

func TestValidator_CheckDependencies(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*executor.MockCommandExecutor)
		wantErr  bool
		errMsg   string
	}{
		{
			name: "all dependencies available",
			setup: func(m *executor.MockCommandExecutor) {
				m.Commands["git [--version]"] = executor.MockResponse{
					Output: []byte("git version 2.39.0\n"),
					Error:  nil,
				}
				m.Commands["filterdiff [--version]"] = executor.MockResponse{
					Output: []byte("filterdiff version 0.3.4\n"),
					Error:  nil,
				}
			},
			wantErr: false,
		},
		{
			name: "git not found",
			setup: func(m *executor.MockCommandExecutor) {
				m.Commands["git [--version]"] = executor.MockResponse{
					Output: nil,
					Error:  errors.New("command not found: git"),
				}
			},
			wantErr: true,
			errMsg:  "git command not found",
		},
		{
			name: "filterdiff not found",
			setup: func(m *executor.MockCommandExecutor) {
				m.Commands["git [--version]"] = executor.MockResponse{
					Output: []byte("git version 2.39.0\n"),
					Error:  nil,
				}
				m.Commands["filterdiff [--version]"] = executor.MockResponse{
					Output: nil,
					Error:  errors.New("command not found: filterdiff"),
				}
			},
			wantErr: true,
			errMsg:  "filterdiff command not found (install patchutils)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := executor.NewMockCommandExecutor()
			tt.setup(mock)
			
			v := NewValidator(mock)
			err := v.CheckDependencies()
			
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckDependencies() error = %v, wantErr %v", err, tt.wantErr)
			}
			
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("CheckDependencies() error message = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestValidator_ValidateArgs(t *testing.T) {
	tests := []struct {
		name      string
		hunks     string
		patchFile string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid args",
			hunks:     "1,2,3",
			patchFile: "test.patch",
			wantErr:   false,
		},
		{
			name:      "empty hunks",
			hunks:     "",
			patchFile: "test.patch",
			wantErr:   true,
			errMsg:    "hunks cannot be empty",
		},
		{
			name:      "empty patch file",
			hunks:     "1,2,3",
			patchFile: "",
			wantErr:   true,
			errMsg:    "patch file cannot be empty",
		},
		{
			name:      "invalid hunk format",
			hunks:     "1,a,3",
			patchFile: "test.patch",
			wantErr:   true,
			errMsg:    "invalid hunk number: a",
		},
		{
			name:      "negative hunk number",
			hunks:     "1,-2,3",
			patchFile: "test.patch",
			wantErr:   true,
			errMsg:    "hunk number must be positive: -2",
		},
		{
			name:      "zero hunk number",
			hunks:     "1,0,3",
			patchFile: "test.patch",
			wantErr:   true,
			errMsg:    "hunk number must be positive: 0",
		},
		// エッジケース追加
		{
			name:      "whitespace in hunks",
			hunks:     " 1 , 2 , 3 ",
			patchFile: "test.patch",
			wantErr:   false, // strings.TrimSpace()により正常処理される
		},
		{
			name:      "empty hunk after trim",
			hunks:     "1, ,3",
			patchFile: "test.patch",
			wantErr:   true,
			errMsg:    "invalid hunk number: ", // 空文字列は無効
		},
		{
			name:      "very large hunk number in old format",
			hunks:     "1,999999,3",
			patchFile: "test.patch",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(nil) // executor not needed for arg validation
			err := v.ValidateArgs(tt.hunks, tt.patchFile)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
			
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("ValidateArgs() error message = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}