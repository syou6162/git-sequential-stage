# テスト構造設計と移行計画

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
e2e_stage_test.go          stageサブコマンドの統合動作      3-5
e2e_count_hunks_test.go    count-hunksサブコマンドの統合動作 2-3
e2e_workflows_test.go      実際の使用ワークフロー           2-3
```

**ルール**:
- サブコマンド単位で1ファイル
- 実際のgitリポジトリでの動作のみをテスト
- ユニットテストで検証できることはE2Eに書かない
- 1ファイル5テスト以内を目安とする

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
│   ├── TestStage_BasicUsage
│   ├── TestStage_Wildcard
│   └── TestStage_MultipleFiles
│
├── e2e_count_hunks_test.go       # count-hunksサブコマンドの統合テスト
│   ├── TestCountHunks_BasicUsage
│   └── TestCountHunks_BinaryFiles
│
└── e2e_workflows_test.go         # 実際の使用ワークフロー
    ├── TestWorkflow_SemanticCommitSplitting
    ├── TestWorkflow_IntentToAddIntegration
    └── TestWorkflow_LargeFilePerformance
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
- レビュー可能なサイズ（変更ファイル5個以内、+/-300行以内）
- 各PR後に全テストがパス

### Phase 0: テスト環境の整備（前提条件）

#### 作業内容

**現状**: 13個のテストが環境要因で失敗（gitコミット署名エラー）
- E2Eテスト: 6個失敗
- `internal/stager/semantic_commit_test.go`: 7個失敗

**原因**: gitコミット署名の設定問題（環境依存）

**対応**:
1. ローカル開発環境またはCIでテストを実行
2. 全テストがパスすることを確認
3. カバレッジを測定（ベースライン）

**完了条件**:
- [ ] `go test ./...` が全てパス
- [ ] テストカバレッジを測定・記録

**所要時間**: 環境により異なる（15-30分）

**重要**: Phase 1以降の作業は、Phase 0完了後に実施すること。

---

### Phase 1: ユニットテストの整理（4 PRs）

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

**レビューポイント**: applyHunkのテストが失われていないか

---

#### PR2: stager_safety_test.goとstager_e2e_test.goの整理

**作業内容**:
```
削除:
- internal/stager/stager_safety_test.go (8 tests, 289行)
- internal/stager/stager_e2e_test.go (1 test, 91行)

移動先:
- stager_safety_test.go → stager_test.go に統合（Stagerの統合テスト）
- stager_e2e_test.go → stager_test.go に統合（リネーム: TestStageHunks_AmbiguousFilename）
```

**理由**:
- `stager_safety_test.go`は`Stager`型のメソッドをテスト（`safety_checker_test.go`とは責務が異なる）
- `stager_e2e_test.go`は曖昧なファイル名のエッジケーステスト（重要なので保持）
- 両方とも`stager_test.go`に統合

**影響範囲**: internal/stager/ のみ

**レビューポイント**:
- 安全性チェックのテストが失われていないか
- 曖昧なファイル名のエッジケーステストが保持されているか

---

#### PR3: 特殊ファイルテストの統合

**作業内容**:
```
削除:
- internal/stager/new_file_test.go (4 tests)

移動先:
- new_file_test.go → special_files_test.go に統合
```

**理由**:
- 新規ファイル処理も「特殊なファイル処理」の一種
- バイナリ、リネーム、削除、新規 → 全て special_files_test.go

**影響範囲**: internal/stager/ のみ

**レビューポイント**: 新規ファイルのテストケースが維持されているか

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
- 計画書の旧版では「統合」と記載していたが、統合先の`errors_test.go`は存在しない
- 実際には「リネーム」が正しい
- リネーム後、ファイル内のコメントを更新して全てのエラー型をテストすることを明記

**影響範囲**: internal/stager/ のみ

**レビューポイント**:
- ファイルが正しくリネームされているか
- エラー型のテストが失われていないか

---

### Phase 2: E2Eテストの整理（6 PRs）

#### PR5: count-hunks E2Eの削減

**作業内容**:
```
e2e_count_hunks_test.go:
  削除: TestE2E_CountHunks_NoChanges
  削除: TestE2E_CountHunks_BinaryFiles
  維持: TestCountHunks_BasicUsage (リネーム)
