# TODO

## 作業状況

### Jira旧仕様 `[URL]` リンクのMarkdown変換対応

Jiraの古い仕様 `[URL]` をMarkdownの `[URL](URL)` に変換する機能を追加。

## 作業項目

- [x] mdwriter.go — `[URL]` → `[URL](URL)` の変換処理を追加
- [x] mdwriter_test.go — `TestConvertSimpleLink` テストを追加
- [x] ビルド・テスト通過確認
- [x] CHANGELOG.md — 更新
- [ ] コミット・PR作成

## 完了項目

### 日付処理のリファクタリング

- [x] mdwriter.go — 日付フォーマット定数 `dateFormatDateTime`, `dateFormatDate`, `jiraTimeLayout` を定義
- [x] mdwriter.go — `formatCommentDate` を削除し `formatTimeString` に統合
- [x] mdwriter.go — `formatRFC3339` 関数を追加（開発情報セクション用）
- [x] mdwriter.go — `formatTime`, `formatTimeString` で定数を使用
- [x] mdwriter.go — 期限表示で `dateFormatDate` 定数を使用
- [x] mdwriter.go — 開発情報セクションの3箇所を `formatRFC3339` で置換
- [x] testdata/generate-markdown.golden — 更新
- [x] mdwriter_test.go — コメントテストの期待値を更新
- [x] ビルド・テスト通過確認
- [x] CHANGELOG.md — 更新
- [x] コミット・PR作成
