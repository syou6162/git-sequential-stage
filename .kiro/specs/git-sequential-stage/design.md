# 設計書

## 概要

git-sequential-stageは、Gitパッチファイルから指定されたhunkを順次ステージングするCLIツールです。パッチID（`git patch-id`）を使用してhunkを一意に識別し、依存関係を正しく処理することで「hunk番号のドリフト問題」を解決します。

## アーキテクチャ

### 全体構成

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   CLI Layer     │    │  Business Logic  │    │  System Layer   │
│   (main.go)     │───▶│     Layer        │───▶│   (executor)    │
│                 │    │                  │    │                 │
│ - 引数解析      │    │ - stager         │    │ - git commands  │
│ - エラー処理    │    │ - validator      │    │                 │
│ - 使用方法表示  │    │ - hunk_info      │    │ - file I/O      │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

### レイヤー構成

1. **CLI Layer**: ユーザーインターフェース層
   - コマンドライン引数の解析
   - エラーメッセージの表示
   - 使用方法の表示

2. **Business Logic Layer**: ビジネスロジック層
   - パッチファイルの解析
   - Hunk情報の管理
   - 順次ステージングの制御
   - 引数検証

3. **System Layer**: システム層
   - 外部コマンドの実行
   - ファイルシステムアクセス
   - テスト用のモック機能

## コンポーネントと インターフェース

### 1. CommandExecutor インターフェース

外部コマンド実行の抽象化を提供します。

```go
type CommandExecutor interface {
    Execute(name string, args ...string) ([]byte, error)
    ExecuteWithStdin(name string, stdin io.Reader, args ...string) ([]byte, error)
}
```

**実装:**
- `RealCommandExecutor`: 実際のコマンド実行
- `MockCommandExecutor`: テスト用のモック実装

### 2. Stager コンポーネント

順次ステージングの核心ロジックを担当します。

**主要メソッド:**
- `StageHunks(hunkSpecs []string, patchFile string) error`
- `calculatePatchIDsForHunks()`: パッチID計算
- `extractHunkContent()`: Hunk内容抽出
- `buildTargetIDs()`: ターゲットID構築

**処理フロー:**
1. パッチファイル解析
2. 全HunkのパッチID計算
3. ターゲットID構築
4. 順次ステージングループ

### 3. Validator コンポーネント

依存関係チェックと引数検証を担当します。

**主要メソッド:**
- `CheckDependencies() error`: gitの存在確認
- `ValidateArgsNew(hunkSpecs []string, patchFile string) error`: 引数検証

### 4. HunkInfo データモデル

Hunk情報を表現するデータ構造です。

```go
type HunkInfo struct {
    GlobalIndex int    // パッチファイル内のグローバル番号
    FilePath    string // ファイルパス
    IndexInFile int    // ファイル内での番号
    PatchID     string // 一意識別子
    StartLine   int    // 開始行番号
    EndLine     int    // 終了行番号
}
```

## データモデル

### パッチファイル構造

```
diff --git a/file1.py b/file1.py
index abc123..def456 100644
--- a/file1.py
+++ b/file1.py
@@ -10,6 +10,8 @@ def function1():
     # 既存のコード
+    # 新しいコード（Hunk 1）
+    print("追加された行")
     return result

@@ -20,4 +22,6 @@ def function2():
     # 既存のコード
+    # 新しいコード（Hunk 2）
+    logger.info("ログ追加")
     return value
```

### Hunk指定フォーマット

```
file.py:1,3    # file.pyのHunk 1と3を指定
src/main.go:2  # src/main.goのHunk 2を指定
```

### パッチID計算

```bash
# 各Hunkに対して実行
git patch-id --stable < hunk_content
# 出力例: a1b2c3d4 commit_hash
# 最初の8文字を使用: a1b2c3d4
```

## エラーハンドリング

### エラー分類

1. **依存関係エラー**
   - git コマンドが見つからない

2. **引数検証エラー**
   - 無効なHunk指定フォーマット
   - 負の数や0のHunk番号
   - 存在しないファイルパス

3. **ファイル操作エラー**
   - パッチファイルが読み取れない
   - 一時ファイル作成失敗

