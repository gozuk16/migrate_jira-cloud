# CHANGELOG

このファイルには、プロジェクトの主要な変更履歴を記録します。

## [未リリース]

### 追加
- カスタムステータスラベルの変換機能を追加
  - JIRAの `{color:#FF991F}*[ テキスト ]*{color}` パターンをカスタムステータスラベルとして変換
  - 出力形式: `<span class="status-label status-label-warning">テキスト</span>`
  - 通常の文字色変更 `{color:#XXX}テキスト{color}` とは別処理
  - 対応色マッピング:
    - `#ff991f` → `status-label-warning`（オレンジ/警告）
    - `#00b8d9` → `status-label-teal`（ティール/OK）
    - `#36b37e` → `status-label-success`（緑/成功）
    - `#ff5630` → `status-label-danger`（赤/危険）
    - `#6554c0` → `status-label-purple`（紫）
    - `#97a0af` → `status-label-gray`（グレー）
  - 未知の色は `<span class="status-label">テキスト</span>` として出力
  - CSS参考（Hugoテーマ用）:
    ```css
    .status-label { display: inline-block; padding: 2px 8px; border-radius: 3px; font-size: 0.85em; font-weight: bold; }
    .status-label-warning { background: #fff3cd; color: #856404; }
    .status-label-teal { background: #d1ecf1; color: #0c5460; }
    .status-label-success { background: #d4edda; color: #155724; }
    .status-label-danger { background: #f8d7da; color: #721c24; }
    .status-label-purple { background: #e2d9f3; color: #4a2c7a; }
    .status-label-gray { background: #e9ecef; color: #495057; }
    ```

- {color}マクロの変換機能を追加
  - JIRAの `{color:#XXX}テキスト{color}` をインラインスタイルで変換
  - 出力形式: `<span style="color:#XXX">テキスト</span>`
  - JIRAで指定されたカラーコードをそのまま使用するため、JIRAと完全に一致した色で表示
  - JIRAのカラーパレット（21色）および色名指定（red等）に対応

- ステータスマクロ変換機能を追加
  - JIRAの `{status:colour=Green}テキスト{status}` マクロをHTMLスパンに変換
  - 出力形式: `<span class="status status-green">テキスト</span>`
  - 対応色: Green, Yellow, Red, Blue, Blue-gray, Grey, Gray
  - `colour` と `color` の両方の綴りに対応
  - 色なしの場合は `<span class="status">テキスト</span>` として出力
  - CSS参考（Hugoテーマ用）:
    ```css
    .status { display: inline-block; padding: 2px 8px; border-radius: 3px; font-weight: bold; }
    .status-green { background: #dff0d8; color: #3c763d; }
    .status-yellow { background: #fcf8e3; color: #8a6d3b; }
    .status-red { background: #f2dede; color: #a94442; }
    .status-blue { background: #d9edf7; color: #31708f; }
    .status-gray { background: #f5f5f5; color: #777; }
    ```

- JSON保存機能とオフライン変換コマンドを追加
  - `json_dir` 設定: APIレスポンスをJSONファイルとして保存
  - `convert` コマンド: JSONファイルからMarkdownを生成（APIアクセス不要）
  - `jsonsaver.go`: JSON保存・読み込み機能の実装
    - `IssueData` 構造体: 課題データ、開発情報、親子課題情報を包含
    - `JSONSaver` 構造体: 保存・読み込みメソッドを提供
  - ユースケース: Markdown出力フォーマット変更後の再変換、バッチ処理、バックアップ

- ユーザーメンション変換機能を追加
  - JIRAのメンション形式 `[~accountid:xxx]` をMarkdown形式 `<span class="mention">@username</span>` に変換
  - HTMLの `<span>` タグに `class="mention"` を付与してCSSでスタイル可能に対応
  - `UserMapping` 型を追加し、アカウントIDから表示名へのマッピングを実装
  - `customfields.go`: `BuildUserMappingFromIssue` 関数を追加
  - Reporter、Assignee、Comment作成者、変更履歴の作成者を自動的にマッピング