```

**理由**:
- NoChanges: ユニットテストで検証済み（count_hunks_test.go）
- BinaryFiles: ユニットテストで検証済み
- BasicUsage: CLIインターフェースの動作確認のみ残す

**影響範囲**: e2e_count_hunks_test.go のみ

**レビューポイント**: ユニットテストが同じシナリオをカバーしているか確認

---

#### PR5: E2Eエラーテストの削除

**作業内容**:
```
削除: e2e_error_test.go (6 tests)

理由: 全てユニットテストで検証可能
- TestErrorCases_NonExistentFile → errors_test.go
- TestErrorCases_InvalidHunkNumber → errors_test.go
- TestErrorCases_EmptyPatchFile → errors_test.go
- TestErrorCases_HunkCountExceeded → errors_test.go
- TestErrorCases_MultipleInvalidHunks → errors_test.go
- TestErrorCases_SameFileConflict → validator_test.go
```

**前提条件**: PR3完了（errors_test.goが整備済み）

**影響範囲**: e2e_error_test.go 削除、内部テスト少し追加

**レビューポイント**:
- 各エラーケースがユニットテストでカバーされているか
- エラーメッセージのフォーマットが維持されているか

---

#### PR6: E2Eファイルの統合とリネーム

**作業内容**:
```
統合:
- e2e_basic_test.go (6) + e2e_advanced_files_test.go (5)
  → e2e_stage_test.go (5-6テスト)

削除:
- TestBasicSetup (他のテストで暗黙的に検証)
- TestBinaryFileHandling (ユニットで検証済み)
- TestMultipleFilesMoveAndModify_Skip (Skip状態)

統合:
- e2e_semantic_test.go (1)
- e2e_advanced_edge_cases_test.go (2)
- e2e_integration_test.go (1)
  → e2e_workflows_test.go (4テスト)

統合:
- e2e_performance_test.go (1)
- e2e_advanced_performance_test.go (1)
  → e2e_workflows_test.go に1テスト追加
```

**理由**:
- 機能（stage / count-hunks / workflows）で分類
- advanced/basic の区別は不要
- パフォーマンステストもワークフローの一種

**影響範囲**: E2Eファイル全体の再構成

**レビューポイント**:
- 重要なテストケースが失われていないか
- ファイルの責務が明確になっているか

---

### Phase 3: ドキュメント化（1 PR）

#### PR7: CLAUDE.md にルール追記

**作業内容**:
```markdown
## テスト配置ルール（Claude Code向け）

### 新しいテストを追加する時のガイド

#### ユニットテスト
1. テストする関数がどのファイルで定義されているか確認
2. そのファイルに対応する_test.goファイルにテストを追加
3. 対応する_test.goファイルがない場合は作成

例:
- `stager.go`の`StageHunks`関数 → `stager_test.go`
- `errors.go`の`NewStagerError`関数 → `errors_test.go`

#### E2Eテスト
1. サブコマンドの統合テスト → そのサブコマンドのe2eファイル
   - stageコマンド → `e2e_stage_test.go`
   - count-hunksコマンド → `e2e_count_hunks_test.go`

2. 実際の使用ワークフロー → `e2e_workflows_test.go`
   - セマンティックコミット分割
   - intent-to-add統合
   - パフォーマンス検証

#### 重複チェック
テストを追加する前に以下を確認:
1. 同じ関数のテストが既に存在しないか（grep）
2. E2Eで同じシナリオをテストしていないか
3. ユニットテストで検証できることをE2Eに書こうとしていないか

