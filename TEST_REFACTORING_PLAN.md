# git-sequential-stage テスト構造改修計画書

**文書バージョン**: 1.0
**作成日**: 2025-10-27
**対象プロジェクト**: git-sequential-stage
**文書目的**: テストピラミッド最適化による保守性・実行速度・信頼性の向上

---

## エグゼクティブサマリー

### 現状の問題点

git-sequential-stageプロジェクトは現在、**テストピラミッドの逆転**が発生しています：

- **E2Eテスト**: 26テスト（3,290行）
- **ユニットテスト**: 75テスト（4,356行）
- **E2E:Unit比率**: 1:3（行数ベース）

数的にはユニットテストが多いものの、**実質的にはE2Eテストが過剰**です。理由：

1. **重複テストの存在**: バイナリファイル、エラーハンドリング、カウント機能などでE2EとUnitが重複
2. **不要なE2E化**: 本来ユニットテストで検証できる内容をE2Eで実装
3. **実行速度の懸念**: 各E2Eテストがgitリポジトリを作成（0.02〜0.31秒/テスト）
4. **保守コストの増大**: E2Eテストは環境依存性が高く、デバッグが困難

### 改修目標

理想的なテストピラミッド構造への移行：

```
     ┌────────┐
     │ E2E: 8 │  ← 真の統合シナリオのみ（-18テスト）
     ├────────┴────┐
     │ Integration: │  ← CLIレイヤー（main_test.go）
     │      15      │
     ├──────────────┴────┐
     │   Unit: 90-95     │  ← ビジネスロジック（+15-20テスト）
     └───────────────────┘
```

### 期待される効果

| 指標 | 現状 | 改修後 | 改善率 |
|-----|------|--------|--------|
| E2Eテスト数 | 26 | 8 | -69% |
| E2E実行時間（推定） | ~5秒 | ~1.5秒 | -70% |
| ユニットテスト数 | 75 | 90-95 | +20-27% |
| テストカバレッジ | 維持 | 維持 | 0% |
| デバッグ容易性 | 低 | 高 | 定性的改善 |

---

## 1. 現状分析

### 1.1 E2Eテストの詳細分析

#### E2Eテストファイル一覧

| ファイル名 | テスト数 | 行数 | 平均行数/テスト | 評価 |
|-----------|---------|------|----------------|------|
| `e2e_basic_test.go` | 6 | 749 | 125 | 🟡 一部統合可 |
| `e2e_semantic_test.go` | 1 | 287 | 287 | 🟢 E2E必須 |
| `e2e_error_test.go` | 6 | 364 | 61 | 🔴 ユニット化可 |
| `e2e_advanced_files_test.go` | 5 | 641 | 128 | 🟡 一部重複あり |
| `e2e_count_hunks_test.go` | 3 | 286 | 95 | 🔴 ユニット化可 |
| `e2e_integration_test.go` | 1 | 390 | 390 | 🟢 E2E必須 |
| `e2e_advanced_performance_test.go` | 1 | 222 | 222 | 🟢 E2E妥当 |
| `e2e_performance_test.go` | 1 | 100 | 100 | 🟡 統合可能 |
| `e2e_advanced_edge_cases_test.go` | 2 | 251 | 126 | 🟡 精査必要 |

**凡例**: 🟢 E2E維持推奨 / 🟡 要検討 / 🔴 ユニット化推奨

#### テストカテゴリ別分類

**カテゴリA: 基本ステージング機能**
- `TestBasicSetup` - セットアップ検証
- `TestSingleFileSingleHunk` - 1ファイル1ハンク
- `TestSingleFileMultipleHunks` - 1ファイル複数ハンク
- `TestMultipleFilesMultipleHunks` - 複数ファイル
- `TestWildcardStaging` - ワイルドカード機能
- `TestWildcardWithMixedInput` - ワイルドカード混在

**判定**: 🟡 `TestBasicSetup`は不要、他は1-2個のE2Eに統合可能

**カテゴリB: エラーハンドリング**
- `TestErrorCases_NonExistentFile` - 存在しないファイル
- `TestErrorCases_InvalidHunkNumber` - 無効なハンク番号
- `TestErrorCases_EmptyPatchFile` - 空のパッチ
- `TestErrorCases_HunkCountExceeded` - ハンク数超過
- `TestErrorCases_MultipleInvalidHunks` - 複数の無効ハンク
- `TestErrorCases_SameFileConflict` - ファイル競合

**判定**: 🔴 **全てユニットテスト化可能** - stager層で直接テスト

**カテゴリC: ファイル操作（特殊ケース）**
- `TestBinaryFileHandling` - バイナリファイル
- `TestFileModificationAndMove` - 変更と移動
- `TestGitMvThenModifyFile` - git mv後の変更
- `TestGitMvThenModifyFileWithoutCommit` - 未コミットmv
- `TestMultipleFilesMoveAndModify_Skip` - 複数ファイル移動

**判定**: 🟡 Unit: `internal/stager/special_files_test.go`と重複あり - 要整理

**カテゴリD: カウント機能**
- `TestE2E_CountHunks_NoChanges` - 変更なし
- `TestE2E_CountHunks_BasicIntegration` - 基本統合
- `TestE2E_CountHunks_BinaryFiles` - バイナリファイル

**判定**: 🔴 Unit: `internal/stager/count_hunks_test.go`（6テスト）と完全重複

**カテゴリE: 統合・パフォーマンス**
- `TestMixedSemanticChanges` - セマンティック分割（最重要）
- `TestE2E_FinalIntegration` - 全機能統合
- `TestLargeFileWithManyHunks` - 大規模ファイル
- `TestE2E_PerformanceWithSafetyChecks` - 安全性込みパフォーマンス
- `TestIntentToAddFileCoexistence` - intent-to-add共存
- `TestUntrackedFile` - 未追跡ファイル

**判定**: 🟢 これらは真のE2Eとして維持

### 1.2 ユニットテストの分析

#### 既存ユニットテストの強み

- **internal/stager/semantic_commit_test.go**: 7テスト（685行）
  - セマンティックコミットのワークフロー検証
  - **重要**: 既にgitリポジトリを作成してテストしている
  - E2Eとの境界が曖昧

- **internal/stager/count_hunks_test.go**: 6テスト（175行）
  - pure function `CountHunksInDiff`のテスト
  - E2Eの`e2e_count_hunks_test.go`と機能重複

- **internal/stager/special_files_test.go**: 2テスト（250行）
  - バイナリ、リネーム、削除ファイルの処理
  - E2Eの`e2e_advanced_files_test.go`と部分的に重複

- **internal/stager/safety_checker_test.go**: 11テスト（379行）
  - ステージングエリア検証ロジック
  - 十分な網羅性

