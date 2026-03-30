package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config はアプリケーション設定を表す構造体
type Config struct {
	JIRA         JIRAConfig        `toml:"jira"`
	Output       OutputConfig      `toml:"output"`
	Search       SearchConfig      `toml:"search"`
	Development  DevelopmentConfig `toml:"development"`
	Display      DisplayConfig     `toml:"display"`
	Confluence   ConfluenceConfig  `toml:"confluence"`
	Convert      ConvertConfig     `toml:"convert"`
	URLReplacements []URLReplacement  `toml:"url_replacements"` // URLプレフィックス置換マッピング
	DeletedUsers    map[string]string `toml:"deletedUsers"`     // 削除済みユーザーのマッピング（accountId -> displayName）
	configDir    string            // 設定ファイルのディレクトリ（内部用、TOMLタグなし）
}

// ProjectKeyCachePath はプロジェクトキーキャッシュファイルのパスを返す
func (c *Config) ProjectKeyCachePath() string {
	return filepath.Join(c.configDir, "project_keys.json")
}

// UserMappingCachePath はUserMappingキャッシュファイルのパスを返す
func (c *Config) UserMappingCachePath() string {
	return filepath.Join(c.configDir, "user_mapping.json")
}

// SearchConfig は検索設定を表す構造体
type SearchConfig struct {
	DefaultJQL string `toml:"default_jql"` // デフォルトのJQLクエリ
}

// JIRAConfig はJIRA接続情報を表す構造体
type JIRAConfig struct {
	URL       string `toml:"url"`        // JIRA Cloud URL (例: https://your-domain.atlassian.net)
	ServerURL string `toml:"server_url"` // オンプレJIRA Server URL（移行元、オプション）
	Email     string `toml:"email"`      // JIRAユーザーのメールアドレス
	APIToken  string `toml:"api_token"`  // JIRA API Token
}

// URLReplacement はURLプレフィックス置換のマッピングを表す構造体
type URLReplacement struct {
	From string `toml:"from"` // 置換前URLプレフィックス
	To   string `toml:"to"`   // 置換後URLプレフィックス
}

// OutputConfig は出力設定を表す構造体
type OutputConfig struct {
	MarkdownDir    string `toml:"markdown_dir"`    // Markdown出力ディレクトリ
	AttachmentsDir string `toml:"attachments_dir"` // 添付ファイル保存ディレクトリ
	JSONDir        string `toml:"json_dir"`        // JSON出力ディレクトリ（空の場合はJSON保存しない）
}

// DevelopmentConfig は開発情報取得の設定を表す構造体
type DevelopmentConfig struct {
	Enabled         bool   `toml:"enabled"`          // 開発情報詳細取得の有効化（デフォルト: false）
	ApplicationType string `toml:"application_type"` // "github", "bitbucket", "stash"
	APIType         string `toml:"api_type"`         // "rest" or "graphql"（デフォルト: "rest"）
}

// DisplayConfig は表示設定を表す構造体
type DisplayConfig struct {
	HiddenCustomFields []string `toml:"hidden_custom_fields"` // 基本情報セクションで非表示にするカスタムフィールドIDのリスト
	RankFieldId        string   `toml:"rank_field_id"`        // RankフィールドのカスタムフィールドID（デフォルト: customfield_10019）
	StartDateFieldId   string   `toml:"start_date_field_id"`  // Start dateフィールドのカスタムフィールドID（デフォルト: customfield_10015）
}

// ConfluenceConfig はConfluence関連の設定を表す構造体
type ConfluenceConfig struct {
	IgnoredTitles []string `toml:"ignored_titles"` // 無視するタイトルのリスト（これらのタイトルはURLそのものが表示される）
}

// ConvertConfig はconvertコマンドの設定を表す構造体
type ConvertConfig struct {
	Workers int `toml:"workers"` // 並行実行するworker数（デフォルト: 4）
}

// LoadConfig は指定されたパスからTOML設定ファイルを読み込む
func LoadConfig(path string) (*Config, error) {
	var config Config

	// ファイルの存在確認
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("設定ファイルが見つかりません: %s", path)
	}

	// TOMLファイルをデコード
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return nil, fmt.Errorf("設定ファイルの読み込みに失敗しました: %w", err)
	}

	// 設定ファイルのディレクトリを記録（キャッシュファイルの置き場所として使用）
	config.configDir = filepath.Dir(path)

	// バリデーション
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("設定ファイルのバリデーションエラー: %w", err)
	}

	return &config, nil
}

// Validate は設定値の妥当性をチェックする
func (c *Config) Validate() error {
	if c.JIRA.URL == "" {
		return fmt.Errorf("jira.urlが設定されていません")
	}
	if c.JIRA.Email == "" {
		return fmt.Errorf("jira.emailが設定されていません")
	}
	if c.JIRA.APIToken == "" {
		return fmt.Errorf("jira.api_tokenが設定されていません")
	}

	// デフォルト値の設定
	if c.Output.MarkdownDir == "" {
		c.Output.MarkdownDir = "output/markdown"
	}
	// AttachmentsDirはconvertコマンドでの旧データ移行用（オプション）

	// Development設定のデフォルト値
	if c.Development.ApplicationType == "" {
		c.Development.ApplicationType = "bitbucket" // デフォルトはBitbucket
	}
	if c.Development.APIType == "" {
		c.Development.APIType = "rest" // デフォルトはREST API
	}

	// Display設定のデフォルト値
	if c.Display.RankFieldId == "" {
		c.Display.RankFieldId = "customfield_10019" // デフォルトはcustomfield_10019
	}
	if c.Display.StartDateFieldId == "" {
		c.Display.StartDateFieldId = "customfield_10015" // デフォルトはcustomfield_10015
	}

	// Convert設定のデフォルト値
	if c.Convert.Workers == 0 {
		c.Convert.Workers = 4 // デフォルトは4 worker
	}

	return nil
}
