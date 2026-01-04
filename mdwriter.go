package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/andygrunwald/go-jira/v2/cloud"
)

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

// MarkdownWriter はMarkdown形式で課題を出力する
type MarkdownWriter struct {
	outputDir      string
	attachmentsDir string
	userMapping    UserMapping
}

// NewMarkdownWriter は新しいMarkdownWriterを作成する
func NewMarkdownWriter(outputDir, attachmentsDir string, userMapping UserMapping) *MarkdownWriter {
	if userMapping == nil {
		userMapping = make(UserMapping)
	}
	return &MarkdownWriter{
		outputDir:      outputDir,
		attachmentsDir: attachmentsDir,
		userMapping:    userMapping,
	}
}

// WriteIssue は課題をMarkdownファイルに出力する
func (mw *MarkdownWriter) WriteIssue(issue *cloud.Issue, attachmentFiles []string, fieldNameCache FieldNameCache, devStatus *DevStatusDetail) error {
	// プロジェクトキーを取得
	projectKey := issue.Fields.Project.Key

	// プロジェクト別の出力ディレクトリの作成
	projectDir := filepath.Join(mw.outputDir, projectKey)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("Markdown出力ディレクトリの作成に失敗しました: %w", err)
	}

	// Markdownコンテンツの生成
	content := mw.generateMarkdown(issue, attachmentFiles, fieldNameCache, devStatus)

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
	sb.WriteString(fmt.Sprintf("title = \"%s\"\n", escapeTOMLString(project.Name)))
	sb.WriteString(fmt.Sprintf("project_key = \"%s\"\n", project.Key))
	sb.WriteString(fmt.Sprintf("project_name = \"%s\"\n", escapeTOMLString(project.Name)))
	sb.WriteString("type = \"project\"\n")
	sb.WriteString("+++\n\n")

	// 本文
	sb.WriteString(fmt.Sprintf("# %s - %s\n\n", project.Key, project.Name))
	if project.Description != "" {
		sb.WriteString(project.Description)
		sb.WriteString("\n")
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
func (mw *MarkdownWriter) generateFrontMatter(sb *strings.Builder, issue *cloud.Issue) {
	sb.WriteString("+++\n")
	sb.WriteString(fmt.Sprintf("title = \"%s\"\n", escapeTOMLString(issue.Fields.Summary)))
	sb.WriteString(fmt.Sprintf("date = %s\n", mw.formatTimeISO8601(issue.Fields.Created)))
	sb.WriteString(fmt.Sprintf("lastmod = %s\n", mw.formatTimeISO8601(issue.Fields.Updated)))
	sb.WriteString(fmt.Sprintf("project = \"%s\"\n", issue.Fields.Project.Key))
	sb.WriteString(fmt.Sprintf("issue_key = \"%s\"\n", issue.Key))
	sb.WriteString(fmt.Sprintf("status = \"%s\"\n", escapeTOMLString(issue.Fields.Status.Name)))
	sb.WriteString(fmt.Sprintf("type = \"%s\"\n", escapeTOMLString(issue.Fields.Type.Name)))
	sb.WriteString(fmt.Sprintf("assignee = \"%s\"\n", escapeTOMLString(mw.getUser(issue.Fields.Assignee))))
	sb.WriteString(fmt.Sprintf("reporter = \"%s\"\n", escapeTOMLString(mw.getUser(issue.Fields.Reporter))))
	sb.WriteString("+++\n\n")
}

// generateTitle は課題のタイトルを生成する
func (mw *MarkdownWriter) generateTitle(sb *strings.Builder, issue *cloud.Issue) {
	sb.WriteString(fmt.Sprintf("# %s: %s\n\n", issue.Key, issue.Fields.Summary))
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

	// 期限が設定されている場合のみ出力
	duedate := time.Time(issue.Fields.Duedate)
	if !duedate.IsZero() {
		sb.WriteString(fmt.Sprintf("- **期限**: %s\n", duedate.Format("2006-01-02")))
	}

	// ラベルが設定されている場合のみ出力
	if len(issue.Fields.Labels) > 0 {
		sb.WriteString(fmt.Sprintf("- **ラベル**: %s\n", strings.Join(issue.Fields.Labels, ", ")))
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

	if issue.Fields.Resolution != nil {
		sb.WriteString(fmt.Sprintf("- **解決状況**: %s\n", issue.Fields.Resolution.Name))
	}

	// カスタムフィールド（値があるもののみ表示）
	customFields := GetAllCustomFields(issue)
	if len(customFields) > 0 {
		sortedKeys := GetSortedCustomFieldKeys(customFields)
		for _, key := range sortedKeys {
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
			if fieldValue == "" {
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
			// プルリクエスト
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

			// ブランチ
			if len(detail.Branches) > 0 {
				sb.WriteString("### ブランチ\n\n")
				for _, branch := range detail.Branches {
					sb.WriteString(fmt.Sprintf("- [`%s`](%s)\n", branch.Name, branch.URL))
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

// generateComments はコメントセクションを生成する
func (mw *MarkdownWriter) generateComments(sb *strings.Builder, issue *cloud.Issue, attachmentMap map[string]string) {
	if issue.Fields.Comments != nil && len(issue.Fields.Comments.Comments) > 0 {
		sb.WriteString("## コメント\n\n")
		for i, comment := range issue.Fields.Comments.Comments {
			sb.WriteString(fmt.Sprintf("### コメント %d\n\n", i+1))
			sb.WriteString(fmt.Sprintf("- **投稿者**: %s\n", mw.getUser(comment.Author)))
			sb.WriteString(fmt.Sprintf("- **投稿日**: %s\n", mw.formatTimeString(comment.Created)))
			sb.WriteString("\n")
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
func (mw *MarkdownWriter) generateMarkdown(issue *cloud.Issue, attachmentFiles []string, fieldNameCache FieldNameCache, devStatus *DevStatusDetail) string {
	var sb strings.Builder

	// 添付ファイルのマッピングを作成（元のファイル名 → 保存されたファイル名）
	attachmentMap := mw.buildAttachmentMap(issue, attachmentFiles)

	// Front Matter
	mw.generateFrontMatter(&sb, issue)

	// タイトル
	mw.generateTitle(&sb, issue)

	sb.WriteString("<!-- PAGE_RIGHT_START -->\n\n")

	// 基本情報
	mw.generateBasicInfo(&sb, issue, fieldNameCache, devStatus)

	// 開発情報
	mw.generateDevelopmentInfo(&sb, devStatus)

	sb.WriteString("<!-- PAGE_RIGHT_END -->\n\n")

	// 説明
	mw.generateDescription(&sb, issue, attachmentMap)

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

	// 5. テーブルを直接変換（プレースホルダー化せず）
	text, tables := mw.extractJIRATables(text)
	for i, table := range tables {
		placeholder := fmt.Sprintf("__TABLE_%d__", i)
		markdownTable := mw.convertJIRATableToMarkdown(table)
		text = strings.ReplaceAll(text, placeholder, markdownTable)
	}

	// 6. メンション変換: [~accountid:xxx] → @ユーザー名
	mentionPattern := regexp.MustCompile(`\[~accountid:([^\]]+)\]`)
	text = mentionPattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := mentionPattern.FindStringSubmatch(match)
		if len(submatches) >= 2 {
			accountID := submatches[1]

			// account IDからユーザー名を取得
			if userName, exists := mw.userMapping[accountID]; exists && userName != "" {
				return "@" + userName
			}

			// マッピングが見つからない場合はaccount IDを表示
			return "@" + accountID
		}
		return match
	})

	// 7. リンク変換: [text|url] → [text](url)
	linkPattern := regexp.MustCompile(`\[([^\]|]+)\|([^\]]+)\]`)
	text = linkPattern.ReplaceAllString(text, `[$1]($2)`)

	// 8. 太字: *text* → **text**
	// 単語境界を考慮して、前後にスペースまたは行頭/行末があることを確認
	boldPattern := regexp.MustCompile(`(^|[\s\n])\*([^\*\n]+)\*([\s\n]|$)`)
	text = boldPattern.ReplaceAllString(text, `${1}**$2**${3}`)

	// 9. イタリック: _text_ → *text*
	italicPattern := regexp.MustCompile(`(^|[\s\n])_([^_\n]+)_([\s\n]|$)`)
	text = italicPattern.ReplaceAllString(text, `${1}*$2*${3}`)

	// 10. 削除線: -text- → ~~text~~
	strikePattern := regexp.MustCompile(`(^|[\s\n])-([^-\n]+)-([\s\n]|$)`)
	text = strikePattern.ReplaceAllString(text, `${1}~~$2~~${3}`)

	// 11. 上付き: ^text^ → <sup>text</sup>
	supPattern := regexp.MustCompile(`\^([^\^]+)\^`)
	text = supPattern.ReplaceAllString(text, `<sup>$1</sup>`)

	// 12. 下付き: ~text~ → <sub>text</sub>
	subPattern := regexp.MustCompile(`~([^~]+)~`)
	text = subPattern.ReplaceAllString(text, `<sub>$1</sub>`)

	// 13. プレースホルダーを元のコードブロックとインラインコードに戻す
	for i, codeBlock := range codeBlocks {
		placeholder := fmt.Sprintf("__CODE_BLOCK_%d__", i)
		text = strings.ReplaceAll(text, placeholder, codeBlock)
	}
	for i, inlineCode := range inlineCodes {
		placeholder := fmt.Sprintf("__INLINE_CODE_%d__", i)
		text = strings.ReplaceAll(text, placeholder, inlineCode)
	}

	return text
}