#### ユニットテストの不足領域

1. **エラーハンドリング**: StagerError型の網羅的テストが不足
2. **ハンク抽出ロジック**: 個別ハンクの抽出テストが不足
3. **パッチID計算**: パッチIDベースのマッチングのユニットテスト不足

### 1.3 テストの重複マトリクス

| 機能 | E2Eテスト | ユニットテスト | 重複度 | 推奨アクション |
|-----|----------|---------------|--------|---------------|
| カウント機能 | 3 | 6 | 🔴 高 | E2E削除 |
| バイナリファイル | 2 | 複数 | 🟡 中 | E2E削減 |
| エラーハンドリング | 6 | 4 | 🟡 中 | E2Eをユニット化 |
| セマンティック分割 | 1 | 7 | 🟢 低（相補的） | 維持 |
| 基本ステージング | 6 | 複数 | 🟡 中 | E2E統合 |

---

## 2. 改修計画

### 2.1 フェーズ概要

改修を4つのフェーズに分けて実施します：

| フェーズ | 内容 | 期間 | リスク |
|---------|------|------|--------|
| Phase 1 | 重複排除（カウント機能） | 2時間 | 低 |
| Phase 2 | エラーハンドリングのユニット化 | 4時間 | 低 |
| Phase 3 | E2Eテストの統合・削減 | 6時間 | 中 |
| Phase 4 | 統合テストの最適化 | 4時間 | 中 |

**合計工数**: 16時間（2営業日）

### 2.2 Phase 1: カウント機能の重複排除

#### 目的
- E2Eの`e2e_count_hunks_test.go`（3テスト）を削除
- ユニットテスト`internal/stager/count_hunks_test.go`（6テスト）を強化
- CLIインターフェーステストを`main_test.go`に追加

#### 具体的作業

**ステップ1.1: ユニットテストの網羅性確認**

`internal/stager/count_hunks_test.go`の既存テスト：
- ✅ `TestCountHunksInDiff_NoChanges` - 空のdiff
- ✅ `TestCountHunksInDiff_SingleFileMultipleHunks` - 複数ハンク
- ✅ `TestCountHunksInDiff_MultipleFiles` - 複数ファイル
- ✅ `TestCountHunksInDiff_BinaryFile` - バイナリファイル
- ✅ `TestCountHunksInDiff_RenamedFile` - リネームファイル
- ✅ `TestCountHunksInDiff_InvalidDiff` - 無効なdiff

**追加が必要なテストケース**: なし（既に網羅的）

**ステップ1.2: CLIインターフェーステストの追加**

`main_test.go`に追加するテスト：

```go
// TestCLI_CountHunksSubcommand tests the count-hunks subcommand CLI interface
func TestCLI_CountHunksSubcommand(t *testing.T) {
    tests := []struct {
        name           string
        setupRepo      func(t *testing.T, repo *testutils.TestRepo)
        expectedOutput map[string]string // file -> hunk count
        expectError    bool
    }{
        {
            name: "no changes",
            setupRepo: func(t *testing.T, repo *testutils.TestRepo) {
                repo.CreateFile("test.txt", "content")
                repo.CommitChanges("Initial commit")
            },
            expectedOutput: map[string]string{},
            expectError:    false,
        },
        {
            name: "single file with changes",
            setupRepo: func(t *testing.T, repo *testutils.TestRepo) {
                repo.CreateFile("test.txt", "line1\nline2\n")
                repo.CommitChanges("Initial commit")
                repo.ModifyFile("test.txt", "line1\nmodified\nline2\n")
            },
            expectedOutput: map[string]string{"test.txt": "1"},
            expectError:    false,
        },
        {
            name: "binary file",
            setupRepo: func(t *testing.T, repo *testutils.TestRepo) {
                repo.CreateFile("test.txt", "content")
                repo.CommitChanges("Initial commit")
                repo.CreateBinaryFile("image.png", []byte{0x89, 0x50, 0x4E, 0x47})
            },
            expectedOutput: map[string]string{"image.png": "*"},
            expectError:    false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            repo := testutils.NewTestRepo(t, "count-hunks-test-*")
            defer repo.Cleanup()
            defer repo.Chdir()()

            tt.setupRepo(t, repo)

            // Execute count-hunks subcommand
            output, err := runCountHunksCommand()

            if tt.expectError {
                require.Error(t, err)
                return
            }

            require.NoError(t, err)

            // Parse and verify output
            result := parseCountHunksOutput(output)
            assert.Equal(t, tt.expectedOutput, result)
        })
    }
}
```

**ステップ1.3: E2Eテストの削除**

削除対象ファイル: `e2e_count_hunks_test.go`（286行）

削除前のチェックリスト：
- [ ] ユニットテストが全シナリオをカバーしているか確認
- [ ] CLIインターフェーステストを追加
- [ ] `go test ./...`でユニットテストがパス
- [ ] `go test -v -run=TestCLI_CountHunksSubcommand ./...`でCLIテストがパス
- [ ] `e2e_count_hunks_test.go`を削除
- [ ] 全テストを再実行して問題ないことを確認

**期待される結果**：
- E2Eテスト: 26 → 23（-3）
- テスト実行時間: ~0.12秒削減（3テスト × 0.04秒）
- 保守性: ユニットテストは環境依存性が低く、デバッグ容易

---

### 2.3 Phase 2: エラーハンドリングのユニット化

#### 目的
- E2Eの`e2e_error_test.go`（6テスト、364行）をユニットテスト化
- `internal/stager/`に`error_handling_test.go`を新規作成
- CLIレイヤーのエラー伝播を`main_test.go`で最小限テスト

#### 具体的作業

**ステップ2.1: エラーシナリオの分析**

`e2e_error_test.go`の各テストが検証している内容：

| テスト名 | 検証内容 | 必要なE2E性 | 移行先 |
|---------|---------|------------|--------|
| `TestErrorCases_NonExistentFile` | 存在しないファイル指定 | 🔴 低 | ユニット |
| `TestErrorCases_InvalidHunkNumber` | 無効なハンク番号 | 🔴 低 | ユニット |
| `TestErrorCases_EmptyPatchFile` | 空のパッチファイル | 🔴 低 | ユニット |
| `TestErrorCases_HunkCountExceeded` | ハンク数超過 | 🔴 低 | ユニット |
| `TestErrorCases_MultipleInvalidHunks` | 複数の無効ハンク | 🔴 低 | ユニット |
| `TestErrorCases_SameFileConflict` | ファイル競合 | 🔴 低 | ユニット |

**分析結果**: 全てstager層の関数を直接呼び出してテスト可能

**ステップ2.2: ユニットテストの作成**

