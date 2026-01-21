package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/syou6162/git-sequential-stage/testutils"
)

// TestIntentToAddFileCoexistence はintent-to-addファイルがある場合でも他のファイルのステージングが可能であることを確認します
// ワーカーパターン: intent-to-addファイルが存在する場合でも、ターゲットファイルが明示的に指定されていれば正常動作する
func TestIntentToAddFileCoexistence(t *testing.T) {
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
	defer func() { _ = os.Chdir(origDir) }()

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

	// 既存ファイルの最初のハンクをステージングしようとする
	// intent-to-addファイルが存在する場合でも、ターゲットファイルが指定されていれば正常動作する
	err = runGitSequentialStage(context.Background(), []string{"existing.go:1"}, absPatchPath)
	if err != nil {
		t.Fatalf("Expected staging to succeed with intent-to-add files present, but got error: %v", err)
	}

	// ステージングが成功したことを確認
	stagedDiff, err := exec.Command("git", "diff", "--cached").Output()
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	// existing.goの変更がステージングされていることを確認
	if !strings.Contains(string(stagedDiff), "Modified function") {
		t.Errorf("Expected existing.go changes to be staged, got: %s", stagedDiff)
	}

	t.Log("Staging succeeded with intent-to-add files present")
}

// TestUntrackedFile tests the behavior when trying to stage hunks from a completely untracked file
// This test verifies that the tool properly handles files that are not tracked by git (status: ??)
func TestUntrackedFile(t *testing.T) {

	// Setup test repository
	testRepo := testutils.NewTestRepo(t, "git-sequential-stage-e2e-*")
	defer testRepo.Cleanup()

	// Create initial commit
	testRepo.CreateAndCommitFile("README.md", "# Test Project\n", "Initial commit")

	// testRepoのディレクトリに移動
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	if err := os.Chdir(testRepo.Path); err != nil {
		t.Fatalf("Failed to change to test repo directory: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create a completely new file (untracked - status ??)
	untrackedFile := "untracked.py"
	untrackedContent := `def hello():
    print("Hello from untracked file")

def world():
    print("World from untracked file")

def main():
    hello()
    world()

if __name__ == "__main__":
    main()
`
	testRepo.CreateFile(untrackedFile, untrackedContent)

	// Verify file is untracked
	statusOutput, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		t.Fatalf("Failed to get git status: %v", err)
	}
	if !strings.Contains(string(statusOutput), "?? "+untrackedFile) {
		t.Fatalf("File should be untracked, got status: %s", statusOutput)
	}

	// Try to generate patch using git diff HEAD (should be empty for untracked files)
	diffOutput, err := exec.Command("git", "diff", "HEAD").Output()
	if err != nil {
		t.Fatalf("Failed to get diff: %v", err)
	}

	// Verify diff is empty for untracked files
	if strings.Contains(string(diffOutput), untrackedFile) {
		t.Errorf("git diff HEAD should not show untracked files, but got: %s", diffOutput)
	}

	// Create patch file (will be empty or not contain the untracked file)
	patchPath := filepath.Join(testRepo.Path, "changes.patch")
	if err := os.WriteFile(patchPath, diffOutput, 0644); err != nil {
		t.Fatalf("Failed to write patch file: %v", err)
	}

	absPatchPath, err := filepath.Abs(patchPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Try to stage hunks from the untracked file - should fail
	err = runGitSequentialStage(context.Background(), []string{untrackedFile + ":1"}, absPatchPath)
	if err == nil {
		t.Fatal("Expected error when trying to stage untracked file, but got none")
	}

	// Check error message
	errorMsg := err.Error()
	t.Logf("Error message for untracked file: %s", errorMsg)

	if !strings.Contains(errorMsg, untrackedFile) || !strings.Contains(errorMsg, "not found") {
		t.Errorf("Expected error about file not found in patch, got: %s", errorMsg)
	}

	// Check if advice about git add -N is included
	if !strings.Contains(errorMsg, "git ls-files --others --exclude-standard | xargs git add -N") {
		t.Errorf("Expected advice about using 'git ls-files --others --exclude-standard | xargs git add -N', got: %s", errorMsg)
	}

	// Now test with git add -N (intent-to-add)
	cmd := exec.Command("git", "add", "-N", untrackedFile)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add file with intent-to-add: %v", err)
	}

	// Verify file is now intent-to-add
	statusOutput2, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		t.Fatalf("Failed to get git status: %v", err)
	}
	if !strings.Contains(string(statusOutput2), " A "+untrackedFile) {
		t.Fatalf("File should be in intent-to-add state, got status: %s", statusOutput2)
	}

	// Now git diff HEAD should show the file
	diffOutput2, err := exec.Command("git", "diff", "HEAD").Output()
	if err != nil {
		t.Fatalf("Failed to get diff: %v", err)
	}

	// Verify diff now contains the file
	if !strings.Contains(string(diffOutput2), untrackedFile) {
		t.Errorf("git diff HEAD should show intent-to-add files, but got: %s", diffOutput2)
	}

	// Create new patch file with intent-to-add content
	patchPath2 := filepath.Join(testRepo.Path, "changes_with_intent.patch")
	if err := os.WriteFile(patchPath2, diffOutput2, 0644); err != nil {
		t.Fatalf("Failed to write patch file: %v", err)
	}

	absPatchPath2, err := filepath.Abs(patchPath2)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Now staging should work
	err = runGitSequentialStage(context.Background(), []string{untrackedFile + ":1"}, absPatchPath2)
	if err != nil {
		t.Fatalf("Failed to stage intent-to-add file: %v", err)
	}

	// Verify staging succeeded
	stagedDiff, err := exec.Command("git", "diff", "--cached").Output()
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	if !strings.Contains(string(stagedDiff), "def hello():") {
		t.Errorf("Expected file content to be staged, got: %s", stagedDiff)
	}
}

