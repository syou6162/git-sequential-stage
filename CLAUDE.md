# git-sequential-stage - Claude開発ガイド

## プロジェクト概要

このプロジェクトは、パッチファイルから指定されたハンクを順次ステージングするGoのCLIツールです。LLMエージェントがセマンティックな意味を持つコミットを作成するためのツールとして設計されています。

## アーキテクチャ

```
git-sequential-stage/
├── main.go                     # CLIエントリーポイント
├── e2e_test.go                # エンドツーエンドテスト
├── internal/
│   ├── executor/              # コマンド実行抽象化
│   ├── stager/                # ハンクステージング核心ロジック
│   └── validator/             # 依存関係・引数検証
└── .github/workflows/
    └── go-test.yml            # CI設定（patchutilsインストール込み）
```

## テスト戦略

### ユニットテスト
- `internal/`配下の各パッケージに個別のテストファイル
- モックを使用した単体機能テスト
- コマンド実行は`executor`パッケージで抽象化

### エンドツーエンドテスト

#### 実装詳細（e2e_test.go）
- **go-git**ライブラリを使用してテスト用Gitリポジトリを作成
- 一時ディレクトリでの完全に独立したテスト環境
- バイナリ実行ではなく`runGitSequentialStage`関数を直接呼び出し

#### テストケース
1. **TestSingleFileSingleHunk**: 基本的な1ファイル1ハンクのケース
2. **TestSingleFileMultipleHunks**: 複数ハンクから特定ハンクのみ選択
3. **TestMultipleFilesMultipleHunks**: 複数ファイルから特定ハンクの組み合わせ
4. **TestMixedSemanticChanges**: 最重要テスト - 異なる意味の変更を分離

### 最重要テスト：TestMixedSemanticChanges

このテストはgit-sequential-stageの核心価値を実証します：

```python
# 元のコード（Webサーバー）
def get_users():
    # TODO: Get users from database
    users = [...]
    return jsonify(users)

# 変更後：3つの異なる意味の変更が混在
def get_users():
    # TODO: Get users from database
    logger.info("Fetching users from database")  # 1. ロギング機能追加
    users = [...]
    return jsonify(users)

def create_user():
    data = request.get_json()
    
    # Add input validation                      # 2. バリデーション機能追加
    if not data or not data.get("name"):
        return jsonify({"error": "Invalid"}), 400
    
    # TODO: Save user to database
    new_user = {...}
    return jsonify(new_user), 201

def health_check():
    return {"status": "ok", "timestamp": "..."}  # 3. API改善
```

**テストシナリオ**：
1. ハンク1のみステージング → "feat: add logging infrastructure" コミット
2. ハンク2のみステージング → "feat: add input validation" コミット  
3. ハンク3のみステージング → "improve: enhance health check" コミット

**実証内容**：
- 単一の複雑な変更を意味的に分離してコミット
- LLMエージェントによる自動的なセマンティックコミット分割
- git add -pでは困難な依存関係のあるハンクの順次適用

## CI/CD

### GitHub Actions設定
- Go 1.22環境
- **patchutils**パッケージの自動インストール（filterdiffコマンド）
- 全テスト（ユニット + E2E）の実行

### 依存関係
- **実行時**: `git`, `filterdiff`（patchutilsパッケージ）
- **テスト時**: `go-git`ライブラリ

## 開発時の注意点

### テスト実行
```bash
# 全テスト実行
go test -v ./...

# E2Eテストのみ
go test -v -run "Test.*" .

# 特定のテストケース
go test -v -run TestMixedSemanticChanges
```

### filterdiffコマンドについて
- Ubuntu/Debian: `sudo apt-get install patchutils`
- macOS: `brew install patchutils`
- ハンク抽出に必須のツール

### go-gitライブラリ
- テスト用の一時Gitリポジトリ作成に使用
- プログラマティックなGit操作（コミット、ステージング）
- テストの独立性とクリーンアップを保証

## パッチIDシステム

git-sequential-stageは内部で`git patch-id`を使用してハンクを追跡します：

1. ユーザーが指定したハンク番号（例：1,3,5）
2. filterdiffでハンクを抽出
3. 各ハンクのパッチIDを計算
4. パッチIDベースでハンクを順次適用

これにより「ハンク番号のずれ」問題を解決し、LLMエージェントワークフローに最適化されています。