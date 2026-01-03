package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfig はLoadConfig関数のテスト
func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(t *testing.T) string // テスト用ファイルを作成して、そのパスを返す
		wantErr     bool
		errContains string
	}{
		{
			name: "正常系: 有効な設定ファイル",
			setupFunc: func(t *testing.T) string {
				t.Helper()
				tmpDir := t.TempDir()
				configPath := filepath.Join(tmpDir, "config.toml")
				content := `[jira]
url = "https://test.atlassian.net"
email = "test@example.com"
api_token = "test-token-123"

[output]
markdown_dir = "output/markdown"
attachments_dir = "output/attachments"

[search]
default_jql = "project = TEST"

[development]
enabled = true
application_type = "github"
`
				if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
					t.Fatalf("テスト用設定ファイルの作成に失敗: %v", err)
				}
				return configPath
			},
			wantErr: false,
		},
		{
			name: "異常系: ファイルが存在しない",
			setupFunc: func(t *testing.T) string {
				return "/path/to/nonexistent/config.toml"
			},
			wantErr:     true,
			errContains: "設定ファイルが見つかりません",
		},
		{
			name: "異常系: 無効なTOML形式",
			setupFunc: func(t *testing.T) string {
				t.Helper()
				tmpDir := t.TempDir()
				configPath := filepath.Join(tmpDir, "config.toml")
				content := `[jira
invalid toml syntax
`
				if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
					t.Fatalf("テスト用設定ファイルの作成に失敗: %v", err)
				}
				return configPath
			},
			wantErr:     true,
			errContains: "設定ファイルの読み込みに失敗しました",
		},
		{
			name: "異常系: 必須項目が欠落（jira.url）",
			setupFunc: func(t *testing.T) string {
				t.Helper()
				tmpDir := t.TempDir()
				configPath := filepath.Join(tmpDir, "config.toml")
				content := `[jira]
email = "test@example.com"
api_token = "test-token-123"
`
				if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
					t.Fatalf("テスト用設定ファイルの作成に失敗: %v", err)
				}
				return configPath
			},
			wantErr:     true,
			errContains: "jira.urlが設定されていません",
		},
		{
			name: "異常系: 必須項目が欠落（jira.email）",
			setupFunc: func(t *testing.T) string {
				t.Helper()
				tmpDir := t.TempDir()
				configPath := filepath.Join(tmpDir, "config.toml")
				content := `[jira]
url = "https://test.atlassian.net"
api_token = "test-token-123"
`
				if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
					t.Fatalf("テスト用設定ファイルの作成に失敗: %v", err)
				}
				return configPath
			},
			wantErr:     true,
			errContains: "jira.emailが設定されていません",
		},
		{
			name: "異常系: 必須項目が欠落（jira.api_token）",
			setupFunc: func(t *testing.T) string {
				t.Helper()
				tmpDir := t.TempDir()
				configPath := filepath.Join(tmpDir, "config.toml")
				content := `[jira]
url = "https://test.atlassian.net"
email = "test@example.com"
`
				if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
					t.Fatalf("テスト用設定ファイルの作成に失敗: %v", err)
				}
				return configPath
			},
			wantErr:     true,
			errContains: "jira.api_tokenが設定されていません",
		},
		{
			name: "正常系: デフォルト値が設定される（output未指定）",
			setupFunc: func(t *testing.T) string {
				t.Helper()
				tmpDir := t.TempDir()
				configPath := filepath.Join(tmpDir, "config.toml")
				content := `[jira]
url = "https://test.atlassian.net"
email = "test@example.com"
api_token = "test-token-123"
`
				if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
					t.Fatalf("テスト用設定ファイルの作成に失敗: %v", err)
				}
				return configPath
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := tt.setupFunc(t)
			config, err := LoadConfig(configPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("エラーが期待されましたが、nilが返されました")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("エラーメッセージが期待と異なります\n期待: %q を含む\n実際: %q",
						tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("予期しないエラー: %v", err)
				return
			}

			if config == nil {
				t.Error("設定がnilです")
				return
			}

			// 正常系の場合、基本的な検証
			if config.JIRA.URL == "" {
				t.Error("JIRA URLが空です")
			}
			if config.JIRA.Email == "" {
				t.Error("JIRA Emailが空です")
			}
			if config.JIRA.APIToken == "" {
				t.Error("JIRA APITokenが空です")
			}

			// デフォルト値のテスト（tt.nameで判定）
			if contains(tt.name, "デフォルト値") {
				if config.Output.MarkdownDir != "output/markdown" {
					t.Errorf("MarkdownDirのデフォルト値が期待と異なります: %q", config.Output.MarkdownDir)
				}
				if config.Output.AttachmentsDir != "output/attachments" {
					t.Errorf("AttachmentsDirのデフォルト値が期待と異なります: %q", config.Output.AttachmentsDir)
				}
				if config.Development.ApplicationType != "bitbucket" {
					t.Errorf("ApplicationTypeのデフォルト値が期待と異なります: %q", config.Development.ApplicationType)
				}
			}
		})
	}
}

