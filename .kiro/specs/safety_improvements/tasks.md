# Safety Improvements - 実装タスク

## 概要

このタスクリストは、[Issue #13「Git操作の安全性向上とテストカバレッジ拡充」](https://github.com/syou6162/git-sequential-stage/issues/13)のフェーズ1実装タスクです。

**機能名:** Safety Improvements  
**目的:** ステージングエリア保護機能とGit操作エラーの適切な処理を実装し、ツールの安全性を向上させる  
**優先度:** 最高（[Issue #13](https://github.com/syou6162/git-sequential-stage/issues/13) フェーズ1）

**現在の実装状況:** 安全性機能は未実装。既存のStagerErrorシステムは存在するが、SafetyError、SafetyChecker、Intent-to-add処理は全て新規実装が必要。

## 実装タスク

### Phase 1: 基盤実装

- [x] 1.1 SafetyError型の実装
  - internal/stager/safety_errors.goファイルを作成
  - SafetyError構造体とSafetyErrorType定数を定義
  - NewSafetyError関数とError()メソッドを実装
  - 既存のStagerErrorとの統合を確保
  - _要件: S4 (Git操作エラーハンドリング)_

- [x] 1.2 SafetyErrorのユニットテスト
  - internal/stager/safety_errors_test.goファイルを作成
  - 各エラータイプのコンストラクタテストを実装
  - エラーメッセージ形式のテストを実装
  - _要件: S4 (Git操作エラーハンドリング)_

### Phase 2: SafetyCheckerコンポーネント

- [ ] 2.1 SafetyChecker基本構造の実装
  - internal/stager/safety_checker.goファイルを作成
  - SafetyChecker構造体とStagingAreaEvaluation構造体を定義
  - NewSafetyChecker関数を実装
  - _要件: S1 (ステージングエリア状態検出)_

- [ ] 2.2 ステージングエリアチェック機能の実装
  - EvaluateStagingArea()メソッドを実装
  - git status --porcelainコマンドの実行と結果解析
  - ファイルタイプ別（M/A/D/R/C）の分類と解析
  - StagingAreaEvaluationの生成
  - _要件: S1 (ステージングエリア状態検出), S3 (ファイルタイプ別エラーメッセージ)_

- [ ] 2.3 Intent-to-addファイル検出機能の実装
  - DetectIntentToAddFiles()メソッドを実装
  - git ls-files --cached --others --exclude-standardを使用
  - Intent-to-addファイルの識別ロジック
  - _要件: S2 (Intent-to-addファイル統合), S5 (Semantic Commitワークフロー統合)_

- [ ] 2.4 詳細エラーメッセージ生成機能の実装
  - generateDetailedStagingError()メソッドを実装
  - buildStagingErrorMessage()メソッドでファイルタイプ別メッセージ生成
  - buildRecommendedActions()メソッドで推奨アクション生成
  - LLM Agent対応の構造化メッセージ形式
  - _要件: S3 (ファイルタイプ別エラーメッセージ), S2 (Intent-to-addファイル統合)_

- [ ] 2.5 SafetyCheckerのユニットテスト
  - internal/stager/safety_checker_test.goファイルを作成
  - モックエグゼキューターを使用したテストを実装
  - 各ユースケース別のテストケース:
    - クリーンなステージングエリア
    - 修正ファイル（M）のステージング
    - 新規ファイル（A）のステージング
    - 削除ファイル（D）のステージング
    - リネームファイル（R）のステージング
    - コピーファイル（C）のステージング
    - Intent-to-addファイルのみ
    - 複数種類混在
  - エラーケース（git コマンド失敗など）のテスト
  - _要件: S1-S3 (全ステージングエリア関連要件)_

### Phase 3: Stager統合

- [ ] 3.0 deprecatedなEvaluateStagingAreaメソッドの完全削除
  - EvaluateStagingAreaメソッドを完全に削除
  - EvaluatePatchContentのみのAPIに統一
  - 関連するテストの更新
  - 後方互換性の完全な破棄（Phase 2で廃止警告済み）
  - _技術的負債解消_

- [ ] 3.1 performSafetyChecks()メソッドの実装
  - Stager構造体にperformSafetyChecks()メソッドを追加
  - SafetyCheckerを使用したステージングエリアチェック
  - 適切なSafetyErrorの生成と返却
  - Intent-to-addファイルの特別処理
  - _要件: S1 (ステージングエリア状態検出), S2 (Intent-to-addファイル統合)_

- [ ] 3.2 StageHunksメソッドの修正
  - Phase 0として安全性チェックを追加
  - performSafetyChecks()の呼び出しとエラーハンドリング
  - 既存のPhase 1, 2処理との統合
  - _要件: S6 (ワークフロー非破壊保証), S7 (正常ケースの動作保証)_

- [ ] 3.3 安全性チェック統合のユニットテスト
  - StageHunksメソッドの安全性チェック部分のテスト
  - モックを使用したクリーン/ダーティ状態のテスト
  - Intent-to-addファイル処理の検証
  - 統合エラーメッセージ形式の検証
  - 既存機能の回帰テスト
  - _要件: S6 (ワークフロー非破壊保証), S7 (正常ケースの動作保証)_

### Phase 4: Semantic Commitワークフロー統合テスト

- [ ] 4.1 基本的なIntent-to-addワークフローテスト
  - git add -N → パッチ生成 → ハンクステージング → コミットの完全フロー
  - internal/stager/semantic_commit_test.goファイルを作成
  - _要件: S5 (Semantic Commitワークフロー統合)_

- [ ] 4.2 混在シナリオテスト
  - Intent-to-addファイルと通常ステージングの混在
  - Intent-to-addは継続、通常ステージングはエラー停止の検証
  - _要件: S2 (Intent-to-addファイル統合), S5 (Semantic Commitワークフロー統合)_

- [ ] 4.3 複数Intent-to-addファイルテスト
  - 複数のIntent-to-addファイルの同時処理
  - 部分ステージングの動作確認
  - _要件: S5 (Semantic Commitワークフロー統合)_

- [ ] 4.4 エラーケーステスト
  - Intent-to-addファイルのハンク適用エラー
  - 適切なエラーメッセージの表示確認
  - _要件: S4 (Git操作エラーハンドリング), S8 (エラーケースの動作保証)_

### Phase 5: 統合テストと最終検証

- [ ] 5.1 E2Eテストの実行と修正
  - 既存のE2Eテストが全て通ることを確認
  - 安全性機能による影響の修正
  - _要件: S9 (基本動作の一貫性)_

- [ ] 5.2 パフォーマンステスト
  - ステージングエリアチェックの実行時間測定
  - 全体の実行時間が120%以内であることを確認
  - _非機能要件: パフォーマンス要件_

- [ ] 5.3 最終統合テスト
  - 全ての要件シナリオの動作確認
  - semantic_commit.mdワークフローの動作確認
  - LLM Agent対応メッセージ形式の検証
  - _要件: S5 (Semantic Commitワークフロー統合), S6 (ワークフロー非破壊保証)_

## 成功基準

### 機能要件
- [ ] ステージングエリア保護機能が確実に動作する
- [ ] SafetyErrorによる統一されたエラーハンドリングが動作する
- [ ] LLM Agent対応の構造化エラーメッセージが表示される
- [ ] Intent-to-addファイルの適切な処理が動作する
- [ ] Semantic Commitワークフローが完全に動作する

### 品質要件
- [ ] 全ユニットテストが通る（新機能の基本動作保証）
- [ ] 既存E2Eテストが全て通る（回帰なし）
- [ ] Semantic Commitワークフローの4つのテストシナリオが全て通る
- [ ] コードカバレッジが80%以上を維持する

### パフォーマンス要件
- [ ] ステージングエリアチェックが100ms以内に完了する
- [ ] 全体の実行時間が従来の120%以内に収まる

### 統合要件
- [ ] Intent-to-addファイルから最終コミットまでの完全なワークフローが動作する
- [ ] 既存のsemantic_commit.mdのワークフローが変更なしで動作する

## 参考資料

- [Issue #13: Git操作の安全性向上とテストカバレッジ拡充](https://github.com/syou6162/git-sequential-stage/issues/13)
- [Safety Improvements 要件書](requirements.md)
- [Safety Improvements 設計書](design.md)
- [開発ガイドライン](../../steering/safety-improvements.md)