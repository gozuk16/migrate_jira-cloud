package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/andygrunwald/go-jira/v2/cloud"
	"golang.org/x/text/unicode/norm"
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
	outFile.Close()

	// .mdファイルの場合はフロントマターのtagsからバッククオートを除去する
	if strings.HasSuffix(strings.ToLower(safeFilename), ".md") {
		if err := sanitizeMarkdownFrontMatter(filepath); err != nil {
			slog.Warn("フロントマターの整理に失敗しました", "file", filename, "error", err)
		}
	}

	return filename, nil
}

var backtickInTagsPattern = regexp.MustCompile("`([^`]*)`")

// sanitizeMarkdownFrontMatter はMarkdownファイルのYAMLフロントマター内のtags行からバッククオートを除去する
func sanitizeMarkdownFrontMatter(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("ファイルの読み込みに失敗しました: %w", err)
	}

	// 先頭の空行のみ除去してフロントマターを検出（末尾の改行は保持）
	text := strings.TrimLeft(string(content), "\r\n")

	// YAMLフロントマター（---で囲まれた部分）を検出
	if !strings.HasPrefix(text, "---") {
		return nil
	}
	end := strings.Index(text[3:], "\n---")
	if end == -1 {
		return nil
	}
	frontMatter := text[3 : end+3]
	rest := text[end+7:]

	// tags行のバッククオートを除去
	newFrontMatter := strings.Join(processTagsLines(strings.Split(frontMatter, "\n")), "\n")

	if newFrontMatter == frontMatter {
		return nil
	}

	newContent := "---" + newFrontMatter + "\n---" + rest
	return os.WriteFile(filePath, []byte(newContent), 0644)
}

// processTagsLines はフロントマター行のtags行からバッククオートを除去する
func processTagsLines(lines []string) []string {
	result := make([]string, len(lines))
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "tags:") {
			line = backtickInTagsPattern.ReplaceAllString(line, "$1")
		}
		result[i] = line
	}
	return result
}

// sanitizeFilename はファイル名を安全な形式にサニタイズする
func (d *Downloader) sanitizeFilename(filename string) string {
	// Unicode正規化（NFC）: macOSのHFS+/APFSがNFD形式に変換する問題を防ぎ、
	// Linuxホスト環境でのファイル名とMarkdownリンクの一致を保証する
	filename = norm.NFC.String(filename)
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