4. **Git操作エラー**
   - `git apply --cached` 失敗
   - `git diff` 実行失敗
   - `git patch-id` 計算失敗

### エラー処理戦略

```go
// 段階的エラーハンドリング
func (s *Stager) StageHunks(hunkSpecs []string, patchFile string) error {
    // 1. 事前検証フェーズ
    if err := s.validateInputs(); err != nil {
        return fmt.Errorf("validation failed: %v", err)
    }

    // 2. 準備フェーズ
    if err := s.preparePatches(); err != nil {
        return fmt.Errorf("preparation failed: %v", err)
    }

    // 3. 実行フェーズ（リカバリ可能）
    if err := s.executeStaging(); err != nil {
        s.saveDebugInfo(err) // デバッグ情報保存
        return fmt.Errorf("staging failed: %v", err)
    }

    return nil
}
```

## テスト戦略

### テストレベル

1. **単体テスト**
   - 各コンポーネントの個別機能
   - MockCommandExecutorを使用
   - エラーケースの網羅

2. **統合テスト**
   - コンポーネント間の連携
   - 実際のGitコマンド使用
   - 一時リポジトリでの検証

3. **E2Eテスト**
   - 実際のユースケース
   - 複数ファイル・複数Hunk
   - パフォーマンステスト

### テストデータ戦略

```go
// テスト用のパッチデータ
var testPatches = map[string]string{
    "single_hunk": `diff --git a/test.py b/test.py
index abc123..def456 100644
--- a/test.py
+++ b/test.py
@@ -1,3 +1,4 @@
 def hello():
+    print("Hello, World!")
     return "hello"`,

    "multiple_hunks": `...`, // 複数Hunkのパッチ
    "new_file": `...`,       // 新規ファイルのパッチ
}
```

### モック戦略

```go
// CommandExecutorのモック設定例
func setupMockExecutor() *MockCommandExecutor {
    mock := NewMockCommandExecutor()

    // git diffコマンドのモック
    mock.Commands["git [diff HEAD --]"] = MockResponse{
        Output: []byte(testPatches["current_diff"]),
        Error:  nil,
    }

    // git patch-idコマンドのモック
    mock.Commands["git [patch-id --stable]"] = MockResponse{
        Output: []byte("a1b2c3d4 commit_hash"),
        Error:  nil,
    }

    return mock
}
```

## パフォーマンス考慮事項

### 最適化ポイント

1. **ファイルフィルタリング**
   ```bash
   # 全ファイルではなく、ターゲットファイルのみ処理
   git diff HEAD -- file1.py file2.py
   ```

2. **一時ファイル管理**
   ```go
   // defer文による確実なクリーンアップ
   tmpFile, err := os.CreateTemp("", "patch_*.tmp")
   if err != nil {
       return err
   }
   defer os.Remove(tmpFile.Name())
   ```

3. **パッチID計算の最適化**
   - 必要なHunkのみ計算
   - 計算結果のキャッシュ（将来の拡張）

### メモリ使用量

- 大きなパッチファイルでも一度にメモリに読み込み
- ストリーミング処理は現在未実装（将来の改善点）

## セキュリティ考慮事項

### 入力検証

1. **パスインジェクション対策**
   ```go
   // ファイルパスの検証
   if strings.Contains(filePath, "..") {
       return errors.New("path traversal detected")
   }
   ```

2. **コマンドインジェクション対策**
   ```go
   // 引数の適切なエスケープ
   cmd := exec.Command("git", "diff", "--", filePath)
   ```

### 一時ファイル

- 適切な権限設定（0644）
- 確実なクリーンアップ
- 機密情報の漏洩防止

## 拡張性

### 将来の拡張ポイント

1. **設定ファイル対応**
   - デフォルトHunk指定
   - カスタムパッチIDアルゴリズム

2. **プラグインシステム**
   - カスタムバリデーター
   - カスタムフィルター

3. **GUI対応**
   - インタラクティブHunk選択
   - ビジュアルdiff表示

4. **パフォーマンス改善**
   - 並列処理
   - ストリーミング処理
   - キャッシュシステム
