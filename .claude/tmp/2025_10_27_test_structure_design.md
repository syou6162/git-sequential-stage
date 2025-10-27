# テスト構造設計と移行計画

**バージョン**: 第3版（分割版 + ドキュメント更新明確化）
**最終更新**: 2025-10-27

## 改訂履歴

- **第3版（分割版 + ドキュメント更新明確化）**:
  - PR8を3つ、PR9を4つに分割してレビュー可能性を向上
  - Phase 0にCLAUDE.md移行中警告追加を明記
  - PR16を「置き換え」に変更（16 PRs、約7.8時間）
- **第2版**: 3エージェントレビュー結果を統合、Phase 0追加、時間見積もり現実化（11 PRs、約6時間）
- **第1版**: 初版（7 PRs、31分 - Claude実行のみ）

---

## 現状分析

### E2Eテスト（9ファイル、26テスト）

```
e2e_basic_test.go (6)
├─ TestBasicSetup
├─ TestSingleFileSingleHunk
├─ TestSingleFileMultipleHunks
├─ TestMultipleFilesMultipleHunks
├─ TestWildcardStaging
└─ TestWildcardWithMixedInput

e2e_semantic_test.go (1)
└─ TestMixedSemanticChanges

e2e_error_test.go (6)
├─ TestErrorCases_NonExistentFile
├─ TestErrorCases_InvalidHunkNumber
├─ TestErrorCases_EmptyPatchFile
├─ TestErrorCases_HunkCountExceeded
├─ TestErrorCases_MultipleInvalidHunks
└─ TestErrorCases_SameFileConflict

e2e_count_hunks_test.go (3)
├─ TestE2E_CountHunks_NoChanges
├─ TestE2E_CountHunks_BasicIntegration
└─ TestE2E_CountHunks_BinaryFiles

e2e_advanced_files_test.go (5)
├─ TestBinaryFileHandling
├─ TestFileModificationAndMove
├─ TestGitMvThenModifyFile
├─ TestGitMvThenModifyFileWithoutCommit
└─ TestMultipleFilesMoveAndModify_Skip

e2e_advanced_edge_cases_test.go (2)
├─ TestIntentToAddFileCoexistence
└─ TestUntrackedFile

e2e_integration_test.go (1)
└─ TestE2E_FinalIntegration

e2e_advanced_performance_test.go (1)
└─ TestLargeFileWithManyHunks

e2e_performance_test.go (1)
└─ TestE2E_PerformanceWithSafetyChecks
```

### ユニットテスト（17ファイル、75+テスト）

```
internal/stager/
├─ apply_hunk_test.go (1)
├─ count_hunks_test.go (6)
├─ enum_test.go (3)
├─ git_status_reader_test.go (4)
├─ new_file_test.go (4)
├─ patch_analyzer_test.go (9)
├─ patch_parser_test.go (2)
├─ safety_checker_test.go (11)
├─ safety_checker_benchmark_test.go (1)
├─ safety_errors_test.go (4)
├─ semantic_commit_test.go (7)
├─ special_files_test.go (2)
├─ stager_e2e_test.go (1)
├─ stager_multi_file_test.go (1)
└─ stager_safety_test.go (8)

internal/executor/
└─ executor_test.go (8)

internal/validator/
└─ validator_test.go (3)
```

### 問題点

1. **命名の不統一**
   - `safety_checker_test.go` vs `stager_safety_test.go` - 両方とも安全性テスト
   - `new_file_test.go` vs `special_files_test.go` - 特殊ファイル系が分散
   - `semantic_commit_test.go` - 何をテストするファイルか不明確

2. **責務の曖昧さ**
   - `stager_e2e_test.go` - internal/にE2E？
   - `apply_hunk_test.go` - 関数単位？機能単位？
   - 機能ベース vs 関数ベース vs 実装ファイルベースが混在

3. **重複の可能性**
   - E2E: `TestE2E_CountHunks_*` (3)
   - Unit: `count_hunks_test.go` (6)
   - E2E: `TestErrorCases_*` (6)
   - Unit: `safety_errors_test.go` (4)
   - E2E: `TestBinaryFileHandling`
   - Unit: `special_files_test.go` (2)

4. **Claude Codeの混乱**
   - 新しいエラーケーステストを追加する時：
     - `e2e_error_test.go`？
     - `safety_errors_test.go`？
     - `stager_safety_test.go`？
   - 新しいファイル操作テストを追加する時：
     - `e2e_advanced_files_test.go`？
     - `new_file_test.go`？
     - `special_files_test.go`？

---

## 提案：テスト配置の原則

### 原則1: 実装ファイルとテストファイルの1対1対応（ユニットテスト）

