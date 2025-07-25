# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## プロジェクト概要

git-sequential-stageは、パッチファイルから指定されたハンクを順次ステージングするGoのCLIツールです。LLMエージェントがセマンティックな意味を持つコミットを作成するためのプログラマティックな制御を提供します。

**解決する核心問題**: 従来の`git add -p`では依存関係のあるハンクの処理やプログラマティックな制御ができません。このツールはパッチIDを使用して行番号変更に関係なくハンクを確実にステージングし、セマンティックなコミット分割を可能にします。

## アーキテクチャ

```
git-sequential-stage/
├── main.go                     # CLIエントリーポイント（hunkListカスタムタイプ）
├── e2e_basic_test.go           # 基本機能E2Eテスト
├── e2e_semantic_test.go        # セマンティックコミット分割テスト（最重要）
├── e2e_error_test.go           # エラーハンドリングテスト
├── e2e_advanced_files_test.go  # ファイル操作系テスト
├── e2e_advanced_performance_test.go # パフォーマンステスト
├── e2e_advanced_edge_cases_test.go  # エッジケーステスト
└── internal/
    ├── executor/               # コマンド実行抽象化レイヤー
    ├── stager/                 # パッチIDシステムによる核心ステージングロジック
    │   ├── stager.go          # メイン実装（StageHunks関数と補助関数）
    │   ├── hunk_info.go       # HunkInfo構造体とパッチ解析のエントリーポイント
    │   ├── patch_parser_gitdiff.go  # go-gitdiffライブラリを使用した堅牢な解析
    │   └── errors.go          # カスタムエラー型（StagerError）の定義
    └── validator/              # 依存関係・引数検証
```

**主要設計パターン**:
- **依存性注入**: `executor.CommandExecutor`インターフェースによるテストでのモック化
- **パッチIDシステム**: `git patch-id`によるコンテンツベースのハンク識別
- **逐次処理**: 依存関係を処理するためのハンク1つずつの適用
- **安全性最優先**: デフォルト有効の安全性チェックによるステージングエリア保護
- **エラーハンドリング**: `StagerError`型によるコンテキスト付きエラー管理
- **解析戦略**: go-gitdiffを優先し、レガシーパーサーへのフォールバック

## 開発コマンド

```bash
# CLIツールのビルド
go build

# 全テスト実行（ユニット + E2E）
go test -v ./...

# E2Eテストのみ実行
go test -v -run "Test.*" .

# 安全性機能テスト実行
go test -v -run TestE2E_FinalIntegration     # 9つの安全性要件（S1-S9）検証
go test -v -run TestE2E_PerformanceWithSafetyChecks  # パフォーマンス要件確認

# 重要テストの個別実行
go test -v -run TestMixedSemanticChanges     # セマンティック分割の核心テスト
go test -v -run TestLargeFileWithManyHunks   # パフォーマンステスト

# 依存関係インストール（macOS）
brew install patchutils

# 依存関係インストール（Ubuntu/Debian）
sudo apt-get install patchutils
```

## 開発時のツール使用方法

CLIは複数の`-hunk`フラグを処理するカスタム`hunkList`タイプを使用します：

```bash
# 基本使用方法（更新されたAPI）
./git-sequential-stage -patch="changes.patch" -hunk="file.go:1,3,5"

# 複数ファイル
./git-sequential-stage -patch="changes.patch" \
  -hunk="src/main.go:1,3" \
  -hunk="src/utils.go:2,4"
```

**重要な注意**: ツールは`file:hunk_numbers`形式を期待します。単なるハンク番号ではありません。

## テスト戦略

**E2Eテスト**（11の包括的テストケース）:
- `TestMixedSemanticChanges`: **最重要** - セマンティックコミット分割の実証
- `TestLargeFileWithManyHunks`: パフォーマンス検証（目標: <5秒、実測: ~230ms）
- `TestBinaryFileHandling`: バイナリファイルのエッジケース
- `TestFileModificationAndMove`: 複雑なファイル操作

**ユニットテスト**:
- `patch_parser_test.go`: go-gitdiffとレガシーパーサーの比較検証
- `special_files_test.go`: 特殊ファイル操作（リネーム、削除、バイナリ）のテスト
- `main_test.go`: CLIインターフェースとフラグ処理のテスト

**テスト環境**:
- `go-git`ライブラリによる独立したテストリポジトリ
- Go 1.20+の`t.Chdir()`によるディレクトリ管理
- バイナリ実行ではなく`runGitSequentialStage`関数の直接呼び出し

## 主要実装詳細

**パッチIDシステム**:
1. go-gitdiffでハンクを解析・抽出
2. `git patch-id`でユニークIDを計算
3. コンテンツベースのマッチングで逐次適用
4. 「ハンク番号のずれ」問題を自動解決

**パッチ解析の2層構造**:
1. **プライマリ**: `go-gitdiff`ライブラリによる堅牢な解析
   - ファイル操作タイプ（追加、削除、リネーム、コピー）の正確な検出
   - バイナリファイルの適切な処理
   - Gitのdiffフォーマット仕様に準拠した解析
