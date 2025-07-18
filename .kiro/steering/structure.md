---
inclusion: always
---

# プロジェクト構造

## ルートレベル
- `main.go` - CLIエントリーポイント、引数解析、メインロジック統合
- `main_test.go` - メイン機能の統合テスト
- `e2e_test.go` - エンドツーエンドテスト
- `go.mod/go.sum` - Goモジュール依存関係
- `git-sequential-stage` - コンパイル済みバイナリ（実行可能ファイル）

## 内部パッケージ

### `/internal/stager`
**コアステージングロジックとパッチ処理**
- `stager.go` - メインStager構造体とStageHunksメソッド
- `patch_parser_gitdiff.go` - go-gitdiffライブラリを使用したパッチ解析
- `hunk_info.go` - HunkInfo構造体とハンク仕様解析
- `errors.go` - コンテキスト付きカスタムエラータイプ
- `*_test.go` - E2Eテストを含む包括的テストカバレッジ

### `/internal/executor`
**コマンド実行抽象化**
- `interface.go` - CommandExecutorインターフェース定義
- `real.go` - 実際のコマンドエグゼキューター実装
- `mock.go` - テスト用モックエグゼキューター
- `executor_test.go` - エグゼキューターテスト

### `/internal/validator`
**依存関係と引数検証**
- `validator.go` - 依存関係チェックと引数検証
- `validator_test.go` - 検証テスト

### `/internal/logger`
**ログユーティリティ**
- `logger.go` - 環境ベースロガー（GIT_SEQUENTIAL_STAGE_VERBOSE）

## 設定とドキュメント
- `.kiro/` - Kiro IDE設定とステアリングルール
- `.github/` - GitHubワークフローとテンプレート
- `README.md` - 例を含む包括的ドキュメント
- `LICENSE` - MITライセンス
- `renovate.json` - 依存関係更新自動化

## テスト戦略
- **ユニットテスト**: 各パッケージに包括的な`*_test.go`ファイル
- **E2Eテスト**: `stager_e2e_test.go`での実際のGitリポジトリテスト
- **モックテスト**: 分離テストのためのインターフェースベースモック
- **カバレッジ**: カバレッジレポート用の`coverage.out`と`coverage.html`

## 命名規則
- **パッケージ**: 単語、小文字（stager、executor、validator）
- **ファイル**: 複数語概念にはsnake_case（`patch_parser_gitdiff.go`）
- **構造体**: PascalCase（Stager、HunkInfo、CommandExecutor）
- **メソッド**: パブリックはPascalCase、プライベートはcamelCase
- **テストファイル**: `*_test.go`サフィックス、テスト関数は`Test`で開始

## アーキテクチャ原則
- **関心の分離**: 各パッケージは単一の責任を持つ
- **インターフェース駆動**: テスト可能性と柔軟性のためのインターフェース使用
- **エラーハンドリング**: コンテキスト情報付きカスタムエラータイプ
- **依存性注入**: 依存関係を作成するのではなく渡す