```
実装ファイル              → テストファイル              テスト対象
-------------------------|---------------------------|-------------------
internal/stager/
  stager.go              → stager_test.go           StageHunks関数とそのヘルパー
  count_hunks.go         → count_hunks_test.go      CountHunksInDiff関数
  patch_parser.go        → patch_parser_test.go     パッチ解析ロジック
  safety_checker.go      → safety_checker_test.go   安全性チェックロジック
  errors.go              → errors_test.go           StagerError型

internal/executor/
  executor.go            → executor_test.go         Executor実装とMock

internal/validator/
  validator.go           → validator_test.go        バリデーションロジック
```

**ルール**:
- 1つの.goファイルには1つの_test.goファイルのみ
- テストファイル名は実装ファイル名に`_test`を付ける
- そのファイルで定義された関数・型のみをテスト
- **他のファイルの関数をテストする場合は該当ファイルのテストに書く**

### 原則2: E2Eテストは機能単位で配置

```
ファイル名                    テスト対象                     テスト数目安
---------------------------|----------------------------|------------
e2e_stage_test.go          stageサブコマンドの統合動作      5-8
e2e_count_hunks_test.go    count-hunksサブコマンドの統合動作 1-2
e2e_workflows_test.go      実際の使用ワークフロー           10-11
e2e_performance_test.go    パフォーマンス検証              2
```

**ルール**:
- サブコマンド単位で1ファイル
- 実際のgitリポジトリでの動作のみをテスト
- ユニットテストで検証できることはE2Eに書かない
- ワークフローテストは複数ステップの統合シナリオ
- パフォーマンステストは独立ファイルで管理

### 原則3: 重複判定ルール

**同じ機能を複数箇所でテストしない**:

| 機能 | テストする場所 | テストしない場所 |
|-----|--------------|----------------|
| `StageHunks`関数のロジック | `internal/stager/stager_test.go` | E2E |
| エラー型とメッセージ | `internal/stager/errors_test.go` | E2E, stager_test.go |
| `CountHunksInDiff`関数 | `internal/stager/count_hunks_test.go` | E2E |
| 安全性チェックロジック | `internal/stager/safety_checker_test.go` | E2E, stager_test.go |
| count-hunksサブコマンド | `e2e_count_hunks_test.go` | ユニット |
| stageサブコマンド | `e2e_stage_test.go` | ユニット |
| セマンティックコミット分割 | `e2e_workflows_test.go` | ユニット |

---

## 理想の構造

### ユニットテスト

```
internal/
├── executor/
│   ├── executor.go
│   └── executor_test.go          # Execute, Mock実装
│
├── stager/
│   ├── stager.go
│   ├── stager_test.go            # StageHunks関数（メインロジック）
│   │
│   ├── count_hunks.go
│   ├── count_hunks_test.go       # CountHunksInDiff関数
│   │
│   ├── patch_parser_gitdiff.go
│   ├── patch_parser_test.go      # パッチ解析ロジック
│   │
│   ├── safety_checker.go
│   ├── safety_checker_test.go    # 安全性チェックロジック
│   │
│   ├── errors.go
│   └── errors_test.go            # StagerError型とエラー生成
│
└── validator/
    ├── validator.go
    └── validator_test.go         # Validate関数
```

### E2Eテスト

```
/
├── e2e_stage_test.go             # stageサブコマンドの統合テスト
│   ├── TestStage_SingleHunk
│   ├── TestStage_MultipleHunks
│   ├── TestStage_MultipleFiles
│   ├── TestStage_Wildcard
│   ├── TestStage_WildcardMixed
│   ├── TestStage_FileModify
│   ├── TestStage_GitMvModify
│   └── TestStage_GitMvUncommitted
│
├── e2e_count_hunks_test.go       # count-hunksサブコマンドの統合テスト
│   └── TestCountHunks_CLI
│
├── e2e_workflows_test.go         # 実際の使用ワークフロー
│   ├── TestWorkflow_SemanticCommit
│   ├── TestWorkflow_IntentToAdd
│   ├── TestWorkflow_UntrackedFile
│   ├── TestE2E_FinalIntegration
│   └── (semantic_commit_test.goからの7テスト)
│
└── e2e_performance_test.go       # パフォーマンス検証
    ├── TestPerformance_LargeFile
    └── TestE2E_PerformanceWithSafetyChecks
```

### 統合テスト（CLIレイヤー）

```
/
└── main_test.go                  # CLIインターフェーステスト
    ├── TestCLI_SubcommandRouting
    ├── TestCLI_FlagParsing
    └── TestCLI_ErrorMessages
```

---

## 移行計画

