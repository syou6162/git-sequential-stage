---
inclusion: always
---

# 技術スタック

## 言語とランタイム
- **Go 1.24.2** - メイン言語
- 依存関係管理にはモダンなGoモジュールを使用

## 主要依存関係
- `github.com/bluekeyes/go-gitdiff` - go-gitdiffライブラリによる拡張パッチ解析
- `github.com/go-git/go-git/v5` - GoでのGit操作
- ほとんどの機能は標準ライブラリを使用

## 外部ツール
- **git** - コアGit操作（必須）
- **filterdiff** - patchutilsパッケージの一部（必須）

## アーキテクチャパターン
- **インターフェースベース設計**: テスト可能性のためのCommandExecutorインターフェース
- **依存性注入**: テスト用の実際のvs モックエグゼキューター
- **構造化エラーハンドリング**: コンテキスト付きカスタムエラータイプ
- **モジュラーパッケージ**: 関心の明確な分離

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