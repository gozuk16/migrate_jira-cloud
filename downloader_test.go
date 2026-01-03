package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/andygrunwald/go-jira/v2/cloud"
)

// TestDownloadAttachments はDownloadAttachmentsメソッドのテスト
func TestDownloadAttachments(t *testing.T) {
	tests := []struct {
		name               string
		issue              *cloud.Issue
		mockServerResponse string
		mockServerStatus   int
		wantErr            bool
		wantFileCount      int
	}{
		{
			name: "正常系: 添付ファイル1件のダウンロード成功",
			issue: &cloud.Issue{
				Key: "TEST-1",
				Fields: &cloud.IssueFields{
					Attachments: []*cloud.Attachment{
						{
							Filename: "test.txt",
							Content:  "", // モックサーバーのURLで上書き
						},
					},
				},
			},
			mockServerResponse: "test file content",
			mockServerStatus:   http.StatusOK,
			wantErr:            false,
			wantFileCount:      1,
		},
		{
			name: "正常系: 添付ファイル複数件のダウンロード",
			issue: &cloud.Issue{
				Key: "TEST-2",
				Fields: &cloud.IssueFields{
					Attachments: []*cloud.Attachment{
						{
							Filename: "file1.txt",
							Content:  "",
						},
						{
							Filename: "file2.jpg",
							Content:  "",
						},
					},
				},
			},
			mockServerResponse: "content",
			mockServerStatus:   http.StatusOK,
			wantErr:            false,
			wantFileCount:      2,
		},
		{
			name: "正常系: 添付ファイルが存在しない",
			issue: &cloud.Issue{
				Key: "TEST-3",
				Fields: &cloud.IssueFields{
					Attachments: nil,
				},
			},
			wantErr:       false,
			wantFileCount: 0,
		},
		{
			name: "正常系: Fieldsがnil",
			issue: &cloud.Issue{
				Key:    "TEST-4",
				Fields: nil,
			},
			wantErr:       false,
			wantFileCount: 0,
		},
		{
			name: "異常系: HTTPエラー（404）",
			issue: &cloud.Issue{
				Key: "TEST-5",
				Fields: &cloud.IssueFields{
					Attachments: []*cloud.Attachment{
						{
							Filename: "notfound.txt",
							Content:  "",
						},
					},
				},
			},
			mockServerStatus: http.StatusNotFound,
			wantErr:          true,
			wantFileCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 一時ディレクトリの作成
			tmpDir := t.TempDir()

			// モックHTTPサーバーの作成（添付ファイルがある場合のみ）
			if tt.issue.Fields != nil && tt.issue.Fields.Attachments != nil {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Basic認証のチェック
					username, password, ok := r.BasicAuth()
					if !ok || username != "test@example.com" || password != "test-token" {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}

					w.WriteHeader(tt.mockServerStatus)
					if tt.mockServerStatus == http.StatusOK {
						w.Write([]byte(tt.mockServerResponse))
					}
				}))
				defer server.Close()

				// 添付ファイルのURLをモックサーバーのURLに設定
				for i := range tt.issue.Fields.Attachments {
					tt.issue.Fields.Attachments[i].Content = server.URL
				}
			}

			// Downloaderの作成
			downloader := NewDownloader(tmpDir, "test@example.com", "test-token")

			// ダウンロードの実行
			files, err := downloader.DownloadAttachments(tt.issue)

			// エラーチェック
			if tt.wantErr {
				if err == nil {
					t.Errorf("エラーが期待されましたが、nilが返されました")
				}
				return
			}

			if err != nil {
				t.Errorf("予期しないエラー: %v", err)
				return
			}

			// ファイル数のチェック
			if len(files) != tt.wantFileCount {
				t.Errorf("ダウンロードされたファイル数が期待と異なります。期待: %d, 実際: %d",
					tt.wantFileCount, len(files))
			}

			// ファイルの存在確認（添付ファイルがある場合）
			for _, filename := range files {
				filePath := filepath.Join(tmpDir, filename)
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					t.Errorf("ファイルが存在しません: %s", filePath)
				}
			}
		})
	}
}