// TestValidate はValidateメソッドのテスト
func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		wantErr     bool
		errContains string
	}{
		{
			name: "正常系: すべての必須項目が設定されている",
			config: Config{
				JIRA: JIRAConfig{
					URL:      "https://test.atlassian.net",
					Email:    "test@example.com",
					APIToken: "test-token-123",
				},
				Output: OutputConfig{
					MarkdownDir:    "output/markdown",
					AttachmentsDir: "output/attachments",
				},
			},
			wantErr: false,
		},
		{
			name: "異常系: jira.urlが空",
			config: Config{
				JIRA: JIRAConfig{
					URL:      "",
					Email:    "test@example.com",
					APIToken: "test-token-123",
				},
			},
			wantErr:     true,
			errContains: "jira.urlが設定されていません",
		},
		{
			name: "異常系: jira.emailが空",
			config: Config{
				JIRA: JIRAConfig{
					URL:      "https://test.atlassian.net",
					Email:    "",
					APIToken: "test-token-123",
				},
			},
			wantErr:     true,
			errContains: "jira.emailが設定されていません",
		},
		{
			name: "異常系: jira.api_tokenが空",
			config: Config{
				JIRA: JIRAConfig{
					URL:   "https://test.atlassian.net",
					Email: "test@example.com",
					APIToken: "",
				},
			},
			wantErr:     true,
			errContains: "jira.api_tokenが設定されていません",
		},
		{
			name: "正常系: デフォルト値が設定される",
			config: Config{
				JIRA: JIRAConfig{
					URL:      "https://test.atlassian.net",
					Email:    "test@example.com",
					APIToken: "test-token-123",
				},
				Output: OutputConfig{
					// 空のまま
				},
				Development: DevelopmentConfig{
					// 空のまま
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("エラーが期待されましたが、nilが返されました")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("エラーメッセージが期待と異なります\n期待: %q を含む\n実際: %q",
						tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("予期しないエラー: %v", err)
				return
			}

			// デフォルト値のテスト
			if contains(tt.name, "デフォルト値") {
				if tt.config.Output.MarkdownDir != "output/markdown" {
					t.Errorf("MarkdownDirのデフォルト値が期待と異なります: %q", tt.config.Output.MarkdownDir)
				}
				if tt.config.Output.AttachmentsDir != "output/attachments" {
					t.Errorf("AttachmentsDirのデフォルト値が期待と異なります: %q", tt.config.Output.AttachmentsDir)
				}
				if tt.config.Development.ApplicationType != "bitbucket" {
					t.Errorf("ApplicationTypeのデフォルト値が期待と異なります: %q", tt.config.Development.ApplicationType)
				}
			}
		})
	}
}

// contains は文字列に部分文字列が含まれるかチェックする
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && hasSubstring(s, substr)))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
