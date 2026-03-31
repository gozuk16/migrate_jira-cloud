package main

import (
	"encoding/json"
	"os"

	cloud "github.com/andygrunwald/go-jira/v2/cloud"
)

// UserMapping はアカウントID→表示名のマッピング
type UserMapping map[string]string

// BuildUserMappingFromIssue は単一の課題からユーザーマッピングを抽出してmappingに追加する
func BuildUserMappingFromIssue(issue *cloud.Issue, mapping UserMapping) {
	if issue == nil || issue.Fields == nil {
		return
	}

	// Reporter
	if issue.Fields.Reporter != nil && issue.Fields.Reporter.AccountID != "" {
		mapping[issue.Fields.Reporter.AccountID] = issue.Fields.Reporter.DisplayName
	}

	// Assignee
	if issue.Fields.Assignee != nil && issue.Fields.Assignee.AccountID != "" {
		mapping[issue.Fields.Assignee.AccountID] = issue.Fields.Assignee.DisplayName
	}

	// Comments
	if issue.Fields.Comments != nil {
		for _, comment := range issue.Fields.Comments.Comments {
			if comment.Author != nil && comment.Author.AccountID != "" {
				mapping[comment.Author.AccountID] = comment.Author.DisplayName
			}
		}
	}

	// Changelog
	if issue.Changelog != nil {
		for _, history := range issue.Changelog.Histories {
			if history.Author.AccountID != "" {
				mapping[history.Author.AccountID] = history.Author.DisplayName
			}
		}
	}
}

// BuildUserMapping は複数の課題からユーザーマッピングを構築する
func BuildUserMapping(issues []*cloud.Issue) UserMapping {
	mapping := make(UserMapping)
	for _, issue := range issues {
		BuildUserMappingFromIssue(issue, mapping)
	}
	return mapping
}

// LoadUserMapping はキャッシュファイルからUserMappingを読み込む
// ファイルが存在しない場合は空のUserMappingを返す
func LoadUserMapping(path string) (UserMapping, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(UserMapping), nil
		}
		return nil, err
	}

	var mapping UserMapping
	if err := json.Unmarshal(data, &mapping); err != nil {
		return nil, err
	}

	return mapping, nil
}

// SaveUserMapping はUserMappingをJSONファイルに保存する
func SaveUserMapping(path string, mapping UserMapping) error {
	data, err := json.MarshalIndent(mapping, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// MergeUserMapping は src の内容を dst にマージする（srcの値でdstを上書き）
func MergeUserMapping(dst, src UserMapping) {
	for k, v := range src {
		dst[k] = v
	}
}