- 期限フィールドを追加
  - 課題の期限（Duedate）をMarkdown出力に追加
  - 期限が設定されている場合のみ "YYYY-MM-DD" 形式で表示

- 時間管理フィールドを追加
  - 初期見積り（Original Estimate）
  - 残り時間（Remaining Estimate）
  - 作業時間（Time Spent）
  - 各フィールドは値が設定されている場合のみ表示

- ラベルと親課題フィールドを追加
  - ラベル（Labels）: 課題に設定されているラベルをカンマ区切りで表示
  - 親課題（Parent）: サブタスクの親課題キーを表示
  - 各フィールドは値が設定されている場合のみ表示

- サブタスクと関連リンクセクションを追加
  - サブタスク（Subtasks）: 課題のサブタスクを独立したセクションとして表示
    - 各サブタスク: キー、タイトル、ステータスを表示
  - 関連リンク（Issue Links）: 課題の関連リンクを独立したセクションとして表示
    - Outward/Inward両方の関連を表示
    - 各リンク: リンクタイプ、関連課題キー、タイトル、ステータスを表示

- 課題キーのMarkdownリンク化
  - サブタスク、関連リンク、親課題の課題キーをクリック可能なMarkdownリンクに変換
  - リンク形式: `[KEY](../KEY/)` （ディレクトリベースURL）
  - Hugoなどの静的サイトジェネレーターでのディレクトリ構造に対応
  - 各課題は `output/markdown/{PROJECT}/{ISSUE-KEY}/index.md` として出力される想定

- 開発統合フィールドの表示改善
  - Bitbucket Cloud、GitHub等のプルリクエスト統合フィールドを人間が読める形式で表示
  - `isDevelopmentField` 関数を追加: 開発フィールドを検出
  - `formatDevelopmentField` 関数を追加: プルリクエスト数と状態を抽出
  - 表示形式: "Pull Request: N open/merged/closed"
  - 情報が抽出できない複雑なフィールドは非表示

- 開発情報の詳細取得機能（Dev-Status API統合）
  - JIRA Dev-Status API（非公式）を使用してブランチ名、PR名、PR作成者などの詳細情報を取得
  - 新しい「開発情報」セクションをMarkdown出力に追加
    - プルリクエスト: PR名、作成者、ブランチ名、状態、URLを表示
    - ブランチ: ブランチ名とURLを表示
  - 設定ファイル（config.toml）に開発情報詳細取得の有効/無効設定を追加
    - `[development]` セクション: `enabled` (デフォルト: false)、`application_type` ("github", "bitbucket", "stash")
  - `jiraclient.go`: Dev-Status API用の構造体とメソッドを追加
    - `DevStatusDetail`, `DevStatusDetailItem`, `DevBranch`, `DevPullRequest`, `DevPullRequestBranch`, `DevAuthor`
    - `GetDevStatusDetails()` メソッド: Dev-Status APIから開発情報の詳細を取得
  - `customfields.go`: 開発フィールドの詳細表示機能を追加
    - `FormatDevelopmentFieldWithDetails()` 関数: Dev-Status詳細情報を含むフォーマット
    - `extractDevelopmentSummary()` ヘルパー関数: サマリー情報を抽出
  - `mdwriter.go`: Markdown出力の拡張
    - `WriteIssue()` と `generateMarkdown()` のシグネチャに `devStatus` パラメータを追加
    - 開発情報セクション（## 開発情報）を追加
    - カスタムフィールド表示で開発フィールドを詳細表示または非表示に変更
  - `main.go`: fetchIssueとsearchIssues関数でDev-Status API呼び出しを統合
    - config.Development.Enabledがtrueの場合のみAPI呼び出し
    - API失敗時は警告ログのみで処理継続（既存機能に影響なし）
  - 基本情報セクションから開発フィールドを非表示化
    - 詳細情報は「開発情報」セクションで表示するため、重複を避ける
  - 非公式APIのため、設定でデフォルト無効、失敗時のフォールバック処理を実装

