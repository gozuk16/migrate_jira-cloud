package main

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/andygrunwald/go-jira/v2/cloud"
)

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
		sb.WriteString(fmt.Sprintf("duedate = \"%s\"\n", duedate.Format(dateFormatDate)))
	}

	// Σ時間情報（サブタスク含む集計値）
	if aggTime := extractAggregateTimeFields(issue); aggTime != nil {
		if aggTime.AggregateTimeOriginalEstimate > 0 {
			sb.WriteString(fmt.Sprintf("aggregate_time_original_estimate = \"%s\"\n", mw.formatTimeSeconds(aggTime.AggregateTimeOriginalEstimate)))
		}
		if aggTime.AggregateTimeEstimate > 0 {
			sb.WriteString(fmt.Sprintf("aggregate_time_estimate = \"%s\"\n", mw.formatTimeSeconds(aggTime.AggregateTimeEstimate)))
		}
		if aggTime.AggregateTimeSpent > 0 {
			sb.WriteString(fmt.Sprintf("aggregate_time_spent = \"%s\"\n", mw.formatTimeSeconds(aggTime.AggregateTimeSpent)))
		}
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

// generateTitle は課題のパンくずリストを生成する
func (mw *MarkdownWriter) generateTitle(sb *strings.Builder, issue *cloud.Issue, parentInfo *ParentIssueInfo) {
	projectIcon := "📦"
	projectLink := fmt.Sprintf("[%s %s](../)", projectIcon, issue.Fields.Project.Name)
	issueIcon := getIssueTypeIcon(issue.Fields.Type.Name)
	issueLink := fmt.Sprintf("[%s %s](../%s/)", issueIcon, issue.Key, strings.ToLower(issue.Key))

	if parentInfo != nil && parentInfo.Key != "" {
		parentIcon := getIssueTypeIcon(parentInfo.Type)
		parentLink := fmt.Sprintf("[%s %s](../%s/)", parentIcon, parentInfo.Key, strings.ToLower(parentInfo.Key))
		sb.WriteString(fmt.Sprintf("%s / %s / %s\n{.breadcrumbs}\n\n", projectLink, parentLink, issueLink))
	} else {
		sb.WriteString(fmt.Sprintf("%s / %s\n{.breadcrumbs}\n\n", projectLink, issueLink))
	}
	sb.WriteString(fmt.Sprintf("# %s\n\n", issue.Fields.Summary))
}

// generateBasicInfo は基本情報セクションを生成する
func (mw *MarkdownWriter) generateBasicInfo(sb *strings.Builder, issue *cloud.Issue, fieldNameCache FieldNameCache, devStatus *DevStatusDetail) {
	sb.WriteString("## 基本情報\n\n")
	sb.WriteString("| 項目 | 値 |\n")
	sb.WriteString("|---|---|\n")
	sb.WriteString(fmt.Sprintf("| 課題キー | %s |\n", issue.Key))
	sb.WriteString(fmt.Sprintf("| 課題タイプ | %s |\n", issue.Fields.Type.Name))
	sb.WriteString(fmt.Sprintf("| ステータス | %s |\n", issue.Fields.Status.Name))
	sb.WriteString(fmt.Sprintf("| 優先度 | %s |\n", mw.getFieldString(issue.Fields.Priority)))
	sb.WriteString(fmt.Sprintf("| 担当者 | %s |\n", mw.getUser(issue.Fields.Assignee)))
	sb.WriteString(fmt.Sprintf("| 報告者 | %s |\n", mw.getUser(issue.Fields.Reporter)))
	sb.WriteString(fmt.Sprintf("| 作成日 | %s |\n", mw.formatTime(issue.Fields.Created)))
	sb.WriteString(fmt.Sprintf("| 更新日 | %s |\n", mw.formatTime(issue.Fields.Updated)))

	// Start date（カスタムフィールド）をここに表示
	customFields := GetAllCustomFields(issue)
	if startDate, exists := customFields[mw.config.Display.StartDateFieldId]; exists && !IsCustomFieldEmpty(startDate) {
		fieldName := fieldNameCache.GetFieldName(mw.config.Display.StartDateFieldId)
		fieldValue := FormatCustomFieldValue(startDate)
		if fieldValue != "" {
			sb.WriteString(fmt.Sprintf("| %s | %s |\n", fieldName, fieldValue))
		}
	}

	// 期限が設定されている場合のみ出力
	duedate := time.Time(issue.Fields.Duedate)
	if !duedate.IsZero() {
		sb.WriteString(fmt.Sprintf("| 期限 | %s |\n", duedate.Format(dateFormatDate)))
	}

	// 修正バージョンが設定されている場合のみ出力
	if len(issue.Fields.FixVersions) > 0 {
		versions := make([]string, len(issue.Fields.FixVersions))
		for i, v := range issue.Fields.FixVersions {
			versions[i] = v.Name
		}
		sb.WriteString(fmt.Sprintf("| 修正バージョン | %s |\n", strings.Join(versions, ", ")))
	}

	// 影響バージョンが設定されている場合のみ出力
	if len(issue.Fields.AffectsVersions) > 0 {
		versions := make([]string, len(issue.Fields.AffectsVersions))
		for i, v := range issue.Fields.AffectsVersions {
			versions[i] = v.Name
		}
		sb.WriteString(fmt.Sprintf("| 影響バージョン | %s |\n", strings.Join(versions, ", ")))
	}

	// 親課題が設定されている場合のみ出力
	if issue.Fields.Parent != nil && issue.Fields.Parent.Key != "" {
		sb.WriteString(fmt.Sprintf("| 親課題 | [%s](../%s/) |\n", issue.Fields.Parent.Key, strings.ToLower(issue.Fields.Parent.Key)))
	}

	// 時間管理情報（値がある場合のみ出力）
	if issue.Fields.TimeTracking != nil {
		tt := issue.Fields.TimeTracking

		if tt.OriginalEstimateSeconds > 0 {
			timeStr := mw.formatTimeSeconds(tt.OriginalEstimateSeconds)
			sb.WriteString(fmt.Sprintf("| 初期見積り | %s |\n", timeStr))
		}
		if tt.RemainingEstimateSeconds > 0 {
			timeStr := mw.formatTimeSeconds(tt.RemainingEstimateSeconds)
			sb.WriteString(fmt.Sprintf("| 残り時間 | %s |\n", timeStr))
		}
		if tt.TimeSpentSeconds > 0 {
			timeStr := mw.formatTimeSeconds(tt.TimeSpentSeconds)
			sb.WriteString(fmt.Sprintf("| 作業時間 | %s |\n", timeStr))
		}
	}

	// Σ時間情報（サブタスク含む集計値）
	if aggTime := extractAggregateTimeFields(issue); aggTime != nil {
		if aggTime.AggregateTimeOriginalEstimate > 0 {
			timeStr := mw.formatTimeSeconds(aggTime.AggregateTimeOriginalEstimate)
			sb.WriteString(fmt.Sprintf("| Σ初期見積り | %s |\n", timeStr))
		}
		if aggTime.AggregateTimeEstimate > 0 {
			timeStr := mw.formatTimeSeconds(aggTime.AggregateTimeEstimate)
			sb.WriteString(fmt.Sprintf("| Σ残り時間 | %s |\n", timeStr))
		}
		if aggTime.AggregateTimeSpent > 0 {
			timeStr := mw.formatTimeSeconds(aggTime.AggregateTimeSpent)
			sb.WriteString(fmt.Sprintf("| Σ作業時間 | %s |\n", timeStr))
		}
	}

	if issue.Fields.Resolution != nil {
		sb.WriteString(fmt.Sprintf("| 解決状況 | %s |\n", issue.Fields.Resolution.Name))
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

			sb.WriteString(fmt.Sprintf("| %s | %s |\n", fieldName, fieldValue))
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
				repoOrder, repoMap := groupBranchesByRepo(detail.Branches)
				for _, repoName := range repoOrder {
					sb.WriteString(fmt.Sprintf("- %s\n", repoName))
					for _, branch := range repoMap[repoName] {
						sb.WriteString(fmt.Sprintf("    - [`%s`](%s)\n", branch.Name, branch.URL))
						if branch.LastCommit != nil && branch.LastCommit.DisplayID != "" {
							sb.WriteString(fmt.Sprintf("        - [`%s`](%s)",
								branch.LastCommit.DisplayID, branch.LastCommit.URL))
							if formatted := mw.formatRFC3339(branch.LastCommit.Timestamp); formatted != "" {
								sb.WriteString(fmt.Sprintf(" %s", formatted))
							}
							sb.WriteString("\n")
						}
					}
				}
				sb.WriteString("\n")
			}

			// コミット（ブランチとプルリクエストの間に出力、JIRA仕様に合わせる）
			if len(detail.Commits) > 0 {
				sb.WriteString("### コミット\n\n")
				repoOrder, repoMap := groupCommitsByRepo(detail.Commits)
				for _, repoName := range repoOrder {
					sb.WriteString(fmt.Sprintf("- %s\n", repoName))
					for _, commit := range repoMap[repoName] {
						sb.WriteString(fmt.Sprintf("    - [`%s`](%s)", commit.DisplayID, commit.URL))
						if formatted := mw.formatRFC3339(commit.Timestamp); formatted != "" {
							sb.WriteString(fmt.Sprintf(" %s", formatted))
						}
						sb.WriteString("\n")
						if commit.Message != "" {
							firstLine := strings.SplitN(commit.Message, "\n", 2)[0]
							firstLine = strings.TrimSpace(firstLine)
							if firstLine != "" {
								sb.WriteString(fmt.Sprintf("        - %s\n", firstLine))
							}
						}
						if commit.Author != "" {
							sb.WriteString(fmt.Sprintf("        - 作成者: %s\n", commit.Author))
						}
					}
				}
				sb.WriteString("\n")
			}

			// プルリクエスト（最後に出力、JIRA仕様に合わせる）
			if len(detail.PullRequests) > 0 {
				sb.WriteString("### プルリクエスト\n\n")
				repoOrder, repoMap := groupPRsByRepo(detail.PullRequests)
				for _, repoName := range repoOrder {
					sb.WriteString(fmt.Sprintf("- %s\n", repoName))
					for _, pr := range repoMap[repoName] {
						sb.WriteString(fmt.Sprintf("    - [%s](%s) `%s`", pr.Name, pr.URL, pr.Status))
						if formatted := mw.formatRFC3339(pr.LastUpdate); formatted != "" {
							sb.WriteString(fmt.Sprintf(" %s", formatted))
						}
						sb.WriteString("\n")
						sb.WriteString(fmt.Sprintf("        - `%s` → `%s`\n", pr.Source.Branch, pr.Destination.Branch))
						if pr.Author.Name != "" {
							sb.WriteString(fmt.Sprintf("        - 作成者: %s\n", pr.Author.Name))
						}
					}
				}
				sb.WriteString("\n")
			}
		}
	}
}

