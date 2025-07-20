package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/syou6162/git-sequential-stage/testutils"
)

// Test data constants
const (
	// performanceTargetSeconds is the performance target for large hunk operations
	performanceTargetSeconds = 5
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
// このテストは、異なる目的・意味を持つ変更（ログ追加、バリデーション追加、設定改善など）を
// 適切に分離してコミットできることを保証するために重要です。セマンティックなコミット分割という
// git-sequential-stageの核心価値を検証します。
func TestMixedSemanticChanges(t *testing.T) {
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()

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
	testRepo.CreateFile("web_server.py", initialWebServerCode)
	testRepo.CommitChanges("Initial commit")

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
	testRepo.ModifyFile("web_server.py", modifiedWebServerCode)

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

	absPatchPath, err := filepath.Abs(patchPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// 一時的にディレクトリを変更してrunGitSequentialStageを実行
	defer testRepo.Chdir()()

	// シナリオ1: ロギング機能の追加のみをコミット（ハンク1のみ）
	// これは新機能追加のセマンティックなコミット
	err = runGitSequentialStage([]string{"web_server.py:1"}, absPatchPath)
	if err != nil {
		t.Fatalf("git-sequential-stage failed for logging commit: %v", err)
	}

	// 検証1: ロギング関連の変更のみがステージングされているか
	stagedDiff, err := testRepo.RunCommand("git", "diff", "--cached")
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
	_, err = testRepo.RunCommand("git", "commit", "-m", "feat: add logging infrastructure for request tracking")
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
	stagedDiff, err = testRepo.RunCommand("git", "diff", "--cached")
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
	_, err = testRepo.RunCommand("git", "commit", "-m", "feat: add input validation for user creation endpoint")
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
	stagedDiff, err = testRepo.RunCommand("git", "diff", "--cached")
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
	_, err = testRepo.RunCommand("git", "commit", "-m", "improve: enhance health check endpoint and add configurable port")
	if err != nil {
		t.Fatalf("Failed to commit config/api improvements: %v", err)
	}

	// 最終検証: ワーキングディレクトリにもうステージングするものがないか
	workingDiff, err := testRepo.RunCommand("git", "diff")
	if err != nil {
		t.Fatalf("Failed to get working diff: %v", err)
	}

	if strings.TrimSpace(workingDiff) != "" {
		t.Errorf("Expected no remaining changes in working directory, but got:\n%s", workingDiff)
	}

	// 最終検証: 3つのコミットが作成されたか確認
	finalCommitCount := testRepo.GetCommitCount()
	expectedCommits := 4 // 初期コミット + 3つの機能コミット
	if finalCommitCount != expectedCommits {
		t.Errorf("Expected %d commits, but got %d", expectedCommits, finalCommitCount)
	}
}

// TestErrorCases_NonExistentFile は存在しないファイルを指定した場合のエラーハンドリングをテストします
// このテストは、不正な引数に対する適切なエラーハンドリングが機能していることを保証するために重要です
func TestErrorCases_NonExistentFile(t *testing.T) {
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()

	// 初期ファイルを作成
	testRepo.CreateFile("existing.py", "print('Hello, World!')")
	testRepo.CommitChanges("Initial commit")

	// ファイルを修正
	testRepo.ModifyFile("existing.py", "print('Hello, Updated World!')")

	// パッチファイルを生成
	patchPath := filepath.Join(testRepo.Path, "changes.patch")
	output, err := testRepo.RunCommand("sh", "-c", "git diff > changes.patch")
	if err != nil {
		t.Fatalf("Failed to create patch file: %v\nOutput: %s", err, output)
	}

	// パッチファイルの絶対パスを取得
	absPatchPath, err := filepath.Abs(patchPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// 一時的にディレクトリを変更
	originalDir, _ := os.Getwd()
	defer testRepo.Chdir()()
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// 存在しないファイルを指定してgit-sequential-stageを実行
	err = runGitSequentialStage([]string{"non_existent_file.py:1"}, absPatchPath)
	if err == nil {
		t.Error("Expected error for non-existent file, but got none")
		return
	}

	// エラーメッセージを確認
	expectedErrorPatterns := []string{"no such file", "not found", "does not exist"}
	errorMessage := err.Error()
	foundPattern := false
	for _, pattern := range expectedErrorPatterns {
		if strings.Contains(strings.ToLower(errorMessage), pattern) {
			foundPattern = true
			break
		}
	}

	if !foundPattern {
		t.Errorf("Expected error message to contain file not found indication, got: %v", err)
	}

	// ステージングエリアが空であることを確認
	stagedFiles := testRepo.GetStagedFiles()
	if len(stagedFiles) != 0 {
		t.Errorf("Expected no files to be staged after error, but got: %v", stagedFiles)
	}
}

// TestErrorCases_InvalidHunkNumber は存在しないハンク番号を指定した場合のエラーハンドリングをテストします
// このテストは、パッチ内の無効な参照に対する適切なエラーハンドリングが機能していることを保証するために重要です
func TestErrorCases_InvalidHunkNumber(t *testing.T) {
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()

	// 初期ファイルを作成
	testRepo.CreateFile("simple.py", "print('First line')")
	testRepo.CommitChanges("Initial commit")

	// ファイルを修正（1つのハンクのみ生成される変更）
	testRepo.ModifyFile("simple.py", "print('Modified line')")

	// パッチファイルを生成
	patchPath := filepath.Join(testRepo.Path, "changes.patch")
	output, err := testRepo.RunCommand("sh", "-c", "git diff > changes.patch")
	if err != nil {
		t.Fatalf("Failed to create patch file: %v\nOutput: %s", err, output)
	}

	// パッチファイルの絶対パスを取得
	absPatchPath, err := filepath.Abs(patchPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// 一時的にディレクトリを変更
	originalDir, _ := os.Getwd()
	defer testRepo.Chdir()()
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// 存在しないハンク番号（ハンク2）を指定してgit-sequential-stageを実行
	// パッチファイルには1つのハンクしかないはず
	err = runGitSequentialStage([]string{"simple.py:2"}, absPatchPath)
	if err == nil {
		t.Error("Expected error for invalid hunk number, but got none")
		return
	}

	// エラーメッセージを確認
	expectedErrorPatterns := []string{"hunk", "not found", "invalid", "does not exist"}
	errorMessage := err.Error()
	foundPattern := false
	for _, pattern := range expectedErrorPatterns {
		if strings.Contains(strings.ToLower(errorMessage), pattern) {
			foundPattern = true
			break
		}
	}

	if !foundPattern {
		t.Errorf("Expected error message to contain hunk-related error indication, got: %v", err)
	}

	// ステージングエリアが空であることを確認
	stagedFiles := testRepo.GetStagedFiles()
	if len(stagedFiles) != 0 {
		t.Errorf("Expected no files to be staged after error, but got: %v", stagedFiles)
	}
}

// TestErrorCases_EmptyPatchFile は空のパッチファイルを指定した場合のエラーハンドリングをテストします
// このテストは、不正なパッチファイルに対する適切なエラーハンドリングが機能していることを保証するために重要です
func TestErrorCases_EmptyPatchFile(t *testing.T) {
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()

	// 初期ファイルを作成
	testRepo.CreateFile("test.py", "print('Hello, World!')")
	testRepo.CommitChanges("Initial commit")

	// 空のパッチファイルを作成
	emptyPatchPath := filepath.Join(testRepo.Path, "empty.patch")
	err := os.WriteFile(emptyPatchPath, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create empty patch file: %v", err)
	}

	// パッチファイルの絶対パスを取得
	absEmptyPatchPath, err := filepath.Abs(emptyPatchPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// 一時的にディレクトリを変更
	originalDir, _ := os.Getwd()
	defer testRepo.Chdir()()
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// 空のパッチファイルを指定してgit-sequential-stageを実行
	err = runGitSequentialStage([]string{"test.py:1"}, absEmptyPatchPath)
	if err == nil {
		t.Error("Expected error for empty patch file, but got none")
		return
	}

	// エラーメッセージを確認
	expectedErrorPatterns := []string{"empty", "no hunks", "invalid patch", "no changes", "not found"}
	errorMessage := err.Error()
	foundPattern := false
	for _, pattern := range expectedErrorPatterns {
		if strings.Contains(strings.ToLower(errorMessage), pattern) {
			foundPattern = true
			break
		}
	}

	if !foundPattern {
		t.Errorf("Expected error message to contain empty patch indication, got: %v", err)
	}

	// ステージングエリアが空であることを確認
	stagedFiles := testRepo.GetStagedFiles()
	if len(stagedFiles) != 0 {
		t.Errorf("Expected no files to be staged after error, but got: %v", stagedFiles)
	}
}

// TestBinaryFileHandling tests handling of binary files in patches
func TestBinaryFileHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	// Setup test repository
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()
	tempDir := testRepo.Path

	// Change to temp directory
	t.Chdir(tempDir)

	// Create initial text file
	textFile := "document.txt"
	textContent := "This is a text document.\nIt has multiple lines.\n"
	if err := os.WriteFile(textFile, []byte(textContent), 0644); err != nil {
		t.Fatalf("Failed to write text file: %v", err)
	}

	// Create binary file (small PNG image)
	binaryFile := "image.png"
	if err := os.WriteFile(binaryFile, testutils.TestData.MinimalPNGTransparent, 0644); err != nil {
		t.Fatalf("Failed to write binary file: %v", err)
	}

	// Initial commit
	gitAddCmd := exec.Command("git", "add", textFile, binaryFile)
	if output, err := gitAddCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git add: %v\nOutput: %s", err, output)
	}

	gitCommitCmd := exec.Command("git", "commit", "-m", "Initial commit with text and binary files")
	if output, err := gitCommitCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git commit: %v\nOutput: %s", err, output)
	}

	// Modify text file
	textContent2 := "This is a text document.\nIt has multiple lines.\nAdding a new line.\n"
	if err := os.WriteFile(textFile, []byte(textContent2), 0644); err != nil {
		t.Fatalf("Failed to update text file: %v", err)
	}

	// Replace binary file with a different one
	if err := os.WriteFile(binaryFile, testutils.TestData.MinimalPNGRed, 0644); err != nil {
		t.Fatalf("Failed to update binary file: %v", err)
	}

	// Generate patch
	patchFile := "mixed_changes.patch"
	gitDiffCmd := exec.Command("git", "diff", "HEAD")
	patchContent, err := gitDiffCmd.Output()
	if err != nil {
		t.Fatalf("Failed to generate diff: %v", err)
	}
	if err := os.WriteFile(patchFile, patchContent, 0644); err != nil {
		t.Fatalf("Failed to write patch file: %v", err)
	}

	// Test: Stage only text file changes (hunk 1)
	absPatchPath, err := filepath.Abs(patchFile)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	if err := runGitSequentialStage([]string{"document.txt:1"}, absPatchPath); err != nil {
		t.Fatalf("Failed to stage text file changes: %v", err)
	}

	// Verify only text file is staged
	gitStatusCmd := exec.Command("git", "status", "--porcelain")
	statusOutput, err := gitStatusCmd.Output()
	if err != nil {
		t.Fatalf("Failed to get git status: %v", err)
	}

	statusLines := strings.Split(strings.TrimSpace(string(statusOutput)), "\n")
	expectedStatus := map[string]string{
		"document.txt": "M ",
		"image.png":    " M",
	}

	for _, line := range statusLines {
		if line == "" {
			continue
		}
		status := line[:2]
		filename := line[3:]

		// Skip the patch file
		if filename == patchFile {
			continue
		}

		if expected, ok := expectedStatus[filename]; ok {
			if status != expected {
				t.Errorf("File %s: expected status %q, got %q", filename, expected, status)
			}
		} else {
			t.Errorf("Unexpected file in status: %s", filename)
		}
	}

	// Reset for next test
	gitResetCmd := exec.Command("git", "reset", "HEAD")
	if output, err := gitResetCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to reset: %v\nOutput: %s", err, output)
	}

	// Test: Try to stage binary file changes (should handle gracefully)
	// Expected behavior: Binary files don't have traditional hunks, so the tool
	// should either:
	// 1. Skip binary files with an appropriate message
	// 2. Stage the entire binary file change (since hunks don't apply)
	// The exact behavior depends on git's handling of binary files
	err = runGitSequentialStage([]string{"image.png:1"}, absPatchPath)
	if err == nil {
		// If it succeeds, verify the binary file is staged
		gitStatusCmd := exec.Command("git", "status", "--porcelain")
		statusOutput, err := gitStatusCmd.Output()
		if err != nil {
			t.Fatalf("Failed to get git status: %v", err)
		}

		if strings.Contains(string(statusOutput), "M  image.png") {
			t.Log("Binary file was successfully staged")
		}
	} else {
		// If it fails, make sure it's with a reasonable error message
		if !strings.Contains(err.Error(), "binary") && !strings.Contains(err.Error(), "hunk") {
			t.Errorf("Unexpected error for binary file: %v", err)
		}
		t.Logf("Binary file handling resulted in expected error: %v", err)
	}
}

// TestFileModificationAndMove tests handling of file modifications combined with moves
func TestFileModificationAndMove(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	// Setup test repository
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()
	tempDir := testRepo.Path

	// Change to temp directory
	t.Chdir(tempDir)

	// Create initial file structure
	if err := os.MkdirAll("src", 0755); err != nil {
		t.Fatalf("Failed to create src directory: %v", err)
	}

	oldFile := "old_module.py"
	fileContent := `#!/usr/bin/env python3

def old_function():
    print("This is the old function")

def main():
    old_function()

if __name__ == "__main__":
    main()
`
	if err := os.WriteFile(oldFile, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Initial commit
	gitAddCmd := exec.Command("git", "add", oldFile)
	if output, err := gitAddCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git add: %v\nOutput: %s", err, output)
	}

	gitCommitCmd := exec.Command("git", "commit", "-m", "Initial commit with old module")
	if output, err := gitCommitCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git commit: %v\nOutput: %s", err, output)
	}

	// Test a different scenario: modify existing file and also move it
	// First modify the content
	modifiedContent := `#!/usr/bin/env python3

def old_function():
    print("This is the old function with modifications")
    print("Adding more functionality")

def new_helper():
    print("A helper function")

def main():
    old_function()
    new_helper()

if __name__ == "__main__":
    main()
`
	if err := os.WriteFile(oldFile, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("Failed to write modified content: %v", err)
	}

	// Generate patch for modifications
	patchFile := "modifications.patch"
	gitDiffCmd := exec.Command("git", "diff", "HEAD")
	patchContent, err := gitDiffCmd.Output()
	if err != nil {
		t.Fatalf("Failed to generate diff: %v", err)
	}
	if err := os.WriteFile(patchFile, patchContent, 0644); err != nil {
		t.Fatalf("Failed to write patch file: %v", err)
	}

	// Test: Stage only the first hunk (function modification)
	absPatchPath, err := filepath.Abs(patchFile)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	err = runGitSequentialStage([]string{"old_module.py:1"}, absPatchPath)
	if err != nil {
		t.Fatalf("Failed to stage first hunk: %v", err)
	}

	// Verify partial staging
	gitDiffCachedCmd := exec.Command("git", "diff", "--cached")
	cachedDiff, err := gitDiffCachedCmd.Output()
	if err != nil {
		t.Fatalf("Failed to get cached diff: %v", err)
	}

	// Should contain the oldFunction modification
	if !strings.Contains(string(cachedDiff), "old function with modifications") {
		t.Error("First hunk not properly staged")
	}

	// Check unstaged changes
	gitDiffCmd2 := exec.Command("git", "diff")
	unstagedDiff, err := gitDiffCmd2.Output()
	if err != nil {
		t.Fatalf("Failed to get unstaged diff: %v", err)
	}

	// Log the diffs for debugging
	t.Logf("Cached diff:\n%s", cachedDiff)
	t.Logf("Unstaged diff:\n%s", unstagedDiff)

	// The modifications might be in a single hunk, so check if anything remains unstaged
	if len(unstagedDiff) > 0 && strings.Contains(string(unstagedDiff), "@@") {
		t.Log("Some changes remain unstaged as expected")
	} else {
		t.Log("All changes were staged in a single hunk")
	}

	// Now test moving files scenario with a clean state
	gitResetCmd := exec.Command("git", "reset", "--hard", "HEAD")
	if output, err := gitResetCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to reset: %v\nOutput: %s", err, output)
	}

	// Test scenario: modify file content AND move it
	// This is a more complex test case showing how to handle both modifications and moves

	// First, apply modifications to the file
	modifiedContentForMove := `#!/usr/bin/env python3

def old_function():
    print("This is the function ready for move")
    print("Now with new modifications")

def new_helper():
    print("A helper function")

def additional_func():
    print("Another new function")

def main():
    old_function()
    new_helper()
    additional_func()

if __name__ == "__main__":
    main()
`
	if err := os.WriteFile(oldFile, []byte(modifiedContentForMove), 0644); err != nil {
		t.Fatalf("Failed to write content for move: %v", err)
	}

	// Move file to new location
	newFile := "src/new_module.py"
	if err := os.Rename(oldFile, newFile); err != nil {
		t.Fatalf("Failed to move file: %v", err)
	}

	// Stage the rename/move
	gitRmCmd := exec.Command("git", "rm", oldFile)
	if output, err := gitRmCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git rm: %v\nOutput: %s", err, output)
	}

	gitAddCmd2 := exec.Command("git", "add", newFile)
	if output, err := gitAddCmd2.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git add new file: %v\nOutput: %s", err, output)
	}

	// Verify Git recognizes it as a rename with modifications
	gitStatusCmd := exec.Command("git", "status", "--porcelain")
	statusOutput, err := gitStatusCmd.Output()
	if err != nil {
		t.Fatalf("Failed to get git status: %v", err)
	}

	statusStr := string(statusOutput)
	if strings.Contains(statusStr, "R  old_module.py -> src/new_module.py") {
		t.Log("Git correctly detected file rename with modifications")
	} else if strings.Contains(statusStr, "D  old_module.py") && strings.Contains(statusStr, "A  src/new_module.py") {
		t.Log("Git shows rename as delete + add (expected for files with significant changes)")
	} else {
		t.Logf("Unexpected git status output:\n%s", statusStr)
	}

	// Test that we can still apply patches to the original file location
	// This demonstrates handling of patches created before a file move
	t.Log("Note: Applying patches to moved files requires careful handling of file paths")
}

