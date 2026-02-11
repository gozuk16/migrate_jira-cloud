package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/andygrunwald/go-jira/v2/cloud"
)

// Downloader は添付ファイルのダウンロードを管理する
type Downloader struct {
	client   *http.Client
	email    string
	apiToken string
}

// NewDownloader は新しいDownloaderを作成する
func NewDownloader(email, apiToken string) *Downloader {
	return &Downloader{
		client:   &http.Client{},
		email:    email,
		apiToken: apiToken,
	}
}

// DownloadAttachments は課題の添付ファイルを指定ディレクトリにすべてダウンロードする
func (d *Downloader) DownloadAttachments(issue *cloud.Issue, targetDir string) ([]string, error) {
	if issue.Fields == nil || issue.Fields.Attachments == nil {
		return []string{}, nil
	}

	// 出力ディレクトリの作成
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("添付ファイルディレクトリの作成に失敗しました: %w", err)
	}

	var downloadedFiles []string

	for _, attachment := range issue.Fields.Attachments {
		filename, err := d.downloadFile(attachment, targetDir)
		if err != nil {
			return downloadedFiles, fmt.Errorf("添付ファイル %s のダウンロードに失敗しました: %w", attachment.Filename, err)
		}
		downloadedFiles = append(downloadedFiles, filename)
	}

	return downloadedFiles, nil
}

// downloadFile は単一の添付ファイルを指定ディレクトリにダウンロードする
func (d *Downloader) downloadFile(attachment *cloud.Attachment, targetDir string) (string, error) {
	safeFilename := d.sanitizeFilename(attachment.Filename)
	filename := safeFilename
	filepath := filepath.Join(targetDir, filename)

	// すでにファイルが存在する場合はスキップ
	if _, err := os.Stat(filepath); err == nil {
		return filename, nil
	}

	// HTTPリクエストの作成
	req, err := http.NewRequest("GET", attachment.Content, nil)
	if err != nil {
		return "", fmt.Errorf("HTTPリクエストの作成に失敗しました: %w", err)
	}

	// Basic認証ヘッダーの設定
	req.SetBasicAuth(d.email, d.apiToken)

	// ファイルのダウンロード
	resp, err := d.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTPリクエストに失敗しました: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ダウンロードに失敗しました。ステータスコード: %d", resp.StatusCode)
	}

	// ファイルの保存
	outFile, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("ファイルの作成に失敗しました: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		return "", fmt.Errorf("ファイルの書き込みに失敗しました: %w", err)
	}

	return filename, nil
}

// sanitizeFilename はファイル名を安全な形式にサニタイズする
func (d *Downloader) sanitizeFilename(filename string) string {
	// パス区切り文字などの危険な文字を置換
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		"..", "_",
		":", "_",
	)
	return replacer.Replace(filename)
}

// IsImageFile はファイル名が画像ファイルかどうかを判定する
func IsImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	imageExts := []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".bmp", ".ico"}
	for _, imageExt := range imageExts {
		if ext == imageExt {
			return true
		}
	}
	return false
}
