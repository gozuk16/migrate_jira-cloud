package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/andygrunwald/go-jira/v2/cloud"
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

// escapeTOMLString はTOML文字列をエスケープする
func escapeTOMLString(s string) string {
	// バックスラッシュをエスケープ（最初に処理）
	s = strings.ReplaceAll(s, "\\", "\\\\")
	// ダブルクォートをエスケープ
	s = strings.ReplaceAll(s, "\"", "\\\"")
	// 改行を除去
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// ParentIssueInfo は親課題の情報を保持する
type ParentIssueInfo struct {
	Key  string
	Type string // issue type name (e.g., "Epic", "Story", "Task")
}

// ChildIssueInfo は子課題の情報を保持する
type ChildIssueInfo struct {
	Key     string
	Summary string
	Status  string
	Type    string // 課題タイプ名
	Rank    string // Rankフィールド（customfield_10019）
}

// getIssueTypeIcon は課題タイプに応じたアイコンを返す
func getIssueTypeIcon(issueType string) string {
	switch issueType {
	case "Epic", "エピック":
		return "🟣"
	case "Story", "ストーリー":
		return "📗"
	case "Task", "タスク":
		return "☑️"
	case "Sub-task", "Subtask", "サブタスク":
		return "➡️"
	case "Bug", "バグ":
		return "🐞"
	default:
		return "📄"
	}
}

// MarkdownWriter はMarkdown形式で課題を出力する
type MarkdownWriter struct {
	outputDir      string
	attachmentsDir string
	userMapping    UserMapping
	config         *Config
}

// NewMarkdownWriter は新しいMarkdownWriterを作成する
func NewMarkdownWriter(outputDir, attachmentsDir string, userMapping UserMapping, config *Config) *MarkdownWriter {
	if userMapping == nil {
		userMapping = make(UserMapping)
	}
	return &MarkdownWriter{
		outputDir:      outputDir,
		attachmentsDir: attachmentsDir,
		userMapping:    userMapping,
		config:         config,
	}
}

// WriteIssue は課題をMarkdownファイルに出力する
func (mw *MarkdownWriter) WriteIssue(issue *cloud.Issue, attachmentFiles []string, fieldNameCache FieldNameCache, devStatus *DevStatusDetail, parentInfo *ParentIssueInfo, childIssues []ChildIssueInfo, remoteLinks []cloud.RemoteLink) error {
	// プロジェクトキーを取得
	projectKey := issue.Fields.Project.Key

	// プロジェクト別の出力ディレクトリの作成
	projectDir := filepath.Join(mw.outputDir, projectKey)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("Markdown出力ディレクトリの作成に失敗しました: %w", err)
	}

	// Markdownコンテンツの生成
	content := mw.generateMarkdown(issue, attachmentFiles, fieldNameCache, devStatus, parentInfo, childIssues, remoteLinks)

	// ファイルパスの作成
	filename := fmt.Sprintf("%s.md", issue.Key)
	outputPath := filepath.Join(projectDir, filename)

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

// generateFrontMatter はHugoのフロントマター（TOML形式）を生成する
func (mw *MarkdownWriter) generateFrontMatter(sb *strings.Builder, issue *cloud.Issue, parentInfo *ParentIssueInfo) {
	sb.WriteString("+++\n")
	sb.WriteString(fmt.Sprintf("title = \"%s\"\n", escapeTOMLString(issue.Fields.Summary)))
	sb.WriteString(fmt.Sprintf("date = %s\n", mw.formatTimeISO8601(issue.Fields.Created)))
	sb.WriteString(fmt.Sprintf("lastmod = %s\n", mw.formatTimeISO8601(issue.Fields.Updated)))
	sb.WriteString(fmt.Sprintf("project = \"%s\"\n", issue.Fields.Project.Key))
	sb.WriteString(fmt.Sprintf("issue_key = \"%s\"\n", issue.Key))
	sb.WriteString(fmt.Sprintf("type = \"page\"\n"))
	sb.WriteString(fmt.Sprintf("issue_type = \"%s\"\n", escapeTOMLString(issue.Fields.Type.Name)))

	// 親課題情報を追加
	if parentInfo != nil && parentInfo.Key != "" {
		sb.WriteString(fmt.Sprintf("parent = \"%s\"\n", parentInfo.Key))
		sb.WriteString(fmt.Sprintf("parent_issue_type = \"%s\"\n", escapeTOMLString(parentInfo.Type)))
	}

	// rank を追加（設定されたRankフィールドIDから取得）
	customFields := GetAllCustomFields(issue)
	if rank, exists := customFields[mw.config.Display.RankFieldId]; exists && !IsCustomFieldEmpty(rank) {
		rankValue := FormatCustomFieldValue(rank)
		if rankValue != "" {
			sb.WriteString(fmt.Sprintf("rank = \"%s\"\n", escapeTOMLString(rankValue)))
		}
	}

	// ラベルをtagsとして追加（Hugo taxonomy）
	if len(issue.Fields.Labels) > 0 {
		tags := make([]string, len(issue.Fields.Labels))
		for i, label := range issue.Fields.Labels {
			tags[i] = fmt.Sprintf("\"%s\"", escapeTOMLString(label))
		}
		sb.WriteString(fmt.Sprintf("tags = [%s]\n", strings.Join(tags, ", ")))
	}

	// ステータス、担当者
	sb.WriteString(fmt.Sprintf("status =  \"%s\"\n", issue.Fields.Status.Name))
	sb.WriteString(fmt.Sprintf("assignee = \"%s\"\n", mw.getUser(issue.Fields.Assignee)))
	// Start date
	if startDate, exists := customFields["customfield_10015"]; exists && !IsCustomFieldEmpty(startDate) {
		fieldValue := FormatCustomFieldValue(startDate)
		if fieldValue != "" {
			sb.WriteString(fmt.Sprintf("startdate = \"%s\"\n", fieldValue))
		}
	}
	// 期限
	duedate := time.Time(issue.Fields.Duedate)
	if !duedate.IsZero() {
		sb.WriteString(fmt.Sprintf("duedate = \"%s\"\n", duedate.Format("2006-01-02")))
	}

	// 修正バージョン（Fix Versions）
	if len(issue.Fields.FixVersions) > 0 {
		versions := make([]string, len(issue.Fields.FixVersions))
		for i, v := range issue.Fields.FixVersions {
			versions[i] = fmt.Sprintf("\"%s\"", escapeTOMLString(v.Name))
		}
		sb.WriteString(fmt.Sprintf("fix_versions = [%s]\n", strings.Join(versions, ", ")))
	}

	// 影響バージョン（Affected Versions）
	if len(issue.Fields.AffectsVersions) > 0 {
		versions := make([]string, len(issue.Fields.AffectsVersions))
		for i, v := range issue.Fields.AffectsVersions {
			versions[i] = fmt.Sprintf("\"%s\"", escapeTOMLString(v.Name))
		}
		sb.WriteString(fmt.Sprintf("affected_versions = [%s]\n", strings.Join(versions, ", ")))
	}

	sb.WriteString("+++\n\n")

}

// isHiddenCustomField は指定されたカスタムフィールドIDが非表示設定になっているかチェックする
func (mw *MarkdownWriter) isHiddenCustomField(fieldID string) bool {
	if mw.config == nil {
		return false
	}
	for _, hiddenField := range mw.config.Display.HiddenCustomFields {
		if hiddenField == fieldID {
			return true
		}
	}
	return false
}

// generateTitle は課題のタイトルを生成する
func (mw *MarkdownWriter) generateTitle(sb *strings.Builder, issue *cloud.Issue, parentInfo *ParentIssueInfo) {
	projectIcon := "📦"
	projectLink := fmt.Sprintf("[%s %s](../)", projectIcon, issue.Fields.Project.Name)
	issueIcon := getIssueTypeIcon(issue.Fields.Type.Name)
	issueLink := fmt.Sprintf("[%s %s](../%s/)", issueIcon, issue.Key, issue.Key)

	if parentInfo != nil && parentInfo.Key != "" {
		parentIcon := getIssueTypeIcon(parentInfo.Type)
		parentLink := fmt.Sprintf("[%s %s](../%s/)", parentIcon, parentInfo.Key, parentInfo.Key)
		sb.WriteString(fmt.Sprintf("%s / %s / %s\n\n", projectLink, parentLink, issueLink))
	} else {
		sb.WriteString(fmt.Sprintf("%s / %s\n\n", projectLink, issueLink))
	}
	sb.WriteString(fmt.Sprintf("# %s\n\n", issue.Fields.Summary))
}

// generateBasicInfo は基本情報セクションを生成する
func (mw *MarkdownWriter) generateBasicInfo(sb *strings.Builder, issue *cloud.Issue, fieldNameCache FieldNameCache, devStatus *DevStatusDetail) {
	sb.WriteString("## 基本情報\n\n")
	sb.WriteString(fmt.Sprintf("- **課題キー**: %s\n", issue.Key))
	sb.WriteString(fmt.Sprintf("- **課題タイプ**: %s\n", issue.Fields.Type.Name))
	sb.WriteString(fmt.Sprintf("- **ステータス**: %s\n", issue.Fields.Status.Name))
	sb.WriteString(fmt.Sprintf("- **優先度**: %s\n", mw.getFieldString(issue.Fields.Priority)))
	sb.WriteString(fmt.Sprintf("- **担当者**: %s\n", mw.getUser(issue.Fields.Assignee)))
	sb.WriteString(fmt.Sprintf("- **報告者**: %s\n", mw.getUser(issue.Fields.Reporter)))
	sb.WriteString(fmt.Sprintf("- **作成日**: %s\n", mw.formatTime(issue.Fields.Created)))
	sb.WriteString(fmt.Sprintf("- **更新日**: %s\n", mw.formatTime(issue.Fields.Updated)))

	// Start date（カスタムフィールド）をここに表示
	customFields := GetAllCustomFields(issue)
	if startDate, exists := customFields["customfield_10015"]; exists && !IsCustomFieldEmpty(startDate) {
		fieldName := fieldNameCache.GetFieldName("customfield_10015")
		fieldValue := FormatCustomFieldValue(startDate)
		if fieldValue != "" {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", fieldName, fieldValue))
		}
	}

	// 期限が設定されている場合のみ出力
	duedate := time.Time(issue.Fields.Duedate)
	if !duedate.IsZero() {
		sb.WriteString(fmt.Sprintf("- **期限**: %s\n", duedate.Format("2006-01-02")))
	}

	// ラベルが設定されている場合のみ出力
	if len(issue.Fields.Labels) > 0 {
		sb.WriteString(fmt.Sprintf("- **ラベル**: %s\n", strings.Join(issue.Fields.Labels, ", ")))
	}

	// 修正バージョンが設定されている場合のみ出力
	if len(issue.Fields.FixVersions) > 0 {
		versions := make([]string, len(issue.Fields.FixVersions))
		for i, v := range issue.Fields.FixVersions {
			versions[i] = v.Name
		}
		sb.WriteString(fmt.Sprintf("- **修正バージョン**: %s\n", strings.Join(versions, ", ")))
	}

	// 影響バージョンが設定されている場合のみ出力
	if len(issue.Fields.AffectsVersions) > 0 {
		versions := make([]string, len(issue.Fields.AffectsVersions))
		for i, v := range issue.Fields.AffectsVersions {
			versions[i] = v.Name
		}
		sb.WriteString(fmt.Sprintf("- **影響バージョン**: %s\n", strings.Join(versions, ", ")))
	}

	// 親課題が設定されている場合のみ出力
	if issue.Fields.Parent != nil && issue.Fields.Parent.Key != "" {
		sb.WriteString(fmt.Sprintf("- **親課題**: [%s](../%s/)\n", issue.Fields.Parent.Key, issue.Fields.Parent.Key))
	}

	// 時間管理情報（値がある場合のみ出力）
	if issue.Fields.TimeTracking != nil {
		tt := issue.Fields.TimeTracking

		if tt.OriginalEstimateSeconds > 0 {
			timeStr := mw.formatTimeSeconds(tt.OriginalEstimateSeconds)
			sb.WriteString(fmt.Sprintf("- **初期見積り**: %s\n", timeStr))
		}
		if tt.RemainingEstimateSeconds > 0 {
			timeStr := mw.formatTimeSeconds(tt.RemainingEstimateSeconds)
			sb.WriteString(fmt.Sprintf("- **残り時間**: %s\n", timeStr))
		}
		if tt.TimeSpentSeconds > 0 {
			timeStr := mw.formatTimeSeconds(tt.TimeSpentSeconds)
			sb.WriteString(fmt.Sprintf("- **作業時間**: %s\n", timeStr))
		}
	}

	// Σ時間情報（サブタスク含む集計値）
	if aggTime := extractAggregateTimeFields(issue); aggTime != nil {
		if aggTime.AggregateTimeOriginalEstimate > 0 {
			timeStr := mw.formatTimeSeconds(aggTime.AggregateTimeOriginalEstimate)
			sb.WriteString(fmt.Sprintf("- **Σ初期見積り**: %s\n", timeStr))
		}
		if aggTime.AggregateTimeEstimate > 0 {
			timeStr := mw.formatTimeSeconds(aggTime.AggregateTimeEstimate)
			sb.WriteString(fmt.Sprintf("- **Σ残り時間**: %s\n", timeStr))
		}
		if aggTime.AggregateTimeSpent > 0 {
			timeStr := mw.formatTimeSeconds(aggTime.AggregateTimeSpent)
			sb.WriteString(fmt.Sprintf("- **Σ作業時間**: %s\n", timeStr))
		}
	}

	if issue.Fields.Resolution != nil {
		sb.WriteString(fmt.Sprintf("- **解決状況**: %s\n", issue.Fields.Resolution.Name))
	}

	// カスタムフィールド（Start dateとRankを除外、値があるもののみ表示）
	if len(customFields) > 0 {
		sortedKeys := GetSortedCustomFieldKeys(customFields)
		for _, key := range sortedKeys {
			// 設定で非表示に指定されたカスタムフィールドをスキップ
			if mw.isHiddenCustomField(key) {
				continue
			}

			// 値が空のフィールドはスキップ
			if IsCustomFieldEmpty(customFields[key]) {
				continue
			}
			fieldName := fieldNameCache.GetFieldName(key)

			// 開発フィールドの場合は詳細情報付きでフォーマット
			var fieldValue string
			if fieldMap, ok := customFields[key].(map[string]interface{}); ok && isDevelopmentField(fieldMap) {
				fieldValue = FormatDevelopmentFieldWithDetails(fieldMap, devStatus)
			} else {
				fieldValue = FormatCustomFieldValue(customFields[key])
			}

			// 値が空の場合はスキップ（開発フィールドの詳細表示が空の場合も含む）
			// 空のmapの場合は map[] または {} となるのでスキップ
			if fieldValue == "" || fieldValue == "map[]" || fieldValue == "{}" {
				continue
			}

			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", fieldName, fieldValue))
		}
	}

	sb.WriteString("\n")
}

// generateDevelopmentInfo は開発情報セクションを生成する
func (mw *MarkdownWriter) generateDevelopmentInfo(sb *strings.Builder, devStatus *DevStatusDetail) {
	// 開発情報セクション（devStatusがある場合のみ）
	if devStatus != nil && len(devStatus.Detail) > 0 {
		sb.WriteString("## 開発情報\n\n")

		for _, detail := range devStatus.Detail {
			// ブランチ（最初に出力、JIRA仕様に合わせる）
			if len(detail.Branches) > 0 {
				sb.WriteString("### ブランチ\n\n")
				for _, branch := range detail.Branches {
					sb.WriteString(fmt.Sprintf("- [`%s`](%s)\n", branch.Name, branch.URL))
				// 最終コミット情報を表示
				if branch.LastCommit != nil && branch.LastCommit.DisplayID != "" {
					sb.WriteString(fmt.Sprintf("  - 最終コミット: [`%s`](%s)",
						branch.LastCommit.DisplayID, branch.LastCommit.URL))
					// タイムスタンプを整形して表示
					if branch.LastCommit.Timestamp != "" {
						// ISO8601形式のタイムスタンプをパース
						if t, err := time.Parse(time.RFC3339, branch.LastCommit.Timestamp); err == nil {
							sb.WriteString(fmt.Sprintf(" (%s)", t.Format("2006-01-02 15:04:05")))
						}
					}
					sb.WriteString("\n")
				}
				}
				sb.WriteString("\n")
			}

			// プルリクエスト（最後に出力、JIRA仕様に合わせる）
			if len(detail.PullRequests) > 0 {
				sb.WriteString("### プルリクエスト\n\n")
				for _, pr := range detail.PullRequests {
					sb.WriteString(fmt.Sprintf("- [%s](%s)\n", pr.Name, pr.URL))
					if pr.Author.Name != "" {
						sb.WriteString(fmt.Sprintf("  - 作成者: %s\n", pr.Author.Name))
					}
					if pr.Source.Branch != "" {
						sb.WriteString(fmt.Sprintf("  - ブランチ: `%s`\n", pr.Source.Branch))
					}
					if pr.Status != "" {
						sb.WriteString(fmt.Sprintf("  - 状態: %s\n", pr.Status))
					}
				}
				sb.WriteString("\n")
			}
		}
	}
}

// generateDescription は説明セクションを生成する
func (mw *MarkdownWriter) generateDescription(sb *strings.Builder, issue *cloud.Issue, attachmentMap map[string]string) {
	if issue.Fields.Description != "" {
		sb.WriteString("## 説明\n\n")
		description := issue.Fields.Description
		// JIRAマークアップをMarkdownに変換
		description = mw.convertJIRAMarkupToMarkdown(description)
		// 画像参照を変換
		description = mw.replaceImageReferences(description, attachmentMap)
		sb.WriteString(description)
		sb.WriteString("\n\n")
	}
}

// generateComments はコメントセクションを生成する（昇順：古いコメントが先）
func (mw *MarkdownWriter) generateComments(sb *strings.Builder, issue *cloud.Issue, attachmentMap map[string]string) {
	if issue.Fields.Comments != nil && len(issue.Fields.Comments.Comments) > 0 {
		sb.WriteString("## コメント\n\n")
		comments := issue.Fields.Comments.Comments
		// 昇順（古い順）で出力
		for _, comment := range comments {
			authorName := mw.getUser(comment.Author)
			dateStr := mw.formatCommentDate(comment.Created)

			// 返信かどうかを判定（本文が[~accountid:で始まる場合）
			isReply := strings.HasPrefix(comment.Body, "[~accountid:")

			// タイトル: 投稿者名 投稿日（返信の場合は↩️を付ける）
			if isReply {
				sb.WriteString(fmt.Sprintf("↩️ %s %s\n\n---\n\n", authorName, dateStr))
			} else {
				sb.WriteString(fmt.Sprintf("%s %s\n\n---\n\n", authorName, dateStr))
			}

			commentBody := comment.Body
			// JIRAマークアップをMarkdownに変換
			commentBody = mw.convertJIRAMarkupToMarkdown(commentBody)
			// 画像参照を変換
			commentBody = mw.replaceImageReferences(commentBody, attachmentMap)
			sb.WriteString(commentBody)
			sb.WriteString("\n\n")
		}
	}
}

// formatCommentDate はコメント用の日付フォーマット（yyyy-mm-dd hh:mm）
func (mw *MarkdownWriter) formatCommentDate(timeStr string) string {
	// JIRAの日付形式: 2026-01-22T00:43:07.025+0900
	t, err := time.Parse("2006-01-02T15:04:05.000-0700", timeStr)
	if err != nil {
		// フォールバック: RFC3339を試す
		t, err = time.Parse(time.RFC3339, timeStr)
		if err != nil {
			return timeStr
		}
	}
	return t.Format("2006-01-02 15:04")
}

// generateSubtasks はサブタスクセクションを生成する
func (mw *MarkdownWriter) generateSubtasks(sb *strings.Builder, issue *cloud.Issue) {
	if len(issue.Fields.Subtasks) > 0 {
		sb.WriteString("## サブタスク\n\n")
		for _, subtask := range issue.Fields.Subtasks {
			sb.WriteString(fmt.Sprintf("- **[%s](../%s/)**: %s", subtask.Key, subtask.Key, subtask.Fields.Summary))
			if subtask.Fields.Status != nil {
				sb.WriteString(fmt.Sprintf(" [%s]", subtask.Fields.Status.Name))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
}

// generateChildIssues は子作業項目セクションを生成する
func (mw *MarkdownWriter) generateChildIssues(sb *strings.Builder, childIssues []ChildIssueInfo) {
	if len(childIssues) > 0 {
		sb.WriteString("## 子作業項目\n\n")
		for _, child := range childIssues {
			icon := getIssueTypeIcon(child.Type)
			sb.WriteString(fmt.Sprintf("- %s **[%s](../%s/)**: %s", icon, child.Key, child.Key, child.Summary))
			if child.Status != "" {
				sb.WriteString(fmt.Sprintf(" [%s]", child.Status))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
}

// generateConfluenceLinks はConfluenceコンテンツセクションを生成する
func (mw *MarkdownWriter) generateConfluenceLinks(sb *strings.Builder, remoteLinks []cloud.RemoteLink) {
	// Confluenceリンクのみフィルタ
	var confluenceLinks []cloud.RemoteLink
	for _, link := range remoteLinks {
		if link.Application != nil &&
			strings.ToLower(link.Application.Type) == "confluence" {
			confluenceLinks = append(confluenceLinks, link)
		}
	}

	if len(confluenceLinks) == 0 {
		return
	}

	sb.WriteString("## Confluenceコンテンツ\n\n")
	for _, link := range confluenceLinks {
		if link.Object != nil {
			title := link.Object.Title
			if title == "" {
				title = "Confluence Page"
			}
			sb.WriteString(fmt.Sprintf("- [%s](%s)\n", title, link.Object.URL))
		}
	}
	sb.WriteString("\n")
}

// generateIssueLinks は関連リンクセクションを生成する
func (mw *MarkdownWriter) generateIssueLinks(sb *strings.Builder, issue *cloud.Issue) {
	if len(issue.Fields.IssueLinks) > 0 {
		sb.WriteString("## 関連リンク\n\n")
		for _, link := range issue.Fields.IssueLinks {
			if link.OutwardIssue != nil {
				sb.WriteString(fmt.Sprintf("- **%s**: [%s](../%s/)", link.Type.Outward, link.OutwardIssue.Key, link.OutwardIssue.Key))
				if link.OutwardIssue.Fields != nil {
					sb.WriteString(fmt.Sprintf(" - %s", link.OutwardIssue.Fields.Summary))
					if link.OutwardIssue.Fields.Status != nil {
						sb.WriteString(fmt.Sprintf(" [%s]", link.OutwardIssue.Fields.Status.Name))
					}
				}
				sb.WriteString("\n")
			}

			// Inward issue（他の課題がこの課題に対して持つ関連）
			if link.InwardIssue != nil {
				sb.WriteString(fmt.Sprintf("- **%s**: [%s](../%s/)", link.Type.Inward, link.InwardIssue.Key, link.InwardIssue.Key))
				if link.InwardIssue.Fields != nil {
					sb.WriteString(fmt.Sprintf(" - %s", link.InwardIssue.Fields.Summary))
					if link.InwardIssue.Fields.Status != nil {
						sb.WriteString(fmt.Sprintf(" [%s]", link.InwardIssue.Fields.Status.Name))
					}
				}
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}
}

// generateAttachments は添付ファイルセクションを生成する
func (mw *MarkdownWriter) generateAttachments(sb *strings.Builder, attachmentFiles []string) {
	if len(attachmentFiles) > 0 {
		sb.WriteString("## 添付ファイル\n\n")
		for _, filename := range attachmentFiles {
			// ファイル名をURLエンコーディング（スペース→%20）
			encodedFilename := url.PathEscape(filename)
			// 相対パスで添付ファイルを参照（プロジェクトディレクトリから2階層上）
			relPath := fmt.Sprintf("../../attachments/%s", encodedFilename)
			sb.WriteString(fmt.Sprintf("- [%s](%s)\n", filename, relPath))
		}
		sb.WriteString("\n")
	}
}

// generateChangeHistory は変更履歴セクションを生成する
func (mw *MarkdownWriter) generateChangeHistory(sb *strings.Builder, issue *cloud.Issue) {
	if issue.Changelog != nil && len(issue.Changelog.Histories) > 0 {
		sb.WriteString("## 変更履歴\n\n")
		for i, history := range issue.Changelog.Histories {
			sb.WriteString(fmt.Sprintf("### 変更 %d\n\n", i+1))
			sb.WriteString(fmt.Sprintf("- **変更者**: %s\n", mw.getUser(&history.Author)))
			sb.WriteString(fmt.Sprintf("- **変更日**: %s\n", mw.formatTimeString(history.Created)))
			sb.WriteString("\n")

			for _, item := range history.Items {
				sb.WriteString(fmt.Sprintf("- **%s**: `%s` → `%s`\n", item.Field, item.FromString, item.ToString))
			}
			sb.WriteString("\n")
		}
	}
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
	return time.Time(jiraTime).Format("2006-01-02 15:04:05")
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

// formatTimeString は文字列の時刻をフォーマットする
func (mw *MarkdownWriter) formatTimeString(timeStr string) string {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return timeStr
	}
	return t.Format("2006-01-02 15:04:05")
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

// replaceImageReferences はJIRA形式の画像参照 !filename.png! をMarkdown形式に変換する
func (mw *MarkdownWriter) replaceImageReferences(text string, attachmentMap map[string]string) string {
	// JIRA形式の画像参照パターン: !filename.png! または !filename.png|属性!
	// 例: !screenshot.png!, !image.jpg|width=300!
	pattern := regexp.MustCompile(`!([^!|]+(?:\.[a-zA-Z0-9]+))(?:\|[^!]*)?!`)

	result := pattern.ReplaceAllStringFunc(text, func(match string) string {
		// マッチからファイル名を抽出
		submatches := pattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		originalFilename := submatches[1]

		// 添付ファイルマップから保存されたファイル名を取得
		savedFilename, exists := attachmentMap[originalFilename]
		if !exists {
			return match // 見つからない場合は元のまま
		}

		// ファイル名をURLエンコーディング（スペース→%20）
		encodedFilename := url.PathEscape(savedFilename)
		// 画像ファイルの場合は画像形式、それ以外はリンク形式
		// Hugoで作成するときに、attachmentsディレクトリはプロジェクトディレクトリの直下になる
		relPath := fmt.Sprintf("/attachments/%s", encodedFilename)
		if IsImageFile(originalFilename) {
			return fmt.Sprintf("![%s](%s)", originalFilename, relPath)
		}
		return fmt.Sprintf("[%s](%s)", originalFilename, relPath)
	})

	return result
}

// extractJIRATables はJIRAテーブルを抽出してプレースホルダーに置き換える
// セル内改行を保持したままテーブル全体を抽出する
func (mw *MarkdownWriter) extractJIRATables(text string) (string, []string) {
	lines := strings.Split(text, "\n")
	tables := []string{}
	result := []string{}

	i := 0
	for i < len(lines) {
		line := lines[i]

		// ヘッダー行を検出
		if strings.HasPrefix(line, "||") && strings.HasSuffix(line, "||") {
			tableLines := []string{line}
			i++

			// データ行を収集
			for i < len(lines) {
				dataLine := lines[i]

				// 次のテーブルヘッダーをチェック
				if strings.HasPrefix(dataLine, "||") && strings.HasSuffix(dataLine, "||") {
					// 次のテーブル開始 → 現在のテーブル終了
					break
				} else if strings.HasPrefix(dataLine, "|") && !strings.HasPrefix(dataLine, "||") {
					// データ行の開始
					completeLine := dataLine
					i++

					// |で終わるまで次の行と結合（セル内改行対応）
					for !strings.HasSuffix(completeLine, "|") && i < len(lines) {
						nextLine := lines[i]
						// 次のテーブルヘッダーが来たら結合を中止
						if strings.HasPrefix(nextLine, "||") && strings.HasSuffix(nextLine, "||") {
							break
						}
						completeLine += "\n" + nextLine
						i++
					}

					if strings.HasSuffix(completeLine, "|") {
						tableLines = append(tableLines, completeLine)
					}
				} else if dataLine == "" {
					// 空行 → テーブル終了
					break
				} else {
					// テーブル外の行（|で始まらない） → テーブル終了
					break
				}
			}

			// テーブルをプレースホルダー化
			tables = append(tables, strings.Join(tableLines, "\n"))
			result = append(result, fmt.Sprintf("__TABLE_%d__", len(tables)-1))
		} else if strings.HasPrefix(line, "|") && !strings.HasPrefix(line, "||") {
			// ヘッダー無しテーブルを検出
			tableLines := []string{}

			// データ行を収集（セル内改行対応）
			for i < len(lines) {
				dataLine := lines[i]

				if strings.HasPrefix(dataLine, "|") && !strings.HasPrefix(dataLine, "||") {
					// データ行の開始
					completeLine := dataLine
					i++

					// |で終わるまで次の行と結合（セル内改行対応）
					for !strings.HasSuffix(completeLine, "|") && i < len(lines) {
						nextLine := lines[i]
						// 次のテーブルヘッダーが来たら結合を中止
						if strings.HasPrefix(nextLine, "||") && strings.HasSuffix(nextLine, "||") {
							break
						}
						// 次のデータ行が来たら結合を中止
						if strings.HasPrefix(nextLine, "|") {
							break
						}
						// 空行が来たら結合を中止
						if nextLine == "" {
							break
						}
						completeLine += "\n" + nextLine
						i++
					}

					if strings.HasSuffix(completeLine, "|") {
						tableLines = append(tableLines, completeLine)
					}
				} else if dataLine == "" {
					// 空行 → テーブル終了
					break
				} else {
					// テーブル外の行 → テーブル終了
					break
				}
			}

			// テーブルをプレースホルダー化
			if len(tableLines) > 0 {
				tables = append(tables, strings.Join(tableLines, "\n"))
				result = append(result, fmt.Sprintf("__TABLE_%d__", len(tables)-1))
			}
		} else {
			result = append(result, line)
			i++
		}
	}

	return strings.Join(result, "\n"), tables
}

// convertJIRATableToMarkdown 1つのJIRAテーブルをMarkdownテーブルに変換する
func (mw *MarkdownWriter) convertJIRATableToMarkdown(table string) string {
	lines := strings.Split(table, "\n")
	var result []string

	// ヘッダーの有無を判定
	hasHeader := false
	if len(lines) > 0 {
		firstLine := lines[0]
		hasHeader = strings.HasPrefix(firstLine, "||") && strings.HasSuffix(firstLine, "||")
	}

	// ヘッダー無しの場合、最初のデータ行からセル数を取得して空ヘッダーを生成
	if !hasHeader && len(lines) > 0 {
		// 最初のデータ行を取得（セル内改行対応）
		firstLine := lines[0]
		if strings.HasPrefix(firstLine, "|") && !strings.HasPrefix(firstLine, "||") {
			// セル内改行を考慮して完全な行を取得
			completeLine := firstLine
			j := 1
			for !strings.HasSuffix(completeLine, "|") && j < len(lines) {
				nextLine := lines[j]
				completeLine += "\n" + nextLine
				j++
			}

			if strings.HasSuffix(completeLine, "|") {
				content := strings.Trim(completeLine, "|")
				cells := strings.Split(content, "|")
				cellCount := len(cells)

				// 空ヘッダー行を生成
				emptyHeaders := make([]string, cellCount)
				for k := range emptyHeaders {
					emptyHeaders[k] = " "
				}
				header := "| " + strings.Join(emptyHeaders, " | ") + " |"
				result = append(result, header)

				// セパレーター行を生成
				separators := make([]string, cellCount)
				for k := range separators {
					separators[k] = "------"
				}
				separator := "| " + strings.Join(separators, " | ") + " |"
				result = append(result, separator)
			}
		}
	}

	i := 0
	for i < len(lines) {
		line := lines[i]

		// ヘッダー行を変換（セル内改行対応）
		if strings.HasPrefix(line, "||") {
			completeLine := line
			i++

			// ||で終わるまで次の行と結合（セル内改行対応）
			for !strings.HasSuffix(completeLine, "||") && i < len(lines) {
				nextLine := lines[i]
				completeLine += "\n" + nextLine
				i++
			}

			if strings.HasSuffix(completeLine, "||") {
				content := strings.Trim(completeLine, "|")
				cells := strings.Split(content, "||")
				// セル内改行を<br>に変換
				for j, cell := range cells {
					cells[j] = strings.ReplaceAll(cell, "\n", "<br>")
				}
				// Markdownテーブルヘッダー
				header := "| " + strings.Join(cells, " | ") + " |"
				result = append(result, header)
				// セパレーター行
				separators := make([]string, len(cells))
				for j := range separators {
					separators[j] = "------"
				}
				separator := "| " + strings.Join(separators, " | ") + " |"
				result = append(result, separator)
			}
		} else if strings.HasPrefix(line, "|") && !strings.HasPrefix(line, "||") {
			// データ行を変換（セル内改行対応）
			completeLine := line
			i++

			// |で終わるまで次の行と結合（セル内改行対応）
			for !strings.HasSuffix(completeLine, "|") && i < len(lines) {
				nextLine := lines[i]
				completeLine += "\n" + nextLine
				i++
			}

			if strings.HasSuffix(completeLine, "|") {
				content := strings.Trim(completeLine, "|")
				cells := strings.Split(content, "|")
				// セル内改行を<br>に変換
				for j, cell := range cells {
					cells[j] = strings.ReplaceAll(cell, "\n", "<br>")
				}
				// Markdownテーブルデータ行
				row := "| " + strings.Join(cells, " | ") + " |"
				result = append(result, row)
			}
		} else {
			i++
		}
	}

	return strings.Join(result, "\n")
}

// convertJIRAMarkupToMarkdown はJIRAマークアップをMarkdown形式に変換する
func (mw *MarkdownWriter) convertJIRAMarkupToMarkdown(text string) string {
	// プレースホルダーでコードブロックとインラインコードを保護
	codeBlocks := []string{}
	placeholderIndex := 0
	inlineCodes := []string{}
	inlineCodeIndex := 0

	// 1. コードブロック（言語指定付き）を抽出して保護
	codeWithLangPattern := regexp.MustCompile(`(?s)\{code:([^}]+)\}(.*?)\{code\}`)
	text = codeWithLangPattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := codeWithLangPattern.FindStringSubmatch(match)
		if len(submatches) >= 3 {
			lang := submatches[1]
			code := submatches[2]
			// Markdownのコードブロック形式に変換
			mdCodeBlock := fmt.Sprintf("```%s\n%s\n```", lang, code)
			placeholder := fmt.Sprintf("__CODE_BLOCK_%d__", placeholderIndex)
			codeBlocks = append(codeBlocks, mdCodeBlock)
			placeholderIndex++
			return placeholder
		}
		return match
	})

	// 2. コードブロック（言語指定なし）を抽出して保護
	codePattern := regexp.MustCompile(`(?s)\{code\}(.*?)\{code\}`)
	text = codePattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := codePattern.FindStringSubmatch(match)
		if len(submatches) >= 2 {
			code := submatches[1]
			mdCodeBlock := fmt.Sprintf("```\n%s\n```", code)
			placeholder := fmt.Sprintf("__CODE_BLOCK_%d__", placeholderIndex)
			codeBlocks = append(codeBlocks, mdCodeBlock)
			placeholderIndex++
			return placeholder
		}
		return match
	})

	// 3. フォーマット済みテキストを抽出して保護
	noformatPattern := regexp.MustCompile(`(?s)\{noformat\}(.*?)\{noformat\}`)
	text = noformatPattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := noformatPattern.FindStringSubmatch(match)
		if len(submatches) >= 2 {
			content := submatches[1]
			mdCodeBlock := fmt.Sprintf("```\n%s\n```", content)
			placeholder := fmt.Sprintf("__CODE_BLOCK_%d__", placeholderIndex)
			codeBlocks = append(codeBlocks, mdCodeBlock)
			placeholderIndex++
			return placeholder
		}
		return match
	})

	// 4. インラインコード: {{text}} → `text`
	inlineCodePattern := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	text = inlineCodePattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := inlineCodePattern.FindStringSubmatch(match)
		if len(submatches) >= 2 {
			code := submatches[1]
			inlineCode := fmt.Sprintf("`%s`", code)
			placeholder := fmt.Sprintf("__INLINE_CODE_%d__", inlineCodeIndex)
			inlineCodes = append(inlineCodes, inlineCode)
			inlineCodeIndex++
			return placeholder
		}
		return match
	})

	// 5. ブレース記法の変換（{quote}, {color}, {status}, {panel}, {note}等）
	// コードブロック保護後、テーブル変換前に処理する
	text = mw.convertQuoteMarkup(text)
	text = mw.convertStatusLabelMarkup(text) // カスタムステータスラベルを先に変換（より具体的なパターン）
	text = mw.convertColorMarkup(text)
	text = mw.convertStatusMarkup(text)
	text = mw.convertPanelMarkup(text)
	text = mw.convertAdmonitionMarkup(text)

	// 6. テーブルを直接変換（プレースホルダー化せず）
	text, tables := mw.extractJIRATables(text)
	for i, table := range tables {
		placeholder := fmt.Sprintf("__TABLE_%d__", i)
		markdownTable := mw.convertJIRATableToMarkdown(table)
		text = strings.ReplaceAll(text, placeholder, markdownTable)
	}

	// 7. メンション変換: [~accountid:xxx] → <span class="mention">@ユーザー名</span>
	mentionPattern := regexp.MustCompile(`\[~accountid:([^\]]+)\]`)
	text = mentionPattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := mentionPattern.FindStringSubmatch(match)
		if len(submatches) >= 2 {
			accountID := submatches[1]

			// account IDからユーザー名を取得
			if userName, exists := mw.userMapping[accountID]; exists && userName != "" {
				return `<span class="mention">@` + userName + `</span>`
			}

			// マッピングが見つからない場合はaccount IDを表示
			return `<span class="mention">@` + accountID + `</span>`
		}
		return match
	})

	// 7. リンク変換: [text|url] → [text](url)
	linkPattern := regexp.MustCompile(`\[([^\]|]+)\|([^\]]+)\]`)
	text = linkPattern.ReplaceAllString(text, `[$1]($2)`)

	// 8-1. 見出し変換: h1. - h6. → # - ######（行単位処理）
	// 見出しをプレースホルダーで保護してからリスト変換を実行
	headings := []string{}
	headingIndex := 0
	headingPattern := regexp.MustCompile(`^h([1-6])\.\s+(.+)$`)
	lines := strings.Split(text, "\n")
	var processedLines []string
	for _, line := range lines {
		if matches := headingPattern.FindStringSubmatch(line); matches != nil {
			levelStr := matches[1]
			title := matches[2]
			level, _ := strconv.Atoi(levelStr)
			hashes := strings.Repeat("#", level)
			heading := hashes + " " + title
			placeholder := fmt.Sprintf("__HEADING_%d__", headingIndex)
			headings = append(headings, heading)
			processedLines = append(processedLines, placeholder)
			headingIndex++
		} else {
			processedLines = append(processedLines, line)
		}
	}
	text = strings.Join(processedLines, "\n")

	// 8-2. リスト変換: * → -、# → 1.（行単位処理）
	text = mw.convertJIRAListsToMarkdown(text)

	// 8-3. 見出しプレースホルダーを復元
	for i, heading := range headings {
		placeholder := fmt.Sprintf("__HEADING_%d__", i)
		text = strings.ReplaceAll(text, placeholder, heading)
	}

	// 8-4. リスト行を保護（装飾変換時の衝突回避）
	text, protectedLists := mw.protectListLines(text)

	// 9. 太字: *text* → **text**（日本語対応版）
	// Go の regexp は negative lookahead/lookbehind を サポートしないため、簡略版を使用
	// 単語境界の厳密な要件を緩和し、行頭・行末の * をサポート
	text = convertBoldMarkup(text)

	// 10. イタリック: _text_ → *text*（日本語対応版）
	text = convertItalicMarkup(text)

	// 11. 削除線: -text- → ~~text~~（日付・URL対応版）
	text = convertStrikethroughMarkup(text)

	// 12. 上付き: ^text^ → <sup>text</sup>
	supPattern := regexp.MustCompile(`\^([^\^]+)\^`)
	text = supPattern.ReplaceAllString(text, `<sup>$1</sup>`)

	// 13. 下付き: ~text~ → <sub>text</sub>
	// ~~は取り消し線なので除外する必要がある
	// ~~で囲まれた部分を一時的に保護する
	strikeProtectPattern := regexp.MustCompile(`~~[^~]*~~`)
	strikes := strikeProtectPattern.FindAllString(text, -1)
	strikeProtectIndex := 0
	text = strikeProtectPattern.ReplaceAllStringFunc(text, func(match string) string {
		placeholder := fmt.Sprintf("___STRIKE_PROTECT_%d___", strikeProtectIndex)
		strikeProtectIndex++
		return placeholder
	})

	// 下付き処理
	subPattern := regexp.MustCompile(`~([^~]+?)~`)
	text = subPattern.ReplaceAllString(text, `<sub>$1</sub>`)

	// 取り消し線を復元
	for i, strike := range strikes {
		placeholder := fmt.Sprintf("___STRIKE_PROTECT_%d___", i)
		text = strings.Replace(text, placeholder, strike, 1)
	}

	// 8-5. リスト行を復元
	text = mw.restoreListLines(text, protectedLists)

	// 14. プレースホルダーを元のコードブロックとインラインコードに戻す
	for i, codeBlock := range codeBlocks {
		placeholder := fmt.Sprintf("__CODE_BLOCK_%d__", i)
		text = strings.ReplaceAll(text, placeholder, codeBlock)
	}
	for i, inlineCode := range inlineCodes {
		placeholder := fmt.Sprintf("__INLINE_CODE_%d__", i)
		text = strings.ReplaceAll(text, placeholder, inlineCode)
	}

	// 15. 改行: text\n → text  \n（スペース2個挿入）
	// 古いチケットと新しいチケットで改行処理が違っていたため、明示的にスペース2個を挿入する方式に統一
	newlinePattern := regexp.MustCompile(`(.+)\n`)
	text = newlinePattern.ReplaceAllString(text, "$1  \n")

	return text
}

