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

func TestValidatorValidateArgsNew(t *testing.T) {
	tests := []struct {
		name      string
		hunkSpecs []string
		patchFile string
		wantErr   bool
		errMsg    string
	}{
		// 正常ケース
		{
			name:      "valid single file spec",
			hunkSpecs: []string{"file1.go:1,2,3"},
			patchFile: "test.patch",
			wantErr:   false,
		},
		{
			name:      "valid multiple file specs",
			hunkSpecs: []string{"file1.go:1,2", "file2.go:3,4"},
			patchFile: "test.patch",
			wantErr:   false,
		},
		{
			name:      "valid single hunk",
			hunkSpecs: []string{"main.go:1"},
			patchFile: "test.patch",
			wantErr:   false,
		},
		{
			name:      "valid complex path with directories",
			hunkSpecs: []string{"internal/validator/validator.go:1,2,3"},
			patchFile: "changes.patch",
			wantErr:   false,
		},
		{
			name:      "path with multiple colons causes error",
			hunkSpecs: []string{"path:to:file.go:1,2"},
			patchFile: "test.patch",
			wantErr:   true, // SplitN(spec, ":", 2)により"path:to"と"file.go:1,2"に分割されるが、"to:file.go:1"は無効な数字
			errMsg:    "invalid hunk number in path:to:file.go:1,2: to:file.go:1",
		},

		// エラーケース - 必須パラメータ不足
		{
			name:      "empty hunk specs",
			hunkSpecs: []string{},
			patchFile: "test.patch",
			wantErr:   true,
			errMsg:    "at least one hunk specification is required",
		},
		{
			name:      "empty patch file",
			hunkSpecs: []string{"file1.go:1"},
			patchFile: "",
			wantErr:   true,
			errMsg:    "patch file cannot be empty",
		},

		// エラーケース - フォーマット不正
		{
			name:      "missing colon",
			hunkSpecs: []string{"file1.go"},
			patchFile: "test.patch",
			wantErr:   true,
			errMsg:    "invalid hunk specification format: file1.go (expected file:numbers)",
		},
		{
			name:      "empty file name",
			hunkSpecs: []string{":1,2,3"},
			patchFile: "test.patch",
			wantErr:   true,
			errMsg:    "invalid hunk specification: :1,2,3",
		},
		{
			name:      "empty hunk numbers",
			hunkSpecs: []string{"file1.go:"},
			patchFile: "test.patch",
			wantErr:   true,
			errMsg:    "invalid hunk specification: file1.go:",
		},

		// エラーケース - ハンク番号不正
		{
			name:      "invalid hunk number format",
			hunkSpecs: []string{"file1.go:1,a,3"},
			patchFile: "test.patch",
			wantErr:   true,
			errMsg:    "invalid hunk number in file1.go:1,a,3: a",
		},
		{
			name:      "negative hunk number",
			hunkSpecs: []string{"file1.go:1,-2,3"},
			patchFile: "test.patch",
			wantErr:   true,
			errMsg:    "hunk number must be positive in file1.go:1,-2,3: -2",
		},
		{
			name:      "zero hunk number",
			hunkSpecs: []string{"file1.go:1,0,3"},
			patchFile: "test.patch",
			wantErr:   true,
			errMsg:    "hunk number must be positive in file1.go:1,0,3: 0",
		},

		// エッジケース - 空白文字処理
		{
			name:      "whitespace in hunk numbers",
			hunkSpecs: []string{"file1.go: 1 , 2 , 3 "},
			patchFile: "test.patch",
			wantErr:   false, // strings.TrimSpace()により正常処理される
		},
		{
			name:      "empty hunk number after trim",
			hunkSpecs: []string{"file1.go:1, ,3"},
			patchFile: "test.patch",
			wantErr:   true,
			errMsg:    "invalid hunk number in file1.go:1, ,3: ", // 空文字列は無効
		},

		// 複数ファイルでの混合エラーケース
		{
			name:      "mixed valid and invalid specs",
			hunkSpecs: []string{"file1.go:1,2", "file2.go:1,a,3"},
			patchFile: "test.patch",
			wantErr:   true,
			errMsg:    "invalid hunk number in file2.go:1,a,3: a",
		},
		{
			name:      "multiple files with negative numbers",
			hunkSpecs: []string{"file1.go:1,2", "file2.go:3,-1"},
			patchFile: "test.patch",
			wantErr:   true,
			errMsg:    "hunk number must be positive in file2.go:3,-1: -1",
		},

		// 境界値テスト
		{
			name:      "very large hunk number",
			hunkSpecs: []string{"file1.go:999999"},
			patchFile: "test.patch",
			wantErr:   false,
		},
		{
			name:      "many hunk numbers",
			hunkSpecs: []string{"file1.go:1,2,3,4,5,6,7,8,9,10,11,12,13,14,15"},
			patchFile: "test.patch",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(nil) // executor not needed for arg validation
			err := v.ValidateArgsNew(tt.hunkSpecs, tt.patchFile)

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateArgsNew() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("ValidateArgsNew() error message = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}