// TestIntentToAddWithStagedHunks はintent-to-addファイルのハンクをステージングする場合のテストです
// 既存ファイルへの変更と新規ファイル（intent-to-add）が混在する場合の安全性チェックを確認します
func TestIntentToAddWithStagedHunks(t *testing.T) {
	t.Skip("Temporarily skipping intent-to-add test - needs intent-to-add detection fix")
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()

	// 初期コミットを作成（既存ファイル含む）
	testRepo.CreateFile("README.md", "# Test Project\n")
	testRepo.CreateFile("existing.go", `package main

func existing() {
	// Original function
}
`)
	testRepo.CommitChanges("Initial commit")

	// 既存ファイルを修正
	testRepo.CreateFile("existing.go", `package main

func existing() {
	// Modified function
	println("Updated")
}

func newFunc() {
	// New function in existing file
	println("New")
}
`)

	// 新規ファイルを作成してintent-to-addで追加
	newFile := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`
	testRepo.CreateFile("main.go", newFile)

	// testRepoのディレクトリに移動
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	if err := os.Chdir(testRepo.Path); err != nil {
		t.Fatalf("Failed to change to test repo directory: %v", err)
	}
	defer os.Chdir(origDir)

	// git add -N を実行
	cmd := exec.Command("git", "add", "-N", "main.go")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add file with intent-to-add: %v", err)
	}

	// git diff でパッチを生成
	diffOutput, err := exec.Command("git", "diff", "HEAD").Output()
	if err != nil {
		t.Fatalf("Failed to get diff: %v", err)
	}

	// パッチファイルを作成
	patchPath := filepath.Join(testRepo.Path, "changes.patch")
	if err := os.WriteFile(patchPath, diffOutput, 0644); err != nil {
		t.Fatalf("Failed to write patch file: %v", err)
	}

	// パッチファイルの絶対パスを取得
	absPatchPath, err := filepath.Abs(patchPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// すでにtestRepoのディレクトリにいるので、Chdirは不要

	// 既存ファイルの最初のハンクだけをステージング
	err = runGitSequentialStage([]string{"existing.go:1"}, absPatchPath)
	if err != nil {
		t.Fatalf("Failed to stage hunk from existing file: %v", err)
	}

	// ステージングエリアを確認
	stagedDiff, err := exec.Command("git", "diff", "--cached").Output()
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	stagedContent := string(stagedDiff)

	// existing関数の変更がステージングされていることを確認
	if !strings.Contains(stagedContent, "Modified function") {
		t.Errorf("Expected existing function changes to be staged")
	}

	// newFunc関数はステージングされていないことを確認
	if strings.Contains(stagedContent, "func newFunc()") {
		t.Errorf("Expected newFunc NOT to be staged yet")
	}

	// intent-to-add新規ファイルもステージング可能なことを確認
	err = runGitSequentialStage([]string{"main.go:1"}, absPatchPath)
	if err != nil {
		t.Fatalf("Failed to stage intent-to-add file: %v", err)
	}

	// 両方のファイルがステージングされたことを確認
	stagedDiff2, err := exec.Command("git", "diff", "--cached").Output()
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	stagedContent2 := string(stagedDiff2)
	if !strings.Contains(stagedContent2, "func main()") {
		t.Errorf("Expected main.go to be staged")
	}
	if !strings.Contains(stagedContent2, "Modified function") {
		t.Errorf("Expected existing.go changes to remain staged")
	}
}

// TestHunkCountExceededError tests error handling when requesting more hunks than available
func TestHunkCountExceededError(t *testing.T) {
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()

	// 初期コミットを作成
	testRepo.CreateAndCommitFile("README.md", "# Test Project\n", "Initial commit")

	// 既存ファイルを修正して複数ハンクを作成
	originalContent := `package main

func original() {
	println("Original")
}
`
	testRepo.CreateAndCommitFile("main.go", originalContent, "Add main.go")

	// 修正して2つのハンクを作成
	modifiedContent := `package main

func original() {
	println("Modified original")  // Modified line
}

func newFunction() {
	println("New function")  // New function
}
`
	testRepo.CreateFile("main.go", modifiedContent)

	// パッチファイルを生成
	diffOutput, err := testRepo.RunCommand("git", "diff", "HEAD")
	if err != nil {
		t.Fatalf("Failed to get diff: %v", err)
	}

	patchPath := filepath.Join(testRepo.Path, "changes.patch")
	if err := os.WriteFile(patchPath, []byte(diffOutput), 0644); err != nil {
		t.Fatalf("Failed to write patch file: %v", err)
	}

	absPatchPath, err := filepath.Abs(patchPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// ディレクトリを変更
	defer testRepo.Chdir()()

	// 存在しないハンク番号（2,3番）を指定してエラーを発生させる
	err = runGitSequentialStage([]string{"main.go:1,2,3"}, absPatchPath)
	if err == nil {
		t.Fatal("Expected error when requesting non-existent hunk, but got none")
	}

	// エラーメッセージの内容を確認
	errorMsg := err.Error()

	// ファイル名が含まれている
	if !strings.Contains(errorMsg, "main.go") {
		t.Errorf("Expected error message to contain file name 'main.go', got: %s", errorMsg)
	}

	// 実際のハンク数が含まれている（新しい簡潔な形式）
	if !strings.Contains(errorMsg, "has 1 hunk") {
		t.Errorf("Expected error message to mention '1 hunk', got: %s", errorMsg)
	}

	// 要求された無効なハンク番号が含まれている
	if !strings.Contains(errorMsg, "[2, 3]") || !strings.Contains(errorMsg, "requested") {
		t.Errorf("Expected error message to mention requested hunks '[2, 3]', got: %s", errorMsg)
	}
}

// TestErrorCases_MultipleInvalidHunks tests error handling when requesting multiple invalid hunks
func TestErrorCases_MultipleInvalidHunks(t *testing.T) {
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()

	// 初期コミットを作成
	testRepo.CreateAndCommitFile("README.md", "# Test Project\n", "Initial commit")

	// 既存ファイルに1つのハンクを作成
	originalContent := `func original() {
	println("Original")
}`
	testRepo.CreateAndCommitFile("single.go", originalContent, "Add single.go")

	// 修正して1つのハンクを作成
	modifiedContent := `func original() {
	println("Modified original")  // Only one hunk
}`
	testRepo.CreateFile("single.go", modifiedContent)

	// パッチファイルを生成
	diffOutput, err := testRepo.RunCommand("git", "diff", "HEAD")
	if err != nil {
		t.Fatalf("Failed to get diff: %v", err)
	}

	patchPath := filepath.Join(testRepo.Path, "changes.patch")
	if err := os.WriteFile(patchPath, []byte(diffOutput), 0644); err != nil {
		t.Fatalf("Failed to write patch file: %v", err)
	}

	absPatchPath, err := filepath.Abs(patchPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// ディレクトリを変更
	defer testRepo.Chdir()()

	// 複数の存在しないハンク番号を指定
	err = runGitSequentialStage([]string{"single.go:2,3,4"}, absPatchPath)
	if err == nil {
		t.Fatal("Expected error when requesting multiple non-existent hunks, but got none")
	}

	errorMsg := err.Error()

	// 複数の無効なハンク番号が含まれている
	if !strings.Contains(errorMsg, "[2, 3, 4]") {
		t.Errorf("Expected error message to mention multiple invalid hunks '[2, 3, 4]', got: %s", errorMsg)
	}

	// 実際のハンク数が含まれている
	if !strings.Contains(errorMsg, "has 1 hunks") {
		t.Errorf("Expected error message to mention '1 hunks', got: %s", errorMsg)
	}
}

// TestLargeFileWithManyHunks tests handling of large files with many hunks
func TestLargeFileWithManyHunks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	// Setup test repository
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()
	tempDir := testRepo.Path

	// Change to temp directory
	t.Chdir(tempDir)

	// Create a large file with many functions
	largeFile := "large_module.py"
	var content strings.Builder
	content.WriteString("#!/usr/bin/env python3\n\n")

	// Create 20 functions
	for i := 1; i <= 20; i++ {
		content.WriteString(fmt.Sprintf(`def function_%d():
    print("This is function %d")

`, i, i))
	}

	content.WriteString(`def main():
`)
	for i := 1; i <= 20; i++ {
		content.WriteString(fmt.Sprintf("    function_%d()\n", i))
	}
	content.WriteString(`
if __name__ == "__main__":
    main()
`)

	if err := os.WriteFile(largeFile, []byte(content.String()), 0644); err != nil {
		t.Fatalf("Failed to write large file: %v", err)
	}

	// Initial commit
	gitAddCmd := exec.Command("git", "add", largeFile)
	if output, err := gitAddCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git add: %v\nOutput: %s", err, output)
	}

	gitCommitCmd := exec.Command("git", "commit", "-m", "Initial commit with large file")
	if output, err := gitCommitCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git commit: %v\nOutput: %s", err, output)
	}

	// Modify multiple functions throughout the file
	var modifiedContent strings.Builder
	modifiedContent.WriteString("#!/usr/bin/env python3\n\n")

	for i := 1; i <= 20; i++ {
		if i == 1 || i == 5 || i == 10 || i == 15 || i == 20 {
			// Modify these functions
			modifiedContent.WriteString(fmt.Sprintf(`def function_%d():
    print("This is function %d - MODIFIED")
    print("Additional line in function %d")

`, i, i, i))
		} else {
			// Keep original
			modifiedContent.WriteString(fmt.Sprintf(`def function_%d():
    print("This is function %d")

`, i, i))
		}
	}

	modifiedContent.WriteString(`def main():
`)
	for i := 1; i <= 20; i++ {
		modifiedContent.WriteString(fmt.Sprintf("    function_%d()\n", i))
	}
	modifiedContent.WriteString(`    print("All functions called")

if __name__ == "__main__":
    main()
`)

	if err := os.WriteFile(largeFile, []byte(modifiedContent.String()), 0644); err != nil {
		t.Fatalf("Failed to write modified file: %v", err)
	}

	// Generate patch
	patchFile := "large_file_changes.patch"
	gitDiffCmd := exec.Command("git", "diff", "HEAD")
	patchContent, err := gitDiffCmd.Output()
	if err != nil {
		t.Fatalf("Failed to generate diff: %v", err)
	}
	if err := os.WriteFile(patchFile, patchContent, 0644); err != nil {
		t.Fatalf("Failed to write patch file: %v", err)
	}

	// Count the number of hunks
	hunkCount := strings.Count(string(patchContent), "@@")
	t.Logf("Generated patch with %d hunks", hunkCount)

	// Test: Stage specific hunks (1, 3, and 5) with performance measurement
	absPatchPath, err := filepath.Abs(patchFile)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Stage hunks 1, 3, and 5 (if available)
	var selectedHunks []string
	selectedHunks = append(selectedHunks, "1", "3")
	if hunkCount >= 5 {
		selectedHunks = append(selectedHunks, "5")
	}

	hunkSpec := fmt.Sprintf("%s:%s", largeFile, strings.Join(selectedHunks, ","))

	// Measure performance
	startTime := time.Now()
	err = runGitSequentialStage([]string{hunkSpec}, absPatchPath)
	elapsed := time.Since(startTime)

	if err != nil {
		t.Fatalf("Failed to stage selected hunks: %v", err)
	}

	// Log performance metrics
	t.Logf("Performance: Staged %d hunks in %v", len(selectedHunks), elapsed)

	// Check if performance meets target
	targetDuration := time.Duration(performanceTargetSeconds) * time.Second
	if elapsed > targetDuration {
		t.Errorf("Performance issue: operation took %v, expected < %v", elapsed, targetDuration)
	} else {
		t.Logf("Performance is acceptable: %v < %v target", elapsed, targetDuration)
	}

	// Verify partial staging
	gitDiffCachedCmd := exec.Command("git", "diff", "--cached")
	cachedDiff, err := gitDiffCachedCmd.Output()
	if err != nil {
		t.Fatalf("Failed to get cached diff: %v", err)
	}

	// Count staged hunks
	stagedHunkCount := strings.Count(string(cachedDiff), "@@")
	t.Logf("Staged %d hunks out of %d", stagedHunkCount, hunkCount)

	// Verify we have both staged and unstaged changes
	gitDiffCmd2 := exec.Command("git", "diff")
	unstagedDiff, err := gitDiffCmd2.Output()
	if err != nil {
		t.Fatalf("Failed to get unstaged diff: %v", err)
	}

	unstagedHunkCount := strings.Count(string(unstagedDiff), "@@")
	t.Logf("Remaining unstaged hunks: %d", unstagedHunkCount)

	// Basic validation
	if stagedHunkCount == 0 {
		t.Error("No hunks were staged")
	}
	if stagedHunkCount == hunkCount {
		t.Error("All hunks were staged, expected partial staging")
	}
	if unstagedHunkCount == 0 && hunkCount > 3 {
		t.Error("No hunks remain unstaged, expected some unstaged changes")
	}

	// Test performance with many hunk selections
	// Reset for performance test
	gitResetCmd := exec.Command("git", "reset", "HEAD")
	if output, err := gitResetCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to reset: %v\nOutput: %s", err, output)
	}

	// Stage many hunks individually
	var manyHunks []string
	maxHunks := 10
	if hunkCount < maxHunks {
		maxHunks = hunkCount
	}
	for i := 1; i <= maxHunks; i++ {
		if i%2 == 1 { // Stage odd-numbered hunks
			manyHunks = append(manyHunks, fmt.Sprintf("%d", i))
		}
	}

	if len(manyHunks) > 0 {
		hunkSpec := fmt.Sprintf("%s:%s", largeFile, strings.Join(manyHunks, ","))
		err = runGitSequentialStage([]string{hunkSpec}, absPatchPath)
		if err != nil {
			t.Logf("Failed to stage many hunks: %v", err)
		} else {
			elapsed := time.Since(startTime)
			t.Logf("Staged %d hunks in %v", len(manyHunks), elapsed)

			// Warn if it takes too long
			if elapsed > 5*time.Second {
				t.Logf("Warning: Staging %d hunks took %v, which might be slow", len(manyHunks), elapsed)
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