// convertJIRAHeadingsToMarkdown は JIRA の見出しマークアップを Markdown に変換する
// h1. 見出し → # 見出し
// h2. 見出し → ## 見出し
func (mw *MarkdownWriter) convertJIRAHeadingsToMarkdown(text string) string {
	lines := strings.Split(text, "\n")
	result := make([]string, 0, len(lines))

	headingPattern := regexp.MustCompile(`^h([1-6])\.\s+(.+)$`)

	for _, line := range lines {
		matches := headingPattern.FindStringSubmatch(line)
		if len(matches) == 3 {
			levelStr := matches[1]
			title := matches[2]
			level, _ := strconv.Atoi(levelStr)
			hashes := strings.Repeat("#", level)
			converted := hashes + " " + title
			result = append(result, converted)
		} else {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// convertJIRAListsToMarkdown は JIRA のリストマークアップを Markdown に変換する
// * リスト → - リスト
// ** りすと2 → (4スペース)- りすと2
// # リスト → 1. リスト
// ## りすと2 → (4スペース)1. りすと2
func (mw *MarkdownWriter) convertJIRAListsToMarkdown(text string) string {
	lines := strings.Split(text, "\n")
	result := make([]string, 0, len(lines))

	// 古いJIRAでは先頭にスペースが入ることがあるため、^\s* で先頭の空白を許容
	bulletListPattern := regexp.MustCompile(`^\s*(\*{1,6})\s+(.+)$`)
	numberedListPattern := regexp.MustCompile(`^\s*(#{1,6})\s+(.+)$`)

	for _, line := range lines {
		// 番号なしリスト（*）の処理
		matches := bulletListPattern.FindStringSubmatch(line)
		if len(matches) == 3 {
			asterisks := matches[1]
			content := matches[2]
			level := len(asterisks) - 1
			indent := strings.Repeat("    ", level)
			converted := indent + "- " + content
			result = append(result, converted)
		} else {
			// 番号付きリスト（#）の処理
			matches := numberedListPattern.FindStringSubmatch(line)
			if len(matches) == 3 {
				hashes := matches[1]
				content := matches[2]
				level := len(hashes) - 1
				indent := strings.Repeat("    ", level)
				converted := indent + "1. " + content
				result = append(result, converted)
			} else {
				result = append(result, line)
			}
		}
	}

	return strings.Join(result, "\n")
}

// protectListLines はリスト行を一時的にプレースホルダーに置き換えて保護します
// 装飾記号の変換時にリストマーカー（*）との衝突を防ぐために使用します
func (mw *MarkdownWriter) protectListLines(text string) (string, []string) {
	lines := strings.Split(text, "\n")
	var result []string
	var protected []string

	// JIRA リストパターン（番号なし * と番号付き #）
	// 古いJIRAでは先頭にスペースが入ることがあるため、^\s* で先頭の空白を許容
	bulletListPattern := regexp.MustCompile(`^\s*(\*{1,6})\s+(.+)$`)
	numberedListPattern := regexp.MustCompile(`^\s*(#{1,6})\s+(.+)$`)

	for i, line := range lines {
		if bulletListPattern.MatchString(line) || numberedListPattern.MatchString(line) {
			// リスト行をプレースホルダーに置き換え
			placeholder := fmt.Sprintf("___LIST_PLACEHOLDER_%d___", i)
			result = append(result, placeholder)
			protected = append(protected, line)
		} else {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n"), protected
}

// restoreListLines はプレースホルダーを元のリスト行に戻します
func (mw *MarkdownWriter) restoreListLines(text string, protected []string) string {
	result := text
	for i, line := range protected {
		placeholder := fmt.Sprintf("___LIST_PLACEHOLDER_%d___", i)
		result = strings.Replace(result, placeholder, line, 1)
	}
	return result
}

// convertBoldMarkup は*text*を**text**に変換します（日本語対応）
// 既に**で囲まれている場合は誤変換を避けます
func convertBoldMarkup(text string) string {
	lines := strings.Split(text, "\n")
	var result []string

	for _, line := range lines {
		// 既に**で囲まれている部分を保護するため、複数回のマッチングを試行
		// パターン：*text*（**textではない）
		converted := line

		// 簡単なパターン：*text*の形式（*の間に0個以上の非*文字）
		pattern := regexp.MustCompile(`\*([^\*\n]+?)\*`)

		for {
			prev := converted
			// マッチする部分を検出
			matches := pattern.FindAllStringSubmatchIndex(converted, -1)
			if len(matches) == 0 {
				break
			}

			// 後ろから処理（インデックスを保つため）
			for i := len(matches) - 1; i >= 0; i-- {
				match := matches[i]
				// マッチ位置から、既に**で囲まれていないかチェック
				start := match[0]
				end := match[1]

				// 前後の文字をチェック
				isBold := false
				if start > 0 && converted[start-1] == '*' {
					// 既に**で囲まれている可能性
					isBold = true
				}
				if end < len(converted) && converted[end] == '*' {
					// 既に**で囲まれている
					isBold = true
				}

				if !isBold {
					// *text* → **text**に変換
					matchText := converted[match[2]:match[3]]
					replacement := fmt.Sprintf("**%s**", matchText)
					converted = converted[:start] + replacement + converted[end:]
					break
				}
			}

			if converted == prev {
				break // 変更がなければ終了
			}
		}

		result = append(result, converted)
	}

	return strings.Join(result, "\n")
}

// convertItalicMarkup は_text_を*text*に変換します（日本語対応）
func convertItalicMarkup(text string) string {
	lines := strings.Split(text, "\n")
	var result []string

	for _, line := range lines {
		converted := line

		// パターン：_text_の形式（_の間に1個以上の非_文字）
		pattern := regexp.MustCompile(`_([^_\n]+?)_`)

		for {
			prev := converted
			matches := pattern.FindAllStringSubmatchIndex(converted, -1)
			if len(matches) == 0 {
				break
			}

			// 後ろから処理
			for i := len(matches) - 1; i >= 0; i-- {
				match := matches[i]
				start := match[0]
				end := match[1]

				// 前後の文字をチェック（既に*で囲まれているかチェック）
				isItalic := false
				if start > 0 && converted[start-1] == '_' {
					isItalic = true
				}
				if end < len(converted) && converted[end] == '_' {
					isItalic = true
				}

				if !isItalic {
					// _text_ → *text*に変換
					matchText := converted[match[2]:match[3]]
					replacement := fmt.Sprintf("*%s*", matchText)
					converted = converted[:start] + replacement + converted[end:]
					break
				}
			}

			if converted == prev {
				break
			}
		}

		result = append(result, converted)
	}

	return strings.Join(result, "\n")
}

// convertStrikethroughMarkup は-text-を~~text~~に変換します（日付・URL・リストアイテム対応）
func convertStrikethroughMarkup(text string) string {
	lines := strings.Split(text, "\n")
	var result []string

	for _, line := range lines {
		converted := line

		// パターン：-text-の形式（-の間に1個以上の非-文字、空白のみは除外、リスト要素（-空白）も除去）
		pattern := regexp.MustCompile(`-([^- \n]+?)-`)

		for {
			prev := converted
			matches := pattern.FindAllStringSubmatchIndex(converted, -1)
			if len(matches) == 0 {
				break
			}

			// 後ろから処理
			for i := len(matches) - 1; i >= 0; i-- {
				match := matches[i]
				start := match[0]
				end := match[1]

				// 前後の文字をチェック（マルチバイト文字対応）
				shouldSkip := false

				// キャプチャグループの内容をチェック（空白のみは変換しない）
				matchContent := converted[match[2]:match[3]]
				if strings.TrimSpace(matchContent) == "" {
					shouldSkip = true
				}

				// リストアイテムのマーカー（行頭の "- "）は変換しない
				if start == 0 && len(matchContent) > 0 && matchContent[0] == ' ' {
					shouldSkip = true
				}

				// 前の文字をチェック
				if !shouldSkip && start > 0 {
					prevRune, _ := utf8.DecodeLastRuneInString(converted[:start])
					if prevRune != utf8.RuneError {
						// ASCII英数字または記号(-/:)の場合のみスキップ
						// 日本語などのマルチバイト文字は変換を許可
						if (prevRune >= '0' && prevRune <= '9') ||
							(prevRune >= 'a' && prevRune <= 'z') ||
							(prevRune >= 'A' && prevRune <= 'Z') ||
							prevRune == '-' || prevRune == '/' || prevRune == ':' {
							shouldSkip = true
						}
					}
				}

				// 後の文字をチェック
				if !shouldSkip && end < len(converted) {
					nextRune, _ := utf8.DecodeRuneInString(converted[end:])
					if nextRune != utf8.RuneError {
						// ASCII英数字または記号(-/:)の場合のみスキップ
						// 日本語などのマルチバイト文字は変換を許可
						if (nextRune >= '0' && nextRune <= '9') ||
							(nextRune >= 'a' && nextRune <= 'z') ||
							(nextRune >= 'A' && nextRune <= 'Z') ||
							nextRune == '-' || nextRune == '/' || nextRune == ':' {
							shouldSkip = true
						}
					}
				}

				// 既に~~で囲まれているかチェック
				if !shouldSkip && start > 1 && converted[start-1:start] == "~" && converted[start-2:start-1] == "~" {
					shouldSkip = true
				}
				if !shouldSkip && end+1 < len(converted) && converted[end:end+1] == "~" && end+2 < len(converted) && converted[end+1:end+2] == "~" {
					shouldSkip = true
				}

				if !shouldSkip {
					// -text- → ~~text~~に変換
					replacement := fmt.Sprintf("~~%s~~", matchContent)
					converted = converted[:start] + replacement + converted[end:]
					break
				}
			}

			if converted == prev {
				break
			}
		}

		result = append(result, converted)
	}

	return strings.Join(result, "\n")
}

// mapStatusColor はJIRAの色名をCSSクラス名にマッピング
func mapStatusColor(color string) string {
	colorMap := map[string]string{
		"green":     "status-green",
		"yellow":    "status-yellow",
		"red":       "status-red",
		"blue":      "status-blue",
		"blue-gray": "status-blue",
		"grey":      "status-gray",
		"gray":      "status-gray",
	}
	return colorMap[color]
}

// statusLabelColorMap はカスタムステータスラベルの16進数カラーコードをCSSクラス名にマッピング
var statusLabelColorMap = map[string]string{
	"#ff991f": "status-label-warning", // オレンジ/警告
	"#00b8d9": "status-label-teal",    // ティール/OK
	"#36b37e": "status-label-success", // 緑/成功
	"#ff5630": "status-label-danger",  // 赤/危険
	"#6554c0": "status-label-purple",  // 紫
	"#97a0af": "status-label-gray",    // グレー
}

// convertStatusLabelMarkup はカスタムステータスラベルをHTMLスパンに変換
// パターン: {color:#XXX}*[ text ]*{color}
func (mw *MarkdownWriter) convertStatusLabelMarkup(text string) string {
	// 正規表現: {color:#HEXCODE}*[ text ]*{color}
	pattern := regexp.MustCompile(`(?i)\{color:(#[0-9a-fA-F]{6})\}\*\[\s*([^\]]+?)\s*\]\*\{color\}`)

	return pattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := pattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}

		colorCode := strings.ToLower(submatches[1])
		labelText := submatches[2]

		if className, ok := statusLabelColorMap[colorCode]; ok {
			return fmt.Sprintf(`<span class="status-label %s">%s</span>`, className, labelText)
		}
		// 未知の色はデフォルトクラス
		return fmt.Sprintf(`<span class="status-label">%s</span>`, labelText)
	})
}

// convertStatusMarkup は{status}マクロをHTMLスパンに変換
func (mw *MarkdownWriter) convertStatusMarkup(content string) string {
	// パターン: {status:colour=Green}text{status} または {status:color=Green}text{status}
	pattern := regexp.MustCompile(`(?i)\{status(?::colou?r=([^}]+))?\}([^{]*)\{status\}`)

	return pattern.ReplaceAllStringFunc(content, func(match string) string {
		submatches := pattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}

		color := strings.ToLower(submatches[1])
		text := submatches[2]

		// 色をCSSクラスにマッピング
		colorClass := mapStatusColor(color)

		if colorClass != "" {
			return fmt.Sprintf(`<span class="status %s">%s</span>`, colorClass, text)
		}
		return fmt.Sprintf(`<span class="status">%s</span>`, text)
	})
}

// convertQuoteMarkup は{quote}...{quote}をMarkdownの引用に変換
func (mw *MarkdownWriter) convertQuoteMarkup(text string) string {
	quotePattern := regexp.MustCompile(`(?s)\{quote\}(.*?)\{quote\}`)
	return quotePattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := quotePattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		content := submatches[1]
		lines := strings.Split(content, "\n")
		var result []string

		for _, line := range lines {
			// 各行を> で始める
			if strings.TrimSpace(line) != "" {
				result = append(result, "> "+line)
			} else {
				result = append(result, ">")
			}
		}

		return strings.Join(result, "\n")
	})
}

