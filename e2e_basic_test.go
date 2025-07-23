package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/syou6162/git-sequential-stage/testutils"
)

// TestBasicSetup は基本的なセットアップが動作することを確認します
// このテストは、テスト環境の基本動作（リポジトリ作成、ファイル作成、コミット）が正常に機能していることを
// 保証するために重要です。他のすべてのテストの前提条件となる基盤機能の動作を検証します。
func TestBasicSetup(t *testing.T) {
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()

	// ファイルを作成
	testRepo.CreateFile("test.txt", "Hello, World!")

	// 変更をコミット
	testRepo.CommitChanges("Initial commit")

	// コミットが存在することを確認
	ref, err := testRepo.Repo.Head()
	if err != nil {
		t.Fatalf("Failed to get HEAD: %v", err)
	}

	commit, err := testRepo.Repo.CommitObject(ref.Hash())
	if err != nil {
		t.Fatalf("Failed to get commit: %v", err)
	}

	if commit.Message != "Initial commit" {
		t.Errorf("Expected commit message 'Initial commit', got '%s'", commit.Message)
	}
}

// TestSingleFileSingleHunk は1ファイル1ハンクのケースをテストします
// このテストは、最も基本的なケース（1つのファイルの1つのハンクのみを選択的にステージング）が
// 正常に動作することを保証するために重要です。git-sequential-stageの核心機能の最小単位を検証します。
func TestSingleFileSingleHunk(t *testing.T) {
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()

	// 初期ファイルを作成（シンプルなPythonファイル）
	initialCode := `#!/usr/bin/env python3

def calculate_sum(a, b):
    return a + b

def main():
    result = calculate_sum(2, 3)
    print(f"Result: {result}")

if __name__ == "__main__":
    main()
`
	testRepo.CreateFile("calculator.py", initialCode)
	testRepo.CommitChanges("Initial commit")

	// ファイルを修正（1つのハンクの変更）
	modifiedCode := `#!/usr/bin/env python3

def calculate_sum(a, b):
    return a + b

def main():
    # Add input validation and better output
    x, y = 5, 7
    result = calculate_sum(x, y)
    print(f"Calculating {x} + {y} = {result}")

if __name__ == "__main__":
    main()
`
	testRepo.ModifyFile("calculator.py", modifiedCode)

	// パッチファイルを生成
	patchPath := filepath.Join(testRepo.Path, "changes.patch")
	output, err := testRepo.RunCommand("sh", "-c", "git diff > changes.patch")
	if err != nil {
		t.Fatalf("Failed to create patch file: %v\nOutput: %s", err, output)
	}

	// パッチファイルが作成されたことを確認
	if _, err := os.Stat(patchPath); os.IsNotExist(err) {
		t.Fatalf("Patch file was not created: %v", err)
	}

	// git-sequential-stageの主要ロジックを直接実行
	// パッチファイルの絶対パスを取得
	absPatchPath, err := filepath.Abs(patchPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// 一時的にディレクトリを変更してrunGitSequentialStageを実行
	defer testRepo.Chdir()()

	err = runGitSequentialStage([]string{"calculator.py:1"}, absPatchPath)
	if err != nil {
		t.Fatalf("git-sequential-stage failed: %v", err)
	}

	// 検証1: ステージングエリアにファイルがあるか
	stagedFiles := testRepo.GetStagedFiles()
	if len(stagedFiles) != 1 || stagedFiles[0] != "calculator.py" {
		t.Errorf("Expected calculator.py to be staged, got: %v", stagedFiles)
	}

	// 検証2: ステージングエリアの内容が正しいか
	stagedDiff, err := testRepo.RunCommand("git", "diff", "--cached")
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	// 検証3: ワーキングディレクトリに変更が残っていないか
	workingDiff, err := testRepo.RunCommand("git", "diff")
	if err != nil {
		t.Fatalf("Failed to get working diff: %v", err)
	}

	// デバッグ用：実際の差分を表示
	t.Logf("=== STAGED DIFF (git diff --cached) ===\n%s", stagedDiff)
	t.Logf("=== WORKING DIFF (git diff) ===\n%s", workingDiff)

	// 期待される変更が含まれているか確認
	expectedChanges := []string{
		"+    # Add input validation and better output",
		"+    x, y = 5, 7",
		"+    print(f\"Calculating {x} + {y} = {result}\")",
		"-    print(f\"Result: {result}\")",
	}

	for _, expected := range expectedChanges {
		if !strings.Contains(stagedDiff, expected) {
			t.Errorf("Expected staged diff to contain '%s', but it didn't.\nActual diff:\n%s", expected, stagedDiff)
		}
	}

	if strings.TrimSpace(workingDiff) != "" {
		t.Errorf("Expected no changes in working directory, but got:\n%s", workingDiff)
	}
}

// TestSingleFileMultipleHunks は1ファイル複数ハンクのケースをテストします
// このテストは、同一ファイル内の複数ハンクから特定のハンクのみを選択的にステージングする部分選択機能が
// 正常に動作することを保証するために重要です。関連する変更と無関係な変更を適切に分離する能力を検証します。
func TestSingleFileMultipleHunks(t *testing.T) {
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()

	// 初期ファイルを作成（複数の関数を持つPythonファイル）
	initialCode := `#!/usr/bin/env python3

def add_numbers(a, b):
    return a + b

def multiply_numbers(a, b):
    return a * b

def divide_numbers(a, b):
    return a / b

def main():
    x, y = 10, 5
    sum_result = add_numbers(x, y)
    mul_result = multiply_numbers(x, y)
    div_result = divide_numbers(x, y)

    print(f"Addition: {sum_result}")
    print(f"Multiplication: {mul_result}")
    print(f"Division: {div_result}")

if __name__ == "__main__":
    main()
`
	testRepo.CreateFile("math_operations.py", initialCode)
	testRepo.CommitChanges("Initial commit")

	// ファイルを修正（複数のハンクが生成される変更）
	modifiedCode := `#!/usr/bin/env python3

def add_numbers(a, b):
    # Add input validation for addition
    if not isinstance(a, (int, float)) or not isinstance(b, (int, float)):
        raise TypeError("Both arguments must be numbers")
    return a + b

def multiply_numbers(a, b):
    return a * b

def divide_numbers(a, b):
    # Add zero division check
    if b == 0:
        raise ValueError("Cannot divide by zero")
    return a / b

def main():
    x, y = 10, 5
    sum_result = add_numbers(x, y)
    mul_result = multiply_numbers(x, y)
    div_result = divide_numbers(x, y)

    # Improved output formatting
    print(f"Results for {x} and {y}:")
    print(f"  Addition: {sum_result}")
    print(f"  Multiplication: {mul_result}")
    print(f"  Division: {div_result}")

if __name__ == "__main__":
    main()
`
	testRepo.ModifyFile("math_operations.py", modifiedCode)

	// パッチファイルを生成
	patchPath := filepath.Join(testRepo.Path, "changes.patch")
	output, err := testRepo.RunCommand("sh", "-c", "git diff > changes.patch")
	if err != nil {
		t.Fatalf("Failed to create patch file: %v\nOutput: %s", err, output)
	}

	// パッチファイルが作成されたことを確認
	if _, err := os.Stat(patchPath); os.IsNotExist(err) {
		t.Fatalf("Patch file was not created: %v", err)
	}

	// 複数のハンクのうち、特定のハンク（2のみ）をステージング
	// ハンク1: add_numbers + divide_numbers関数の修正（このハンクは除外）
	// ハンク2: main関数の出力改善（このハンクのみをステージング）
	absPatchPath, err := filepath.Abs(patchPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// 一時的にディレクトリを変更してrunGitSequentialStageを実行
	defer testRepo.Chdir()()

	err = runGitSequentialStage([]string{"math_operations.py:2"}, absPatchPath)
	if err != nil {
		t.Fatalf("git-sequential-stage failed: %v", err)
	}

	// 検証1: ステージングエリアにファイルがあるか
	stagedFiles := testRepo.GetStagedFiles()
	if len(stagedFiles) != 1 || stagedFiles[0] != "math_operations.py" {
		t.Errorf("Expected math_operations.py to be staged, got: %v", stagedFiles)
	}

	// 検証2: ステージングエリアにハンク2の変更のみが含まれているか
	stagedDiff, err := testRepo.RunCommand("git", "diff", "--cached")
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	// 検証3: ワーキングディレクトリにハンク1の変更が残っているか
	workingDiff, err := testRepo.RunCommand("git", "diff")
	if err != nil {
		t.Fatalf("Failed to get working diff: %v", err)
	}

	// デバッグ用：実際の差分を表示
	t.Logf("=== STAGED DIFF (git diff --cached) - Should contain ONLY hunk 2 ===\n%s", stagedDiff)
	t.Logf("=== WORKING DIFF (git diff) - Should contain remaining hunk 1 ===\n%s", workingDiff)

	// ハンク2の変更（main関数の出力改善）が含まれているか確認
	expectedChangesHunk2 := []string{
		"+    # Improved output formatting",
		"+    print(f\"Results for {x} and {y}:\")",
		"+    print(f\"  Addition: {sum_result}\")",
		"+    print(f\"  Multiplication: {mul_result}\")",
		"+    print(f\"  Division: {div_result}\")",
	}

	// ハンク1の変更（add_numbers + divide_numbers関数の修正）が含まれていないことを確認
	unexpectedChangesHunk1 := []string{
		"+    # Add input validation for addition",
		"+    if not isinstance(a, (int, float)) or not isinstance(b, (int, float)):",
		"+        raise TypeError(\"Both arguments must be numbers\")",
		"+    # Add zero division check",
		"+    if b == 0:",
		"+        raise ValueError(\"Cannot divide by zero\")",
	}

	for _, expected := range expectedChangesHunk2 {
		if !strings.Contains(stagedDiff, expected) {
			t.Errorf("Expected staged diff to contain hunk 2 change '%s', but it didn't.\nActual diff:\n%s", expected, stagedDiff)
		}
	}

	for _, unexpected := range unexpectedChangesHunk1 {
		if strings.Contains(stagedDiff, unexpected) {
			t.Errorf("Staged diff should not contain hunk 1 change '%s', but it did.\nActual diff:\n%s", unexpected, stagedDiff)
		}
	}

	// ハンク1の変更がワーキングディレクトリに残っていることを確認
	for _, expected := range unexpectedChangesHunk1 {
		if !strings.Contains(workingDiff, expected) {
			t.Errorf("Expected working diff to contain remaining hunk 1 change '%s', but it didn't.\nActual working diff:\n%s", expected, workingDiff)
		}
	}
}

// TestMultipleFilesMultipleHunks は複数ファイル複数ハンクのケースをテストします
// このテストは、複数ファイルにまたがる変更から特定のハンクのみを選択的にステージングする
// 複数ファイル横断機能が正常に動作することを保証するために重要です。プロジェクト全体の変更を
// 論理的な単位で分割する能力を検証します。
func TestMultipleFilesMultipleHunks(t *testing.T) {
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()

	// 初期ファイル1を作成（ユーザー管理システム）
	userManagerCode := `#!/usr/bin/env python3

class UserManager:
    def __init__(self):
        self.users = {}

    def add_user(self, username, email):
        self.users[username] = {"email": email}
        return True

    def get_user(self, username):
        return self.users.get(username)

    def delete_user(self, username):
        if username in self.users:
            del self.users[username]
            return True
        return False
`
	testRepo.CreateFile("user_manager.py", userManagerCode)

	// 初期ファイル2を作成（データバリデーター）
	validatorCode := `#!/usr/bin/env python3

import re

class DataValidator:
    @staticmethod
    def validate_email(email):
        pattern = r'^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$'
        return re.match(pattern, email) is not None

    @staticmethod
    def validate_username(username):
        return len(username) >= 3 and username.isalnum()
`
	testRepo.CreateFile("validator.py", validatorCode)

	testRepo.CommitChanges("Initial commit")

	// ファイル1を修正（複数のハンクが生成される変更）
	modifiedUserManagerCode := `#!/usr/bin/env python3

class UserManager:
    def __init__(self):
        self.users = {}
        # Add logging capability
        self.log_enabled = True

    def add_user(self, username, email):
        # Add input validation
        if not username or not email:
            raise ValueError("Username and email are required")

        self.users[username] = {"email": email}
        if self.log_enabled:
            print(f"User {username} added successfully")
        return True

    def get_user(self, username):
        return self.users.get(username)

    def delete_user(self, username):
        if username in self.users:
            del self.users[username]
            if self.log_enabled:
                print(f"User {username} deleted successfully")
            return True
        return False
`
	testRepo.ModifyFile("user_manager.py", modifiedUserManagerCode)

	// ファイル2を修正（複数のハンクが生成される変更）
	modifiedValidatorCode := `#!/usr/bin/env python3

import re

class DataValidator:
    @staticmethod
    def validate_email(email):
        # Improved email validation with better error handling
        if not email or not isinstance(email, str):
            return False
        pattern = r'^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$'
        return re.match(pattern, email) is not None

    @staticmethod
    def validate_username(username):
        # Enhanced username validation
        if not username or not isinstance(username, str):
            return False
        return len(username) >= 3 and len(username) <= 20 and username.isalnum()

    @staticmethod
    def validate_password(password):
        # New password validation method
        if not password or not isinstance(password, str):
            return False
        return len(password) >= 8 and any(c.isupper() for c in password) and any(c.islower() for c in password)
`
	testRepo.ModifyFile("validator.py", modifiedValidatorCode)

	// パッチファイルを生成
	patchPath := filepath.Join(testRepo.Path, "changes.patch")
	output, err := testRepo.RunCommand("sh", "-c", "git diff > changes.patch")
	if err != nil {
		t.Fatalf("Failed to create patch file: %v\nOutput: %s", err, output)
	}

	// パッチファイルが作成されたことを確認
	if _, err := os.Stat(patchPath); os.IsNotExist(err) {
		t.Fatalf("Patch file was not created: %v", err)
	}

	// 複数ファイルから特定のハンクのみをステージング
	// user_manager.py: ハンク2（delete_user関数のログ追加）のみをステージング
	// validator.py: ハンク1（全ての変更）をステージング
	absPatchPath, err := filepath.Abs(patchPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// 一時的にディレクトリを変更してrunGitSequentialStageを実行
	defer testRepo.Chdir()()

	err = runGitSequentialStage([]string{"user_manager.py:2", "validator.py:1"}, absPatchPath)
	if err != nil {
		t.Fatalf("git-sequential-stage failed: %v", err)
	}

	// 検証1: ステージングエリアに両方のファイルがあるか
	stagedFiles := testRepo.GetStagedFiles()
	if len(stagedFiles) != 2 {
		t.Errorf("Expected 2 files to be staged, got: %d files %v", len(stagedFiles), stagedFiles)
	}

	expectedFiles := []string{"user_manager.py", "validator.py"}
	for _, expectedFile := range expectedFiles {
		found := false
		for _, stagedFile := range stagedFiles {
			if stagedFile == expectedFile {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected %s to be staged, but it wasn't. Staged files: %v", expectedFile, stagedFiles)
		}
	}

	// 検証2: ステージングエリアに特定のハンクのみが含まれているか
	stagedDiff, err := testRepo.RunCommand("git", "diff", "--cached")
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	// user_manager.py のハンク2（delete_user関数のログ追加）が含まれているか確認
	expectedUserManagerChanges := []string{
		"+            if self.log_enabled:",
		"+                print(f\"User {username} deleted successfully\")",
	}

	// validator.py のハンク1（全ての変更）が含まれているか確認
	expectedValidatorChanges := []string{
		"+        # Improved email validation with better error handling",
		"+        if not email or not isinstance(email, str):",
		"+        # Enhanced username validation",
		"+    def validate_password(password):",
		"+        # New password validation method",
	}

	// user_manager.py のハンク1（コンストラクタとadd_user関数の変更）が含まれていないことを確認
	unexpectedUserManagerChanges := []string{
		"+        # Add logging capability",
		"+        self.log_enabled = True",
		"+        # Add input validation",
		"+        if not username or not email:",
		"+            raise ValueError(\"Username and email are required\")",
	}

	// validator.pyは全ハンクをステージングするため、除外される変更はなし
	unexpectedValidatorChanges := []string{}

	for _, expected := range expectedUserManagerChanges {
		if !strings.Contains(stagedDiff, expected) {
			t.Errorf("Expected staged diff to contain user_manager.py change '%s', but it didn't.\nActual diff:\n%s", expected, stagedDiff)
		}
	}

	for _, expected := range expectedValidatorChanges {
		if !strings.Contains(stagedDiff, expected) {
			t.Errorf("Expected staged diff to contain validator.py change '%s', but it didn't.\nActual diff:\n%s", expected, stagedDiff)
		}
	}

	for _, unexpected := range unexpectedUserManagerChanges {
		if strings.Contains(stagedDiff, unexpected) {
			t.Errorf("Staged diff should not contain user_manager.py change '%s', but it did.\nActual diff:\n%s", unexpected, stagedDiff)
		}
	}

	for _, unexpected := range unexpectedValidatorChanges {
		if strings.Contains(stagedDiff, unexpected) {
			t.Errorf("Staged diff should not contain validator.py change '%s', but it did.\nActual diff:\n%s", unexpected, stagedDiff)
		}
	}

	// 検証3: ワーキングディレクトリに残りの変更があるか
	workingDiff, err := testRepo.RunCommand("git", "diff")
	if err != nil {
		t.Fatalf("Failed to get working diff: %v", err)
	}

	// 機能的検証: 期待される変更内容が含まれているかチェック
	// スナップショット比較ではなく、実際の機能をテスト

	// Staged diff の検証: 期待される変更が含まれているか
	expectedStagedChanges := []string{
		"if self.log_enabled:",
		"print(f\"User {username} deleted successfully\")",
		"# Improved email validation with better error handling",
		"# Enhanced username validation",
		"# New password validation method",
	}

	for _, expectedChange := range expectedStagedChanges {
		if !strings.Contains(stagedDiff, expectedChange) {
			t.Errorf("Staged diff should contain '%s', but it doesn't.\nActual diff:\n%s", expectedChange, stagedDiff)
		}
	}

	// Working diff の検証: 期待される変更が含まれているか
	expectedWorkingChanges := []string{
		"# Add logging capability",
		"self.log_enabled = True",
		"# Add input validation",
		"if not username or not email:",
		"raise ValueError(\"Username and email are required\")",
		"if self.log_enabled:",
		"print(f\"User {username} added successfully\")",
	}

	for _, expectedChange := range expectedWorkingChanges {
		if !strings.Contains(workingDiff, expectedChange) {
			t.Errorf("Working diff should contain '%s', but it doesn't.\nActual diff:\n%s", expectedChange, workingDiff)
		}
	}

	// ファイル別の検証: 正しいファイルが変更されているか
	if !strings.Contains(stagedDiff, "user_manager.py") {
		t.Error("Staged diff should contain changes to user_manager.py")
	}
	if !strings.Contains(stagedDiff, "validator.py") {
		t.Error("Staged diff should contain changes to validator.py")
	}
	if !strings.Contains(workingDiff, "user_manager.py") {
		t.Error("Working diff should contain changes to user_manager.py")
	}
}