**原則**:
- 1 PR = 1つの移行タスク
- レビュー可能なサイズ（変更ファイル5個以内、+/-300行以内を目安）
- 各PR後に全テストがパス

### Phase 0: テスト環境の整備と移行準備（前提条件）

#### 作業内容

**Part 1: テスト環境整備**

**現状**: 13個のテストが環境要因で失敗（gitコミット署名エラー）
- E2Eテスト: 6個失敗
- `internal/stager/semantic_commit_test.go`: 7個失敗

**原因**: gitコミット署名の設定問題（環境依存）

**対応**:
1. ローカル開発環境またはCIでテストを実行
2. 全テストがパスすることを確認
3. カバレッジを測定（ベースライン）

**Part 2: CLAUDE.mdに移行中警告を追加**

**理由**: Phase 1-2の移行中、Claude Codeが古い構造を参照して混乱しないようにする

**追加内容** (CLAUDE.md 行250の直前に挿入):
```markdown
---

## ⚠️ テスト構造移行中の注意

**現在、テスト構造を移行中です。**

新しいテスト配置ルールは `.claude/tmp/2025_10_27_test_structure_design.md` を参照してください。

**移行完了までの暫定ルール**:
- 既存のE2Eテストファイル構造は変更中
- 新規テストの追加は移行計画完了まで待つこと
- 緊急の場合はユーザーに確認すること

このセクションは移行完了後（Phase 3: PR16）に削除されます。

---
```

**完了条件**:
- [ ] `go test ./...` が全てパス
- [ ] テストカバレッジを測定・記録
- [ ] CLAUDE.mdに移行中警告を追加してコミット

**所要時間**: 環境により異なる（20-35分）
  - テスト環境整備: 15-30分
  - CLAUDE.md警告追加: 5分

**重要**: Phase 1以降の作業は、Phase 0完了後に実施すること。

---

### Phase 1: ユニットテストの整理（4 PRs、約105分）

#### PR1: apply_hunk_test.goの統合

**作業内容**:
```
削除:
- internal/stager/apply_hunk_test.go (1 test, 127行)

移動先:
- apply_hunk_test.go → stager_test.go に統合
```

**理由**:
- `apply_hunk_test.go`は`stager.go`の`applyHunk`メソッドをテスト
- 実装ファイルとの1対1対応原則に従い、`stager_test.go`に統合すべき

**影響範囲**: internal/stager/ のみ

**所要時間**: 5分（Claude） + 10分（レビュー） + 5分（修正） = 20分

**レビューポイント**:
- [ ] applyHunkのテストが失われていないか
- [ ] stager_test.goに正しく統合されているか

---

#### PR2: stager関連テストファイルの整理

**作業内容**:
```
削除:
- internal/stager/stager_multi_file_test.go (1 test, 24行)
- internal/stager/stager_safety_test.go (8 tests, 289行)
- internal/stager/stager_e2e_test.go (1 test, 91行)

移動先:
- 全て stager_test.go に統合

特記事項:
- stager_e2e_test.goは削除せず、統合する（曖昧なファイル名のエッジケーステスト）
- TestStageHunks_E2E_AmbiguousFilename → TestStageHunks_AmbiguousFilename にリネーム
```

**理由**:
- `stager_safety_test.go`は`Stager`型のメソッドをテスト（`safety_checker_test.go`とは責務が異なる）
- `stager_e2e_test.go`は曖昧なファイル名のエッジケーステスト（重要なので保持）
- `stager_multi_file_test.go`は複数ファイルステージングのテスト
- 全て`stager.go`のメソッドテストなので`stager_test.go`に統合

**影響範囲**: internal/stager/ のみ

**所要時間**: 10分（Claude） + 20分（レビュー） + 10分（修正） = 40分

**レビューポイント**:
- [ ] 10個のテストが全て統合されているか
- [ ] 曖昧なファイル名のテストが保持されているか
- [ ] ヘルパー関数も正しく移動されているか

---

#### PR3: 特殊ファイルテストの統合

**作業内容**:
```
削除:
- internal/stager/new_file_test.go (4 tests, 456行)

移動先:
- new_file_test.go → special_files_test.go に統合
```

**理由**:
- 新規ファイル処理も「特殊なファイル処理」の一種
- バイナリ、リネーム、削除、新規 → 全て special_files_test.go

**影響範囲**: internal/stager/ のみ

**所要時間**: 8分（Claude） + 15分（レビュー） + 7分（修正） = 30分

**レビューポイント**:
- [ ] 新規ファイルのテストケースが維持されているか
- [ ] 統合後のファイルサイズが適切か（約700行）

---

#### PR4: エラーテストファイルのリネーム

**作業内容**:
```
リネーム:
- internal/stager/safety_errors_test.go → errors_test.go

注意: errors_test.goは現在存在しない
```

