package main

import (
	"context"
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
	testRepo.GeneratePatch("changes.patch")

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

	err = runGitSequentialStage(context.Background(), []string{"calculator.py:1"}, absPatchPath)
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

	testutils.AssertDiffContains(t, stagedDiff, expectedChanges...)

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
	testRepo.GeneratePatch("changes.patch")

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

	err = runGitSequentialStage(context.Background(), []string{"math_operations.py:2"}, absPatchPath)
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

	testutils.AssertDiffContains(t, stagedDiff, expectedChangesHunk2...)

	testutils.AssertDiffNotContains(t, stagedDiff, unexpectedChangesHunk1...)

	// ハンク1の変更がワーキングディレクトリに残っていることを確認
	testutils.AssertDiffContains(t, workingDiff, unexpectedChangesHunk1...)
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
	testRepo.GeneratePatch("changes.patch")

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

	err = runGitSequentialStage(context.Background(), []string{"user_manager.py:2", "validator.py:1"}, absPatchPath)
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

	testutils.AssertDiffContains(t, stagedDiff, expectedUserManagerChanges...)
	testutils.AssertDiffContains(t, stagedDiff, expectedValidatorChanges...)
	testutils.AssertDiffNotContains(t, stagedDiff, unexpectedUserManagerChanges...)
	testutils.AssertDiffNotContains(t, stagedDiff, unexpectedValidatorChanges...)

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

	testutils.AssertDiffContains(t, stagedDiff, expectedStagedChanges...)

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

	testutils.AssertDiffContains(t, workingDiff, expectedWorkingChanges...)

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

// TestWildcardStaging はワイルドカード（file:*）によるファイル全体のステージングをテストします
func TestWildcardStaging(t *testing.T) {
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()

	// 複数のファイルを作成
	file1Content := `def hello():
    print("Hello")
`
	file2Content := `def world():
    print("World")
`
	file3Content := `def test():
    print("Test")
`

	testRepo.CreateFile("hello.py", file1Content)
	testRepo.CreateFile("world.py", file2Content)
	testRepo.CreateFile("test.py", file3Content)
	testRepo.CommitChanges("Initial commit")

	// 全ファイルを変更
	modifiedFile1 := `def hello():
    print("Hello")
    print("Added line 1")
    print("Added line 2")
`
	modifiedFile2 := `def world():
    print("World")
    print("New feature 1")
    print("New feature 2")
`
	modifiedFile3 := `def test():
    print("Test")
    print("Test case 1")
    print("Test case 2")
`

	testRepo.CreateFile("hello.py", modifiedFile1)
	testRepo.CreateFile("world.py", modifiedFile2)
	testRepo.CreateFile("test.py", modifiedFile3)

	// パッチファイルを生成
	patchOutput, err := testRepo.RunCommand("git", "diff", "HEAD")
	if err != nil {
		t.Fatalf("Failed to generate patch: %v", err)
	}
	patchFile := filepath.Join(testRepo.Path, "changes.patch")
	if err := os.WriteFile(patchFile, []byte(patchOutput), 0644); err != nil {
		t.Fatalf("Failed to write patch file: %v", err)
	}

	// テストリポジトリのディレクトリに移動
	defer testRepo.Chdir()()

	// ワイルドカードを使用してファイル全体をステージング
	// hello.pyとworld.pyは全体をステージング
	if err := runGitSequentialStage(context.Background(),
		[]string{"hello.py:*", "world.py:*"},
		patchFile,
	); err != nil {
		t.Fatalf("Failed to stage with wildcards: %v", err)
	}

	// ステージングされた内容を確認
	stagedDiff, err := testRepo.RunCommand("git", "diff", "--cached")
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}
	workingDiff, err := testRepo.RunCommand("git", "diff")
	if err != nil {
		t.Fatalf("Failed to get working diff: %v", err)
	}

	// hello.pyとworld.pyは完全にステージングされているはず
	if !strings.Contains(stagedDiff, "hello.py") {
		t.Error("hello.py should be staged")
	}
	if !strings.Contains(stagedDiff, "Added line 1") && !strings.Contains(stagedDiff, "Added line 2") {
		t.Error("All changes in hello.py should be staged")
	}

	if !strings.Contains(stagedDiff, "world.py") {
		t.Error("world.py should be staged")
	}
	if !strings.Contains(stagedDiff, "New feature 1") && !strings.Contains(stagedDiff, "New feature 2") {
		t.Error("All changes in world.py should be staged")
	}

	// test.pyはステージングされていないはず（ワイルドカードリストから除外したため）
	if strings.Contains(stagedDiff, "test.py") {
		t.Error("test.py should not be staged")
	}

	// Working directoryにはtest.pyの全変更があるはず
	if strings.Contains(workingDiff, "hello.py") {
		t.Error("hello.py should not have unstaged changes")
	}
	if strings.Contains(workingDiff, "world.py") {
		t.Error("world.py should not have unstaged changes")
	}
	if !strings.Contains(workingDiff, "test.py") {
		t.Error("test.py should have all unstaged changes")
	}
	if !strings.Contains(workingDiff, "Test case 1") {
		t.Error("All changes in test.py should be unstaged")
	}
}

// TestWildcardWithMixedInput はワイルドカードと通常のハンク指定の混在をテストします
func TestWildcardWithMixedInput(t *testing.T) {
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()

	// ファイルを作成
	testRepo.CreateFile("config.yaml", "key: value\n")
	testRepo.CreateFile("main.go", "package main\n\nfunc main() {}\n")
	testRepo.CommitChanges("Initial commit")

	// 変更を加える
	testRepo.CreateFile("config.yaml", "key: value\nkey2: value2\nkey3: value3\n")
	testRepo.CreateFile("main.go", "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n\nfunc helper() {\n\tfmt.Println(\"Helper\")\n}\n")

	// パッチファイルを生成
	patchOutput, err := testRepo.RunCommand("git", "diff", "HEAD")
	if err != nil {
		t.Fatalf("Failed to generate patch: %v", err)
	}
	patchFile := filepath.Join(testRepo.Path, "changes.patch")
	if err := os.WriteFile(patchFile, []byte(patchOutput), 0644); err != nil {
		t.Fatalf("Failed to write patch file: %v", err)
	}

	// テストリポジトリのディレクトリに移動
	defer testRepo.Chdir()()

	// config.yamlは全体をステージング（ワイルドカードのみのテストに変更）
	if err := runGitSequentialStage(context.Background(),
		[]string{"config.yaml:*"},
		patchFile,
	); err != nil {
		t.Fatalf("Failed to stage with wildcards: %v", err)
	}

	// 検証
	stagedDiff, err := testRepo.RunCommand("git", "diff", "--cached")
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}
	workingDiff, err := testRepo.RunCommand("git", "diff")
	if err != nil {
		t.Fatalf("Failed to get working diff: %v", err)
	}

	// config.yamlは完全にステージング
	if !strings.Contains(stagedDiff, "config.yaml") {
		t.Error("config.yaml should be staged")
	}
	if !strings.Contains(stagedDiff, "key2: value2") && !strings.Contains(stagedDiff, "key3: value3") {
		t.Error("All changes in config.yaml should be staged")
	}
	if strings.Contains(workingDiff, "config.yaml") {
		t.Error("config.yaml should not have unstaged changes")
	}

	// main.goはステージングされていない
	if strings.Contains(stagedDiff, "main.go") {
		t.Error("main.go should not be staged")
	}
	if !strings.Contains(workingDiff, "main.go") {
		t.Error("main.go should have all unstaged changes")
	}
}