// setupIntentToAddCoexistenceTest creates a common test setup with an existing file modification
// and a new file added with intent-to-add
func setupIntentToAddCoexistenceTest(t *testing.T) (testRepo *testutils.TestRepo, patchPath string, cleanup func()) {
	testRepo = testutils.NewTestRepo(t, "git-sequential-stage-intent-to-add-*")

	// 初期コミットを作成（既存ファイル含む）
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
`)

	// 新規ファイルを作成してintent-to-addで追加
	newFile := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`
	testRepo.CreateFile("new_file.go", newFile)

	// testRepoのディレクトリに移動
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	if err := os.Chdir(testRepo.Path); err != nil {
		t.Fatalf("Failed to change to test repo directory: %v", err)
	}

	// git add -N を実行
	cmd := exec.Command("git", "add", "-N", "new_file.go")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add file with intent-to-add: %v", err)
	}

	// git diff でパッチを生成
	diffOutput, err := exec.Command("git", "diff", "HEAD").Output()
	if err != nil {
		t.Fatalf("Failed to get diff: %v", err)
	}

	// パッチファイルを作成
	patchPath = filepath.Join(testRepo.Path, "changes.patch")
	if err := os.WriteFile(patchPath, diffOutput, 0644); err != nil {
		t.Fatalf("Failed to write patch file: %v", err)
	}

	// パッチファイルの絶対パスを取得
	absPatchPath, err := filepath.Abs(patchPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	cleanup = func() {
		_ = os.Chdir(origDir)
		testRepo.Cleanup()
	}

	return testRepo, absPatchPath, cleanup
}

