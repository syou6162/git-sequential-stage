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

### Phase 1: ユニットテストの整理（3 PRs）

#### PR 1-1: stager パッケージの命名統一

**作業内容**:
```
削除:
- internal/stager/stager_safety_test.go (8 tests)
- internal/stager/stager_multi_file_test.go (1 test)
- internal/stager/stager_e2e_test.go (1 test)

移動先:
- stager_safety_test.go → safety_checker_test.go に統合
- stager_multi_file_test.go → stager_test.go に統合
- stager_e2e_test.go → 削除（真のE2Eでない）
```

**理由**:
- `stager_*_test.go` という命名が曖昧
- `safety_checker.go` をテストするなら `safety_checker_test.go`
- 複数ファイルのテストは `stager_test.go` で十分

**影響範囲**: internal/stager/ のみ

**レビューポイント**: テストが失われていないか

---

#### PR 1-2: 特殊ファイルテストの統合

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

#### PR 1-3: エラーテストの統一

**作業内容**:
```
削除:
- internal/stager/safety_errors_test.go (4 tests)

移動先:
- safety_errors_test.go → errors_test.go に統合
```

**理由**:
- `errors.go` をテストするなら `errors_test.go`
- safety系とそれ以外で分ける理由がない

**影響範囲**: internal/stager/ のみ

**レビューポイント**: エラー型のテストが網羅的か

---

### Phase 2: E2Eテストの整理（3 PRs）

#### PR 2-1: count-hunks E2Eの削減

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

#### PR 2-2: E2Eエラーテストの削除

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

**前提条件**: PR 1-3完了（errors_test.goが整備済み）

**影響範囲**: e2e_error_test.go 削除、内部テスト少し追加

**レビューポイント**:
- 各エラーケースがユニットテストでカバーされているか
- エラーメッセージのフォーマットが維持されているか

---

#### PR 2-3: E2Eファイルの統合とリネーム

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

#### PR 3-1: CLAUDE.md にルール追記

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

### PRサマリー

| Phase | PR | 変更内容 | 影響ファイル | 推定工数 |
|-------|-----|---------|-------------|---------|
| 1 | 1-1 | stagerテストの命名統一 | 3ファイル削除、1ファイル更新 | 2時間 |
| 1 | 1-2 | 特殊ファイルテスト統合 | 1ファイル削除、1ファイル更新 | 1時間 |
| 1 | 1-3 | エラーテスト統一 | 1ファイル削除、1ファイル更新 | 1時間 |
| 2 | 2-1 | count-hunks E2E削減 | 1ファイル更新 | 1時間 |
| 2 | 2-2 | E2Eエラーテスト削除 | 1ファイル削除、2ファイル更新 | 2時間 |
| 2 | 2-3 | E2Eファイル統合 | 9ファイル → 3ファイル | 4時間 |
| 3 | 3-1 | ドキュメント化 | 1ファイル更新 | 1時間 |

**合計**: 7 PRs、12時間（1.5営業日）

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