新規ファイル: `internal/stager/error_handling_test.go`

```go
package stager

import (
    "strings"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/syou6162/git-sequential-stage/internal/executor"
)

// TestStageHunks_NonExistentFile tests error when specified file doesn't exist
func TestStageHunks_NonExistentFile(t *testing.T) {
    // Setup
    patch := `diff --git a/existing.py b/existing.py
index 1234567..abcdefg 100644
--- a/existing.py
+++ b/existing.py
@@ -1,1 +1,2 @@
 print('Hello, World!')
+print('Updated')
`

    mockExec := executor.NewMockExecutor()
    mockExec.SetFileExists("existing.py", true)

    // Execute: try to stage non-existent file
    err := StageHunks(
        []string{"non_existent.py:1"},
        patch,
        mockExec,
    )

    // Assert
    require.Error(t, err)

    var stagerErr *StagerError
    require.ErrorAs(t, err, &stagerErr)
    assert.Equal(t, ErrorTypeFileNotFound, stagerErr.Type)
    assert.Contains(t, strings.ToLower(err.Error()), "not found")

    // Verify no git operations were attempted
    assert.Equal(t, 0, mockExec.CallCount("git apply"))
}

// TestStageHunks_InvalidHunkNumber tests error when hunk number doesn't exist
func TestStageHunks_InvalidHunkNumber(t *testing.T) {
    patch := `diff --git a/test.py b/test.py
index 1234567..abcdefg 100644
--- a/test.py
+++ b/test.py
@@ -1,1 +1,2 @@
 print('Hello')
+print('World')
`

    mockExec := executor.NewMockExecutor()
    mockExec.SetFileExists("test.py", true)

    // Execute: request hunk 5 when only 1 hunk exists
    err := StageHunks(
        []string{"test.py:5"},
        patch,
        mockExec,
    )

    // Assert
    require.Error(t, err)

    var stagerErr *StagerError
    require.ErrorAs(t, err, &stagerErr)
    assert.Equal(t, ErrorTypeHunkNotFound, stagerErr.Type)
    assert.Contains(t, err.Error(), "hunk 5")
    assert.Contains(t, err.Error(), "only 1 hunk")
}

// TestStageHunks_EmptyPatch tests error handling for empty patch
func TestStageHunks_EmptyPatch(t *testing.T) {
    mockExec := executor.NewMockExecutor()

    err := StageHunks(
        []string{"test.py:1"},
        "", // empty patch
        mockExec,
    )

    require.Error(t, err)

    var stagerErr *StagerError
    require.ErrorAs(t, err, &stagerErr)
    assert.Equal(t, ErrorTypeParsing, stagerErr.Type)
}

// TestStageHunks_HunkCountExceeded tests multiple invalid hunks
func TestStageHunks_HunkCountExceeded(t *testing.T) {
    patch := `diff --git a/calc.go b/calc.go
index 1234567..abcdefg 100644
--- a/calc.go
+++ b/calc.go
@@ -1,2 +1,3 @@
 func add() {
+    return 0
 }
@@ -5,2 +6,3 @@
 func multiply() {
+    return 1
 }
`

    mockExec := executor.NewMockExecutor()
    mockExec.SetFileExists("calc.go", true)

    // Request hunks 1, 2, 3 when only 2 hunks exist
    err := StageHunks(
        []string{"calc.go:1,2,3"},
        patch,
        mockExec,
    )

    require.Error(t, err)
    assert.Contains(t, err.Error(), "hunk 3")
}

// TestStageHunks_MultipleInvalidHunks tests multiple files with errors
func TestStageHunks_MultipleInvalidHunks(t *testing.T) {
    patch := `diff --git a/file1.go b/file1.go
index 1234567..abcdefg 100644
--- a/file1.go
+++ b/file1.go
@@ -1,1 +1,2 @@
 package main
+import "fmt"
`

    mockExec := executor.NewMockExecutor()
    mockExec.SetFileExists("file1.go", true)

    // Request non-existent file and invalid hunk
    err := StageHunks(
        []string{"file1.go:5", "non_existent.go:1"},
        patch,
        mockExec,
    )

    require.Error(t, err)
    // First error should be reported
}

// TestStageHunks_ConflictingRequests tests same file requested multiple times
func TestStageHunks_ConflictingRequests(t *testing.T) {
    patch := `diff --git a/test.go b/test.go
index 1234567..abcdefg 100644
--- a/test.go
+++ b/test.go
@@ -1,1 +1,2 @@
 package main
+import "fmt"
@@ -5,1 +6,2 @@
 func main() {
+    fmt.Println("hello")
}
`

    mockExec := executor.NewMockExecutor()
    mockExec.SetFileExists("test.go", true)

    // Request same file with both specific hunks and wildcard
    err := StageHunks(
        []string{"test.go:1", "test.go:*"},
        patch,
        mockExec,
    )

    // This should be caught at validation layer
    require.Error(t, err)
    assert.Contains(t, strings.ToLower(err.Error()), "conflict")
}
```

**ステップ2.3: モックExecutorの拡張**

`internal/executor/executor_test.go`のMockExecutorに追加が必要な機能：

```go
type MockExecutor struct {
    calls       []MockCall
    fileExists  map[string]bool  // NEW: track which files exist
    responses   map[string]MockResponse
}

func (m *MockExecutor) SetFileExists(path string, exists bool) {
    if m.fileExists == nil {
        m.fileExists = make(map[string]bool)
    }
    m.fileExists[path] = exists
}

func (m *MockExecutor) CallCount(command string) int {
    count := 0
    for _, call := range m.calls {
        if strings.HasPrefix(call.Command, command) {
            count++
        }
    }
    return count
}

func (m *MockExecutor) GetAppliedPatchContent() string {
    for _, call := range m.calls {
        if strings.Contains(call.Command, "git apply") {
            // Extract patch content from stdin
            return call.Stdin
        }
    }
    return ""
}
```

**ステップ2.4: CLIレイヤーのエラー伝播テスト**

`main_test.go`に追加：