### ファイル数の上限
- ユニットテスト: 実装ファイルと1対1なので制限なし
- E2Eテスト: 3ファイル（stage, count-hunks, workflows）に制限
```

**影響範囲**: CLAUDE.md のみ

**レビューポイント**: Claude Codeが理解しやすい表現か

---

## まとめ

### 移行後の構造

```
ユニットテスト: 12ファイル（-5ファイル）
├─ internal/executor/executor_test.go
├─ internal/validator/validator_test.go
└─ internal/stager/
   ├─ stager_test.go               # StageHunks（メイン）
   ├─ count_hunks_test.go          # CountHunksInDiff
   ├─ patch_parser_test.go         # パッチ解析
   ├─ patch_analyzer_test.go       # パッチ分析
   ├─ safety_checker_test.go       # 安全性（統合済み）
   ├─ safety_checker_benchmark_test.go
   ├─ git_status_reader_test.go    # git status読み取り
   ├─ special_files_test.go        # 特殊ファイル（統合済み）
   ├─ errors_test.go               # エラー型（統合済み）
   └─ enum_test.go                 # Enum型

E2Eテスト: 3ファイル（-6ファイル）
├─ e2e_stage_test.go              # stageサブコマンド（5-6テスト）
├─ e2e_count_hunks_test.go        # count-hunksサブコマンド（1-2テスト）
└─ e2e_workflows_test.go          # 実使用ワークフロー（4-5テスト）

統合テスト: 1ファイル
└─ main_test.go                   # CLIインターフェース
```

### PRサマリー（Claude Code作業想定）

| Phase | PR | 変更内容 | 影響ファイル | Claude実行 | レビュー | 修正対応 | 合計 |
|-------|-----|---------|-------------|-----------|---------|---------|------|
| 1 | PR1 | stagerテストの命名統一 | 3ファイル削除、1ファイル更新 | 5分 | 15分 | 10分 | 30分 |
| 1 | PR2 | 特殊ファイルテスト統合 | 1ファイル削除、1ファイル更新 | 3分 | 10分 | 5分 | 18分 |
| 1 | PR3 | エラーテスト統一 | 1ファイル削除、1ファイル更新 | 3分 | 10分 | 5分 | 18分 |
| 2 | PR4 | count-hunks E2E削減 | 1ファイル更新 | 3分 | 10分 | 5分 | 18分 |
| 2 | PR5 | E2Eエラーテスト削除 | 1ファイル削除、2ファイル更新 | 5分 | 20分 | 10分 | 35分 |
| 2 | PR6 | E2Eファイル統合 | 9ファイル → 3ファイル | 10分 | 30分 | 15分 | 55分 |
| 3 | PR7 | ドキュメント化 | 1ファイル更新 | 2分 | 10分 | 5分 | 17分 |

**合計**: 7 PRs、約3時間（Claude Code実行31分 + 人間レビュー105分 + 修正対応55分）

**想定作業フロー**:
1. プロンプト準備（この計画書を参照）
2. Claude Codeに指示を出す（例：「PR1の作業を実行して」）
3. Claude Codeがファイル移動・統合・テスト実行
4. 人間がdiffをレビュー
5. 必要に応じてClaude Codeに修正指示
6. コミット・プッシュ

### 効果

**定量的**:
- ユニットテストファイル: 17 → 12（-29%）
- E2Eテストファイル: 9 → 3（-67%）
- 総テストファイル: 27 → 16（-41%）

**定性的**:
- ✅ テストの配置が明確（実装ファイルとの1対1対応）
- ✅ Claude Codeが迷わない（ルールが明示的）
- ✅ 重複が見つけやすい（責務が明確）
- ✅ レビューしやすい（1ファイル = 1責務）
- ✅ テストマップが導出可能（構造から自明）

---
---

## 【2025-10-27 更新】3エージェントレビュー反映版

### レビュー実施結果

**レビュー方法**: 3つの独立したサブエージェントによる並行レビュー
- エージェント1: 技術的実現可能性（Goエンジニア視点）
- エージェント2: レビュー可能性（厳格なレビュアー視点）
- エージェント3: Claude Code実行可能性

**評価結果**:
- 技術的実現可能性: **2.5/5** (実現困難)
- レビュー可能性: **2/5** (レビュー困難)
- Claude Code実行可能性: **3/5** (中程度の難易度)

### 主要な指摘事項

#### 重大な問題（3エージェント共通）

1. **Phase 0が不足**
   - 13個のテストが環境要因で失敗している
   - 全テストをパスさせてから移行を開始すべき

2. **PR4の記述エラー**
   - 「errors_test.goに統合」と記載されているが、errors_test.goは存在しない
   - 正しくは「safety_errors_test.go を errors_test.go にリネーム」

3. **PR番号の問題（Phase 1）**
   - apply_hunk_test.go、stager_multi_file_test.goの扱いが不明確
   - stager_e2e_test.goを削除すると記載されているが、重要なエッジケーステスト

4. **旧PR6が大きすぎる**
   - 9ファイル、推定2,640行の変更
   - レビュー不可能（基準300行の9倍）
   - 3つのPRに分割すべき

5. **semantic_commit_test.goの扱いが不明**
   - 685行の大きなテストファイルが計画に含まれていない

6. **時間見積もりが楽観的**
   - 計画: 31分
   - 実際予想: 115-155分（3.7-5倍）

### 修正版移行計画

#### Phase 0: テスト環境の整備（前提条件）

**目的**: 全テストをパスさせる

**作業内容**:
1. ローカル開発環境またはCI環境でテストを実行
2. gitコミット署名エラーを解決
3. 全テストがパスすることを確認
4. テストカバレッジを測定（ベースライン）

**完了条件**:
- [ ] `go test ./...` が全てパス
- [ ] カバレッジ: X% (記録)

**所要時間**: 15-30分（環境により異なる）

---

#### Phase 1: ユニットテストの整理（4 PRs → 115-120分）

##### PR1: apply_hunk_test.goの統合

**作業内容**:
```
削除:
- internal/stager/apply_hunk_test.go (1 test, 127行)

