# TODO

## 作業状況

### 画像配置をHugo Page Bundle形式に変更

出力構造を変更し、画像をMarkdownファイルと同じディレクトリに配置する。

**現在:** `SCRUM/SCRUM-1.md` + `attachments/SCRUM-1_image.png`
**変更後:** `SCRUM/SCRUM-1/index.md` + `SCRUM/SCRUM-1/image.png`

## 作業項目

- [x] config.go — `AttachmentsDir` をオプション化
- [x] config.toml / config.toml.example — `attachments_dir` をコメントアウト
- [x] downloader.go — ダウンロード先を課題ディレクトリに変更
- [x] mdwriter.go — index.md出力 + 相対パス化
- [x] main.go — 新しいAPIへの配線
- [x] テスト更新（config_test.go, downloader_test.go, mdwriter_test.go, goldenファイル）
- [x] makefile更新
- [x] CHANGELOG.md更新
- [ ] コミット・PR作成

## 完了項目

- [x] 全テスト通過確認
- [x] ビルド成功確認