// groupBranchesByRepo はブランチをリポジトリ名でグルーピングする
func groupBranchesByRepo(branches []DevBranch) ([]string, map[string][]DevBranch) {
	var order []string
	grouped := make(map[string][]DevBranch)
	for _, b := range branches {
		if _, exists := grouped[b.RepositoryName]; !exists {
			order = append(order, b.RepositoryName)
		}
		grouped[b.RepositoryName] = append(grouped[b.RepositoryName], b)
	}
	return order, grouped
}

// groupCommitsByRepo はコミットをリポジトリ名でグルーピングする
func groupCommitsByRepo(commits []DevRepoCommit) ([]string, map[string][]DevRepoCommit) {
	var order []string
	grouped := make(map[string][]DevRepoCommit)
	for _, c := range commits {
		if _, exists := grouped[c.RepositoryName]; !exists {
			order = append(order, c.RepositoryName)
		}
		grouped[c.RepositoryName] = append(grouped[c.RepositoryName], c)
	}
	return order, grouped
}

// groupPRsByRepo はプルリクエストをリポジトリ名でグルーピングする
func groupPRsByRepo(prs []DevPullRequest) ([]string, map[string][]DevPullRequest) {
	var order []string
	grouped := make(map[string][]DevPullRequest)
	for _, pr := range prs {
		if _, exists := grouped[pr.RepositoryName]; !exists {
			order = append(order, pr.RepositoryName)
		}
		grouped[pr.RepositoryName] = append(grouped[pr.RepositoryName], pr)
	}
	return order, grouped
}