**理由**:
- `errors.go` をテストするなら `errors_test.go`
- 現在は`SafetyError`のみだが、将来的に他のエラー型も追加される可能性

**重要な注意事項**:
- 旧版では「統合」と記載していたが、統合先の`errors_test.go`は存在しない
- 実際には「リネーム」が正しい
- リネーム後、ファイル内のコメントを更新して全てのエラー型をテストすることを明記

**影響範囲**: internal/stager/ のみ

**所要時間**: 3分（Claude） + 8分（レビュー） + 4分（修正） = 15分

**レビューポイント**:
- [ ] ファイルが正しくリネームされているか
- [ ] ファイル内のコメントが更新されているか
- [ ] エラー型のテストが失われていないか

---

### Phase 2: E2Eテストの整理（11 PRs、約334分）

#### PR5: count-hunks E2Eの削減

**作業内容**:
```
e2e_count_hunks_test.go:
  削除: TestE2E_CountHunks_NoChanges
  削除: TestE2E_CountHunks_BinaryFiles
  維持: TestE2E_CountHunks_BasicIntegration → TestCountHunks_CLI にリネーム
```

**理由**:
- NoChanges: ユニットテストで検証済み（count_hunks_test.go）
- BinaryFiles: ユニットテストで検証済み
- BasicUsage: CLIインターフェースの動作確認のみ残す

**影響範囲**: e2e_count_hunks_test.go のみ

**所要時間**: 5分（Claude） + 10分（レビュー） + 5分（修正） = 20分

**レビューポイント**:
- [ ] ユニットテストが同じシナリオをカバーしているか確認
- [ ] CLIテストとして適切な粒度か

---

#### PR6: semantic_commit_test.goの移動

**作業内容**:
```
移動:
- internal/stager/semantic_commit_test.go (7 tests, 685行)
  → ルートディレクトリに移動（ファイル名そのまま）

理由:
- semantic_commit_test.goは実際にはワークフローテスト
- internal/stager/に配置されているのは不適切
- E2Eワークフローテストとして再配置
- PR11でe2e_workflows_test.goに統合される

注意: PR11との関係
- このPRではルートディレクトリに移動するだけ
- PR11で他のE2Eファイルと一緒にe2e_workflows_test.goに統合
```

**影響範囲**: internal/stager/とルートディレクトリ

**所要時間**: 5分（Claude） + 10分（レビュー） + 5分（修正） = 20分

**レビューポイント**:
- [ ] 7個のテストが全て移動されているか
- [ ] gitリポジトリ操作を伴うテストが正しく動作するか
- [ ] テストヘルパー関数も適切に移動されているか

---

#### PR7: E2Eエラーテストのユニット化

**作業内容**:
```
Phase 7a: ユニットテストでカバレッジ確認（人間が実行）
  - 各E2Eテストに対応するユニットテストを確認
  - 不足しているテストケースをリストアップ

Phase 7b: 不足テストの追加（Claude Code実行）
  - TestErrorCases_NonExistentFile → errors_test.go に追加
  - TestErrorCases_InvalidHunkNumber → errors_test.go に追加
  - TestErrorCases_EmptyPatchFile → patch_parser_test.go に追加
  - TestErrorCases_HunkCountExceeded → stager_test.go に追加
  - TestErrorCases_MultipleInvalidHunks → stager_test.go に追加
  - TestErrorCases_SameFileConflict → validator_test.go に追加

Phase 7c: E2Eテストの削除（Claude Code実行）
  - e2e_error_test.go を削除
```

**理由**:
- 全てユニットテストで検証可能
- E2Eでのエラーテストは冗長

**前提条件**: PR4完了（errors_test.goが整備済み）

**影響範囲**: e2e_error_test.go 削除、内部テスト複数ファイル更新

**所要時間**: 30分（カバレッジ確認） + 20分（テスト追加） + 10分（削除・検証） = 60分

**レビューポイント**:
- [ ] 各エラーケースがユニットテストでカバーされているか
- [ ] エラーメッセージのフォーマットが維持されているか
- [ ] カバレッジが低下していないか

---

#### PR8: E2E basic テストの移行（Phase 1/3）

**作業内容**:
```
移行:
- e2e_basic_test.go (6 tests, 749行)
  → e2e_stage_test.go に移行（新規作成）

削除:
- TestBasicSetup (セットアップのみ、他で検証済み)

維持:
- TestSingleFileSingleHunk → TestStage_SingleHunk
- TestSingleFileMultipleHunks → TestStage_MultipleHunks
- TestMultipleFilesMultipleHunks → TestStage_MultipleFiles
- TestWildcardStaging → TestStage_Wildcard
- TestWildcardWithMixedInput → TestStage_WildcardMixed
```

