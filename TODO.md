# TODO

## 作業状況

### 添付Markdownファイルのtagsからバッククオートを除去

JIRAチケットに添付されたMarkdownファイルのフロントマターにある`tags`のバッククオートを除去する。

## 作業項目

- [x] downloader.go — `sanitizeMarkdownFrontMatter()` メソッドを追加
- [x] downloader.go — `downloadFile()` で`.md`ファイルに後処理を追加
- [x] downloader_test.go — `TestSanitizeMarkdownFrontMatter` テストを追加
- [x] テストゴールデンファイル確認・更新（変更なし）
- [x] ビルド・テスト通過確認
- [x] CHANGELOG.md — 更新
- [ ] コミット・PR作成

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