// generateDescription は説明セクションを生成する
func (mw *MarkdownWriter) generateDescription(sb *strings.Builder, issue *cloud.Issue, attachmentMap map[string]string) {
	// 常に fields.description を使用する（renderedFields.description は <ul><li> 等のHTMLを含むため使用しない）
	description := issue.Fields.Description

	if description != "" {
		sb.WriteString("## 説明\n\n")

		// 添付ファイル参照を変換（convertJIRAMarkupToMarkdownの前に実行）
		description = mw.replaceAttachmentReferences(description, attachmentMap)

		// JIRA Wiki形式をMarkdownに変換
		description = mw.convertJIRAMarkupToMarkdown(description, issue.Fields.Project.Key)

		// 画像参照を変換（属性付き→属性なしの順）
		description = mw.replaceImageReferencesWithAttributes(description, attachmentMap)
		description = mw.replaceImageReferences(description, attachmentMap)
		description = ensureBlankLinesAroundImages(description)
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
			dateStr := mw.formatTimeString(comment.Created)

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
			commentBody = ensureBlankLinesAroundImages(commentBody)

			// コメント本文の出力
			sb.WriteString(commentBody)
			sb.WriteString("\n")

			// Shortcode終了タグ
			sb.WriteString("{{< /comment >}}\n\n")
		}
	}
}