**理由**:
- PR8を3つに分割して、レビュー可能なサイズに
- まずbasicテストを新規ファイルに移行

**影響範囲**: e2e_basic_test.go と e2e_stage_test.go（新規）

**所要時間**: 8分（Claude） + 15分（レビュー） + 7分（修正） = 30分

**レビューポイント**:
- [ ] 5個のテストが正しく移行されているか
- [ ] テスト名が命名規則に従っているか
- [ ] e2e_basic_test.goは削除されているか

---

#### PR9: E2E file operation テストの移行（Phase 2/3）

**作業内容**:
```
移行:
- e2e_advanced_files_test.go (5 tests, 641行)
  → e2e_stage_test.go に追加

削除:
- TestBinaryFileHandling (ユニットテストで十分)
- TestMultipleFilesMoveAndModify_Skip (Skip状態)

維持:
- TestFileModificationAndMove → TestStage_FileModify
- TestGitMvThenModifyFile → TestStage_GitMvModify
- TestGitMvThenModifyFileWithoutCommit → TestStage_GitMvUncommitted
```

**理由**:
- ファイル操作系テストをe2e_stage_test.goに統合
- バイナリとスキップテストは削除

**影響範囲**: e2e_advanced_files_test.go と e2e_stage_test.go

**所要時間**: 8分（Claude） + 15分（レビュー） + 7分（修正） = 30分

**レビューポイント**:
- [ ] 3個のテストが正しく追加されているか
- [ ] 削除すべきテストが削除されているか
- [ ] e2e_advanced_files_test.goは削除されているか

---

#### PR10: E2E stage テストの最終調整（Phase 3/3）

**作業内容**:
```
最終調整:
- e2e_stage_test.go の整理
  - ヘルパー関数の統合
  - 重複コードの削除
  - テストの実行順序最適化
  - コメントとドキュメント追加

最終構成: 8テスト、約1,000行
```

**理由**:
- PR8とPR9で統合したテストの最終調整
- コードの整理とドキュメント化

**影響範囲**: e2e_stage_test.go のみ

**所要時間**: 10分（Claude） + 20分（レビュー） + 10分（修正） = 40分

**レビューポイント**:
- [ ] ヘルパー関数が適切に統合されているか
- [ ] 全テストが正常に実行されるか
- [ ] ドキュメントが適切か

---

#### PR11: semantic_commit_test.go の統合準備（Phase 1/4）

**作業内容**:
```
統合:
- semantic_commit_test.go (7 tests, 685行) - PR6で移動済み
  → e2e_workflows_test.go に移行（新規作成）

維持:
- 7つのテスト全てそのまま（既に適切な名前）
```

**理由**:
- PR6でルートに移動したsemantic_commit_test.goを統合開始
- まずこのファイルを新規e2e_workflows_test.goに移行

**影響範囲**: semantic_commit_test.go と e2e_workflows_test.go（新規）

**所要時間**: 8分（Claude） + 15分（レビュー） + 7分（修正） = 30分

**レビューポイント**:
- [ ] 7個のテストが全て移行されているか
- [ ] テストヘルパー関数も移行されているか
- [ ] semantic_commit_test.goは削除されているか

---

#### PR12: E2E semantic テストの統合（Phase 2/4）

**作業内容**:
```
統合:
- e2e_semantic_test.go (1 test, 287行)
  → e2e_workflows_test.go に追加

維持:
- TestMixedSemanticChanges → TestWorkflow_SemanticCommit
```

**理由**:
- セマンティックコミット関連のテストを統合
- ワークフローとしての一貫性を持たせる

**影響範囲**: e2e_semantic_test.go と e2e_workflows_test.go

**所要時間**: 5分（Claude） + 10分（レビュー） + 5分（修正） = 20分

**レビューポイント**:
- [ ] テストが正しく統合されているか
- [ ] テスト名が適切か
- [ ] e2e_semantic_test.goは削除されているか

---

#### PR13: E2E edge cases テストの統合（Phase 3/4）

**作業内容**:
```
統合:
- e2e_advanced_edge_cases_test.go (2 tests, 251行)
  → e2e_workflows_test.go に追加

維持:
- TestIntentToAddFileCoexistence → TestWorkflow_IntentToAdd
- TestUntrackedFile → TestWorkflow_UntrackedFile
```

**理由**:
- エッジケース系のワークフローテストを統合
- 実使用シナリオとして整理

**影響範囲**: e2e_advanced_edge_cases_test.go と e2e_workflows_test.go

**所要時間**: 5分（Claude） + 10分（レビュー） + 5分（修正） = 20分

