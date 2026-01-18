# migrate_jira-cloud

Jira Cloud の課題をMarkdown形式で抽出し、Hugo等の静的サイトジェネレーターで使用可能な形式に変換するツールです。

## 機能

### 基本機能
- **課題の取得**: Jira Cloud REST API を使用して課題情報を取得
- **Markdown変換**: 課題をMarkdown形式に変換
- **添付ファイルのダウンロード**: 課題に含まれる添付ファイルを自動的にダウンロード
- **Front Matter**: Hugo形式のFront Matter（TOML）を生成

### 課題情報の表示
- 課題キー、タイプ、ステータス、優先度
- 担当者、報告者、作成日、更新日
- 期限、ラベル、親課題
- 時間管理フィールド（初期見積り、作業時間、残り時間）
- 解決状況

### 関連情報の表示
- **サブタスク**: 子課題を独立したセクションで表示
- **関連リンク**: 親課題や関連課題をMarkdownリンク形式で表示
- **開発情報**（GitHub/Bitbucket統合）:
  - プルリクエスト情報（PR名、作成者、ブランチ、状態）
  - ブランチ情報とURL

### テキスト変換
- **JIRA記法 → Markdown**: 見出し、リスト、太字、斜体等を自動変換
- **ユーザーメンション**: JIRA形式のメンション（`[~accountid:xxx]`）をHTML形式に変換
- **テーブル抽出**: JIRA形式のテーブルを独立したセクションで抽出

### ビジュアル機能
- **パンくずナビゲーション**: プロジェクト → 課題の階層を表示
- **課題タイプアイコン**: 課題タイプごとに絵文字を表示（Epic 🟣、Story 📗等）

## セットアップ

### 前提条件
- Go 1.24.0以上
- Jira Cloud インスタンスへのアクセス（APIトークン）

### インストール

```bash
go build -o migJira
```

### 設定

`config.toml` ファイルで以下を設定します：

```toml
[jira]
host = "https://your-domain.atlassian.net"
email = "your-email@example.com"
api_token = "your-api-token"

[output]
markdown_dir = "./output/markdown"
attachments_dir = "./output/attachments"

[display]
hidden_custom_fields = ["customfield_10015", "customfield_10019"]

[development]
enabled = false
application_type = "github"  # or "bitbucket", "stash"
```

## 使用方法

### 単一課題の取得

```bash
./migJira get <ISSUE-KEY>
```

### プロジェクトの全課題を取得

```bash
./migJira project <PROJECT-KEY>
```

### JQL検索で課題を取得

```bash
./migJira search "project = TEST AND type = Task"
```

## 出力形式

課題は以下のディレクトリ構造で出力されます：

```
output/
├── markdown/
│   ├── PROJECT1/
│   │   ├── KEY-1.md
│   │   ├── KEY-2.md
│   │   └── _index.md
│   └── PROJECT2/
│       └── KEY-10.md
└── attachments/
    ├── KEY-1_file.pdf
    └── KEY-2_screenshot.png
```

## Front Matter

各課題のMarkdownファイルには、Hugo形式のFront Matter（TOML）が含まれます。

### 基本フィールド
- `title`: 課題のサマリー
- `date`: 作成日時（ISO8601形式）
- `lastmod`: 更新日時（ISO8601形式）
- `project`: プロジェクトキー
- `issue_key`: 課題キー
- `type`: ページタイプ（常に "page"）
- `issue_type`: 課題タイプ（タスク、バグ、エピック等）

### ステータス・担当者フィールド
- `status`: ステータス名（未着手、進行中、完了等）
- `assignee`: 担当者の表示名（未割り当ての場合は "未設定"）

### 日付フィールド
- `startdate`: 開始日（YYYY-MM-DD形式、設定されている場合のみ）
- `duedate`: 期限（YYYY-MM-DD形式、設定されている場合のみ）

### 階層フィールド
- `parent`: 親課題キー（サブタスクの場合のみ）
- `parent_issue_type`: 親課題タイプ（サブタスクの場合のみ）
- `rank`: 優先順位（Scrum/Kanban等で使用、設定されている場合のみ）

### タグ
- `tags`: ラベル配列（Hugo taxonomy対応）

**出力例**:

```toml
+++
title = "ユーザー登録機能の実装"
date = 2025-01-15T10:00:00+09:00
lastmod = 2025-01-18T14:30:00+09:00
project = "PROJ"
issue_key = "PROJ-123"
type = "page"
issue_type = "タスク"
status = "進行中"
assignee = "山田太郎"
startdate = "2025-01-15"
duedate = "2025-01-20"
tags = ["機能追加", "優先度高"]
+++
```

これらのフィールドにより、Hugo等の静的サイトジェネレーターでのフィルタリングやソート機能が向上します。

## テスト

```bash
go test ./...
```

テストカバレッジを確認：

```bash
go test -cover ./...
```

## 開発

### ビルドとテスト

```bash
make build
make test
make coverage
```

### デバッグログの有効化

```bash
export MIGRATE_JIRA_DEBUG=1
./migJira ...
```

## 技術仕様

### API統合
- **Jira REST API v3**: 課題情報の取得
- **Dev-Status API**（非公式）: GitHub/Bitbucket統合情報の取得

### 対応するカスタムフィールド
- テキストフィールド
- 数値フィールド
- 選択フィールド
- ユーザーピッカー
- 日付フィールド
- プルリクエスト統合フィールド

## ライセンス

MIT License