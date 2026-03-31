package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andygrunwald/go-jira/v2/cloud"
)

const (
	dateFormatDateTime = "2006-01-02 15:04:05"
	dateFormatDate     = "2006-01-02"
	jiraTimeLayout     = "2006-01-02T15:04:05.000-0700"
)

// AggregateTimeFields は集計時間フィールドを保持する構造体
type AggregateTimeFields struct {
	AggregateTimeOriginalEstimate int `json:"aggregatetimeoriginalestimate"`
	AggregateTimeEstimate         int `json:"aggregatetimeestimate"`
	AggregateTimeSpent            int `json:"aggregatetimespent"`
}

// extractAggregateTimeFields はissueのJSONから集計時間フィールドを抽出する
func extractAggregateTimeFields(issue *cloud.Issue) *AggregateTimeFields {
	// issueをJSONにマーシャルして再度パースすることで、集計フィールドを取得する
	jsonData, err := json.Marshal(issue)
	if err != nil {
		return nil
	}

	// fieldsの中に集計時間フィールドがある
	var rawIssue struct {
		Fields AggregateTimeFields `json:"fields"`
	}
	if err := json.Unmarshal(jsonData, &rawIssue); err != nil {
		return nil
	}

	return &rawIssue.Fields
}

// removeControlCharacters はテキストからコントロールコード（U+0000〜U+001F）を除去する。
// ただし、改行（\n）、キャリッジリターン（\r）、タブ（\t）は保持する。
func removeControlCharacters(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\n' && r != '\r' && r != '\t' {
			return -1
		}
		return r
	}, s)
}

// escapeTOMLString はTOML文字列をエスケープする
func escapeTOMLString(s string) string {
	// コントロールコードを除去
	s = removeControlCharacters(s)
	// バックスラッシュをエスケープ（最初に処理）
	s = strings.ReplaceAll(s, "\\", "\\\\")
	// ダブルクォートをエスケープ
	s = strings.ReplaceAll(s, "\"", "\\\"")
	// 改行を除去
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// MarkdownWriter はMarkdown形式で課題を出力する
type MarkdownWriter struct {
	outputDir        string
	userMapping      UserMapping
	config           *Config
	confluenceClient *ConfluenceClient
	confluenceSpaces map[string]string // pageID -> spaceName（事前解決済み）
	projectKeys      map[string]bool   // プロジェクトキーのセット（プレーンテキストリンク変換用）
}

// NewMarkdownWriter は新しいMarkdownWriterを作成する
func NewMarkdownWriter(outputDir string, userMapping UserMapping, config *Config) *MarkdownWriter {
	if userMapping == nil {
		userMapping = make(UserMapping)
	}
	return &MarkdownWriter{
		outputDir:        outputDir,
		userMapping:      userMapping,
		config:           config,
		confluenceClient: nil,
	}
}

// SetConfluenceClient はConfluenceクライアントを設定
func (mw *MarkdownWriter) SetConfluenceClient(client *ConfluenceClient) {
	mw.confluenceClient = client
}

// SetConfluenceSpaces は事前解決済みのConfluenceスペース名マップを設定する
func (mw *MarkdownWriter) SetConfluenceSpaces(spaces map[string]string) {
	mw.confluenceSpaces = spaces
}

// SetProjectKeys はプロジェクトキー一覧を設定する
func (mw *MarkdownWriter) SetProjectKeys(keys []string) {
	mw.projectKeys = make(map[string]bool, len(keys))
	for _, k := range keys {
		mw.projectKeys[k] = true
	}
}

// WriteIssue は課題をMarkdownファイルに出力する
func (mw *MarkdownWriter) WriteIssue(issue *cloud.Issue, attachmentFiles []string, fieldNameCache FieldNameCache, devStatus *DevStatusDetail, parentInfo *ParentIssueInfo, childIssues []ChildIssueInfo, remoteLinks []cloud.RemoteLink) error {
	// プロジェクトキーを取得
	projectKey := issue.Fields.Project.Key

	// 課題別の出力ディレクトリの作成（Hugo Leaf Bundle形式）
	issueDir := filepath.Join(mw.outputDir, projectKey, issue.Key)
	if err := os.MkdirAll(issueDir, 0755); err != nil {
		return fmt.Errorf("Markdown出力ディレクトリの作成に失敗しました: %w", err)
	}

	// Markdownコンテンツの生成
	content := mw.generateMarkdown(issue, attachmentFiles, fieldNameCache, devStatus, parentInfo, childIssues, remoteLinks)

	// ファイルパスの作成（Leaf Bundle: index.md）
	outputPath := filepath.Join(issueDir, "index.md")

	// ファイルの書き込み
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("Markdownファイルの書き込みに失敗しました: %w", err)
	}

	return nil
}

// WriteProjectIndex はプロジェクトの_index.mdを生成する
func (mw *MarkdownWriter) WriteProjectIndex(project *cloud.Project) error {
	// プロジェクト別の出力ディレクトリの作成
	projectDir := filepath.Join(mw.outputDir, project.Key)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("プロジェクトディレクトリの作成に失敗しました: %w", err)
	}

	var sb strings.Builder

	// Front Matter
	sb.WriteString("+++\n")
	projectIcon := "📦"
	sb.WriteString(fmt.Sprintf("title = \"%s%s\"\n", projectIcon, escapeTOMLString(project.Name)))
	sb.WriteString(fmt.Sprintf("project_key = \"%s\"\n", project.Key))
	sb.WriteString(fmt.Sprintf("project_name = \"%s\"\n", escapeTOMLString(project.Name)))
	sb.WriteString("type = \"project\"\n")
	sb.WriteString("+++\n\n")

	// 本文
	sb.WriteString(fmt.Sprintf("# %s\n\n", project.Name))
	if project.Description != "" {
		sb.WriteString(project.Description)
		sb.WriteString("\n\n")
	}

	// ファイルパスの作成
	indexPath := filepath.Join(projectDir, "_index.md")

	// ファイルの書き込み
	if err := os.WriteFile(indexPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("_index.mdファイルの書き込みに失敗しました: %w", err)
	}

	return nil
}

