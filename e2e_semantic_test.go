package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/syou6162/git-sequential-stage/testutils"
)

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
