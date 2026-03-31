package main

import (
	"fmt"
	"log/slog"
	"sort"
)

// fetchDevStatus は開発情報の詳細を取得する（設定で有効な場合のみ）
// エラー発生時はnilを返してスキップする
func fetchDevStatus(jiraClient *JIRAClient, config *Config, issueKey, issueID string) (*DevStatusDetail, []byte) {
	if !config.Development.Enabled || issueID == "" {
		return nil, nil
	}

	apiType := config.Development.APIType
	if apiType == "" {
		apiType = "rest"
	}

	if apiType == "graphql" {
		devStatus, rawJSON, err := jiraClient.GetDevStatusGraphQL(issueID)
		if err != nil {
			slog.Debug("GraphQL API 開発情報取得失敗",
				"issueKey", issueKey,
				"issueID", issueID,
				"error", err)
			slog.Warn("開発情報の詳細取得に失敗（スキップして継続）",
				"issueKey", issueKey,
				"error", err)
			return nil, nil
		}
		return devStatus, rawJSON
	}

	// REST APIを使用
	appType := config.Development.ApplicationType
	if appType == "" {
		appType = "bitbucket"
	}

	devStatus, rawJSON, err := jiraClient.GetDevStatusDetails(issueID, appType, "pullrequest")
	if err != nil {
		slog.Debug("REST API 開発情報取得失敗",
			"issueKey", issueKey,
			"issueID", issueID,
			"appType", appType,
			"error", err)
		slog.Warn("開発情報の詳細取得に失敗（スキップして継続）",
			"issueKey", issueKey,
			"error", err)
		return nil, nil
	}
	return devStatus, rawJSON
}

// fetchAndSortChildIssues は子課題を取得してRankでソートして返す
// Sub-task系の課題タイプは除外する
func fetchAndSortChildIssues(jiraClient *JIRAClient, config *Config, issueKey string) ([]ChildIssueInfo, error) {
	childKeys, err := jiraClient.GetChildIssues(issueKey, 100)
	if err != nil {
		return nil, fmt.Errorf("子課題の取得に失敗しました（課題: %s）: %w", issueKey, err)
	}

	if len(childKeys) == 0 {
		return nil, nil
	}

	childIssues := make([]ChildIssueInfo, 0, len(childKeys))
	for _, childKey := range childKeys {
		childIssue, err := jiraClient.GetIssue(childKey)
		if err != nil {
			fmt.Printf("  警告: 子課題 %s の取得に失敗しました: %v\n", childKey, err)
			continue
		}
		// Sub-task課題タイプは除外
		issueType := childIssue.Fields.Type.Name
		if issueType == "Sub-task" || issueType == "Subtask" || issueType == "サブタスク" {
			continue
		}

		// Rankフィールドを取得
		rankValue := ""
		if rank, exists := childIssue.Fields.Unknowns[config.Display.RankFieldId]; exists {
			if rankStr, ok := rank.(string); ok {
				rankValue = rankStr
			}
		}
		childIssues = append(childIssues, ChildIssueInfo{
			Key:     childIssue.Key,
			Summary: childIssue.Fields.Summary,
			Status:  childIssue.Fields.Status.Name,
			Type:    childIssue.Fields.Type.Name,
			Rank:    rankValue,
		})
	}

	// Rankフィールドでソート
	if len(childIssues) > 0 {
		sort.Slice(childIssues, func(i, j int) bool {
			if childIssues[i].Rank == "" && childIssues[j].Rank != "" {
				return false
			}
			if childIssues[i].Rank != "" && childIssues[j].Rank == "" {
				return true
			}
			return childIssues[i].Rank < childIssues[j].Rank
		})
	}

	return childIssues, nil
}
