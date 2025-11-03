package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/syou6162/git-sequential-stage/testutils"
)

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
	testRepo.GeneratePatch("changes.patch")

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
	defer func() { _ = os.Chdir(originalDir) }()

	// 存在しないファイルを指定してgit-sequential-stageを実行
	err = runGitSequentialStage(context.Background(), []string{"non_existent_file.py:1"}, absPatchPath)
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
	testRepo.GeneratePatch("changes.patch")

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
	defer func() { _ = os.Chdir(originalDir) }()

	// 存在しないハンク番号（ハンク2）を指定してgit-sequential-stageを実行
	// パッチファイルには1つのハンクしかないはず
	err = runGitSequentialStage(context.Background(), []string{"simple.py:2"}, absPatchPath)
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
	defer func() { _ = os.Chdir(originalDir) }()

	// 空のパッチファイルを指定してgit-sequential-stageを実行
	err = runGitSequentialStage(context.Background(), []string{"test.py:1"}, absEmptyPatchPath)
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

// TestErrorCases_HunkCountExceeded tests error handling when requesting more hunks than available
func TestErrorCases_HunkCountExceeded(t *testing.T) {
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
	err = runGitSequentialStage(context.Background(), []string{"main.go:1,2,3"}, absPatchPath)
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
	err = runGitSequentialStage(context.Background(), []string{"single.go:2,3,4"}, absPatchPath)
	if err == nil {
		t.Fatal("Expected error when requesting multiple non-existent hunks, but got none")
	}

	errorMsg := err.Error()

	// 複数の無効なハンク番号が含まれている
	if !strings.Contains(errorMsg, "[2, 3, 4]") {
		t.Errorf("Expected error message to mention multiple invalid hunks '[2, 3, 4]', got: %s", errorMsg)
	}

	// 実際のハンク数が含まれている（新しい簡潔な形式）
	if !strings.Contains(errorMsg, "has 1 hunk") {
		t.Errorf("Expected error message to mention '1 hunk', got: %s", errorMsg)
	}
}

// TestErrorCases_SameFileConflict は同一ファイルに対してワイルドカードとハンク番号が混在した場合のエラーテストです
func TestErrorCases_SameFileConflict(t *testing.T) {
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()

	// ファイルを作成
	testRepo.CreateFile("main.go", "package main\n\nfunc main() {}\n")
	testRepo.CommitChanges("Initial commit")

	// ファイルを変更
	testRepo.CreateFile("main.go", "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n\nfunc helper() {\n\tfmt.Println(\"Helper\")\n}\n")

	// パッチファイルを生成
	testRepo.GeneratePatch("changes.patch")
	patchFile := filepath.Join(testRepo.Path, "changes.patch")

	// テストリポジトリのディレクトリに移動
	defer testRepo.Chdir()()

	// 同一ファイルに対してワイルドカードとハンク番号を混在させる
	err := runGitSequentialStage(context.Background(),
		[]string{"main.go:1", "main.go:*"},
		patchFile,
	)

	// エラーが発生することを確認
	if err == nil {
		t.Fatal("Expected error for mixed wildcard and hunk numbers, but got none")
	}

	if !strings.Contains(err.Error(), "mixed wildcard and hunk numbers not allowed") {
		t.Errorf("Expected error message to contain 'mixed wildcard and hunk numbers not allowed', got: %v", err)
	}
}
