---
inclusion: always
---

# 技術スタック

## 言語とランタイム
- **Go 1.24.2** - メイン言語
- 依存関係管理にはモダンなGoモジュールを使用

## 主要依存関係
- `github.com/bluekeyes/go-gitdiff` - 高度なパッチ解析とGitdiff処理
- 標準ライブラリを中心とした軽量な依存関係構成

## 外部ツール
- **git** - コアGit操作（必須）
  - `git diff` - パッチ生成
  - `git apply --cached` - ハンクのステージング
  - `git patch-id --stable` - パッチID計算

## アーキテクチャパターン
- **インターフェースベース設計**: テスト可能性のためのCommandExecutorインターフェース
- **依存性注入**: テスト用の実際のvs モックエグゼキューター
- **構造化エラーハンドリング**: 型付きカスタムエラーとコンテキスト情報
- **モジュラーパッケージ**: 関心の明確な分離
- **パッチID駆動**: git patch-idを使用したハンク追跡とドリフト問題解決
- **段階的処理**: 準備フェーズと実行フェーズの分離

## よく使うコマンド

### 開発
```bash
# 全テスト実行
go test ./...

# カバレッジ付きテスト実行
go test -cover ./...

# バイナリビルド
go build

# ソースからインストール
go install
```

### テスト
```bash
# E2Eテスト実行（gitとfilterdiffが必要）
go test ./internal/stager -run E2E

# 詳細出力付きデバッグモード
GIT_SEQUENTIAL_STAGE_VERBOSE=1 ./git-sequential-stage -patch=changes.patch -hunk="file.go:1,3"
```

### 使用方法
```bash
# パッチ生成と特定ハンクのステージング
git diff > changes.patch
git-sequential-stage -patch=changes.patch -hunk="main.go:1,3" -hunk="internal/stager/stager.go:2,4,5"
```

## ビルドシステム
- 標準Goツールチェーン
- 追加のビルドツール不要
- クロスプラットフォーム対応（macOS/Linuxでテスト済み）