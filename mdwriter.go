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

// RenderedFields はレンダリング済みのフィールドを保持する構造体
type RenderedFields struct {
	Description string `json:"description"`
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

// extractRenderedFields はissueのJSONからレンダリング済みフィールドを抽出する
func extractRenderedFields(issue *cloud.Issue) *RenderedFields {
	jsonData, err := json.Marshal(issue)
	if err != nil {
		return nil
	}

	var rawIssue struct {
		RenderedFields RenderedFields `json:"renderedFields"`
	}
	if err := json.Unmarshal(jsonData, &rawIssue); err != nil {
		return nil
	}

	if rawIssue.RenderedFields.Description == "" {
		return nil
	}
	return &rawIssue.RenderedFields
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
	outputDir        string
	userMapping      UserMapping
	config           *Config
	confluenceClient *ConfluenceClient
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
	// Start date（設定から取得）
	if startDate, exists := customFields[mw.config.Display.StartDateFieldId]; exists && !IsCustomFieldEmpty(startDate) {
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
	if startDate, exists := customFields[mw.config.Display.StartDateFieldId]; exists && !IsCustomFieldEmpty(startDate) {
		fieldName := fieldNameCache.GetFieldName(mw.config.Display.StartDateFieldId)
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
		sb.WriteString(fmt.Sprintf("- **親課題**: [%s](../%s/)\n", issue.Fields.Parent.Key, strings.ToLower(issue.Fields.Parent.Key)))
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

			// コミット（ブランチとプルリクエストの間に出力、JIRA仕様に合わせる）
			if len(detail.Commits) > 0 {
				sb.WriteString("### コミット\n\n")
				for _, commit := range detail.Commits {
					// 1行目: displayId + タイムスタンプ
					sb.WriteString(fmt.Sprintf("- [`%s`](%s)", commit.DisplayID, commit.URL))
					if commit.Timestamp != "" {
						if t, err := time.Parse(time.RFC3339, commit.Timestamp); err == nil {
							sb.WriteString(fmt.Sprintf(" %s", t.Format("2006-01-02 15:04:05")))
						}
					}
					sb.WriteString("\n")
					// 2行目: メッセージ（最初の1行のみ）
					if commit.Message != "" {
						firstLine := strings.SplitN(commit.Message, "\n", 2)[0]
						firstLine = strings.TrimSpace(firstLine)
						if firstLine != "" {
							sb.WriteString(fmt.Sprintf("    - %s\n", firstLine))
						}
					}
					// 3行目: 作成者
					if commit.Author != "" {
						sb.WriteString(fmt.Sprintf("    - 作成者: %s\n", commit.Author))
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
	description := ""

	// ステップ1: fields.description から課題キーを検索
	if issue.Fields.Description != "" {
		if mw.containsIssueLink(issue.Fields.Description) {
			description = issue.Fields.Description
		} else {
			// ステップ2: リンクがない場合、renderedFields.description を確認
			renderedFields := extractRenderedFields(issue)
			if renderedFields != nil && renderedFields.Description != "" && mw.containsHTMLIssueMacro(renderedFields.Description) {
				description = renderedFields.Description
			} else {
				// ステップ3: renderedFields にもリンクがない場合、fields を採用
				description = issue.Fields.Description
			}
		}
	} else {
		// fields.description が空の場合、renderedFields から開始
		renderedFields := extractRenderedFields(issue)
		if renderedFields != nil && renderedFields.Description != "" {
			description = renderedFields.Description
		}
	}

	if description != "" {
		sb.WriteString("## 説明\n\n")

		// 添付ファイル参照を変換（convertJIRAMarkupToMarkdownの前に実行）
		description = mw.replaceAttachmentReferences(description, attachmentMap)

		// JIRA Wiki形式の処理: fields.descriptionから来た場合（HTML形式以外）
		if !strings.Contains(description, "jira-issue-macro") {
			description = mw.convertJIRAMarkupToMarkdown(description, issue.Fields.Project.Key)
		}

		// HTML形式の処理
		if strings.Contains(description, "jira-issue-macro") {
			description = mw.convertHTMLJIRAIssueMacroToRelative(description, issue.Fields.Project.Key)
		}

		// 画像参照を変換（属性付き→属性なしの順）
		description = mw.replaceImageReferencesWithAttributes(description, attachmentMap)
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
			avatarURL := mw.getAvatarURL(comment.Author)
			dateStr := mw.formatCommentDate(comment.Created)

			// 返信かどうかを判定（本文が[~accountid:で始まる場合）
			isReply := strings.HasPrefix(comment.Body, "[~accountid:")

			// Shortcode開始タグの構築
			sb.WriteString(`{{< comment `)

			// iconパラメータ（URLが存在する場合のみ）
			if avatarURL != "" {
				sb.WriteString(fmt.Sprintf(`icon="%s" `, avatarURL))
			}

			// nameパラメータ（返信の場合は↩️を含める）
			if isReply {
				sb.WriteString(fmt.Sprintf(`name="↩️ %s" `, authorName))
			} else {
				sb.WriteString(fmt.Sprintf(`name="%s" `, authorName))
			}

			// createdパラメータ
			sb.WriteString(fmt.Sprintf(`created="%s" `, dateStr))

			// replyパラメータ（返信の場合のみ）
			if isReply {
				sb.WriteString(`reply="true" `)
			}

			sb.WriteString(">}}\n")

			// コメント本文の変換
			commentBody := comment.Body
			// 添付ファイル参照を変換（convertJIRAMarkupToMarkdownの前に実行）
			commentBody = mw.replaceAttachmentReferences(commentBody, attachmentMap)
			// JIRAマークアップをMarkdownに変換
			commentBody = mw.convertJIRAMarkupToMarkdown(commentBody, issue.Fields.Project.Key)
			// 画像参照を変換（属性付き→属性なしの順）
			commentBody = mw.replaceImageReferencesWithAttributes(commentBody, attachmentMap)
			commentBody = mw.replaceImageReferences(commentBody, attachmentMap)

			// コメント本文の出力
			sb.WriteString(commentBody)
			sb.WriteString("\n")

			// Shortcode終了タグ
			sb.WriteString("{{< /comment >}}\n\n")
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
			strings.Contains(strings.ToLower(link.Application.Type), "confluence") {
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

			// 設定ファイルで無視対象とされたタイトルは空として扱う
			if mw.config != nil {
				for _, ignored := range mw.config.Confluence.IgnoredTitles {
					if title == ignored {
						title = ""
						break
					}
				}
			}

			// スペース名を取得
			spaceName := ""
			if mw.confluenceClient != nil && link.GlobalID != "" {
				pageID, err := ExtractPageIDFromGlobalID(link.GlobalID)
				if err == nil {
					spaceName, _ = mw.confluenceClient.GetSpaceName(pageID)
				}
			}

			// タイトルがある場合
			if title != "" {
				if spaceName != "" {
					sb.WriteString(fmt.Sprintf("- [%s / %s](%s)\n", spaceName, title, link.Object.URL))
				} else {
					sb.WriteString(fmt.Sprintf("- [%s](%s)\n", title, link.Object.URL))
				}
			} else {
				// タイトルがない場合はURLをリンク形式で表示
				sb.WriteString(fmt.Sprintf("- [%s](%s)\n", link.Object.URL, link.Object.URL))
			}
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
			// 同じディレクトリ内の相対パスで添付ファイルを参照
			sb.WriteString(fmt.Sprintf("- [%s](%s)\n", filename, encodedFilename))
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

// ImageAttributes は画像の属性を保持する構造体
type ImageAttributes struct {
	Width string // 例: "300px"
	Alt   string // 例: "説明文"
}

// splitAttributeString は属性文字列をカンマで分割する（引用符内は除外）
func splitAttributeString(s string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, ch := range s {
		switch ch {
		case '"', '\'':
			if !inQuote {
				inQuote = true
				quoteChar = ch
			} else if ch == quoteChar {
				inQuote = false
			}
			current.WriteRune(ch)
		case ',':
			if inQuote {
				current.WriteRune(ch)
			} else {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// parseImageAttributes は属性文字列をパースする
// 入力例: "width=300,alt=\"スクリーンショット\""
func parseImageAttributes(attrStr string) ImageAttributes {
	attrs := ImageAttributes{}

	// カンマで分割（ただし引用符内のカンマは除外）
	parts := splitAttributeString(attrStr)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// key=value 形式を分割
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		// 引用符を除去
		value = strings.Trim(value, "\"'")

		switch key {
		case "width":
			// 数値のみの場合は "px" を追加
			if matched, _ := regexp.MatchString(`^\d+$`, value); matched {
				attrs.Width = value + "px"
			} else {
				attrs.Width = value
			}
		case "alt":
			attrs.Alt = value
		}
	}

	return attrs
}

// replaceImageReferencesWithAttributes はJIRA形式の属性付き画像参照を変換する
// パターン: !$filename.png|width=300,alt="説明"!
func (mw *MarkdownWriter) replaceImageReferencesWithAttributes(text string, attachmentMap map[string]string) string {
	// JIRA形式の属性付き画像参照パターン: !filename.png|属性! または !$filename.png|属性!
	pattern := regexp.MustCompile(`!(?:\$)?([^!|]+(?:\.[a-zA-Z0-9]+))\|([^!]+)!`)

	result := pattern.ReplaceAllStringFunc(text, func(match string) string {
		// マッチからファイル名と属性を抽出
		submatches := pattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		originalFilename := submatches[1]
		attrStr := submatches[2]

		// 添付ファイルマップから保存されたファイル名を取得
		savedFilename, exists := attachmentMap[originalFilename]
		if !exists {
			return match // 見つからない場合は元のまま
		}

		// ファイル名をURLエンコーディング（スペース→%20）
		encodedFilename := url.PathEscape(savedFilename)

		// 属性をパース
		attrs := parseImageAttributes(attrStr)

		// 画像ファイルの場合のみ処理
		if !IsImageFile(originalFilename) {
			return match
		}

		// alt が指定されていない場合はファイル名を使用
		alt := attrs.Alt
		if alt == "" {
			alt = originalFilename
		}

		// Markdown形式に変換（相対パス）
		// title属性を使って幅を指定: ![alt](path "width=250")
		if attrs.Width != "" {
			return fmt.Sprintf("![%s](%s \"%s\")", alt, encodedFilename, "width="+attrs.Width)
		}
		return fmt.Sprintf("![%s](%s)", alt, encodedFilename)
	})

	return result
}

// replaceAttachmentReferences はJIRA形式の添付ファイル参照 [^filename.ext] をMarkdownリンクに変換する
func (mw *MarkdownWriter) replaceAttachmentReferences(text string, attachmentMap map[string]string) string {
	// パターン1: [表示テキスト|^filename.ext]（テキスト指定版を先に処理）
	textPattern := regexp.MustCompile(`\[([^\]|]+)\|\^([^\]]+)\]`)
	text = textPattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := textPattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		displayText := submatches[1]
		filename := submatches[2]
		savedFilename, exists := attachmentMap[filename]
		if !exists {
			return match
		}
		encodedFilename := url.PathEscape(savedFilename)
		return fmt.Sprintf("[%s](%s)", displayText, encodedFilename)
	})

	// パターン2: [^filename.ext]
	simplePattern := regexp.MustCompile(`\[\^([^\]]+)\]`)
	text = simplePattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := simplePattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		filename := submatches[1]
		savedFilename, exists := attachmentMap[filename]
		if !exists {
			return match
		}
		encodedFilename := url.PathEscape(savedFilename)
		return fmt.Sprintf("[%s](%s)", filename, encodedFilename)
	})

	return text
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
		// 画像ファイルの場合は画像形式、それ以外はリンク形式（同じディレクトリ内の相対パス）
		if IsImageFile(originalFilename) {
			return fmt.Sprintf("![%s](%s)", originalFilename, encodedFilename)
		}
		return fmt.Sprintf("[%s](%s)", originalFilename, encodedFilename)
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

// listInfo represents information about an open list
type listInfo struct {
	listType string // "ul" or "ol"
	level    int
}

// convertCellListsToHTML converts JIRA list elements within a table cell to HTML list tags
func convertCellListsToHTML(cell string) string {
	lines := strings.Split(cell, "\n")
	var listStack []listInfo // tracks open lists with type and level
	var output strings.Builder

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for unordered list (* ** ***)
		if match := regexp.MustCompile(`^(\*+)\s+(.+)$`).FindStringSubmatch(trimmed); match != nil {
			level := len(match[1])
			content := match[2]
			items := processListItem("ul", level, content, &listStack)
			for _, item := range items {
				output.WriteString(item)
			}
			continue
		}

		// Check for ordered list (# ## ###)
		if match := regexp.MustCompile(`^(#+)\s+(.+)$`).FindStringSubmatch(trimmed); match != nil {
			level := len(match[1])
			content := match[2]
			items := processListItem("ol", level, content, &listStack)
			for _, item := range items {
				output.WriteString(item)
			}
			continue
		}

		// Non-list line: close all open lists
		hadOpenLists := len(listStack) > 0
		items := closeAllLists(&listStack)
		for _, item := range items {
			output.WriteString(item)
		}

		if trimmed != "" {
			// Add newline after list closing tags if there were open lists
			if hadOpenLists && len(items) > 0 {
				output.WriteString("\n")
			}
			output.WriteString(line)
			// Add newline between lines (except after the last line)
			if i < len(lines)-1 {
				output.WriteString("\n")
			}
		} else if i < len(lines)-1 {
			// Empty line: still add newline if not the last line
			output.WriteString("\n")
		}
	}

	// Close any remaining open lists
	items := closeAllLists(&listStack)
	for _, item := range items {
		output.WriteString(item)
	}

	return output.String()
}

// processListItem handles a single list item, managing list opening/closing
func processListItem(listType string, level int, content string, stack *[]listInfo) []string {
	var result []string

	// Close lists that are deeper or different type at same level
	for len(*stack) > 0 {
		top := (*stack)[len(*stack)-1]
		if top.level > level || (top.level == level && top.listType != listType) {
			result = append(result, closeList(top.listType))
			*stack = (*stack)[:len(*stack)-1]
		} else {
			break
		}
	}

	// Open new list if needed
	if len(*stack) == 0 || (*stack)[len(*stack)-1].level < level {
		result = append(result, openList(listType))
		*stack = append(*stack, listInfo{listType: listType, level: level})
	}

	result = append(result, fmt.Sprintf("<li>%s</li>", content))
	return result
}

// openList returns the opening tag for a list
func openList(listType string) string {
	return fmt.Sprintf("<%s>", listType)
}

// closeList returns the closing tag for a list
func closeList(listType string) string {
	return fmt.Sprintf("</%s>", listType)
}

// closeAllLists closes all open lists in the stack
func closeAllLists(stack *[]listInfo) []string {
	var result []string
	for i := len(*stack) - 1; i >= 0; i-- {
		result = append(result, closeList((*stack)[i].listType))
	}
	*stack = (*stack)[:0]
	return result
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
				// セル内のリスト要素をHTMLに変換（<br>置換前）
				for j, cell := range cells {
					cells[j] = convertCellListsToHTML(cell)
				}
				// セル内の残りの改行を <br> に置換
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
				// セル内のリスト要素をHTMLに変換（<br>置換前）
				for j, cell := range cells {
					cells[j] = convertCellListsToHTML(cell)
				}
				// セル内の残りの改行を <br> に置換
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
func (mw *MarkdownWriter) convertJIRAMarkupToMarkdown(text string, currentProjectKey string) string {
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

	// 6-2. JIRA課題URLを相対パスリンクに変換（リンク変換の前に実行）
	text = mw.convertJIRAIssueLinksToRelative(text, currentProjectKey)

	// 7. リンク変換: [text|url] → [text](url)、[text|url|smart-link] → [text](url)
	linkPattern := regexp.MustCompile(`\[([^\]|]+)\|([^\]|]+)(?:\|[^\]]+)?\]`)
	text = linkPattern.ReplaceAllString(text, `[$1]($2)`)

	// 7-1. Markdownリンクを保護（装飾変換でURL内の~等が誤変換されないようにする）
	mdLinkPattern := regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)
	var protectedLinks []string
	linkProtectIndex := 0
	text = mdLinkPattern.ReplaceAllStringFunc(text, func(match string) string {
		placeholder := fmt.Sprintf("___LINK_PROTECT_%d___", linkProtectIndex)
		protectedLinks = append(protectedLinks, match)
		linkProtectIndex++
		return placeholder
	})

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

	// 10-1. イタリック変換後に残った _ は本文の記号なので \_ にエスケープ
	text = escapeRemainingUnderscores(text)

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

	// 7-2. Markdownリンクを復元
	for i, link := range protectedLinks {
		placeholder := fmt.Sprintf("___LINK_PROTECT_%d___", i)
		text = strings.Replace(text, placeholder, link, 1)
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
	// Markdownの見出しパターン（行頭から#が始まる、スペースなし）
	markdownHeadingPattern := regexp.MustCompile(`^#{1,6}\s+.+$`)

	for _, line := range lines {
		// Markdownの見出しは保護対象から除外
		if markdownHeadingPattern.MatchString(line) {
			result = append(result, line)
			continue
		}

		if bulletListPattern.MatchString(line) || numberedListPattern.MatchString(line) {
			// リスト行をプレースホルダーに置き換え
			// 修正: 元の行番号iではなく、protected配列のインデックスを使用
			placeholder := fmt.Sprintf("___LIST_PLACEHOLDER_%d___", len(protected))
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

// isInvalidItalicBoundary は、文字がイタリック記法の無効な境界文字かどうかを判定します。
// 正規表現の記号（%, \, [, ]など）に隣接した _ はイタリック記法として無効です。
func isInvalidItalicBoundary(r rune) bool {
	switch r {
	case '%', '\\', '[', ']', '$', '(', ')', '{', '}', '+', '^', '|', '?', '.':
		return true
	}
	return false
}

// isAlphanumeric は、文字が英数字かどうかを判定します。
func isAlphanumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || (r >= '０' && r <= '９')
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

				// 前の文字をチェック
				var prevChar, nextChar rune
				prevIsAlphanumeric := false
				nextIsAlphanumeric := false

				if !isItalic && start > 0 {
					prevRunes := []rune(converted[:start])
					if len(prevRunes) > 0 {
						prevChar = prevRunes[len(prevRunes)-1]
						prevIsAlphanumeric = isAlphanumeric(prevChar)
						// 無効な境界記号かチェック
						if isInvalidItalicBoundary(prevChar) {
							isItalic = true
						}
					}
				}

				// 後の文字をチェック
				if !isItalic && end < len(converted) {
					nextRunes := []rune(converted[end:])
					if len(nextRunes) > 0 {
						nextChar = nextRunes[0]
						nextIsAlphanumeric = isAlphanumeric(nextChar)
						// 無効な境界記号かチェック
						if isInvalidItalicBoundary(nextChar) {
							isItalic = true
						}
					}
				}

				// スネークケース判定：前後が両方とも英数字なら無効（Japanese_Bushu_Kakusu_140_CI_SA など）
				if !isItalic && prevIsAlphanumeric && nextIsAlphanumeric {
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

// escapeRemainingUnderscores はイタリック変換後に残った _ を \_ にエスケープする。
// プレースホルダーやMarkdownリンクのURLの _ はエスケープしない。
func escapeRemainingUnderscores(text string) string {
	if !strings.Contains(text, "_") {
		return text
	}

	const markerStart = "\uE000"
	const markerEnd = "\uE001"

	// 1. プレースホルダーを保護: __XXX__ や ___XXX___ 形式
	phPattern := regexp.MustCompile(`_{2,}[A-Z][A-Z_0-9]+_{2,}`)
	var phs []string
	text = phPattern.ReplaceAllStringFunc(text, func(match string) string {
		idx := len(phs)
		phs = append(phs, match)
		return fmt.Sprintf("%sPH%d%s", markerStart, idx, markerEnd)
	})

	// 2. MarkdownリンクのURL部分を保護: ](url)
	urlPattern := regexp.MustCompile(`\]\([^)]*\)`)
	var urls []string
	text = urlPattern.ReplaceAllStringFunc(text, func(match string) string {
		idx := len(urls)
		urls = append(urls, match)
		return fmt.Sprintf("%sURL%d%s", markerStart, idx, markerEnd)
	})

	// 3. 全ての _ を \_ にエスケープ
	text = strings.ReplaceAll(text, "_", `\_`)

	// 4. URLを復元
	for i, url := range urls {
		text = strings.Replace(text, fmt.Sprintf("%sURL%d%s", markerStart, i, markerEnd), url, 1)
	}

	// 5. プレースホルダーを復元
	for i, ph := range phs {
		text = strings.Replace(text, fmt.Sprintf("%sPH%d%s", markerStart, i, markerEnd), ph, 1)
	}

	return text
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

// convertQuoteListsToMarkdown は引用内のJIRAリストをMarkdownリストに変換
func convertQuoteListsToMarkdown(content string) string {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))

	bulletListPattern := regexp.MustCompile(`^(\*+)\s+(.+)$`)
	numberedListPattern := regexp.MustCompile(`^(#+)\s+(.+)$`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 箇条書きリスト (*) の処理
		if matches := bulletListPattern.FindStringSubmatch(trimmed); len(matches) == 3 {
			asterisks := matches[1]
			itemContent := matches[2]
			level := len(asterisks) - 1
			indent := strings.Repeat("    ", level)
			result = append(result, indent+"- "+itemContent)
			continue
		}

		// 番号付きリスト (#) の処理
		if matches := numberedListPattern.FindStringSubmatch(trimmed); len(matches) == 3 {
			hashes := matches[1]
			itemContent := matches[2]
			level := len(hashes) - 1
			indent := strings.Repeat("    ", level)
			result = append(result, indent+"1. "+itemContent)
			continue
		}

		// リストではない通常の行
		result = append(result, line)
	}

	return strings.Join(result, "\n")
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

		// 引用内のJIRAリストをMarkdownリストに変換
		content = convertQuoteListsToMarkdown(content)

		// 各行の先頭に引用記号を追加
		lines := strings.Split(content, "\n")
		var result []string

		for _, line := range lines {
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

// containsIssueLink は JIRA Wiki形式の課題リンクが含まれているか判定
func (mw *MarkdownWriter) containsIssueLink(text string) bool {
	// JIRA Wiki形式: [text|URL|smart-link] または [URL|URL|smart-link] または [text|URL]
	return strings.Contains(text, "smart-link") ||
		(strings.Contains(text, "[") && strings.Contains(text, "|") && strings.Contains(text, "browse/"))
}

// containsHTMLIssueMacro は HTML形式のJIRA課題マクロが含まれているか判定
func (mw *MarkdownWriter) containsHTMLIssueMacro(text string) bool {
	return strings.Contains(text, "jira-issue-macro") && strings.Contains(text, "data-jira-key")
}

// convertHTMLJIRAIssueMacroToRelative は HTML形式のJIRA課題マクロを相対パスリンクに変換
func (mw *MarkdownWriter) convertHTMLJIRAIssueMacroToRelative(text string, currentProjectKey string) string {
	// ステップ1: 全てのJIRA issue macro spanを見つける
	// パターン: <span class="jira-issue-macro ..." data-jira-key="ISSUE-KEY">...
	macroPattern := regexp.MustCompile(
		`<span\s+class="jira-issue-macro[^"]*"\s+data-jira-key="([A-Z][A-Z0-9_]*-[0-9]+)"[^>]*>`,
	)

	// 各マクロを処理
	for {
		match := macroPattern.FindStringSubmatchIndex(text)
		if match == nil {
			break
		}

		// マクロの開始位置とissue keyを取得
		macroStart := match[0]
		macroEnd := match[1]
		issueKeyStart := match[2]
		issueKeyEnd := match[3]

		if issueKeyStart < 0 {
			break
		}

		issueKey := text[issueKeyStart:issueKeyEnd]
		projectKey := strings.ToLower(strings.Split(issueKey, "-")[0])
		issueKeyLower := strings.ToLower(issueKey)

		// ステップ2: マクロの内容を探して、対応する</span>とステータス情報を抽出
		// マクロのspan深度を追跡
		spanDepth := 1
		pos := macroEnd
		macroContent := ""
		statusText := ""

		for pos < len(text) && spanDepth > 0 {
			// 次の<span または </span> を見つける
			nextOpen := strings.Index(text[pos:], "<span")
			nextClose := strings.Index(text[pos:], "</span>")

			// どちらが先か判定
			if nextClose == -1 {
				// </span> がもう見つからない
				break
			}

			if nextOpen != -1 && nextOpen < nextClose {
				// <span が先
				spanDepth++
				macroContent += text[pos : pos+nextOpen+5]
				pos += nextOpen + 5
			} else {
				// </span> が先
				spanDepth--
				if spanDepth == 0 {
					macroContent += text[pos : pos+nextClose]
					macroEnd = pos + nextClose + 7
					break
				}
				macroContent += text[pos : pos+nextClose+7]
				pos += nextClose + 7
			}
		}

		// ステップ3: マクロの内容からステータスを抽出
		if strings.Contains(macroContent, "aui-lozenge") {
			// aui-lozenge を含むspan内のテキストを抽出
			statusPattern := regexp.MustCompile(`<span\s+class="[^"]*aui-lozenge[^"]*"[^>]*>([^<]+)</span>`)
			if matches := statusPattern.FindStringSubmatch(macroContent); len(matches) > 1 {
				statusText = strings.TrimSpace(matches[1])
			}
		}

		// ステップ4: Markdown形式に変換（同プロジェクト: ../key/、別プロジェクト: ../../project/key/）
		currentProject := strings.ToLower(currentProjectKey)
		var result string
		if projectKey == currentProject {
			result = fmt.Sprintf("[%s](../%s/)", issueKey, issueKeyLower)
		} else {
			result = fmt.Sprintf("[%s](../../%s/%s/)", issueKey, projectKey, issueKeyLower)
		}
		if statusText != "" {
			result += fmt.Sprintf(" (%s)", statusText)
		}

		// ステップ5: テキストを置き換え
		text = text[:macroStart] + result + text[macroEnd:]
	}

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

// convertJIRAIssueLinksToRelative はJIRA課題URLを相対パスリンクに変換する
// config.tomlで設定されたJIRAインスタンスのURLと一致するリンクのみ変換する
func (mw *MarkdownWriter) convertJIRAIssueLinksToRelative(text string, currentProjectKey string) string {
	// config.JIRA.URLからベースURLを取得
	if mw.config == nil || mw.config.JIRA.URL == "" {
		return text // 設定がない場合は変換しない
	}

	baseURL := mw.config.JIRA.URL
	baseURL = strings.TrimSuffix(baseURL, "/")
	escapedURL := regexp.QuoteMeta(baseURL)

	// パターン1: JIRA形式 [URL|smart-link] を変換
	// 同プロジェクト: [SCRUM-6](../scrum-6/)、別プロジェクト: [KT-3](../../kt/kt-3/)
	currentProject := strings.ToLower(currentProjectKey)
	pattern1 := regexp.MustCompile(
		`\[` + escapedURL + `/browse/([A-Z][A-Z0-9_]*)-([0-9]+)\|[^\]]*\]`,
	)
	text = pattern1.ReplaceAllStringFunc(text, func(match string) string {
		submatches := pattern1.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		targetProject := strings.ToLower(submatches[1])
		issueKey := strings.ToLower(submatches[1] + "-" + submatches[2])
		linkText := submatches[1] + "-" + submatches[2]
		if targetProject == currentProject {
			return "[" + linkText + "](../" + issueKey + "/)"
		}
		return "[" + linkText + "](../../" + targetProject + "/" + issueKey + "/)"
	})

	// パターン2: Markdown形式 [URL](URL) を変換（フォールバック）
	// 例: [https://...browse/SCRUM-6](https://...browse/SCRUM-6) → [SCRUM-6](../scrum-6/)
	pattern2 := regexp.MustCompile(
		`\[` + escapedURL + `/browse/([A-Z][A-Z0-9_]*)-([0-9]+)\]\(` +
			escapedURL + `/browse/[A-Z][A-Z0-9_]*-[0-9]+\)`,
	)
	text = pattern2.ReplaceAllStringFunc(text, func(match string) string {
		submatches := pattern2.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		targetProject := strings.ToLower(submatches[1])
		issueKey := strings.ToLower(submatches[1] + "-" + submatches[2])
		linkText := submatches[1] + "-" + submatches[2]
		if targetProject == currentProject {
			return "[" + linkText + "](../" + issueKey + "/)"
		}
		return "[" + linkText + "](../../" + targetProject + "/" + issueKey + "/)"
	})

	return text
}