移動先:
- stager_test.go に統合（stager.goのメソッドなので）
```

**所要時間**: 5分（Claude） + 10分（レビュー） + 5分（修正） = 20分

**レビューポイント**:
- [ ] applyHunkのテストが失われていないか
- [ ] stager_test.goに正しく統合されているか

---

##### PR2: stager関連テストファイルの整理

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

**所要時間**: 10分（Claude） + 20分（レビュー） + 10分（修正） = 40分

**レビューポイント**:
- [ ] 10個のテストが全て統合されているか
- [ ] 曖昧なファイル名のテストが保持されているか
- [ ] ヘルパー関数も正しく移動されているか

---

##### PR3: 特殊ファイルテストの統合

**作業内容**:
```
削除:
- internal/stager/new_file_test.go (4 tests, 456行)

移動先:
- special_files_test.go に統合
```

**所要時間**: 8分（Claude） + 15分（レビュー） + 7分（修正） = 30分

**レビューポイント**:
- [ ] 新規ファイルのテストケースが維持されているか
- [ ] 統合後のファイルサイズが700行程度（許容範囲）

---

##### PR4: エラーテストファイルのリネーム

**作業内容**:
```
リネーム:
- internal/stager/safety_errors_test.go → errors_test.go

注意: 
- 計画書の旧版では「統合」と記載していたが誤り
- errors_test.goは存在しないため、リネームが正しい
```

**所要時間**: 3分（Claude） + 8分（レビュー） + 4分（修正） = 15分

**レビューポイント**:
- [ ] ファイルが正しくリネームされているか
- [ ] ファイル内のコメントが更新されているか

---

#### Phase 2: E2Eテストの整理（7 PRs → 140-180分）

##### PR5: count-hunks E2Eの削減

**作業内容**:
```
e2e_count_hunks_test.go:
  削除: TestE2E_CountHunks_NoChanges
  削除: TestE2E_CountHunks_BinaryFiles
  維持: TestE2E_CountHunks_BasicIntegration → TestCountHunks_CLI にリネーム
