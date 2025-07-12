package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// setupTestRepo はテスト用のGitリポジトリを作成し、クリーンアップ関数を返します
func setupTestRepo(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "git-sequential-stage-e2e-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Gitリポジトリの初期化
	_, err = git.PlainInit(tempDir, false)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to initialize git repository: %v", err)
	}

	// Git設定（ユーザー名とメールアドレス）
	repo, err := git.PlainOpen(tempDir)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to open repository: %v", err)
	}

	cfg, err := repo.Config()
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to get config: %v", err)
	}

	cfg.User.Name = "Test User"
	cfg.User.Email = "test@example.com"
	err = repo.SetConfig(cfg)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to set config: %v", err)
	}

	// クリーンアップ関数
	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

// createFile はテスト用のファイルを作成します
func createFile(t *testing.T, dir, filename, content string) {
	filepath := filepath.Join(dir, filename)
	err := os.WriteFile(filepath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create file %s: %v", filename, err)
	}
}

// commitChanges は変更をコミットします
func commitChanges(t *testing.T, dir, message string) {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatalf("Failed to open repository: %v", err)
	}

	w, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	// すべてのファイルをステージング
	_, err = w.Add(".")
	if err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}

	// コミット
	_, err = w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}
}

// TestBasicSetup は基本的なセットアップが動作することを確認します
func TestBasicSetup(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// ファイルを作成
	createFile(t, dir, "test.txt", "Hello, World!")

	// 変更をコミット
	commitChanges(t, dir, "Initial commit")

	// リポジトリが正しく作成されたことを確認
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatalf("Failed to open repository: %v", err)
	}

	// コミットが存在することを確認
	ref, err := repo.Head()
	if err != nil {
		t.Fatalf("Failed to get HEAD: %v", err)
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		t.Fatalf("Failed to get commit: %v", err)
	}

	if commit.Message != "Initial commit" {
		t.Errorf("Expected commit message 'Initial commit', got '%s'", commit.Message)
	}
}

// runCommand は指定されたディレクトリでコマンドを実行します
func runCommand(t *testing.T, dir string, command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// modifyFile はファイルの内容を変更します
func modifyFile(t *testing.T, dir, filename, newContent string) {
	filepath := filepath.Join(dir, filename)
	err := os.WriteFile(filepath, []byte(newContent), 0644)
	if err != nil {
		t.Fatalf("Failed to modify file %s: %v", filename, err)
	}
}

// getCommitCount はリポジトリのコミット数を取得します
func getCommitCount(t *testing.T, dir string) int {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatalf("Failed to open repository: %v", err)
	}

	iter, err := repo.Log(&git.LogOptions{})
	if err != nil {
		t.Fatalf("Failed to get log: %v", err)
	}

	count := 0
	err = iter.ForEach(func(c *object.Commit) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to count commits: %v", err)
	}

	return count
}

// getStagedFiles はステージングエリアのファイル一覧を取得します
func getStagedFiles(t *testing.T, dir string) []string {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatalf("Failed to open repository: %v", err)
	}

	w, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	status, err := w.Status()
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}

	var stagedFiles []string
	for file, fileStatus := range status {
		// ステージングエリアに変更があるファイルを取得
		if fileStatus.Staging != git.Untracked && fileStatus.Staging != git.Unmodified {
			stagedFiles = append(stagedFiles, file)
		}
	}

	return stagedFiles
}