**レビューポイント**:
- [ ] 2個のテストが正しく統合されているか
- [ ] テスト名が命名規則に従っているか
- [ ] e2e_advanced_edge_cases_test.goは削除されているか

---

#### PR14: E2E integration テストの統合（Phase 4/4）

**作業内容**:
```
統合:
- e2e_integration_test.go (1 test, 390行)
  → e2e_workflows_test.go に追加

最終調整:
- ヘルパー関数の統合と重複削除
- テストの実行順序最適化
- ドキュメント追加

最終構成: 11テスト、約930行

維持:
- TestE2E_FinalIntegration → そのまま
```

**理由**:
- 最終統合テストを追加して完成
- ワークフローテスト全体の最終調整

**影響範囲**: e2e_integration_test.go と e2e_workflows_test.go

**所要時間**: 10分（Claude） + 20分（レビュー） + 10分（修正） = 40分

**レビューポイント**:
- [ ] 全11テストが正しく統合されているか
- [ ] ワークフローテストとしての一貫性があるか
- [ ] テストヘルパー関数が適切に整理されているか
- [ ] e2e_integration_test.goは削除されているか

---

#### PR15: E2E performance 統合

**作業内容**:
```
統合:
- e2e_performance_test.go (1 test, 100行)
- e2e_advanced_performance_test.go (1 test, 222行)
  → e2e_performance_test.go (2テスト)

維持:
- TestE2E_PerformanceWithSafetyChecks → そのまま
- TestLargeFileWithManyHunks → TestPerformance_LargeFile

推奨: 独立ファイルとして e2e_performance_test.go を維持
```

**理由**:
- パフォーマンステストは性質が異なるため独立ファイル推奨
- 実行時間が長いため、通常のE2Eテストと分離
- ベンチマーク的な用途で選択的に実行可能

**影響範囲**: E2Eパフォーマンステスト（2ファイル → 1ファイル）

**所要時間**: 8分（Claude） + 15分（レビュー） + 7分（修正） = 30分

**レビューポイント**:
- [ ] パフォーマンス測定ロジックが維持されているか
- [ ] 性能基準（5秒目標など）が明記されているか
- [ ] テストの独立性が保たれているか

---

### Phase 3: ドキュメント化（1 PR、約25分）

#### PR16: CLAUDE.md のテスト配置ルール更新

**作業内容**:

**削除** (CLAUDE.md 行250-274):
```markdown
## テストファイル分割方針

**重要**: このプロジェクトのE2Eテストは機能別に最適化された構造で分割されています。

### テストファイル構造
- e2e_basic_test.go: 基本機能テスト
- e2e_count_hunks_test.go: count-hunksサブコマンド
- e2e_semantic_test.go: セマンティックコミット分割テスト
- e2e_error_test.go: エラーハンドリング
- e2e_advanced_files_test.go: ファイル操作系テスト
- e2e_advanced_performance_test.go: パフォーマンステスト
- e2e_advanced_edge_cases_test.go: エッジケーステスト

### Claude Code制約事項
1. テストファイルの新規作成禁止: 既存の7つのE2Eテストファイル以外は作成しない
2. テストファイルの自動分割禁止: ファイルサイズや行数を理由に勝手に分割しない
3. テスト内容の変更禁止: 既存テストの動作を変更・削除・追加しない
4. 構造の維持: フラットなファイル構造を維持し、ディレクトリ分割をしない

### 新規テスト追加時
- 新しいテストが必要な場合は、最も関連性の高い既存ファイルに追加する
- テスト分類が不明な場合は、ユーザーに確認する
- 各ファイルの責務範囲を越える場合のみ、ユーザーと相談して対応を決定する
```

**および Phase 0で追加した移行中警告セクション全体を削除**

