# TODO

## 作業状況

### PRのマージ先ブランチ名を取得・表示

GraphQLクエリとDevPullRequest構造体にマージ先ブランチ名を追加し、Markdown出力に表示する。

## 作業項目

- [x] jiraclient.go — GraphQLクエリに `destinationBranchName` 追加
- [x] jiraclient.go — `DevPullRequest` に `Destination` フィールド追加
- [x] jiraclient.go — `convertGraphQLToDevStatus` の3箇所のPR変換を更新
- [x] mdwriter.go — PR表示にマージ先ブランチを追加
- [x] CHANGELOG.md — 更新
- [x] ビルド・テスト通過確認
- [ ] コミット・PR作成

## 完了項目

- [x] 全テスト通過確認
- [x] ビルド成功確認