- フロントマターに新しいフィールドを追加
  - ステータス（status）: 課題のステータス名を常に出力
  - 担当者（assignee）: 担当者の表示名を常に出力（未割り当ての場合は "未設定"）
  - 開始日（startdate）: Start dateフィールドの値（設定されている場合のみ、YYYY-MM-DD形式）
  - 期限（duedate）: 課題の期限（設定されている場合のみ、YYYY-MM-DD形式）
  - これらのフィールドにより、Hugo等の静的サイトジェネレーターでのフィルタリングやソート機能が向上

### 変更
- 時間表示形式を小数点形式に変更
  - JIRAの文字列形式（例：2h 30m）から小数点形式（例：2.50h）に変更
  - `formatTimeSeconds` メソッドを追加し、秒数を時間に変換
  - TimeTrackingの秒数フィールド（`OriginalEstimateSeconds`、`RemainingEstimateSeconds`、`TimeSpentSeconds`）を使用
  - JIRAのConfiguration API設定（1日=X時間）は既に秒数に反映されているため、秒数÷3600で正しい時間数を計算

- 開発統合フィールドの表示改善（型の問題を修正）
  - 開発フィールド（customfield_10000）がstring型として格納されていることを発見
  - `FormatCustomFieldValue` 関数のstring caseに開発フィールドのパターン検出を追加
  - 文字列中に "pullrequest=" または "\"pullrequest\"" が含まれる場合、ユーザーフレンドリーなメッセージを表示
  - 表示メッセージ: "Development field (詳細はJIRAで確認)"
  - リフレクションを使った汎用的なマップ型処理を追加（default caseでreflect.Kindを使用）
  - デバッグログの追加と削除により根本原因を特定

- 開発フィールドの表示を詳細セクションに移動
  - 基本情報セクションから開発フィールドを非表示化（空文字列を返す）
  - 「開発情報」セクションで詳細情報を表示するため、基本情報での重複表示を避ける
  - `FormatCustomFieldValue` 関数: 文字列型の開発フィールドは空文字列を返すように変更
  - `mdwriter.go`: カスタムフィールド表示で `isDevelopmentField` を使用して開発フィールドを検出
    - 開発フィールドの場合は `FormatDevelopmentFieldWithDetails` を使用
    - 値が空の場合はフィールド自体を非表示

### テスト

#### mdwriter_test.go
- `TestConvertStatusMarkup`: ステータスマクロ変換のテスト（11ケース）
  - 各色（Green, Yellow, Red, Blue, Grey, Gray, Blue-gray）、色なし、複数ステータス、大文字小文字混在
- `TestConvertJIRAMention`: ユーザーメンション変換のテスト（5ケース）
- `TestDuedateField`: 期限フィールドのテスト（2ケース）
- `TestTimeTrackingFields`: 時間管理フィールドのテスト（4ケース）
- `TestFormatTimeSeconds`: 秒数から時間への変換テスト（10ケース）
  - 0秒、1時間、30分、7時間15分、1時間30分、2時間30分、15分、2時間、8時間、10時間
- `TestLabelsAndParentFields`: ラベルと親課題フィールドのテスト（6ケース）
  - 単一ラベル、複数ラベル、ラベル無し、親課題有り、親課題無し、両方有り
- `TestSubtasksField`: サブタスクフィールドのテスト（3ケース）
  - サブタスク有り、サブタスク無し、ステータスnil
  - 期待値を `../KEY/` 形式のリンクに更新
- `TestIssueLinksField`: 関連リンクフィールドのテスト（5ケース）
  - Outward関連、Inward関連、両方有り、関連無し、Fieldsがnil
  - 期待値を `../KEY/` 形式のリンクに更新

