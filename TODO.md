# TODO

## 作業状況

### 日付フォーマットの統一とタイムゾーン対応

日付表示を `2006-01-02 15:04:05` に統一し、システムタイムゾーンに変換。フォールバック処理を削除。

## 作業項目

- [x] mdwriter.go — `formatCommentDate` のフォーマット統一・フォールバック削除・`.Local()` 追加
- [x] mdwriter.go — `formatTimeString` のパース形式をJIRA形式に変更・`.Local()` 追加
- [x] mdwriter.go — `formatTime` に `.Local()` 追加
- [x] mdwriter.go — 開発情報セクションの3箇所に `.Local()` 追加
- [x] mdwriter_test.go — コメント日付テストの期待値を秒付きに更新
- [x] testdata/generate-markdown.golden — 更新
- [x] CHANGELOG.md — 更新
- [x] ビルド・テスト通過確認
- [ ] コミット・PR作成

## 完了項目

- [x] 全テスト通過確認
- [x] ビルド成功確認
