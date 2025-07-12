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
	output, err := runCommand(t, dir, "git", "diff", "--cached", "--name-only")
	if err != nil {
		t.Fatalf("Failed to get staged files: %v", err)
	}
	
	if output == "" {
		return []string{}
	}
	
	return strings.Split(strings.TrimSpace(output), "\n")
}

// TestSingleFileSingleHunk は1ファイル1ハンクのケースをテストします
func TestSingleFileSingleHunk(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// 初期ファイルを作成（シンプルなGoファイル）
	initialCode := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`
	createFile(t, dir, "main.go", initialCode)
	commitChanges(t, dir, "Initial commit")

	// ファイルを修正（1つのハンクの変更）
	modifiedCode := `package main

import "fmt"

func main() {
	// Adding a greeting message
	greeting := "Hello, Sequential Stage!"
	fmt.Println(greeting)
}
`
	modifyFile(t, dir, "main.go", modifiedCode)

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

	// git-sequential-stageコマンドを実行
	// 現在のワーキングディレクトリから実行ファイルのパスを取得
	wd, _ := os.Getwd()
	gitSeqStagePath := filepath.Join(wd, "git-sequential-stage")
	output, err = runCommand(t, dir, gitSeqStagePath, "-patch=changes.patch", "-hunk=main.go:1")
	if err != nil {
		t.Fatalf("git-sequential-stage failed: %v\nOutput: %s", err, output)
	}

	// 検証1: ステージングエリアにファイルがあるか
	stagedFiles := getStagedFiles(t, dir)
	if len(stagedFiles) != 1 || stagedFiles[0] != "main.go" {
		t.Errorf("Expected main.go to be staged, got: %v", stagedFiles)
	}

	// 検証2: ステージングエリアの内容が正しいか
	stagedDiff, err := runCommand(t, dir, "git", "diff", "--cached")
	if err != nil {
		t.Fatalf("Failed to get staged diff: %v", err)
	}

	// 期待される変更が含まれているか確認
	expectedChanges := []string{
		"+\t// Adding a greeting message",
		"+\tgreeting := \"Hello, Sequential Stage!\"",
		"+\tfmt.Println(greeting)",
		"-\tfmt.Println(\"Hello, World!\")",
	}

	for _, expected := range expectedChanges {
		if !strings.Contains(stagedDiff, expected) {
			t.Errorf("Expected staged diff to contain '%s', but it didn't.\nActual diff:\n%s", expected, stagedDiff)
		}
	}

	// 検証3: ワーキングディレクトリに変更が残っていないか
	workingDiff, err := runCommand(t, dir, "git", "diff")
	if err != nil {
		t.Fatalf("Failed to get working diff: %v", err)
	}

	if strings.TrimSpace(workingDiff) != "" {
		t.Errorf("Expected no changes in working directory, but got:\n%s", workingDiff)
	}
}