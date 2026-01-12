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