// TestSanitizeFilename はsanitizeFilenameメソッドのテスト
func TestSanitizeFilename(t *testing.T) {
	downloader := NewDownloader("", "", "")

	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "正常系: 通常のファイル名",
			filename: "test.txt",
			want:     "test.txt",
		},
		{
			name:     "パス区切り文字の置換（スラッシュ）",
			filename: "path/to/file.txt",
			want:     "path_to_file.txt",
		},
		{
			name:     "パス区切り文字の置換（バックスラッシュ）",
			filename: "path\\to\\file.txt",
			want:     "path_to_file.txt",
		},
		{
			name:     "危険な文字の置換（..）",
			filename: "../../../etc/passwd",
			want:     "______etc_passwd",
		},
		{
			name:     "コロンの置換",
			filename: "file:name.txt",
			want:     "file_name.txt",
		},
		{
			name:     "複数の特殊文字",
			filename: "path/../file:name\\test.txt",
			want:     "path___file_name_test.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := downloader.sanitizeFilename(tt.filename)
			if got != tt.want {
				t.Errorf("sanitizeFilename() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestIsImageFile はIsImageFile関数のテスト
func TestIsImageFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{
			name:     "画像ファイル: PNG",
			filename: "image.png",
			want:     true,
		},
		{
			name:     "画像ファイル: JPG",
			filename: "photo.jpg",
			want:     true,
		},
		{
			name:     "画像ファイル: JPEG",
			filename: "photo.jpeg",
			want:     true,
		},
		{
			name:     "画像ファイル: GIF",
			filename: "animation.gif",
			want:     true,
		},
		{
			name:     "画像ファイル: SVG",
			filename: "icon.svg",
			want:     true,
		},
		{
			name:     "画像ファイル: WebP",
			filename: "modern.webp",
			want:     true,
		},
		{
			name:     "画像ファイル: BMP",
			filename: "bitmap.bmp",
			want:     true,
		},
		{
			name:     "画像ファイル: ICO",
			filename: "favicon.ico",
			want:     true,
		},
		{
			name:     "画像ファイル: 大文字拡張子",
			filename: "IMAGE.PNG",
			want:     true,
		},
		{
			name:     "非画像ファイル: TXT",
			filename: "document.txt",
			want:     false,
		},
		{
			name:     "非画像ファイル: PDF",
			filename: "document.pdf",
			want:     false,
		},
		{
			name:     "非画像ファイル: ZIP",
			filename: "archive.zip",
			want:     false,
		},
		{
			name:     "非画像ファイル: 拡張子なし",
			filename: "noextension",
			want:     false,
		},
		{
			name:     "非画像ファイル: 空文字列",
			filename: "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsImageFile(tt.filename)
			if got != tt.want {
				t.Errorf("IsImageFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

// TestDownloadAttachmentsDirectoryCreation はディレクトリ作成のテスト
func TestDownloadAttachmentsDirectoryCreation(t *testing.T) {
	// 一時ディレクトリの作成
	tmpDir := t.TempDir()
	attachmentsDir := filepath.Join(tmpDir, "nested", "attachments", "dir")

	// モックHTTPサーバーの作成
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test content"))
	}))
	defer server.Close()

	// Downloaderの作成
	downloader := NewDownloader(attachmentsDir, "test@example.com", "test-token")

	// テスト用のissue
	issue := &cloud.Issue{
		Key: "TEST-DIR",
		Fields: &cloud.IssueFields{
			Attachments: []*cloud.Attachment{
				{
					Filename: "test.txt",
					Content:  server.URL,
				},
			},
		},
	}

	// ダウンロードの実行
	_, err := downloader.DownloadAttachments(issue)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	// ディレクトリが作成されたことを確認
	if _, err := os.Stat(attachmentsDir); os.IsNotExist(err) {
		t.Errorf("ディレクトリが作成されていません: %s", attachmentsDir)
	}
}

// TestDownloadAttachmentsFileExists は既存ファイルのスキップテスト
func TestDownloadAttachmentsFileExists(t *testing.T) {
	// 一時ディレクトリの作成
	tmpDir := t.TempDir()

	// 既存ファイルを作成
	existingFilePath := filepath.Join(tmpDir, "TEST-EXIST_existing.txt")
	if err := os.WriteFile(existingFilePath, []byte("existing content"), 0644); err != nil {
		t.Fatalf("既存ファイルの作成に失敗: %v", err)
	}

	// モックHTTPサーバーの作成（呼ばれないはず）
	serverCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("new content"))
	}))
	defer server.Close()

	// Downloaderの作成
	downloader := NewDownloader(tmpDir, "test@example.com", "test-token")

	// テスト用のissue
	issue := &cloud.Issue{
		Key: "TEST-EXIST",
		Fields: &cloud.IssueFields{
			Attachments: []*cloud.Attachment{
				{
					Filename: "existing.txt",
					Content:  server.URL,
				},
			},
		},
	}

	// ダウンロードの実行
	files, err := downloader.DownloadAttachments(issue)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	// ファイルがスキップされたことを確認
	if serverCalled {
		t.Error("既存ファイルがスキップされませんでした")
	}

	// ファイル内容が変更されていないことを確認
	content, err := os.ReadFile(existingFilePath)
	if err != nil {
		t.Fatalf("ファイルの読み込みに失敗: %v", err)
	}
	if string(content) != "existing content" {
		t.Errorf("ファイル内容が変更されています: %q", string(content))
	}

	// ダウンロードされたファイルリストに含まれることを確認
	if len(files) != 1 || files[0] != "TEST-EXIST_existing.txt" {
		t.Errorf("ファイルリストが期待と異なります: %v", files)
	}
}