// convertColorMarkup は{color:...}...{color}をHTMLのspanタグに変換
// JIRAの色指定をそのままインラインスタイルとして出力
func (mw *MarkdownWriter) convertColorMarkup(text string) string {
	colorPattern := regexp.MustCompile(`(?s)\{color:([^}]+)\}(.*?)\{color\}`)
	return colorPattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := colorPattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}

		colorValue := submatches[1] // 元のカラーコードをそのまま使用
		content := submatches[2]

		return fmt.Sprintf(`<span style="color:%s">%s</span>`, colorValue, content)
	})
}

// getPanelClass はbgColorからCSSクラスを判別
func getPanelClass(bgColor string) string {
	bgColor = strings.ToLower(strings.TrimSpace(bgColor))
	if !strings.HasPrefix(bgColor, "#") {
		bgColor = "#" + bgColor
	}

	switch bgColor {
	case "#ffebe6":
		return "panel-error"
	case "#e3fcef":
		return "panel-success"
	case "#fffae6":
		return "panel-warning"
	case "#deebff":
		return "panel-info"
	default:
		return "panel-info"
	}
}

// parsePanelParams はpanelのパラメータ文字列を解析
func parsePanelParams(paramStr string) map[string]string {
	params := make(map[string]string)
	paramPattern := regexp.MustCompile(`(\w+)=([^|]+)`)
	matches := paramPattern.FindAllStringSubmatch(paramStr, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			key := strings.TrimSpace(match[1])
			value := strings.TrimSpace(match[2])
			params[key] = value
		}
	}

	return params
}

