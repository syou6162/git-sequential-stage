# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## プロジェクト概要

git-sequential-stageは、パッチファイルから指定されたハンクを順次ステージングするGoのCLIツールです。LLMエージェントがセマンティックな意味を持つコミットを作成するためのプログラマティックな制御を提供します。

**解決する核心問題**: 従来の`git add -p`では依存関係のあるハンクの処理やプログラマティックな制御ができません。このツールはパッチIDを使用して行番号変更に関係なくハンクを確実にステージングし、セマンティックなコミット分割を可能にします。

## アーキテクチャ

```
git-sequential-stage/
├── main.go                     # CLIエントリーポイント（hunkListカスタムタイプ）
├── e2e_test.go                 # 包括的E2Eテスト（11テストケース）
└── internal/
    ├── executor/               # コマンド実行抽象化レイヤー
    ├── stager/                 # パッチIDシステムによる核心ステージングロジック
    └── validator/              # 依存関係・引数検証
```

**主要設計パターン**:
- **依存性注入**: `executor.CommandExecutor`インターフェースによるテストでのモック化
- **パッチIDシステム**: `git patch-id`によるコンテンツベースのハンク識別
- **逐次処理**: 依存関係を処理するためのハンク1つずつの適用

## 開発コマンド

```bash
# CLIツールのビルド
go build

# 全テスト実行（ユニット + E2E）
go test -v ./...

# E2Eテストのみ実行
go test -v -run "Test.*" .

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

**テスト環境**:
- `go-git`ライブラリによる独立したテストリポジトリ
- Go 1.20+の`t.Chdir()`によるディレクトリ管理
- バイナリ実行ではなく`runGitSequentialStage`関数の直接呼び出し

## 主要実装詳細

**パッチIDシステム**: 
1. `filterdiff --hunks=N`でハンクを抽出
2. `git patch-id`でユニークIDを計算
3. コンテンツベースのマッチングで逐次適用
4. 「ハンク番号のずれ」問題を自動解決

**依存関係**:
- 実行時: `git`, `filterdiff`（patchutils）
- テスト時: `go-git`ライブラリ
- CI: patchutils自動インストール付きGitHub Actions

**パフォーマンス**: 20関数・12ハンクのファイルを~230msで処理（5秒目標を大幅クリア）

## LLMエージェント統合

このツールはLLMワークフロー自動化のために特別に設計されました。`TestMixedSemanticChanges`で実証されるセマンティックコミット分割機能は、単一の複雑な変更を意味のあるコミットに自動分割できることを示します：

- ロギングインフラ → `feat:`コミット
- 入力バリデーション → `feat:`コミット  
- API改善 → `improve:`コミット

パッチIDシステムにより、ハンクに依存関係がある場合や重複する行範囲を変更する場合でも確実に動作します。