```

**所要時間**: 5分（Claude） + 10分（レビュー） + 5分（修正） = 20分

---

##### PR6: semantic_commit_test.goの移動

**作業内容**:
```
移動:
- internal/stager/semantic_commit_test.go (7 tests, 685行)
  → ルートディレクトリの e2e_workflows_test.go に統合

理由:
- semantic_commit_test.goは実際にはワークフローテスト
- internal/stager/に配置されているのは不適切
```

**所要時間**: 12分（Claude） + 20分（レビュー） + 10分（修正） = 42分

**レビューポイント**:
- [ ] 7個のテストが全て移動されているか
- [ ] gitリポジトリ操作を伴うテストが正しく動作するか

---

##### PR7: E2Eエラーテストのユニット化

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

**所要時間**: 30分（カバレッジ確認） + 20分（テスト追加） + 10分（削除・検証） = 60分

**前提条件**: PR4完了（errors_test.go存在）

---

##### PR8: E2E basic + files 統合

**作業内容**:
```
統合:
- e2e_basic_test.go (6 tests, 749行)
- e2e_advanced_files_test.go (5 tests, 641行)
  → e2e_stage_test.go (8-9 tests)

削除:
- TestBasicSetup (セットアップのみ、他で検証済み)
- TestBinaryFileHandling (ユニットテストで十分)
- TestMultipleFilesMoveAndModify_Skip (Skip状態)

維持:
- TestSingleFileSingleHunk → TestStage_SingleHunk
- TestSingleFileMultipleHunks → TestStage_MultipleHunks
- TestMultipleFilesMultipleHunks → TestStage_MultipleFiles
- TestWildcardStaging → TestStage_Wildcard
- TestWildcardWithMixedInput → TestStage_WildcardMixed
- TestFileModificationAndMove → TestStage_FileModify
- TestGitMvThenModifyFile → TestStage_GitMvModify
- TestGitMvThenModifyFileWithoutCommit → TestStage_GitMvUncommitted
```

**所要時間**: 15分（Claude） + 25分（レビュー） + 10分（修正） = 50分

---

##### PR9: E2E workflows 統合

**作業内容**:
```
統合:
- e2e_semantic_test.go (1 test, 287行)
- e2e_advanced_edge_cases_test.go (2 tests, 251行)
- e2e_integration_test.go (1 test, 390行)
- semantic_commit_test.go からの移動分 (7 tests, 685行)
  → e2e_workflows_test.go (10-11 tests)

維持:
- TestMixedSemanticChanges → TestWorkflow_SemanticCommit
- TestIntentToAddFileCoexistence → TestWorkflow_IntentToAdd
- TestUntrackedFile → TestWorkflow_UntrackedFile
- TestE2E_FinalIntegration → そのまま
- semantic_commit_test.goの7テスト → そのまま（既に適切な名前）
```

**所要時間**: 12分（Claude） + 20分（レビュー） + 10分（修正） = 42分

---

##### PR10: E2E performance 統合

**作業内容**:
```
統合:
- e2e_performance_test.go (1 test, 100行)
- e2e_advanced_performance_test.go (1 test, 222行)
  → e2e_workflows_test.go に追加 または独立ファイルとして維持

推奨: 独立ファイルとして e2e_performance_test.go を維持
  - TestE2E_PerformanceWithSafetyChecks → そのまま
  - TestLargeFileWithManyHunks → TestPerformance_LargeFile
```

**所要時間**: 8分（Claude） + 15分（レビュー） + 7分（修正） = 30分

---

#### Phase 3: ドキュメント化（1 PR → 15分）

##### PR11: CLAUDE.md にルール追記

**作業内容**:
```markdown
## テスト配置ルール（Claude Code向け）

### 新しいテストを追加する時のガイド

#### ユニットテスト
1. テストする関数がどのファイルで定義されているか確認
2. そのファイルに対応する_test.goファイルにテストを追加
3. 対応する_test.goファイルがない場合は作成

例:
- stager.goのStageHunks関数 → stager_test.go
- errors.goのNewStagerError関数 → errors_test.go