**置き換え** (同じ位置に以下を挿入):
```markdown
## テスト配置ルール（Claude Code向け）

このプロジェクトは実装ファイルとテストファイルの1対1対応を基本原則としています。

### 新しいテストを追加する時のガイド

#### ユニットテスト
1. テストする関数がどのファイルで定義されているか確認
2. そのファイルに対応する_test.goファイルにテストを追加
3. 対応する_test.goファイルがない場合は作成

**例**:
- `stager.go`の`StageHunks`関数 → `stager_test.go`
- `errors.go`の`NewStagerError`関数 → `errors_test.go`
- `count_hunks.go`の`CountHunksInDiff`関数 → `count_hunks_test.go`

#### E2Eテスト

**4つのファイルに制限**:

1. **`e2e_stage_test.go`** - stageサブコマンドの統合テスト
   - 単一ファイル・単一ハンク
   - 複数ファイル・複数ハンク
   - ワイルドカード
   - ファイル操作（git mv + modify等）

2. **`e2e_count_hunks_test.go`** - count-hunksサブコマンドの統合テスト
   - CLIインターフェースの動作確認

3. **`e2e_workflows_test.go`** - 実際の使用ワークフロー
   - セマンティックコミット分割
   - intent-to-add統合
   - 複雑な統合シナリオ
   - エッジケース

4. **`e2e_performance_test.go`** - パフォーマンス検証
   - 大規模ファイルの処理
   - 性能ベンチマーク

#### 重複チェック
テストを追加する前に以下を確認:
1. 同じ関数のテストが既に存在しないか（grep）
2. E2Eで同じシナリオをテストしていないか
3. ユニットテストで検証できることをE2Eに書こうとしていないか

### ファイル数の上限
- **ユニットテスト**: 実装ファイルと1対1なので制限なし
- **E2Eテスト**: 4ファイルに厳密に制限
- **統合テスト**: main_test.go のみ（CLIインターフェース）

### Claude Code制約事項
1. **E2Eテストファイルの新規作成禁止**: 4つ以外は作成しない
2. **テストファイルの自動分割禁止**: ファイルサイズを理由に勝手に分割しない
3. **構造の維持**: フラットなファイル構造を維持

### 新規テスト追加時
- ユニットテスト: 対応する実装ファイルの_test.goに追加
- E2Eテスト: 上記4ファイルのいずれかに追加
- 分類が不明な場合: ユーザーに確認
```

**理由**:
- Phase 0-2の移行作業が完了し、新しい構造が確立
- 古い7ファイル構造の記述を削除
- 新しい4ファイル構造のルールに置き換え
- Phase 0で追加した「移行中」警告も削除

**影響範囲**: CLAUDE.md のみ

**所要時間**: 8分（Claude） + 12分（レビュー） + 5分（修正） = 25分

**レビューポイント**:
- [ ] Phase 0で追加した移行中警告が削除されているか
- [ ] 旧セクション（行250-274）が完全に削除されているか
- [ ] 新しいルールが4ファイル構造を正確に反映しているか
- [ ] Claude Codeが理解しやすい表現か
- [ ] 具体例が適切か

---

## まとめ

### 移行後の構造

```
ユニットテスト: 12ファイル（-5ファイル）
├─ internal/executor/executor_test.go
├─ internal/validator/validator_test.go
└─ internal/stager/
   ├─ stager_test.go               # StageHunks（メイン、統合済み）
   ├─ count_hunks_test.go          # CountHunksInDiff
   ├─ patch_parser_test.go         # パッチ解析
   ├─ patch_analyzer_test.go       # パッチ分析
   ├─ safety_checker_test.go       # 安全性チェック
   ├─ safety_checker_benchmark_test.go
   ├─ git_status_reader_test.go    # git status読み取り
   ├─ special_files_test.go        # 特殊ファイル（統合済み）
   ├─ errors_test.go               # エラー型（リネーム済み）
   └─ enum_test.go                 # Enum型

E2Eテスト: 4ファイル（-5ファイル）
├─ e2e_stage_test.go              # stageサブコマンド（8テスト）
├─ e2e_count_hunks_test.go        # count-hunksサブコマンド（1テスト）
├─ e2e_workflows_test.go          # 実使用ワークフロー（11テスト）
└─ e2e_performance_test.go        # パフォーマンス検証（2テスト）

統合テスト: 1ファイル
└─ main_test.go                   # CLIインターフェース
```

### PRサマリー（Claude Code作業想定）

| Phase | PR | 変更内容 | 影響ファイル | Claude実行 | レビュー | 修正 | 合計 |
|-------|-----|---------|-------------|-----------|---------|------|------|
| 0 | - | 環境整備+警告追加 | CLAUDE.md | - | - | 20-35分 | 20-35分 |
| 1 | PR1 | apply_hunk統合 | 2ファイル | 5分 | 10分 | 5分 | 20分 |
| 1 | PR2 | stager関連統合 | 4ファイル | 10分 | 20分 | 10分 | 40分 |
| 1 | PR3 | 特殊ファイル統合 | 2ファイル | 8分 | 15分 | 7分 | 30分 |
| 1 | PR4 | エラーファイルリネーム | 1ファイル | 3分 | 8分 | 4分 | 15分 |
| 2 | PR5 | count-hunks削減 | 1ファイル | 5分 | 10分 | 5分 | 20分 |
| 2 | PR6 | semantic移動 | 2ファイル | 5分 | 10分 | 5分 | 20分 |
| 2 | PR7 | E2Eエラーユニット化 | 複数ファイル | - | 30分 | 30分 | 60分 |
| 2 | PR8 | E2E basic移行 (1/3) | 2ファイル | 8分 | 15分 | 7分 | 30分 |
| 2 | PR9 | E2E files移行 (2/3) | 2ファイル | 8分 | 15分 | 7分 | 30分 |
| 2 | PR10 | E2E stage調整 (3/3) | 1ファイル | 10分 | 20分 | 10分 | 40分 |
| 2 | PR11 | semantic統合 (1/4) | 2ファイル | 8分 | 15分 | 7分 | 30分 |
| 2 | PR12 | semantic統合 (2/4) | 2ファイル | 5分 | 10分 | 5分 | 20分 |
| 2 | PR13 | edge cases統合 (3/4) | 2ファイル | 5分 | 10分 | 5分 | 20分 |
| 2 | PR14 | integration統合 (4/4) | 2ファイル | 10分 | 20分 | 10分 | 40分 |
| 2 | PR15 | E2E performance | 3ファイル | 8分 | 15分 | 7分 | 30分 |
| 3 | PR16 | ドキュメント更新 | CLAUDE.md | 8分 | 12分 | 5分 | 25分 |