```go
// TestCLI_StageSubcommand_ErrorHandling tests error propagation from stager to CLI
func TestCLI_StageSubcommand_ErrorHandling(t *testing.T) {
    tests := []struct {
        name          string
        hunkArgs      []string
        expectError   bool
        errorContains string
    }{
        {
            name:          "non-existent file",
            hunkArgs:      []string{"non_existent.py:1"},
            expectError:   true,
            errorContains: "not found",
        },
        {
            name:          "invalid hunk number",
            hunkArgs:      []string{"test.py:999"},
            expectError:   true,
            errorContains: "hunk",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            repo := testutils.NewTestRepo(t, "cli-error-test-*")
            defer repo.Cleanup()
            defer repo.Chdir()()

            // Setup minimal repo
            repo.CreateFile("test.py", "print('hello')")
            repo.CommitChanges("Initial")
            repo.ModifyFile("test.py", "print('hello')\nprint('world')")

            patchPath := "changes.patch"
            repo.RunCommand("sh", "-c", "git diff > "+patchPath)

            // Execute
            err := runGitSequentialStage(tt.hunkArgs, patchPath)

            // Assert
            if tt.expectError {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.errorContains)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

**ステップ2.5: E2Eテストファイルの削除**

削除対象: `e2e_error_test.go`（364行）

削除前チェックリスト：
- [ ] 全エラーシナリオがユニットテストでカバーされている
- [ ] CLIレイヤーのエラー伝播テストを追加
- [ ] MockExecutorが必要な機能を持っている
- [ ] `go test internal/stager/...`で全テストパス
- [ ] `go test -v -run=TestCLI_StageSubcommand_ErrorHandling`でパス
- [ ] `e2e_error_test.go`を削除
- [ ] 全テスト再実行

**期待される結果**：
- E2Eテスト: 23 → 17（-6）
- テスト実行時間: ~0.42秒削減（6テスト × 0.07秒）
- デバッグ容易性: 大幅向上（モックで瞬時に再現可能）

---

### 2.4 Phase 3: E2Eテストの統合・削減

#### 目的
- 基本ステージング機能のE2Eテストを統合（6 → 2テスト）
- ファイル操作系テストの重複排除
- パフォーマンステストの統合

#### ステップ3.1: 基本ステージングテストの統合

**現状**: `e2e_basic_test.go`に6テスト

統合プラン：

1. **削除**: `TestBasicSetup`
   - 理由: 他のテストで暗黙的に検証済み

2. **統合**: 残り5テストを2つに統合
   - `TestBasicStaging_HappyPath`: 成功パスの基本検証
   - `TestBasicStaging_WildcardFeature`: ワイルドカード機能検証

**新しいテスト構造**:

```go
// TestBasicStaging_HappyPath tests fundamental staging functionality
// Covers: single hunk, multiple hunks, multiple files
func TestBasicStaging_HappyPath(t *testing.T) {
    testRepo := testutils.NewTestRepo(t, "basic-staging-*")
    defer testRepo.Cleanup()
    defer testRepo.Chdir()()

    // Create initial files
    testRepo.CreateFile("file1.py", initialContent1)
    testRepo.CreateFile("file2.go", initialContent2)
    testRepo.CommitChanges("Initial commit")

    // Modify both files (file1: 2 hunks, file2: 1 hunk)
    testRepo.ModifyFile("file1.py", modifiedContent1)
    testRepo.ModifyFile("file2.go", modifiedContent2)

    patchPath := testRepo.CreatePatch()

    // Scenario 1: Stage single hunk from file1
    t.Run("single_hunk", func(t *testing.T) {
        err := runGitSequentialStage([]string{"file1.py:1"}, patchPath)
        require.NoError(t, err)

        staged := testRepo.GetStagedDiff()
        assert.Contains(t, staged, expectedChange1)
        assert.NotContains(t, staged, unexpectedChange2)
    })

    // Scenario 2: Stage multiple hunks from same file
    t.Run("multiple_hunks", func(t *testing.T) {
        testRepo.ResetStaging()

        err := runGitSequentialStage([]string{"file1.py:1,2"}, patchPath)
        require.NoError(t, err)

        staged := testRepo.GetStagedDiff()
        assert.Contains(t, staged, expectedChange1)
        assert.Contains(t, staged, expectedChange2)
    })

    // Scenario 3: Stage from multiple files
    t.Run("multiple_files", func(t *testing.T) {
        testRepo.ResetStaging()

        err := runGitSequentialStage(
            []string{"file1.py:1", "file2.go:1"},
            patchPath,
        )
        require.NoError(t, err)

        staged := testRepo.GetStagedDiff()
        assert.Contains(t, staged, "file1.py")
        assert.Contains(t, staged, "file2.go")
    })
}

// TestBasicStaging_WildcardFeature tests wildcard staging functionality
func TestBasicStaging_WildcardFeature(t *testing.T) {
    testRepo := testutils.NewTestRepo(t, "wildcard-test-*")
    defer testRepo.Cleanup()
    defer testRepo.Chdir()()

    // Setup
    testRepo.CreateFile("logger.go", loggerContent)
    testRepo.CreateFile("config.yaml", configContent)
    testRepo.CommitChanges("Initial")

    testRepo.ModifyFile("logger.go", loggerModified)
    testRepo.ModifyFile("config.yaml", configModified)

    patchPath := testRepo.CreatePatch()

    // Scenario 1: Wildcard for entire file
    t.Run("wildcard_entire_file", func(t *testing.T) {
        err := runGitSequentialStage([]string{"config.yaml:*"}, patchPath)
        require.NoError(t, err)

        staged := testRepo.GetStagedFiles()
        assert.Contains(t, staged, "config.yaml")
        assert.NotContains(t, staged, "logger.go")
    })

    // Scenario 2: Mix wildcard and specific hunks
    t.Run("mixed_wildcard_specific", func(t *testing.T) {
        testRepo.ResetStaging()

        err := runGitSequentialStage(
            []string{"config.yaml:*", "logger.go:1"},
            patchPath,
        )
        require.NoError(t, err)

        staged := testRepo.GetStagedDiff()
        // config.yaml: all changes
        assert.Contains(t, staged, configExpectedChange1)
        assert.Contains(t, staged, configExpectedChange2)
        // logger.go: only hunk 1
        assert.Contains(t, staged, loggerExpectedChange1)
        assert.NotContains(t, staged, loggerExpectedChange2)
    })
}
```

削除対象：
- `TestBasicSetup`
- `TestSingleFileSingleHunk`
- `TestSingleFileMultipleHunks`
- `TestMultipleFilesMultipleHunks`
- `TestWildcardStaging`
- `TestWildcardWithMixedInput`

追加：
- `TestBasicStaging_HappyPath` (with 3 sub-scenarios)
- `TestBasicStaging_WildcardFeature` (with 2 sub-scenarios)

**削減効果**: 6テスト → 2テスト（-4）

#### ステップ3.2: ファイル操作テストの整理

**現状分析**:
- E2E: `e2e_advanced_files_test.go`（5テスト）
- Unit: `internal/stager/special_files_test.go`（2テスト）

重複している機能：
- バイナリファイル処理
- リネームファイル処理

**整理方針**:

1. **ユニットテストで十分なもの** → E2Eから削除
   - `TestBinaryFileHandling` → Unit: `TestStageHunks_BinaryFile`で既にカバー

2. **E2Eが必要なもの** → 維持（ただし簡素化）
   - `TestFileModificationAndMove` → git操作の統合が必要
   - `TestGitMvThenModifyFile` → 同上

**具体的アクション**:

```go
// internal/stager/special_files_test.go に追加