// TestIntentToAddCoexistence_ExistingOnly_Wildcard tests staging existing file only with wildcard
// when intent-to-add file is present
func TestIntentToAddCoexistence_ExistingOnly_Wildcard(t *testing.T) {
	_, patchPath, cleanup := setupIntentToAddCoexistenceTest(t)
	defer cleanup()

	// 既存ファイルのみをワイルドカードでステージング
	err := runGitSequentialStage(context.Background(), []string{"existing.go:*"}, patchPath)
	if err != nil {
		t.Fatalf("Expected staging to succeed, but got error: %v", err)
	}

	// ステージングが成功したことを確認
	stagedDiff, err := exec.Command("git", "diff", "--cached").Output()
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	// existing.goの変更がステージングされていることを確認
	if !strings.Contains(string(stagedDiff), "Modified function") {
		t.Errorf("Expected existing.go changes to be staged, got: %s", stagedDiff)
	}

	// new_file.goはステージングされていないことを確認
	if strings.Contains(string(stagedDiff), "new_file.go") {
		t.Errorf("new_file.go should not be staged, got: %s", stagedDiff)
	}
}

// TestIntentToAddCoexistence_NewOnly_Hunk tests staging new file only with hunk number
// when intent-to-add file is present
func TestIntentToAddCoexistence_NewOnly_Hunk(t *testing.T) {
	_, patchPath, cleanup := setupIntentToAddCoexistenceTest(t)
	defer cleanup()

	// 新規ファイルのみをhunk指定でステージング
	err := runGitSequentialStage(context.Background(), []string{"new_file.go:1"}, patchPath)
	if err != nil {
		t.Fatalf("Expected staging to succeed, but got error: %v", err)
	}

	// ステージングが成功したことを確認
	stagedDiff, err := exec.Command("git", "diff", "--cached").Output()
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	// new_file.goの変更がステージングされていることを確認
	if !strings.Contains(string(stagedDiff), "Hello, World!") {
		t.Errorf("Expected new_file.go changes to be staged, got: %s", stagedDiff)
	}

	// existing.goはステージングされていないことを確認
	if strings.Contains(string(stagedDiff), "Modified function") {
		t.Errorf("existing.go should not be staged, got: %s", stagedDiff)
	}
}

// TestIntentToAddCoexistence_NewOnly_Wildcard tests staging new file only with wildcard
// when intent-to-add file is present
func TestIntentToAddCoexistence_NewOnly_Wildcard(t *testing.T) {
	_, patchPath, cleanup := setupIntentToAddCoexistenceTest(t)
	defer cleanup()

	// 新規ファイルのみをワイルドカードでステージング
	err := runGitSequentialStage(context.Background(), []string{"new_file.go:*"}, patchPath)
	if err != nil {
		t.Fatalf("Expected staging to succeed, but got error: %v", err)
	}

	// ステージングが成功したことを確認
	stagedDiff, err := exec.Command("git", "diff", "--cached").Output()
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	// new_file.goの変更がステージングされていることを確認
	if !strings.Contains(string(stagedDiff), "Hello, World!") {
		t.Errorf("Expected new_file.go changes to be staged, got: %s", stagedDiff)
	}

	// existing.goはステージングされていないことを確認
	if strings.Contains(string(stagedDiff), "Modified function") {
		t.Errorf("existing.go should not be staged, got: %s", stagedDiff)
	}
}

// TestIntentToAddCoexistence_Both_HunkHunk tests staging both files with hunk numbers
// when intent-to-add file is present
func TestIntentToAddCoexistence_Both_HunkHunk(t *testing.T) {
	_, patchPath, cleanup := setupIntentToAddCoexistenceTest(t)
	defer cleanup()

	// 両方のファイルをhunk指定でステージング
	err := runGitSequentialStage(context.Background(), []string{"existing.go:1", "new_file.go:1"}, patchPath)
	if err != nil {
		t.Fatalf("Expected staging to succeed, but got error: %v", err)
	}

	// ステージングが成功したことを確認
	stagedDiff, err := exec.Command("git", "diff", "--cached").Output()
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	// existing.goの変更がステージングされていることを確認
	if !strings.Contains(string(stagedDiff), "Modified function") {
		t.Errorf("Expected existing.go changes to be staged, got: %s", stagedDiff)
	}

	// new_file.goの変更がステージングされていることを確認
	if !strings.Contains(string(stagedDiff), "Hello, World!") {
		t.Errorf("Expected new_file.go changes to be staged, got: %s", stagedDiff)
	}
}

