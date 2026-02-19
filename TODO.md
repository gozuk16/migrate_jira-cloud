# TODO

## 作業状況

### DevDetailsDialog 生レスポンスJSON を IssueData に保存

DevDetailsDialog（GraphQL API）またはDev-Status REST APIから取得した生のAPIレスポンスJSONを
IssueData に `DevStatusRawJSON` フィールドとして追加し、JSONファイルに保存する。

## 作業項目

- [x] jsonsaver.go — `IssueData` に `DevStatusRawJSON json.RawMessage` フィールド追加
- [x] jiraclient.go — `GetDevStatusDetails` / `GetDevStatusGraphQL` の戻り値に `[]byte` 追加
- [x] main.go — `fetchIssue` / `searchIssues` で生バイトを受け取り `IssueData` に設定
- [x] jiraclient_test.go — 戻り値を3つに変更、rawJSON検証追加
- [x] jsonsaver_test.go — `DevStatusRawJSON` のテストケースと検証追加
- [x] ビルド・テスト通過確認
- [ ] コミット・PR作成

## 完了項目

- [x] 全テスト通過確認
- [x] ビルド成功確認