// TestStageHunks_BinaryFileWildcard tests binary file staging with wildcard
func TestStageHunks_BinaryFileWildcard(t *testing.T) {
    patch := `diff --git a/image.png b/image.png
index 1234567..abcdefg 100644
Binary files a/image.png and b/image.png differ
`

    mockExec := executor.NewMockExecutor()
    mockExec.SetFileExists("image.png", true)

    // Execute: stage binary file with wildcard
    err := StageHunks([]string{"image.png:*"}, patch, mockExec)
    require.NoError(t, err)

    // Verify: git add (not git apply) was called
    assert.Equal(t, 1, mockExec.CallCount("git add image.png"))
    assert.Equal(t, 0, mockExec.CallCount("git apply"))
}

// TestStageHunks_RenamedFileDetection tests renamed file detection
func TestStageHunks_RenamedFileDetection(t *testing.T) {
    patch := `diff --git a/old_name.go b/new_name.go
similarity index 100%
rename from old_name.go
rename to new_name.go
`

    mockExec := executor.NewMockExecutor()

    err := StageHunks([]string{"new_name.go:*"}, patch, mockExec)
    require.NoError(t, err)

    // Verify appropriate git command was used
    assert.Equal(t, 1, mockExec.CallCount("git add"))
}
```

E2E側の整理：
- `TestBinaryFileHandling` を削除（ユニットで十分）
- `TestFileModificationAndMove` を簡素化して維持
- `TestMultipleFilesMoveAndModify_Skip` を削除（skipされている）

**削減効果**: 5テスト → 2テスト（-3）

#### ステップ3.3: パフォーマンステストの統合

**現状**:
- `e2e_performance_test.go`: 1テスト（100行）
- `e2e_advanced_performance_test.go`: 1テスト（222行）

**統合プラン**:

2つのパフォーマンステストを1つに統合：

```go
// e2e_performance_test.go を更新

// TestPerformance_LargeFileStaging tests performance with large files
func TestPerformance_LargeFileStaging(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping performance test in short mode")
    }

    testRepo := testutils.NewTestRepo(t, "performance-test-*")
    defer testRepo.Cleanup()
    defer testRepo.Chdir()()

    // Generate large file with many hunks (20 functions, 12 hunks)
    largeFile := generateLargeGoFile(20, 12)
    testRepo.CreateFile("large.go", largeFile.Original)
    testRepo.CommitChanges("Initial")
    testRepo.ModifyFile("large.go", largeFile.Modified)

    patchPath := testRepo.CreatePatch()

    // Test 1: Basic performance without safety checks
    t.Run("basic_staging", func(t *testing.T) {
        start := time.Now()

        err := runGitSequentialStage(
            []string{"large.go:1,2,3,4,5,6,7,8,9,10,11,12"},
            patchPath,
        )

        elapsed := time.Since(start)

        require.NoError(t, err)
        assert.Less(t, elapsed, 5*time.Second, "Should complete within 5 seconds")
        t.Logf("Staging 12 hunks took: %v", elapsed)
    })

    // Test 2: Performance with safety checks enabled (default)
    t.Run("with_safety_checks", func(t *testing.T) {
        testRepo.ResetStaging()

        start := time.Now()

        err := runGitSequentialStage(
            []string{"large.go:1,2,3,4,5,6"},
            patchPath,
        )

        elapsed := time.Since(start)

        require.NoError(t, err)
        assert.Less(t, elapsed, 500*time.Millisecond,
            "Should complete within 500ms even with safety checks")
        t.Logf("Staging with safety checks took: %v", elapsed)
    })
}
```

削除対象:
- `e2e_advanced_performance_test.go` を削除
- `e2e_performance_test.go` を上記内容で置き換え

**削減効果**: 2ファイル → 1ファイル（統合）

---

### 2.5 Phase 4: 統合テストの最適化

#### 目的
- E2Eとして維持すべきテストを最適化
- セマンティックコミット分割テストの強化
- ドキュメンタリーバリューの向上

#### ステップ4.1: セマンティックコミットテストの二層化

**現状**:
- E2E: `TestMixedSemanticChanges`（287行） - 実際のgitワークフロー
- Unit: `internal/stager/semantic_commit_test.go`（7テスト） - ロジックテスト

**改善プラン**:

1. **ユニットテストを追加**: ハンク分離ロジックの純粋検証

```go
// internal/stager/hunk_separation_test.go (新規)

// TestHunkSeparation_SemanticChanges tests semantic hunk separation logic
func TestHunkSeparation_SemanticChanges(t *testing.T) {
    // Load test patch with mixed semantic changes
    patch := loadTestData("testdata/mixed_semantic_changes.patch")

    mockExec := executor.NewMockExecutor()
    mockExec.SetFileExists("web_server.py", true)

    tests := []struct {
        name          string
        hunkSpec      string
        shouldContain []string
        shouldNotContain []string
    }{
        {
            name:     "logging_infrastructure",
            hunkSpec: "web_server.py:1",
            shouldContain: []string{
                "+import logging",
                "+logging.basicConfig",
                "+logger = logging.getLogger",
                "+    logger.info(\"Fetching users",
            },
            shouldNotContain: []string{
                "+    # Add input validation",
                "+    if not data or not data.get(\"name\")",
                "+        return jsonify({\"error\":",
                "+    return {\"status\": \"ok\", \"timestamp\":",
                "+    port = int(os.environ.get('PORT'",
            },
        },
        {
            name:     "input_validation",
            hunkSpec: "web_server.py:2",
            shouldContain: []string{
                "+    # Add input validation",
                "+    if not data or not data.get(\"name\")",
                "+        return jsonify({\"error\":",
            },
            shouldNotContain: []string{
                "+import logging",
                "+    return {\"status\": \"ok\", \"timestamp\":",
            },
        },
        {
            name:     "config_improvements",
            hunkSpec: "web_server.py:3",
            shouldContain: []string{
                "+    return {\"status\": \"ok\", \"timestamp\":",
                "+    port = int(os.environ.get('PORT'",
            },
            shouldNotContain: []string{
                "+import logging",
                "+    # Add input validation",
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Stage specific hunk
            err := StageHunks([]string{tt.hunkSpec}, patch, mockExec)
            require.NoError(t, err)

            // Get applied patch content
            applied := mockExec.GetLastAppliedPatch()

            // Verify expected changes are present
            for _, expected := range tt.shouldContain {
                assert.Contains(t, applied, expected,
                    "Expected change not found in hunk %s", tt.hunkSpec)
            }

            // Verify unexpected changes are absent
            for _, unexpected := range tt.shouldNotContain {
                assert.NotContains(t, applied, unexpected,
                    "Unexpected change found in hunk %s", tt.hunkSpec)
            }
        })
    }
}
```

2. **E2Eテストを簡素化**: ワークフローの動作確認に集中

```go
// e2e_semantic_test.go を簡素化

