# TODO

## 作業状況

### トップページのプロジェクト一覧を名前昇順に表示

- [x] `hugo-jira/themes/hugo-theme-issues/layouts/home.html` — `range sort` で Title 昇順ソートを追加
- [x] CHANGELOG.md — 更新
- [x] コミット・PR作成

### convertコマンドの並行実行 + Confluenceスペース名の事前保存

- [x] jsonsaver.go — `IssueData`に`ConfluenceSpaces`フィールドを追加
- [x] config.go — `ConvertConfig`構造体を追加（`workers`設定）
- [x] mdwriter.go — `SetConfluenceSpaces()`メソッドを追加、`generateConfluenceLinks()`を事前解決優先に変更
- [x] main.go — `resolveConfluenceSpaces()`共通関数を追加
- [x] main.go — `fetchIssue`/`searchIssues`でConfluenceスペース名を事前解決してJSONに保存
- [x] main.go — `convertFromJSON`に`--workers`フラグを追加し並行実行対応
- [x] ビルド・テスト通過確認
- [x] テスト強化（レースコンディション含む）
  - [x] confluenceclient_test.go — SpaceCache並行アクセステスト、GetSpaceName HTTPモックテスト
  - [x] main_test.go — resolveConfluenceSpaces, successCount並行安全性, convertFromJSON並行実行テスト
  - [x] mdwriter_test.go — generateConfluenceLinks事前解決優先テスト（APIを呼ばないことも確認）
  - [x] jsonsaver_test.go — ConfluenceSpaces serialize/deserializeテスト
  - [x] go test -race ./... 通過確認
- [x] CHANGELOG.md — 更新
- [x] コミット・PR作成

### 添付Markdownファイルのtagsからバッククオートを除去

JIRAチケットに添付されたMarkdownファイルのフロントマターにある`tags`のバッククオートを除去する。

## 作業項目

- [x] downloader.go — `sanitizeMarkdownFrontMatter()` メソッドを追加
- [x] downloader.go — `downloadFile()` で`.md`ファイルに後処理を追加
- [x] downloader_test.go — `TestSanitizeMarkdownFrontMatter` テストを追加
- [x] テストゴールデンファイル確認・更新（変更なし）
- [x] ビルド・テスト通過確認
- [x] CHANGELOG.md — 更新
- [x] コミット・PR作成

### 添付ファイル名のUnicode正規化（NFC）対応

- [x] go.mod/go.sum — `golang.org/x/text` 依存追加
- [x] downloader.go — `sanitizeFilename()` にNFC正規化を追加
- [x] main.go — `sanitizeFilenameForConvert()` にNFC正規化を追加
- [x] downloader_test.go — NFD→NFC正規化のテストケースを追加
- [x] ビルド・テスト通過確認
- [x] CHANGELOG.md — 更新
- [ ] コミット・PR作成

### ボールド+イタリック修正 & 下線サポート

- [x] mdwriter.go — ステップ9-1として `**_text_**` → `***text***` 変換を追加
- [x] mdwriter.go — ステップ14として `+text+` → `<u>text</u>` 変換を追加
- [x] mdwriter_test.go — `TestConvertJIRAMarkupToMarkdown_BoldItalic` テストを追加
- [x] mdwriter_test.go — `TestConvertJIRAMarkupToMarkdown_Underline` テストを追加
- [x] ビルド・テスト通過確認
- [x] CHANGELOG.md — 更新
- [x] コミット・PR作成（#100）

## 完了項目

### renderedFields廃止 + プレーンテキスト課題リンク変換

`renderedFields.description` 廃止による `<ul><li>` 問題の解消と、プロジェクトキーキャッシュを使ったプレーンテキスト課題リンク変換の追加。

- [x] jiraclient.go — `GetAllProjects()` メソッドを追加
- [x] config.go — `configDir` フィールドと `ProjectKeyCachePath()` メソッドを追加
- [x] projectkeys.go — 新規作成（`ProjectKeyCache` 構造体、`LoadProjectKeys`、`SaveProjectKeys`）
- [x] mdwriter.go — `MarkdownWriter` に `projectKeys` フィールドと `SetProjectKeys()` を追加
- [x] mdwriter.go — `generateDescription()` を簡素化（renderedFields廃止）
- [x] mdwriter.go — `convertPlainTextIssueKeysToLinks()` メソッドを追加
- [x] mdwriter.go — `convertJIRAMarkupToMarkdown()` に呼び出しを追加
- [x] main.go — `fetchIssue`/`searchIssues` でプロジェクトキー取得・キャッシュ保存・MW設定
- [x] main.go — `convertFromJSON` でキャッシュ読み込み・MW設定
- [x] projectkeys_test.go — キャッシュ読み書きテスト
- [x] mdwriter_test.go — `convertPlainTextIssueKeysToLinks()` テスト追加
- [x] ビルド・テスト通過確認
- [x] CHANGELOG.md — 更新
- [x] コミット・PR作成

### Jira旧仕様の[URL]リンク記法をMarkdownリンクに変換

- [x] mdwriter.go — `[URL]` → `[URL](URL)` の変換処理を追加
- [x] mdwriter_test.go — `TestConvertSimpleLink` テストを追加
- [x] ビルド・テスト通過確認
- [x] CHANGELOG.md — 更新
- [x] コミット・PR作成
