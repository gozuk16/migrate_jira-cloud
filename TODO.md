# TODO

## 作業状況

### 開発情報にリポジトリ名を追加

DevStatusの各要素（ブランチ/コミット/PR）にリポジトリ名を追加し、Markdown出力をリポジトリ毎にグルーピングして表示する。

## 作業項目

- [x] jiraclient.go — `DevBranch`, `DevRepoCommit`, `DevPullRequest` に `RepositoryName` 追加
- [x] jiraclient.go — `convertGraphQLToDevStatus` で `repo.Name` を設定
- [x] mdwriter.go — `generateDevelopmentInfo` をリポジトリ毎にグルーピング表示
- [x] mdwriter_test.go — テストデータに `RepositoryName` 追加
- [x] testdata/generate-markdown.golden — 更新
- [x] CHANGELOG.md — 更新
- [x] ビルド・テスト通過確認
- [ ] コミット・PR作成

## 完了項目

- [x] 全テスト通過確認
- [x] ビルド成功確認