// generateMarkdown は課題情報からMarkdownコンテンツを生成する
func (mw *MarkdownWriter) generateMarkdown(issue *cloud.Issue, attachmentFiles []string, fieldNameCache FieldNameCache, devStatus *DevStatusDetail, parentInfo *ParentIssueInfo, childIssues []ChildIssueInfo, remoteLinks []cloud.RemoteLink) string {
	var sb strings.Builder

	// 添付ファイルのマッピングを作成（元のファイル名 → 保存されたファイル名）
	attachmentMap := mw.buildAttachmentMap(issue, attachmentFiles)

	// Front Matter
	mw.generateFrontMatter(&sb, issue, parentInfo)

	// タイトル
	mw.generateTitle(&sb, issue, parentInfo)

	sb.WriteString("<!-- PAGE_RIGHT_START -->\n\n")

	// 基本情報
	mw.generateBasicInfo(&sb, issue, fieldNameCache, devStatus)

	// 開発情報
	mw.generateDevelopmentInfo(&sb, devStatus)

	sb.WriteString("<!-- PAGE_RIGHT_END -->\n\n")

	// 説明
	mw.generateDescription(&sb, issue, attachmentMap)

	// 子作業項目（子課題が存在する場合）
	mw.generateChildIssues(&sb, childIssues)

	// Confluenceコンテンツ
	mw.generateConfluenceLinks(&sb, remoteLinks)

	// コメント
	mw.generateComments(&sb, issue, attachmentMap)

	// サブタスク
	mw.generateSubtasks(&sb, issue)

	// 関連リンク
	mw.generateIssueLinks(&sb, issue)

	// 添付ファイル
	mw.generateAttachments(&sb, attachmentFiles)

	// 変更履歴
	mw.generateChangeHistory(&sb, issue)

	return sb.String()
}

// getUser はユーザー情報から表示名を取得する
func (mw *MarkdownWriter) getUser(user *cloud.User) string {
	if user == nil {
		return "未設定"
	}

	// accountTypeが"unknown"の場合（削除済みユーザー）、設定からマッピングを検索
	if user.AccountType == "unknown" && user.AccountID != "" {
		if mw.config != nil && mw.config.DeletedUsers != nil {
			if name, ok := mw.config.DeletedUsers[user.AccountID]; ok {
				return name
			}
		}
		// マッピングがない場合はDisplayNameを返す
	}

	return user.DisplayName
}

// getAvatarURL はユーザーのアバターURLを取得する（24x24サイズ）
func (mw *MarkdownWriter) getAvatarURL(user *cloud.User) string {
	if user == nil {
		return ""
	}

	// AvatarUrlsが存在しない場合は空文字を返す
	if user.AvatarUrls.Two4X24 == "" {
		return ""
	}

	return user.AvatarUrls.Two4X24
}

// getFieldString はフィールド情報から文字列を取得する
func (mw *MarkdownWriter) getFieldString(field interface{}) string {
	if field == nil {
		return "未設定"
	}
	if priority, ok := field.(*cloud.Priority); ok {
		if priority == nil {
			return "未設定"
		}
		return priority.Name
	}
	return fmt.Sprintf("%v", field)
}

// formatTime は時刻をフォーマットする
func (mw *MarkdownWriter) formatTime(jiraTime cloud.Time) string {
	return time.Time(jiraTime).Local().Format(dateFormatDateTime)
}

// formatTimeISO8601 は時刻をISO8601形式でフォーマットする（Front Matter用）
func (mw *MarkdownWriter) formatTimeISO8601(jiraTime cloud.Time) string {
	return time.Time(jiraTime).Format(time.RFC3339)
}

// formatTimeSeconds は秒数を小数点形式の時間（h）に変換する
func (mw *MarkdownWriter) formatTimeSeconds(seconds int) string {
	if seconds == 0 {
		return ""
	}

	hours := float64(seconds) / 3600.0
	return fmt.Sprintf("%.2fh", hours)
}

// formatTimeString はJIRA形式の文字列の時刻をフォーマットする
func (mw *MarkdownWriter) formatTimeString(timeStr string) string {
	t, err := time.Parse(jiraTimeLayout, timeStr)
	if err != nil {
		return timeStr
	}
	return t.Local().Format(dateFormatDateTime)
}

// formatRFC3339 はRFC3339形式の文字列の時刻をフォーマットする（開発情報用）
func (mw *MarkdownWriter) formatRFC3339(timeStr string) string {
	if timeStr == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return ""
	}
	return t.Local().Format(dateFormatDateTime)
}

// buildAttachmentMap は添付ファイルのマッピングを作成する（元のファイル名 → 保存されたファイル名）
func (mw *MarkdownWriter) buildAttachmentMap(issue *cloud.Issue, attachmentFiles []string) map[string]string {
	attachmentMap := make(map[string]string)
	if issue.Fields == nil || issue.Fields.Attachments == nil {
		return attachmentMap
	}

	// 添付ファイルリストと保存されたファイル名を対応付ける
	for i, attachment := range issue.Fields.Attachments {
		if i < len(attachmentFiles) {
			// 元のファイル名 → 保存されたファイル名（課題キー付き）
			attachmentMap[attachment.Filename] = attachmentFiles[i]
		}
	}
	return attachmentMap
}