**合計**:
- **Phase 0**: 20-35分（環境整備 + 移行中警告追加）
- **16 PRs**: 約470分（約7.8時間）
  - Claude実行: 約111分
  - レビュー: 約235分
  - 修正対応: 約124分
- **総計**: 約8.2-8.4時間

**旧計画（第1回レビュー前）との比較**:
- 旧: 7 PRs、31分（Claude実行のみ）
- 第1回改訂: 11 PRs、約364分（約6時間）
- **今回（分割版）: 16 PRs、約470分（約7.8時間）**

**第1回改訂版からの変更**:
- PR8を3つに分割（50分 → 100分）
- PR9を4つに分割（42分 → 110分）
- PR6の時間短縮（42分 → 20分）
- PR16の時間増加（15分 → 25分、削除+置き換えのため）
- Phase 0に警告追加（+5分）
- 合計: 364分 → 470分（+106分、約1.8時間増）

### 想定作業フロー

1. **Phase 0完了確認**:
   - 全テストがパスすることを確認
   - CLAUDE.mdに移行中警告を追加してコミット
2. **プロンプト準備**: この計画書を参照
3. **Claude Codeに指示**: 例「PR1の作業を実行して」
4. **Claude Code実行**: ファイル移動・統合・テスト実行
5. **人間レビュー**: diffを確認（15-30分/PR）
6. **修正対応**: 必要に応じてClaude Codeに修正指示
7. **コミット・プッシュ**: PRを作成
8. **次のPRへ**: 順番通りに実施
9. **Phase 3（PR16）完了後**: 移行中警告が削除され、新構造が確定

**重要**:
- Phase 0で追加する移行中警告により、Phase 1-2実行中にClaude Codeが混乱しない
- PR8-10とPR11-14は段階的な統合プロセスのため、順序を守ること
- PR16で移行中警告と旧構造を削除し、新構造のみが残る

### 効果

**定量的**:
- ユニットテストファイル: 17 → 12（-29%）
- E2Eテストファイル: 9 → 4（-56%）
- 総テストファイル: 27 → 17（-37%）

**定性的**:
- ✅ テストの配置が明確（実装ファイルとの1対1対応）
- ✅ Claude Codeが迷わない（ルールが明示的）
- ✅ 重複が見つけやすい（責務が明確）
- ✅ レビューしやすい（1ファイル = 1責務）
- ✅ テストマップが導出可能（構造から自明）
- ✅ E2Eテストが4つの明確な責務に分類（stage, count-hunks, workflows, performance）

### 実装時の注意事項

1. **Phase 0は必須**: テストが全てパスし、CLAUDE.mdに移行中警告を追加してから移行開始
2. **PR7は人間の判断が必要**: カバレッジ確認を人間が実施
3. **PRの順序厳守**: 依存関係があるため順番通り実施
   - **PR8-10は連続**: E2E stageテストの3段階統合
   - **PR11-14は連続**: E2E workflowsテストの4段階統合
4. **各PR後に全テスト実行**: リグレッション防止
5. **カバレッジ測定**: Phase 0と全PR完了後に測定し、低下していないこと確認
6. **段階的な統合のメリット**:
   - 各PRが20-40分でレビュー可能
   - 問題発生時の切り戻しが容易
   - 進捗が明確に可視化される
7. **テスト名の一貫性**: リネーム時は命名規則に従う（Test[対象]_[シナリオ]）
8. **PR6とPR11の関係**: PR6でsemantic_commit_test.goを移動、PR11で統合開始
9. **PR16は置き換え**: Phase 0で追加した移行中警告と、旧テスト構造セクションの両方を削除