#### customfields_test.go（新規作成）
- `TestGetSortedCustomFieldKeys`: カスタムフィールドキーのソートテスト（3ケース）
- `TestBuildFieldNameCache`: フィールド名キャッシュ構築テスト（2ケース）
- `TestGetFieldName`: フィールド名取得テスト（3ケース）
- `TestFormatCustomFieldName`: カスタムフィールド名フォーマットテスト（3ケース）
- `TestIsCustomFieldEmpty`: カスタムフィールド空判定テスト（8ケース）
- `TestFormatCustomFieldValue`: カスタムフィールド値フォーマットテスト（16ケース）
  - nil、空文字列、文字列、数値、bool、配列、オブジェクトなど
  - 開発フィールド（pullrequest）のテストケースを追加（2ケース）
- `TestBuildUserMappingFromIssue`: 課題からユーザーマッピング構築テスト（4ケース）
- `TestBuildUserMapping`: 複数課題からユーザーマッピング構築テスト（2ケース）
- `TestIsDevelopmentField`: 開発フィールド検出のテスト（5ケース）
- `TestFormatDevelopmentField`: 開発フィールド整形のテスト（5ケース）

#### テストサマリー
- テストファイル数: 2個（mdwriter_test.go、customfields_test.go）
- テストケース合計: 120個（すべてパス、109個から11個追加）
- テストカバレッジ: 46.3%（34.2%から12.1%向上）

### 技術的な詳細
- `mdwriter.go`: `convertStatusMarkup()` メソッドを追加（ステータスマクロ変換）
- `mdwriter.go`: `mapStatusColor()` ヘルパー関数を追加（JIRA色名→CSSクラス名のマッピング）
- `mdwriter.go`: `MarkdownWriter` 構造体に `userMapping` フィールドを追加
- `mdwriter.go`: `convertJIRAMarkupToMarkdown` メソッドでメンション変換を実装
- `mdwriter.go`: ラベル（`issue.Fields.Labels`）と親課題（`issue.Fields.Parent`）の表示を追加
  - 親課題もMarkdownリンク形式で表示: `[KEY](../KEY/)`
- `mdwriter.go`: サブタスク（`issue.Fields.Subtasks`）と関連リンク（`issue.Fields.IssueLinks`）の独立したセクションを追加
  - サブタスク: 各サブタスクをMarkdownリンク形式で表示（227-240行目）
    - 形式: `[KEY](../KEY/)` （課題キー → ディレクトリへのリンク）
  - 関連リンク: Outward/Inward両方の関連をMarkdownリンク形式で表示（242-274行目）
    - 形式: `[KEY](../KEY/)` （課題キー → ディレクトリへのリンク）
  - ディレクトリベースURLを使用し、Hugoなどの静的サイトジェネレーターに対応
- `customfields.go`: 開発統合フィールドの特別処理を追加
  - `isDevelopmentField` 関数: 開発フィールド（Bitbucket、GitHub等）を検出（93-99行目）
  - `formatDevelopmentField` 関数: プルリクエスト数と状態を抽出（101-147行目）
  - `FormatCustomFieldValue` 関数: 開発フィールドの特別処理を追加（196-200行目）
  - `IsCustomFieldEmpty` 関数: 開発フィールドの空判定を追加（85-91行目）
- `customfields.go`: 開発フィールドの型の問題を修正
  - `reflect` パッケージをインポートに追加（リフレクションによる型判定）
  - `FormatCustomFieldValue` 関数のstring caseに開発フィールドパターン検出を追加（166-177行目）
  - default caseにリフレクションベースのマップ処理を追加（220-250行目）
  - デバッグログを使用して開発フィールドがstring型として格納されていることを特定
- `main.go`: `fetchIssue` と `searchIssues` 関数で `UserMapping` を構築して使用