// TestSingleFileSingleHunk は1ファイル1ハンクのケースをテストします
func TestSingleFileSingleHunk(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

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
	createFile(t, dir, "calculator.py", initialCode)
	commitChanges(t, dir, "Initial commit")

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
	modifyFile(t, dir, "calculator.py", modifiedCode)

	// パッチファイルを生成
	patchPath := filepath.Join(dir, "changes.patch")
	output, err := runCommand(t, dir, "git", "diff", ">", patchPath)
	if err != nil {
		// シェルのリダイレクトを使うために sh -c を使用
		output, err = runCommand(t, dir, "sh", "-c", "git diff > changes.patch")
		if err != nil {
			t.Fatalf("Failed to create patch file: %v\nOutput: %s", err, output)
		}
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
	originalDir, _ := os.Getwd()
	err = os.Chdir(dir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)
	
	err = runGitSequentialStage([]string{"calculator.py:1"}, absPatchPath)
	if err != nil {
		t.Fatalf("git-sequential-stage failed: %v", err)
	}

	// 検証1: ステージングエリアにファイルがあるか
	stagedFiles := getStagedFiles(t, dir)
	if len(stagedFiles) != 1 || stagedFiles[0] != "calculator.py" {
		t.Errorf("Expected calculator.py to be staged, got: %v", stagedFiles)
	}

	// 検証2: ステージングエリアの内容が正しいか
	stagedDiff, err := runCommand(t, dir, "git", "diff", "--cached")
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	// 検証3: ワーキングディレクトリに変更が残っていないか
	workingDiff, err := runCommand(t, dir, "git", "diff")
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
func TestSingleFileMultipleHunks(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

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
	createFile(t, dir, "math_operations.py", initialCode)
	commitChanges(t, dir, "Initial commit")

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
	modifyFile(t, dir, "math_operations.py", modifiedCode)

	// パッチファイルを生成
	patchPath := filepath.Join(dir, "changes.patch")
	output, err := runCommand(t, dir, "git", "diff", ">", patchPath)
	if err != nil {
		// シェルのリダイレクトを使うために sh -c を使用
		output, err = runCommand(t, dir, "sh", "-c", "git diff > changes.patch")
		if err != nil {
			t.Fatalf("Failed to create patch file: %v\nOutput: %s", err, output)
		}
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
	originalDir, _ := os.Getwd()
	err = os.Chdir(dir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)
	
	err = runGitSequentialStage([]string{"math_operations.py:2"}, absPatchPath)
	if err != nil {
		t.Fatalf("git-sequential-stage failed: %v", err)
	}

	// 検証1: ステージングエリアにファイルがあるか
	stagedFiles := getStagedFiles(t, dir)
	if len(stagedFiles) != 1 || stagedFiles[0] != "math_operations.py" {
		t.Errorf("Expected math_operations.py to be staged, got: %v", stagedFiles)
	}

	// 検証2: ステージングエリアにハンク2の変更のみが含まれているか
	stagedDiff, err := runCommand(t, dir, "git", "diff", "--cached")
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	// 検証3: ワーキングディレクトリにハンク1の変更が残っているか
	workingDiff, err := runCommand(t, dir, "git", "diff")
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
func TestMultipleFilesMultipleHunks(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

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
	createFile(t, dir, "user_manager.py", userManagerCode)

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
	createFile(t, dir, "validator.py", validatorCode)

	commitChanges(t, dir, "Initial commit")

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
	modifyFile(t, dir, "user_manager.py", modifiedUserManagerCode)

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
	modifyFile(t, dir, "validator.py", modifiedValidatorCode)

	// パッチファイルを生成
	patchPath := filepath.Join(dir, "changes.patch")
	output, err := runCommand(t, dir, "git", "diff", ">", patchPath)
	if err != nil {
		// シェルのリダイレクトを使うために sh -c を使用
		output, err = runCommand(t, dir, "sh", "-c", "git diff > changes.patch")
		if err != nil {
			t.Fatalf("Failed to create patch file: %v\nOutput: %s", err, output)
		}
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
	originalDir, _ := os.Getwd()
	err = os.Chdir(dir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)
	
	err = runGitSequentialStage([]string{"user_manager.py:2", "validator.py:1"}, absPatchPath)
	if err != nil {
		t.Fatalf("git-sequential-stage failed: %v", err)
	}

	// 検証1: ステージングエリアに両方のファイルがあるか
	stagedFiles := getStagedFiles(t, dir)
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
	stagedDiff, err := runCommand(t, dir, "git", "diff", "--cached")
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
	workingDiff, err := runCommand(t, dir, "git", "diff")
	if err != nil {
		t.Fatalf("Failed to get working diff: %v", err)
	}

	// スナップショットテスト: ステージングエリアの期待される差分
	expectedStagedDiff := `diff --git a/user_manager.py b/user_manager.py
index bd33e43..20b402c 100644
--- a/user_manager.py
+++ b/user_manager.py
@@ -14,5 +14,7 @@ class UserManager:
     def delete_user(self, username):
         if username in self.users:
             del self.users[username]
+            if self.log_enabled:
+                print(f"User {username} deleted successfully")
             return True
         return False
diff --git a/validator.py b/validator.py
index 7eaf039..acbb7a6 100644
--- a/validator.py
+++ b/validator.py
@@ -5,9 +5,22 @@ import re
 class DataValidator:
     @staticmethod
     def validate_email(email):
+        # Improved email validation with better error handling
+        if not email or not isinstance(email, str):
+            return False
         pattern = r'^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$'
         return re.match(pattern, email) is not None
     
     @staticmethod
     def validate_username(username):
-        return len(username) >= 3 and username.isalnum()
+        # Enhanced username validation
+        if not username or not isinstance(username, str):
+            return False
+        return len(username) >= 3 and len(username) <= 20 and username.isalnum()
+    
+    @staticmethod
+    def validate_password(password):
+        # New password validation method
+        if not password or not isinstance(password, str):
+            return False
+        return len(password) >= 8 and any(c.isupper() for c in password) and any(c.islower() for c in password)
`

	// スナップショットテスト: ワーキングディレクトリの期待される差分  
	expectedWorkingDiff := `diff --git a/user_manager.py b/user_manager.py
index 20b402c..be9ace8 100644
--- a/user_manager.py
+++ b/user_manager.py
@@ -3,9 +3,17 @@
 class UserManager:
     def __init__(self):
         self.users = {}
+        # Add logging capability
+        self.log_enabled = True
     
     def add_user(self, username, email):
+        # Add input validation
+        if not username or not email:
+            raise ValueError("Username and email are required")
+        
         self.users[username] = {"email": email}
+        if self.log_enabled:
+            print(f"User {username} added successfully")
         return True
     
     def get_user(self, username):
`

	// 実際の差分と期待される差分を比較
	if strings.TrimSpace(stagedDiff) != strings.TrimSpace(expectedStagedDiff) {
		t.Errorf("Staged diff does not match expected snapshot.\nExpected:\n%s\n\nActual:\n%s", expectedStagedDiff, stagedDiff)
	}

	if strings.TrimSpace(workingDiff) != strings.TrimSpace(expectedWorkingDiff) {
		t.Errorf("Working diff does not match expected snapshot.\nExpected:\n%s\n\nActual:\n%s", expectedWorkingDiff, workingDiff)
	}
}

// TestMixedSemanticChanges は異なる意味を持つ変更が混在するケースをテストします
// これはgit-sequential-stageの最も重要な機能：セマンティックなコミット分割を実証します
func TestMixedSemanticChanges(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// 初期ファイルを作成（Webサーバーのコード）
	initialWebServerCode := `#!/usr/bin/env python3

import os
from flask import Flask, request, jsonify

app = Flask(__name__)

@app.route('/api/users', methods=['GET'])
def get_users():
    # TODO: Get users from database
    users = [
        {"id": 1, "name": "Alice", "email": "alice@example.com"},
        {"id": 2, "name": "Bob", "email": "bob@example.com"}
    ]
    return jsonify(users)

@app.route('/api/users', methods=['POST'])
def create_user():
    data = request.get_json()
    # TODO: Save user to database
    new_user = {
        "id": 3,
        "name": data.get("name"),
        "email": data.get("email")
    }
    return jsonify(new_user), 201

@app.route('/health', methods=['GET'])
def health_check():
    return {"status": "ok"}

if __name__ == '__main__':
    app.run(debug=True, port=5000)
`
	createFile(t, dir, "web_server.py", initialWebServerCode)
	commitChanges(t, dir, "Initial commit")

	// ファイルを修正（3つの異なる意味の変更を含む）
	modifiedWebServerCode := `#!/usr/bin/env python3

import os
import logging
from flask import Flask, request, jsonify

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

@app.route('/api/users', methods=['GET'])
def get_users():
    # TODO: Get users from database
    logger.info("Fetching users from database")
    users = [
        {"id": 1, "name": "Alice", "email": "alice@example.com"},
        {"id": 2, "name": "Bob", "email": "bob@example.com"}
    ]
    return jsonify(users)

@app.route('/api/users', methods=['POST'])
def create_user():
    data = request.get_json()
    
    # Add input validation
    if not data or not data.get("name") or not data.get("email"):
        return jsonify({"error": "Name and email are required"}), 400
    
    # TODO: Save user to database
    new_user = {
        "id": 3,
        "name": data.get("name"),
        "email": data.get("email")
    }
    return jsonify(new_user), 201

@app.route('/health', methods=['GET'])
def health_check():
    return {"status": "ok", "timestamp": "2024-01-01T00:00:00Z"}

if __name__ == '__main__':
    # Use environment variable for port configuration
    port = int(os.environ.get('PORT', 5000))
    app.run(debug=True, port=port)
`
	modifyFile(t, dir, "web_server.py", modifiedWebServerCode)

	// パッチファイルを生成
	patchPath := filepath.Join(dir, "changes.patch")
	output, err := runCommand(t, dir, "sh", "-c", "git diff > changes.patch")
	if err != nil {
		t.Fatalf("Failed to create patch file: %v\nOutput: %s", err, output)
	}

	// パッチファイルが作成されたことを確認
	if _, err := os.Stat(patchPath); os.IsNotExist(err) {
		t.Fatalf("Patch file was not created: %v", err)
	}

	absPatchPath, err := filepath.Abs(patchPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}
	
	// 一時的にディレクトリを変更してrunGitSequentialStageを実行
	originalDir, _ := os.Getwd()
	err = os.Chdir(dir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// シナリオ1: ロギング機能の追加のみをコミット（ハンク1のみ）
	// これは新機能追加のセマンティックなコミット
	err = runGitSequentialStage([]string{"web_server.py:1"}, absPatchPath)
	if err != nil {
		t.Fatalf("git-sequential-stage failed for logging commit: %v", err)
	}

	// 検証1: ロギング関連の変更のみがステージングされているか
	stagedDiff, err := runCommand(t, dir, "git", "diff", "--cached")
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	// デバッグ用：シナリオ1のスナップショット表示
	t.Logf("=== SCENARIO 1: LOGGING COMMIT (web_server.py:1) ===")
	t.Logf("STAGED DIFF:\n%s", stagedDiff)

	// ロギング機能（ハンク1）の変更が含まれているか確認
	expectedLoggingChanges := []string{
		"+import logging",
		"+# Configure logging",
		"+logging.basicConfig(level=logging.INFO)",
		"+logger = logging.getLogger(__name__)",
		"+    logger.info(\"Fetching users from database\")",
	}

	// バリデーション機能（ハンク2）とヘルスチェック改善（ハンク3）とポート設定（ハンク4）が含まれていないことを確認
	unexpectedChanges := []string{
		"+    # Add input validation",
		"+    if not data or not data.get(\"name\") or not data.get(\"email\"):",
		"+        return jsonify({\"error\": \"Name and email are required\"}), 400",
		"+    return {\"status\": \"ok\", \"timestamp\": \"2024-01-01T00:00:00Z\"}",
		"+    # Use environment variable for port configuration",
		"+    port = int(os.environ.get('PORT', 5000))",
		"+    app.run(debug=True, port=port)",
	}

	for _, expected := range expectedLoggingChanges {
		if !strings.Contains(stagedDiff, expected) {
			t.Errorf("Expected staged diff to contain logging change '%s', but it didn't.\nActual diff:\n%s", expected, stagedDiff)
		}
	}

	for _, unexpected := range unexpectedChanges {
		if strings.Contains(stagedDiff, unexpected) {
			t.Errorf("Staged diff should not contain non-logging change '%s', but it did.\nActual diff:\n%s", unexpected, stagedDiff)
		}
	}

	// 最初のコミットを作成（ロギング機能追加）
	_, err = runCommand(t, dir, "git", "commit", "-m", "feat: add logging infrastructure for request tracking")
	if err != nil {
		t.Fatalf("Failed to commit logging changes: %v", err)
	}

	// シナリオ2: バリデーション機能の追加のみをコミット（ハンク2のみ）
	// これはセキュリティ向上のセマンティックなコミット
	err = runGitSequentialStage([]string{"web_server.py:2"}, absPatchPath)
	if err != nil {
		t.Fatalf("git-sequential-stage failed for validation commit: %v", err)
	}

	// 検証2: バリデーション関連の変更のみがステージングされているか
	stagedDiff, err = runCommand(t, dir, "git", "diff", "--cached")
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	// デバッグ用：シナリオ2のスナップショット表示
	t.Logf("=== SCENARIO 2: VALIDATION COMMIT (web_server.py:2) ===")
	t.Logf("STAGED DIFF:\n%s", stagedDiff)

	// バリデーション機能（ハンク2）の変更が含まれているか確認
	expectedValidationChanges := []string{
		"+    # Add input validation",
		"+    if not data or not data.get(\"name\") or not data.get(\"email\"):",
		"+        return jsonify({\"error\": \"Name and email are required\"}), 400",
	}

	// ヘルスチェック改善（ハンク3）とポート設定（ハンク4）が含まれていないことを確認
	unexpectedValidationChanges := []string{
		"+    return {\"status\": \"ok\", \"timestamp\": \"2024-01-01T00:00:00Z\"}",
		"+    # Use environment variable for port configuration",
		"+    port = int(os.environ.get('PORT', 5000))",
		"+    app.run(debug=True, port=port)",
	}

	for _, expected := range expectedValidationChanges {
		if !strings.Contains(stagedDiff, expected) {
			t.Errorf("Expected staged diff to contain validation change '%s', but it didn't.\nActual diff:\n%s", expected, stagedDiff)
		}
	}

	for _, unexpected := range unexpectedValidationChanges {
		if strings.Contains(stagedDiff, unexpected) {
			t.Errorf("Staged diff should not contain non-validation change '%s', but it did.\nActual diff:\n%s", unexpected, stagedDiff)
		}
	}

	// 2番目のコミットを作成（バリデーション機能追加）
	_, err = runCommand(t, dir, "git", "commit", "-m", "feat: add input validation for user creation endpoint")
	if err != nil {
		t.Fatalf("Failed to commit validation changes: %v", err)
	}

	// シナリオ3: 残りの変更をまとめてコミット（ハンク3のみ）
	// これは設定とAPIの改善のセマンティックなコミット
	err = runGitSequentialStage([]string{"web_server.py:3"}, absPatchPath)
	if err != nil {
		t.Fatalf("git-sequential-stage failed for config/api improvements commit: %v", err)
	}

	// 検証3: 設定とAPI改善の変更がステージングされているか
	stagedDiff, err = runCommand(t, dir, "git", "diff", "--cached")
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	// デバッグ用：シナリオ3のスナップショット表示
	t.Logf("=== SCENARIO 3: CONFIG/API IMPROVEMENTS COMMIT (web_server.py:3) ===")
	t.Logf("STAGED DIFF:\n%s", stagedDiff)

	// ヘルスチェック改善（ハンク3）とポート設定（ハンク4）の変更が含まれているか確認
	expectedConfigChanges := []string{
		"+    return {\"status\": \"ok\", \"timestamp\": \"2024-01-01T00:00:00Z\"}",
		"+    # Use environment variable for port configuration",
		"+    port = int(os.environ.get('PORT', 5000))",
		"+    app.run(debug=True, port=port)",
	}

	for _, expected := range expectedConfigChanges {
		if !strings.Contains(stagedDiff, expected) {
			t.Errorf("Expected staged diff to contain config/api change '%s', but it didn't.\nActual diff:\n%s", expected, stagedDiff)
		}
	}

	// 3番目のコミットを作成（設定とAPI改善）
	_, err = runCommand(t, dir, "git", "commit", "-m", "improve: enhance health check endpoint and add configurable port")
	if err != nil {
		t.Fatalf("Failed to commit config/api improvements: %v", err)
	}

	// 最終検証: ワーキングディレクトリにもうステージングするものがないか
	workingDiff, err := runCommand(t, dir, "git", "diff")
	if err != nil {
		t.Fatalf("Failed to get working diff: %v", err)
	}

	if strings.TrimSpace(workingDiff) != "" {
		t.Errorf("Expected no remaining changes in working directory, but got:\n%s", workingDiff)
	}

	// 最終検証: 3つのコミットが作成されたか確認
	finalCommitCount := getCommitCount(t, dir)
	expectedCommits := 4 // 初期コミット + 3つの機能コミット
	if finalCommitCount != expectedCommits {
		t.Errorf("Expected %d commits, but got %d", expectedCommits, finalCommitCount)
	}
}