2. **フォールバック**: レガシー文字列ベースパーサー
   - go-gitdiffが失敗した場合の後方互換性を保証
   - シンプルなパッチに対する軽量な処理

**エラーハンドリング設計**:
- `StagerError`型: エラータイプとコンテキスト情報を含む構造化エラー
- `errors.Is/As`との互換性: Go標準のエラーハンドリングパターンをサポート
- エラータイプ: FileNotFound、Parsing、GitCommand、HunkNotFound等を明確に分類

**依存関係**:
- 実行時: `git`
- ビルド時: `github.com/bluekeyes/go-gitdiff`（パッチ解析）
- テスト時: `go-git`ライブラリ
- CI: GitHub Actions（go-gitdiffベースの完全実装により外部依存関係不要）

**安全性機能**:
- **ステージングエリア検証**: デフォルトで有効、意図しない変更の上書きを防止
- **LLMエージェント対応メッセージ**: `SAFETY_CHECK_FAILED`タグ付きの構造化エラー
- **ファイル操作分類**: NEW、MODIFIED、DELETED、RENAMED状態の自動検出と適切なアドバイス
- **Intent-to-add対応**: `git add -N`ファイルの適切な検出と処理

**パフォーマンス**: 20関数・12ハンクのファイルを~230msで処理（5秒目標を大幅クリア）、安全性チェック込みで276ms制限内

## LLMエージェント統合

このツールはLLMワークフロー自動化のために特別に設計されました。`TestMixedSemanticChanges`で実証されるセマンティックコミット分割機能は、単一の複雑な変更を意味のあるコミットに自動分割できることを示します：

- ロギングインフラ → `feat:`コミット
- 入力バリデーション → `feat:`コミット
- API改善 → `improve:`コミット

パッチIDシステムにより、ハンクに依存関係がある場合や重複する行範囲を変更する場合でも確実に動作します。

### Intent-to-addワークフロー

LLMエージェントは制限された`git add`コマンドのみ使用可能で、以下のワークフローが標準的です：

**エージェント制約**:
- 個別ファイルの`git add -N`は禁止
- 許可されるのは`git ls-files --others --exclude-standard | xargs git add -N`（一括intent-to-add）のみ

**ワークフロー手順**:
1. **新規ファイル発見**: エージェントが複数の新規ファイルを検出
2. **一括intent-to-add**: `git ls-files --others | xargs git add -N`でまとめて追加
3. **パッチ生成**: `git diff HEAD`で全変更を含むパッチを生成
4. **選択的ステージング**: `git-sequential-stage`で特定ファイルのハンクのみをステージング
5. **セマンティックコミット**: 意味単位でのコミット作成

**重要な設計判断**:
- Intent-to-addファイルの存在下でも、ターゲットファイルが明示的に指定されていれば正常動作
- 安全性チェックはintent-to-addファイルを例外として扱い、エラーを発生させない
- これにより、エージェントが効率的なセマンティックコミット分割を実現可能

**実装例**:
```bash
# エージェントワークフロー例
git ls-files --others --exclude-standard | xargs git add -N
git diff HEAD > changes.patch
./git-sequential-stage -patch="changes.patch" -hunk="specific_file.go:1,3"
git commit -m "feat: Add logging functionality"
```

## テストファイル分割方針

**重要**: このプロジェクトのE2Eテストは機能別に最適化された構造で分割されています。Claude Codeは以下の指針に従ってください：

### テストファイル構造
- **`e2e_basic_test.go`**: 基本機能テスト（TestBasicSetup, TestSingleFileSingleHunk等）
- **`e2e_semantic_test.go`**: セマンティックコミット分割テスト（TestMixedSemanticChanges - 最重要）
- **`e2e_error_test.go`**: 全エラーハンドリングテスト
- **`e2e_advanced_files_test.go`**: ファイル操作系テスト
- **`e2e_advanced_performance_test.go`**: パフォーマンステスト
- **`e2e_advanced_edge_cases_test.go`**: エッジケーステスト

### Claude Code制約事項
1. **テストファイルの新規作成禁止**: 既存の6つのE2Eテストファイル以外は作成しない
2. **テストファイルの自動分割禁止**: ファイルサイズや行数を理由に勝手に分割しない
3. **テスト内容の変更禁止**: 既存テストの動作を変更・削除・追加しない
4. **構造の維持**: フラットなファイル構造を維持し、ディレクトリ分割をしない

### 新規テスト追加時
- 新しいテストが必要な場合は、最も関連性の高い既存ファイルに追加する
- テスト分類が不明な場合は、ユーザーに確認する
- 各ファイルの責務範囲を越える場合のみ、ユーザーと相談して対応を決定する

この分割構造は、Go言語のベストプラクティスに従いつつ、Claude Codeでの理解性と保守性を両立させるために設計されています。