// TestIntentToAddCoexistence_Both_WildcardWildcard tests staging both files with wildcards
// when intent-to-add file is present
func TestIntentToAddCoexistence_Both_WildcardWildcard(t *testing.T) {
	_, patchPath, cleanup := setupIntentToAddCoexistenceTest(t)
	defer cleanup()

	// 両方のファイルをワイルドカードでステージング
	err := runGitSequentialStage(context.Background(), []string{"existing.go:*", "new_file.go:*"}, patchPath)
	if err != nil {
		t.Fatalf("Expected staging to succeed, but got error: %v", err)
	}

	// ステージングが成功したことを確認
	stagedDiff, err := exec.Command("git", "diff", "--cached").Output()
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	// existing.goの変更がステージングされていることを確認
	if !strings.Contains(string(stagedDiff), "Modified function") {
		t.Errorf("Expected existing.go changes to be staged, got: %s", stagedDiff)
	}

	// new_file.goの変更がステージングされていることを確認
	if !strings.Contains(string(stagedDiff), "Hello, World!") {
		t.Errorf("Expected new_file.go changes to be staged, got: %s", stagedDiff)
	}
}

// TestIntentToAddCoexistence_Both_HunkWildcard tests staging existing file with hunk and new file with wildcard
// when intent-to-add file is present
func TestIntentToAddCoexistence_Both_HunkWildcard(t *testing.T) {
	_, patchPath, cleanup := setupIntentToAddCoexistenceTest(t)
	defer cleanup()

	// 既存ファイルをhunk、新規ファイルをワイルドカードでステージング
	err := runGitSequentialStage(context.Background(), []string{"existing.go:1", "new_file.go:*"}, patchPath)
	if err != nil {
		t.Fatalf("Expected staging to succeed, but got error: %v", err)
	}

	// ステージングが成功したことを確認
	stagedDiff, err := exec.Command("git", "diff", "--cached").Output()
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	// existing.goの変更がステージングされていることを確認
	if !strings.Contains(string(stagedDiff), "Modified function") {
		t.Errorf("Expected existing.go changes to be staged, got: %s", stagedDiff)
	}

	// new_file.goの変更がステージングされていることを確認
	if !strings.Contains(string(stagedDiff), "Hello, World!") {
		t.Errorf("Expected new_file.go changes to be staged, got: %s", stagedDiff)
	}
}

// TestIntentToAddCoexistence_Both_WildcardHunk tests staging existing file with wildcard and new file with hunk
// when intent-to-add file is present
func TestIntentToAddCoexistence_Both_WildcardHunk(t *testing.T) {
	_, patchPath, cleanup := setupIntentToAddCoexistenceTest(t)
	defer cleanup()

	// 既存ファイルをワイルドカード、新規ファイルをhunkでステージング
	err := runGitSequentialStage(context.Background(), []string{"existing.go:*", "new_file.go:1"}, patchPath)
	if err != nil {
		t.Fatalf("Expected staging to succeed, but got error: %v", err)
	}

	// ステージングが成功したことを確認
	stagedDiff, err := exec.Command("git", "diff", "--cached").Output()
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	// existing.goの変更がステージングされていることを確認
	if !strings.Contains(string(stagedDiff), "Modified function") {
		t.Errorf("Expected existing.go changes to be staged, got: %s", stagedDiff)
	}

	// new_file.goの変更がステージングされていることを確認
	if !strings.Contains(string(stagedDiff), "Hello, World!") {
		t.Errorf("Expected new_file.go changes to be staged, got: %s", stagedDiff)
	}
}