// generateSubtasks はサブタスクセクションを生成する
func (mw *MarkdownWriter) generateSubtasks(sb *strings.Builder, issue *cloud.Issue) {
	if len(issue.Fields.Subtasks) > 0 {
		sb.WriteString("## サブタスク\n\n")
		for _, subtask := range issue.Fields.Subtasks {
			sb.WriteString(fmt.Sprintf("- **[%s](../%s/)**: %s", subtask.Key, strings.ToLower(subtask.Key), subtask.Fields.Summary))
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
			sb.WriteString(fmt.Sprintf("- %s **[%s](../%s/)**: %s", icon, child.Key, strings.ToLower(child.Key), child.Summary))
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

			// スペース名を取得（事前解決済みのキャッシュを優先、なければAPIで取得）
			spaceName := ""
			if link.GlobalID != "" {
				pageID, err := ExtractPageIDFromGlobalID(link.GlobalID)
				if err == nil {
					if mw.confluenceSpaces != nil {
						spaceName = mw.confluenceSpaces[pageID]
					} else if mw.confluenceClient != nil {
						spaceName, _ = mw.confluenceClient.GetSpaceName(pageID)
					}
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
				sb.WriteString(fmt.Sprintf("- **%s**: [%s](../%s/)", link.Type.Outward, link.OutwardIssue.Key, strings.ToLower(link.OutwardIssue.Key)))
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
				sb.WriteString(fmt.Sprintf("- **%s**: [%s](../%s/)", link.Type.Inward, link.InwardIssue.Key, strings.ToLower(link.InwardIssue.Key)))
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

// getAttachmentIcon はファイル名の拡張子からファイルタイプ別の絵文字アイコンを返す
func getAttachmentIcon(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".svg", ".webp":
		return "🖼️"
	case ".xlsx", ".xls", ".csv", ".ppt", ".pptx":
		return "📊"
	case ".pdf":
		return "📄"
	case ".doc", ".docx", ".txt", ".md", ".log":
		return "📝"
	case ".zip", ".tar", ".gz", ".rar", ".7z":
		return "📦"
	case ".java", ".py", ".js", ".ts", ".go", ".html", ".css", ".xml", ".json", ".yaml", ".yml", ".sql":
		return "💻"
	default:
		return "📎"
	}
}

// generateAttachments は添付ファイルセクションを生成する
func (mw *MarkdownWriter) generateAttachments(sb *strings.Builder, attachmentFiles []string) {
	if len(attachmentFiles) > 0 {
		sb.WriteString("## 添付ファイル\n\n")
		for _, filename := range attachmentFiles {
			// ファイル名をURLエンコーディング（スペース→%20）
			encodedFilename := url.PathEscape(filename)
			icon := getAttachmentIcon(filename)
			// 同じディレクトリ内の相対パスで添付ファイルを参照
			sb.WriteString(fmt.Sprintf("- %s [%s](%s)\n", icon, filename, encodedFilename))
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