#### E2Eテスト
1. サブコマンドの統合テスト → そのサブコマンドのe2eファイル
   - stageコマンド → e2e_stage_test.go
   - count-hunksコマンド → e2e_count_hunks_test.go

2. 実際の使用ワークフロー → e2e_workflows_test.go
   - セマンティックコミット分割
   - intent-to-add統合

3. パフォーマンステスト → e2e_performance_test.go

#### 重複チェック
テストを追加する前に以下を確認:
1. 同じ関数のテストが既に存在しないか（grep）
2. E2Eで同じシナリオをテストしていないか
3. ユニットテストで検証できることをE2Eに書こうとしていないか

### ファイル数の上限
- ユニットテスト: 実装ファイルと1対1なので制限なし
- E2Eテスト: 4ファイル（stage, count-hunks, workflows, performance）に制限
```

**所要時間**: 5分（Claude） + 8分（レビュー） + 2分（修正） = 15分

---

### 修正後のサマリー

| Phase | PR | 変更内容 | 影響ファイル | Claude実行 | レビュー | 修正 | 合計 |
|-------|-----|---------|-------------|-----------|---------|------|------|
| 0 | - | テスト環境整備 | テスト実行のみ | - | - | 15-30分 | 15-30分 |
| 1 | PR1 | apply_hunk統合 | 2ファイル | 5分 | 10分 | 5分 | 20分 |
| 1 | PR2 | stager関連統合 | 4ファイル | 10分 | 20分 | 10分 | 40分 |
| 1 | PR3 | 特殊ファイル統合 | 2ファイル | 8分 | 15分 | 7分 | 30分 |
| 1 | PR4 | エラーファイルリネーム | 1ファイル | 3分 | 8分 | 4分 | 15分 |
| 2 | PR5 | count-hunks削減 | 1ファイル | 5分 | 10分 | 5分 | 20分 |
| 2 | PR6 | semantic移動 | 2ファイル | 12分 | 20分 | 10分 | 42分 |
| 2 | PR7 | E2Eエラーユニット化 | 複数ファイル | - | 30分 | 30分 | 60分 |
| 2 | PR8 | E2E basic+files | 3ファイル | 15分 | 25分 | 10分 | 50分 |
| 2 | PR9 | E2E workflows | 5ファイル | 12分 | 20分 | 10分 | 42分 |
| 2 | PR10 | E2E performance | 3ファイル | 8分 | 15分 | 7分 | 30分 |
| 3 | PR11 | ドキュメント化 | 1ファイル | 5分 | 8分 | 2分 | 15分 |

**合計**: 
- **Phase 0**: 15-30分
- **11 PRs**: 約360分（6時間）
- **総計**: 6-6.5時間

**旧計画との比較**:
- 旧: 7 PRs、31分（Claude実行のみ）
- 新: 11 PRs、約100分（Claude実行）+ 約170分（レビュー）+ 約100分（修正）
- **実際の所要時間が5倍以上に**

### 修正版の改善点

1. ✅ Phase 0追加（テスト環境整備）
2. ✅ PR4の記述修正（「統合」→「リネーム」）
3. ✅ apply_hunk_test.go、stager_multi_file_test.go、stager_e2e_test.goの扱いを明確化
4. ✅ semantic_commit_test.goの移動を計画に追加
5. ✅ 旧PR6を3つに分割（PR8, PR9, PR10）
6. ✅ PR7（E2Eエラーユニット化）を3段階に分割
7. ✅ 時間見積もりを現実的に修正（31分 → 100分Claude実行）
8. ✅ E2Eテストファイル数を4に（stage, count-hunks, workflows, performance）

### 実装時の注意事項

1. **Phase 0は必須**: テストが全てパスしてから移行開始
2. **PR7は人間の判断が必要**: カバレッジ確認を人間が実施
3. **PRの順序厳守**: 依存関係があるため順番通り実施
4. **各PR後に全テスト実行**: リグレッション防止
5. **カバレッジ測定**: Phase 0と全PR完了後に測定し、低下していないこと確認

