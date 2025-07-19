# Safety Improvements - 実装タスク

## 概要

このタスクリストは、[Issue #13「Git操作の安全性向上とテストカバレッジ拡充」](https://github.com/syou6162/git-sequential-stage/issues/13)のフェーズ1実装タスクです。

**機能名:** Safety Improvements  
**目的:** ステージングエリア保護機能とGit操作エラーの適切な処理を実装し、ツールの安全性を向上させる  
**優先度:** 最高（[Issue #13](https://github.com/syou6162/git-sequential-stage/issues/13) フェーズ1）

## 前提条件

- [ ] [Issue #13](https://github.com/syou6162/git-sequential-stage/issues/13)の問題分析が完了していること
- [ ] 既存のコードベースの安全性要件と設計書が確認済みであること
- [ ] 開発環境にGo 1.24.2とgit-sequential-stageの依存関係が準備済みであること

## 実装タスク

### Phase 1.1: 基盤実装

- [ ] 1. SafetyError型とエラーハンドリング基盤の実装
  - SafetyError構造体とSafetyErrorType列挙型を定義
  - エラータイプ別のコンストラクタ関数を実装
  - Error()メソッドでユーザーフレンドリーなエラーメッセージを生成
  - 既存のStagerErrorとの統合を確保
  - **期限:** 1週目
  - **成果物:** `internal/stager/safety_errors.go`, `internal/stager/safety_errors_test.go`

- [ ] 1.1 SafetyError型の基本実装
  - internal/stager/safety_errors.goファイルを作成
  - SafetyError構造体とSafetyErrorType定数を定義
  - NewSafetyError関数とError()メソッドを実装

- [ ] 1.2 SafetyErrorのユニットテスト
  - internal/stager/safety_errors_test.goファイルを作成
  - 各エラータイプのコンストラクタテストを実装
  - エラーメッセージ形式のテストを実装

### Phase 1.2: SafetyCheckerコンポーネント

- [ ] 2. SafetyCheckerコンポーネントの実装
  - ステージングエリアの状態チェック機能を実装
  - git status --porcelainコマンドの実行と結果解析
  - ファイルタイプ別分類機能（M/A/D/R/C）
  - 7つのユースケース対応のSafetyCheckResult構造体
  - CommandExecutorインターフェースとの統合
  - **期限:** 2週目
  - **成果物:** `internal/stager/safety_checker.go`, `internal/stager/safety_checker_test.go`

- [ ] 2.1 SafetyChecker基本構造の実装
  - internal/stager/safety_checker.goファイルを作成
  - SafetyChecker構造体とSafetyCheckResult構造体を定義
  - NewSafetyChecker関数を実装

- [ ] 2.2 ステージングエリアチェック機能の実装
  - CheckStagingArea()メソッドを実装
  - git status --porcelainコマンドの実行
  - ファイルタイプ別（M/A/D/R/C）の分類と解析
  - 各ユースケース（S1.1-S1.7）に対応したSafetyCheckResultの生成

- [ ] 2.3 SafetyCheckerのユニットテスト
  - internal/stager/safety_checker_test.goファイルを作成
  - モックエグゼキューターを使用したテストを実装
  - 7つのユースケース（S1.1-S1.7）別のテストケース
    - S1.1: 通常ファイル修正のテスト
    - S1.2: 新規ファイルのテスト
    - S1.3: ファイル削除のテスト
    - S1.4: ファイルリネームのテスト
    - S1.5: ファイルコピーのテスト
    - S1.6: 複数種類混在のテスト
    - S1.7: 部分ステージングのテスト
  - エラーケース（git コマンド失敗など）のテスト

### Phase 1.3: 詳細エラーメッセージ実装

- [ ] 2.4 ユースケース別エラーメッセージ生成機能
  - generateDetailedStagingError()メソッドを実装
  - buildStagingErrorMessage()メソッドでファイルタイプ別メッセージ生成
  - buildStagingAdvice()メソッドで状況別対処法生成
  - 統合エラーメッセージ形式の実装
  - **期限:** 2週目後半
  - **成果物:** `internal/stager/safety_checker.go`の拡張

- [ ] 2.5 ユースケース別エラーメッセージのテスト
  - 7つのユースケース別のエラーメッセージテスト
  - 統合エラーメッセージ形式のテスト
  - 対処法メッセージの正確性テスト
  - **期限:** 2週目後半
  - **成果物:** `internal/stager/safety_checker_test.go`の拡張

### Phase 1.4: Semantic Commit Workflow統合

- [ ] 2.6 Intent-to-addファイル検出機能の実装
  - DetectIntentToAddFiles()メソッドを実装
  - git ls-files --cached --others --exclude-standardを使用
  - Intent-to-addファイルの識別ロジック
  - **期限:** 3週目前半
  - **成果物:** `internal/stager/intent_to_add_handler.go`

- [ ] 2.7 Semantic Commitワークフロー対応のSafetyChecker拡張
  - CheckStagingAreaWithIntentToAdd()メソッドを実装
  - Intent-to-addファイルと通常ステージングの区別
  - AllowContinueフラグの適切な設定
  - **期限:** 3週目前半
  - **成果物:** `internal/stager/safety_checker.go`の拡張

- [ ] 2.8 Semantic Commitワークフロー統合テスト
  - 4つのテストシナリオの実装（要件書記載）
  - Intent-to-addファイルの基本ワークフローテスト
  - 混在シナリオのテスト
  - 複数Intent-to-addファイルのテスト
  - 部分ステージングのテスト
  - **期限:** 3週目後半
  - **成果物:** `internal/stager/semantic_commit_test.go`

### Phase 1.5: Stager統合

- [ ] 3. Stager.StageHunksへの安全性チェック統合
  - StageHunksメソッドの開始時にperformSafetyChecksWithSemanticCommit()を呼び出し
  - ステージングエリアが汚れている場合のエラーハンドリング
  - Intent-to-addファイルの特別処理
  - 詳細なエラーメッセージと対処法の表示
  - 既存の処理フローとの統合
  - **期限:** 4週目
  - **成果物:** `internal/stager/stager.go`の修正、関連テスト

- [ ] 3.1 performSafetyChecks()メソッドの実装
  - Stager構造体にperformSafetyChecks()メソッドを追加
  - SafetyCheckerを使用したステージングエリアチェック
  - 適切なSafetyErrorの生成と返却

- [ ] 3.2 StageHunksメソッドの修正
  - Phase 0として安全性チェックを追加
  - performSafetyChecks()の呼び出しとエラーハンドリング
  - 既存のPhase 1, 2処理との統合

- [ ] 3.3 安全性チェック統合のユニットテスト
  - StageHunksメソッドの安全性チェック部分のテスト
  - モックを使用したクリーン/ダーティ状態のテスト
  - 8つのユースケース別エラーメッセージの内容検証（S1.1-S1.8）
  - Intent-to-addファイル処理の検証
  - 統合エラーメッセージ形式の検証

## 成功基準

### Phase 1完了の必須条件
- [ ] **機能要件**
  - ステージングエリア保護機能が確実に動作する
  - SafetyErrorによる統一されたエラーハンドリングが動作する
  - ユーザーフレンドリーなエラーメッセージが表示される
  - **Semantic Commitワークフローが完全に動作する**

- [ ] **品質要件**
  - 全ユニットテストが通る（新機能の基本動作保証）
  - 既存E2Eテストが全て通る（回帰なし）
  - **Semantic Commitワークフローの4つのテストシナリオが全て通る**
  - コードカバレッジが80%以上を維持する

- [ ] **パフォーマンス要件**
  - ステージングエリアチェックが100ms以内に完了する
  - 全体の実行時間が従来の120%以内に収まる

- [ ] **統合要件**
  - Intent-to-addファイルから最終コミットまでの完全なワークフローが動作する
  - 既存のsemantic_commit.mdのワークフローが変更なしで動作する

## 参考資料

- [Issue #13: Git操作の安全性向上とテストカバレッジ拡充](https://github.com/syou6162/git-sequential-stage/issues/13)
- [Safety Improvements 要件書](requirements.md)
- [Safety Improvements 設計書](design.md)
- [開発ガイドライン](../../steering/safety-improvements.md)