// TestMixedSemanticChanges tests semantic commit splitting workflow
// This E2E test verifies the complete workflow of splitting a complex change
// into multiple semantic commits using real git operations
func TestMixedSemanticChanges(t *testing.T) {
    testRepo := testutils.NewTestRepo(t, "semantic-test-*")
    defer testRepo.Cleanup()
    defer testRepo.Chdir()()

    // Setup: Create initial file
    testRepo.CreateFile("server.py", initialWebServerCode)
    testRepo.CommitChanges("Initial commit")

    // Make mixed semantic changes (logging + validation)
    testRepo.ModifyFile("server.py", modifiedWebServerCode)

    patchPath := testRepo.CreatePatch()

    // Workflow Step 1: Stage and commit logging feature
    err := runGitSequentialStage([]string{"server.py:1"}, patchPath)
    require.NoError(t, err)

    testRepo.CommitChanges("feat: add logging infrastructure")

    // Workflow Step 2: Stage and commit validation feature
    err = runGitSequentialStage([]string{"server.py:2"}, patchPath)
    require.NoError(t, err)

    testRepo.CommitChanges("feat: add input validation")

    // Verify: All changes committed, working dir clean
    assert.Empty(t, testRepo.GetWorkingDiff())

    // Verify: Two semantic commits created
    commits := testRepo.GetRecentCommits(3)
    assert.Len(t, commits, 3) // initial + 2 feature commits
    assert.Contains(t, commits[0].Message, "logging")
    assert.Contains(t, commits[1].Message, "validation")
}
```

**改善ポイント**:
- ユニットテストでハンク分離ロジックの正確性を検証
- E2Eテストではワークフロー全体の動作のみを確認
- E2Eは287行 → 約80行に削減（testdataファイル利用）

#### ステップ4.2: testdata/ ディレクトリの活用

テストデータを外部ファイル化：

```
git-sequential-stage/
├── internal/
│   └── stager/
│       ├── testdata/
│       │   ├── mixed_semantic_changes.patch
│       │   ├── large_file_original.go
│       │   ├── large_file_modified.go
│       │   ├── binary_file_sample.png
│       │   └── renamed_file.patch
│       ├── hunk_separation_test.go
│       └── ...
```

メリット：
- テストコードが簡潔に
- テストデータの再利用が容易
- 実際のパッチ例がドキュメントとして機能

#### ステップ4.3: 最終的なE2Eテスト構成

改修後に残すE2Eテスト（計8テスト）：

| テスト名 | 目的 | ファイル | 行数 |
|---------|------|---------|------|
| `TestBasicStaging_HappyPath` | 基本ステージング | e2e_basic_test.go | ~80 |
| `TestBasicStaging_WildcardFeature` | ワイルドカード | e2e_basic_test.go | ~60 |
| `TestMixedSemanticChanges` | セマンティック分割 | e2e_semantic_test.go | ~80 |
| `TestE2E_FinalIntegration` | 全機能統合 | e2e_integration_test.go | ~390 |
| `TestPerformance_LargeFileStaging` | パフォーマンス | e2e_performance_test.go | ~100 |
| `TestFileModificationAndMove` | ファイル操作 | e2e_advanced_files_test.go | ~150 |
| `TestIntentToAddFileCoexistence` | intent-to-add | e2e_advanced_edge_cases_test.go | ~120 |
| `TestUntrackedFile` | 未追跡ファイル | e2e_advanced_edge_cases_test.go | ~100 |

**合計**: 8テスト、約1,080行（現状3,290行から67%削減）

---

## 3. リスク管理

### 3.1 リスク評価マトリクス

| リスク | 影響度 | 発生確率 | 対策 |
|-------|-------|---------|------|
| テストカバレッジ低下 | 高 | 低 | Phase毎にカバレッジ測定 |
| モック不完全 | 中 | 中 | MockExecutorの段階的拡張 |
| 既存バグの見逃し | 高 | 低 | 全テスト並行実行期間を設ける |
| 互換性問題 | 中 | 低 | 既存テスト削除前に新テストを追加 |
| パフォーマンス劣化 | 低 | 低 | ベンチマークで継続監視 |

### 3.2 ロールバック計画

各Phase終了時点でgit tagを作成：

```bash
# Phase 1完了時
git tag refactor-phase1-complete
git push origin refactor-phase1-complete

# 問題発生時
git reset --hard refactor-phase1-complete
```

### 3.3 テストカバレッジ監視

Phase毎にカバレッジを測定：

```bash
# 現状のカバレッジ測定
go test -coverprofile=coverage_before.out ./...
go tool cover -func=coverage_before.out

# Phase完了後
go test -coverprofile=coverage_phase1.out ./...
go tool cover -func=coverage_phase1.out

# 比較
diff <(go tool cover -func=coverage_before.out) \
     <(go tool cover -func=coverage_phase1.out)
```

**許容範囲**: カバレッジ低下は±2%以内

---

## 4. 実装ガイドライン

### 4.1 テスト命名規則

**ユニットテスト**:
```
Test<Function>_<Scenario>

例:
- TestStageHunks_NonExistentFile
- TestCountHunksInDiff_BinaryFile
- TestParseHunkInfo_InvalidFormat
```

**E2Eテスト**:
```
Test<Feature>_<Scenario>

例:
- TestBasicStaging_HappyPath
- TestSemanticCommit_MultipleFiles
```

**統合テスト（main_test.go）**:
```
TestCLI_<Subcommand>_<Scenario>

例:
- TestCLI_StageSubcommand_ErrorHandling
- TestCLI_CountHunksSubcommand_BinaryFiles
```

### 4.2 テストデータ管理

**原則**:
1. 小さなテストデータはテスト内にインライン記述
2. 100行以上のデータは`testdata/`ディレクトリに配置
3. テストデータファイル名は`<feature>_<scenario>.ext`形式

**例**:
```
internal/stager/testdata/
├── mixed_semantic_changes.patch          # セマンティック分割テスト用
├── large_file_20_functions.go            # パフォーマンステスト用
├── binary_file_handling.patch            # バイナリファイルテスト用
└── renamed_file_with_changes.patch       # リネームテスト用
```

### 4.3 MockExecutorの設計方針

**必須機能**:
1. コマンド実行履歴の記録
2. ファイル存在チェックのシミュレーション
3. git apply/add/diff等の出力モック
4. エラー注入機能（エラーケーステスト用）

**実装例**:
```go
type MockExecutor struct {
    calls       []MockCall
    fileExists  map[string]bool
    responses   map[string]MockResponse
    errorInject map[string]error  // コマンドに対するエラー注入
}