// convertPanelMarkup は{panel:...}...{panel}をHTMLのdivタグに変換
func (mw *MarkdownWriter) convertPanelMarkup(text string) string {
	// パラメータ付きpanel
	panelWithParamsPattern := regexp.MustCompile(`(?s)\{panel:([^}]+)\}(.*?)\{panel\}`)
	text = panelWithParamsPattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := panelWithParamsPattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}

		paramStr := submatches[1]
		content := submatches[2]
		params := parsePanelParams(paramStr)

		bgColor := params["bgColor"]
		title := params["title"]
		panelClass := getPanelClass(bgColor)

		var result string
		if title != "" {
			result = fmt.Sprintf(`<div class="panel %s"><div class="panel-title">%s</div><div class="panel-body">%s</div></div>`,
				panelClass, title, content)
		} else {
			result = fmt.Sprintf(`<div class="panel %s"><div class="panel-body">%s</div></div>`,
				panelClass, content)
		}

		return result
	})

	// パラメータなしpanel
	panelSimplePattern := regexp.MustCompile(`(?s)\{panel\}(.*?)\{panel\}`)
	text = panelSimplePattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := panelSimplePattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		content := submatches[1]
		return fmt.Sprintf(`<div class="panel panel-info"><div class="panel-body">%s</div></div>`, content)
	})

	return text
}

// getAdmonitionClass はadmonitionタイプからCSSクラスを取得
func getAdmonitionClass(admonitionType string) string {
	switch strings.ToLower(admonitionType) {
	case "note":
		return "panel-note"
	case "info":
		return "panel-info"
	case "warning":
		return "panel-warning"
	case "tip":
		return "panel-success"
	default:
		return "panel-info"
	}
}

// convertAdmonitionMarkup は{note}等のadmonitionをpanelに変換
func (mw *MarkdownWriter) convertAdmonitionMarkup(text string) string {
	// Goのregexpはバックリファレンスをサポートしないため、各タイプ別に処理
	admonitionTypes := []string{"note", "info", "warning", "tip"}

	for _, adType := range admonitionTypes {
		pattern := regexp.MustCompile(`(?s)\{` + adType + `\}(.*?)\{` + adType + `\}`)
		text = pattern.ReplaceAllStringFunc(text, func(match string) string {
			submatches := pattern.FindStringSubmatch(match)
			if len(submatches) < 2 {
				return match
			}

			content := submatches[1]
			panelClass := getAdmonitionClass(adType)

			return fmt.Sprintf(`<div class="panel %s"><div class="panel-body">%s</div></div>`,
				panelClass, content)
		})
	}

	return text
}