// エラー注入機能
func (m *MockExecutor) InjectError(command string, err error) {
    if m.errorInject == nil {
        m.errorInject = make(map[string]error)
    }
    m.errorInject[command] = err
}

// 使用例（テストで）
mockExec.InjectError("git apply", errors.New("patch does not apply"))
```

### 4.4 アサーション戦略

**ユニットテスト**:
- 具体的な値を検証（`assert.Equal`）
- エラー型を検証（`assert.ErrorAs`）
- モックの呼び出し回数を検証

**E2Eテスト**:
- ワークフロー全体の結果を検証
- ファイルシステム状態を検証
- gitコミット履歴を検証

**例**:
```go
// Unit: 具体的
assert.Equal(t, ErrorTypeFileNotFound, err.Type)
assert.Equal(t, 1, mockExec.CallCount("git apply"))

// E2E: 全体的
assert.Empty(t, testRepo.GetWorkingDiff())
assert.Len(t, testRepo.GetCommits(), 3)
```

---

## 5. 成功基準

### 5.1 定量的指標

| 指標 | 現状 | 目標 | 測定方法 |
|-----|------|------|---------|
| E2Eテスト数 | 26 | ≤ 10 | `grep -c "^func Test" e2e_*.go` |
| E2E総行数 | 3,290 | ≤ 1,200 | `wc -l e2e_*.go` |
| ユニットテスト数 | 75 | ≥ 90 | `find internal -name "*_test.go" -exec grep "^func Test" {} \; \| wc -l` |
| テスト実行時間 | ~5秒 | ≤ 2秒 | `time go test ./...` |
| テストカバレッジ | 測定 | ±2% | `go test -cover ./...` |

### 5.2 定性的指標

| 指標 | 評価方法 |
|-----|---------|
| デバッグ容易性 | 新規メンバーがテスト失敗を5分以内に理解できるか |
| テスト可読性 | テストが仕様ドキュメントとして機能するか |
| 保守性 | 新機能追加時にテスト追加が5分以内に完了するか |
| 信頼性 | テストがflakyでないか（100回実行で全てパス） |

### 5.3 検収基準

**Phase 1完了基準**:
- [ ] `e2e_count_hunks_test.go`削除完了
- [ ] ユニットテストが全シナリオをカバー
- [ ] CLIテストが`main_test.go`に追加
- [ ] 全テストがパス（`go test ./...`）
- [ ] カバレッジが維持されている

**Phase 2完了基準**:
- [ ] `e2e_error_test.go`削除完了
- [ ] `internal/stager/error_handling_test.go`作成
- [ ] MockExecutorが必要機能を実装
- [ ] 全テストがパス
- [ ] エラーケースが網羅的にテストされている

**Phase 3完了基準**:
- [ ] 基本ステージングテストが6→2に削減
- [ ] ファイル操作テストが5→2に削減
- [ ] パフォーマンステストが統合
- [ ] 全テストがパス
- [ ] テスト実行時間が30%以上短縮

**Phase 4完了基準**:
- [ ] セマンティックテストが二層化
- [ ] testdataディレクトリ作成・活用
- [ ] E2Eテストが最終的に8個以下
- [ ] 全テストがパス
- [ ] ドキュメント更新（CLAUDE.md）

**最終検収基準**:
- [ ] 全Phase完了
- [ ] 定量的指標が全て達成
- [ ] 既存機能が全て動作（リグレッションなし）
- [ ] テストカバレッジが維持（±2%以内）
- [ ] CI/CDが正常動作
- [ ] ドキュメント更新完了

---

## 6. タイムライン

### 6.1 実装スケジュール

```
Week 1:
├─ Day 1 (4h)
│  ├─ Phase 1: カウント機能の重複排除 (2h)
│  │  ├─ ユニットテスト確認 (30m)
│  │  ├─ CLIテスト追加 (1h)
│  │  └─ E2E削除・検証 (30m)
│  └─ Phase 2: エラーハンドリングのユニット化 (2h)
│     ├─ error_handling_test.go作成 (1h)
│     ├─ MockExecutor拡張 (30m)
│     └─ E2E削除・検証 (30m)
│
└─ Day 2 (4h)
   ├─ Phase 2続き: エラーハンドリング (2h)
   │  └─ CLIエラーテスト追加・検証 (2h)
   └─ Phase 3: E2Eテストの統合 (2h)
      ├─ 基本ステージングテスト統合 (1h)
      └─ ファイル操作テスト整理 (1h)

Week 2:
├─ Day 3 (4h)
│  ├─ Phase 3続き (2h)
│  │  └─ パフォーマンステスト統合 (2h)
│  └─ Phase 4: 統合テスト最適化 (2h)
│     └─ セマンティックテスト二層化開始 (2h)
│
└─ Day 4 (4h)
   ├─ Phase 4続き (3h)
   │  ├─ testdataディレクトリ整備 (1h)
   │  ├─ E2Eテスト簡素化 (1h)
   │  └─ ドキュメント更新 (1h)
   └─ 最終検証・調整 (1h)
      ├─ 全テスト実行・確認 (30m)
      └─ カバレッジ測定・レポート (30m)
```

### 6.2 マイルストーン

| 日付 | マイルストーン | 成果物 |
|------|--------------|--------|
| Day 1 | Phase 1-2完了 | E2E 9個削減、error_handling_test.go作成 |
| Day 2 | Phase 3開始 | 基本テスト統合、E2E 4個削減 |
| Day 3 | Phase 3完了 | パフォーマンステスト統合 |
| Day 4 | Phase 4完了・検収 | testdata整備、ドキュメント完成 |

---

## 7. 保守・運用

### 7.1 新規テスト追加ガイドライン

**フローチャート**:
```
新しい機能を追加する
    ↓
その機能は複数のコンポーネントを統合するか？
    ↓ Yes → E2Eテストを追加（e2e_*.go）
    ↓ No
単一の関数/メソッドの動作検証か？
    ↓ Yes → ユニットテストを追加（internal/*/`*_test.go）
    ↓ No
CLIインターフェースの検証か？
    ↓ Yes → 統合テストを追加（main_test.go）
```

**判断基準**:
1. **ユニットテスト**:
   - 単一関数の入出力検証
   - エラーハンドリング
   - エッジケース

2. **統合テスト（main_test.go）**:
   - CLIフラグ解析
   - サブコマンドルーティング
   - エラーメッセージ表示

3. **E2Eテスト**:
   - 実際のgitリポジトリでのワークフロー
   - 複数のgit操作の連携
   - セマンティックコミット分割などの高レベルシナリオ

### 7.2 継続的改善

**四半期レビュー項目**:
1. テスト実行時間の推移
2. テストカバレッジの推移
3. E2E/Unit比率の維持
4. flakyテストの有無
5. テスト失敗からデバッグまでの平均時間

**改善アクション**:
- E2Eテストが10個を超えたら、ユニット化を検討
- テスト実行時間が2秒を超えたら、最適化を検討
- カバレッジが5%以上低下したら、原因調査

---

## 8. 参考資料

### 8.1 テストピラミッドの原則

**推奨比率**:
```
     ┌──────┐
     │ E2E  │  10%  ← 遅い、壊れやすい、デバッグ困難
     ├──────┴───┐
     │Integration│ 20%  ← 中速、中程度の安定性
     ├────────────┴───┐
     │     Unit       │ 70%  ← 高速、安定、デバッグ容易
     └────────────────┘
```

**出典**:
- Martin Fowler: "TestPyramid"
- Google Testing Blog: "Just Say No to More End-to-End Tests"

### 8.2 Go言語テストのベストプラクティス

- **Table-Driven Tests**: 同様のテストケースを効率的に記述
- **Subtests**: `t.Run()`でテストを構造化
- **testdata/**: テストデータの標準的な配置場所
- **Test Helpers**: `t.Helper()`でテストヘルパーを明示

**参考リンク**:
- https://go.dev/doc/effective_go#testing
- https://github.com/golang/go/wiki/TestComments

### 8.3 プロジェクト固有の考慮事項

**git-sequential-stageの特殊性**:
1. **gitコマンド依存**: 実際のgit操作が必要なテストは真のE2E
2. **パッチID計算**: `git patch-id`の動作検証には実環境が必要
3. **LLMエージェント統合**: intent-to-addワークフローは実gitで検証

**テスト戦略への影響**:
- gitコマンド不要な部分は積極的にユニット化
- gitコマンド必須な部分は最小限のE2Eテストで検証
- MockExecutorでgitコマンドをシミュレーション

---

## 9. 付録

### 9.1 チェックリスト

**Phase 1: カウント機能重複排除**
- [ ] ユニットテストの網羅性を確認（internal/stager/count_hunks_test.go）
- [ ] CLIテストをmain_test.goに追加
- [ ] e2e_count_hunks_test.goを削除
- [ ] go test ./... でパス確認
- [ ] カバレッジ測定（維持確認）
- [ ] git commit & tag: refactor-phase1

**Phase 2: エラーハンドリングユニット化**
- [ ] internal/stager/error_handling_test.go作成
- [ ] MockExecutor拡張（SetFileExists, CallCount等）
- [ ] 全6エラーケースをユニットテストでカバー
- [ ] CLIエラー伝播テストをmain_test.goに追加
- [ ] e2e_error_test.goを削除
- [ ] go test ./... でパス確認
- [ ] カバレッジ測定
- [ ] git commit & tag: refactor-phase2

**Phase 3: E2Eテスト統合**
- [ ] 基本ステージングテストを2つに統合（TestBasicStaging_HappyPath, TestBasicStaging_WildcardFeature）
- [ ] 旧テスト6個を削除
- [ ] ファイル操作テストの重複排除（binary file等）
- [ ] パフォーマンステストを1ファイルに統合
- [ ] go test ./... でパス確認
- [ ] 実行時間測定（30%短縮確認）
- [ ] git commit & tag: refactor-phase3

**Phase 4: 統合テスト最適化**
- [ ] internal/stager/hunk_separation_test.go作成
- [ ] testdata/ディレクトリ作成
- [ ] テストデータをtestdata/に移動
- [ ] e2e_semantic_test.go簡素化
- [ ] E2Eテスト最終数確認（≤10個）
- [ ] CLAUDE.md更新
- [ ] go test ./... でパス確認
- [ ] カバレッジ最終測定
- [ ] git commit & tag: refactor-phase4-complete

**最終検収**
- [ ] 全定量的指標達成確認
- [ ] 全定性的指標レビュー
- [ ] CI/CD正常動作確認
- [ ] リグレッションテスト（既存機能の動作確認）
- [ ] ドキュメント完全性確認
- [ ] プルリクエスト作成

### 9.2 用語集

| 用語 | 定義 |
|-----|------|
| E2Eテスト | End-to-Endテスト。実際の環境で全システムを通してテストする手法 |
| ユニットテスト | 単一の関数やメソッドを独立してテストする手法 |
| 統合テスト | 複数のコンポーネントの連携をテストする手法（E2Eより小規模） |
| テストピラミッド | ユニット:統合:E2E = 70:20:10 の理想的なテスト構成 |
| Mock | 実装を模倣したテスト用のダミーオブジェクト |
| Flaky Test | 実行毎に結果が変わる不安定なテスト |
| Test Coverage | コードのどれだけがテストでカバーされているかの指標 |
| パッチID | gitが計算するパッチの一意識別子（内容ベース） |
| intent-to-add | git add -Nで新規ファイルを追跡開始する機能 |
| セマンティックコミット | 意味のある単位で分割されたコミット |

### 9.3 連絡先・リソース

**プロジェクト関連**:
- リポジトリ: https://github.com/syou6162/git-sequential-stage
- Issue Tracker: GitHub Issues
- ドキュメント: `CLAUDE.md`

**参考リソース**:
- Go Testing: https://go.dev/doc/tutorial/add-a-test
- Test Pyramid: https://martinfowler.com/articles/practical-test-pyramid.html
- Table-Driven Tests in Go: https://go.dev/wiki/TableDrivenTests

---

## まとめ

本計画書は、git-sequential-stageプロジェクトのテスト構造を理想的なテストピラミッド形状に改修するための詳細なロードマップです。

**改修の核心**:
- E2Eテストの過剰な使用を是正（26 → 8テスト、-69%）
- ユニットテストの充実化（+15-20テスト）
- テスト実行速度の向上（~5秒 → ~2秒、-60%）
- デバッグ容易性とドキュメンタリーバリューの向上

**期待される成果**:
- 保守性の向上
- 開発速度の向上（テストが速い）
- 新規メンバーのオンボーディング容易化
- テストコードが仕様書として機能

4つのPhaseを2営業日（16時間）で完了し、リスクを最小限に抑えながら段階的に改善を進めます。

---

**文書承認**:
- [ ] 技術リード承認
- [ ] QAリード承認
- [ ] プロダクトオーナー承認

**改訂履歴**:
- v1.0 (2025-10-27): 初版作成
