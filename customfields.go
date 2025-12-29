package main

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/andygrunwald/go-jira/v2/cloud"
)

// GetAllCustomFields は課題から全てのカスタムフィールドを取得する
func GetAllCustomFields(issue *cloud.Issue) map[string]interface{} {
	customFields := make(map[string]interface{})

	if issue == nil || issue.Fields == nil || issue.Fields.Unknowns == nil {
		return customFields
	}

	// Unknownsマップから全てのカスタムフィールドを抽出
	for key, value := range issue.Fields.Unknowns {
		// customfield_ で始まるフィールドのみを対象とする
		if strings.HasPrefix(key, "customfield_") {
			customFields[key] = value
		}
	}

	return customFields
}

// GetSortedCustomFieldKeys はカスタムフィールドのキーをソート済みで返す
func GetSortedCustomFieldKeys(customFields map[string]interface{}) []string {
	keys := make([]string, 0, len(customFields))
	for key := range customFields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// FieldNameCache はフィールドID→名称のマッピング
type FieldNameCache map[string]string

// BuildFieldNameCache はフィールドリストからフィールド名キャッシュを構築する
func BuildFieldNameCache(fields []cloud.Field) FieldNameCache {
	cache := make(FieldNameCache)
	for _, field := range fields {
		cache[field.ID] = field.Name
	}
	return cache
}

// GetFieldName はキャッシュからフィールド名を取得する
// 見つからない場合はフォールバック名を返す
func (cache FieldNameCache) GetFieldName(fieldID string) string {
	if name, exists := cache[fieldID]; exists && name != "" {
		return name
	}
	// フォールバック: 従来の形式
	return FormatCustomFieldName(fieldID)
}

// FormatCustomFieldName はカスタムフィールドIDを読みやすい名前に変換する（フォールバック用）
func FormatCustomFieldName(fieldID string) string {
	// "customfield_10001" -> "カスタムフィールド 10001"
	if strings.HasPrefix(fieldID, "customfield_") {
		id := strings.TrimPrefix(fieldID, "customfield_")
		return fmt.Sprintf("カスタムフィールド %s", id)
	}
	return fieldID
}

// IsCustomFieldEmpty はカスタムフィールドの値が空かどうかを判定する
func IsCustomFieldEmpty(value interface{}) bool {
	if value == nil {
		return true
	}

	switch v := value.(type) {
	case string:
		return v == ""

	case []interface{}:
		return len(v) == 0

	case map[string]interface{}:
		// 開発フィールドの場合、整形結果が空かどうかで判定
		if isDevelopmentField(v) {
			formatted := formatDevelopmentField(v)
			return formatted == ""
		}
		return len(v) == 0

	default:
		return false
	}
}

// isDevelopmentField は開発統合フィールド（Bitbucket、GitHub等）かどうかを判定
func isDevelopmentField(v map[string]interface{}) bool {
	// "pullrequest"または"json"キーがあれば開発フィールドと判定
	_, hasPullRequest := v["pullrequest"]
	_, hasJSON := v["json"]
	return hasPullRequest || hasJSON
}

// formatDevelopmentField は開発フィールドから有用な情報を抽出して整形
func formatDevelopmentField(v map[string]interface{}) string {
	// pullrequestキーから情報を抽出
	if pr, ok := v["pullrequest"].(map[string]interface{}); ok {
		state := ""
		count := 0

		if s, ok := pr["state"].(string); ok {
			state = s
		}
		if c, ok := pr["stateCount"].(float64); ok {
			count = int(c)
		}

		if state != "" && count > 0 {
			return fmt.Sprintf("Pull Request: %d %s", count, strings.ToLower(state))
		}
	}

	// jsonキーから詳細情報を抽出（フォールバック）
	if jsonData, ok := v["json"].(map[string]interface{}); ok {
		if cached, ok := jsonData["cachedValue"].(map[string]interface{}); ok {
			if summary, ok := cached["summary"].(map[string]interface{}); ok {
				if prSummary, ok := summary["pullrequest"].(map[string]interface{}); ok {
					if overall, ok := prSummary["overall"].(map[string]interface{}); ok {
						count := 0
						state := ""

						if c, ok := overall["count"].(float64); ok {
							count = int(c)
						}
						if s, ok := overall["state"].(string); ok {
							state = s
						}

						if count > 0 {
							return fmt.Sprintf("Pull Request: %d %s", count, strings.ToLower(state))
						}
					}
				}
			}
		}
	}

	// 情報が抽出できない場合は空文字列を返す（フィールドを非表示にする）
	return ""
}

// FormatCustomFieldValue はカスタムフィールドの値を文字列に変換する
func FormatCustomFieldValue(value interface{}) string {
	if value == nil {
		return "未設定"
	}

	switch v := value.(type) {
	case string:
		if v == "" {
			return "未設定"
		}
		// 開発フィールド（Bitbucket、GitHub等）の文字列表現を検出
		// JIRAから既に文字列化されている場合の対処
		if strings.Contains(v, "pullrequest=") || strings.Contains(v, "\"pullrequest\"") {
			// 開発統合フィールドは「開発情報」セクションで詳細表示するため、ここでは非表示
			return ""
		}
		return v

	case float64:
		return fmt.Sprintf("%.2f", v)

	case int, int64:
		return fmt.Sprintf("%d", v)

	case bool:
		if v {
			return "はい"
		}
		return "いいえ"

	case []interface{}:
		// 配列の場合は各要素を抽出
		if len(v) == 0 {
			return "未設定"
		}
		parts := make([]string, 0, len(v))
		for _, item := range v {
			// オブジェクトの場合は "value" や "name" フィールドを探す
			if obj, ok := item.(map[string]interface{}); ok {
				if name, exists := obj["name"]; exists {
					parts = append(parts, fmt.Sprintf("%v", name))
				} else if value, exists := obj["value"]; exists {
					parts = append(parts, fmt.Sprintf("%v", value))
				} else {
					parts = append(parts, fmt.Sprintf("%v", obj))
				}
			} else {
				parts = append(parts, fmt.Sprintf("%v", item))
			}
		}
		return strings.Join(parts, ", ")

	case map[string]interface{}:
		// 開発統合フィールド（Bitbucket、GitHub等）の特別処理
		if isDevelopmentField(v) {
			return formatDevelopmentField(v)
		}

		// オブジェクトの場合は "value" や "name" フィールドを探す
		if name, exists := v["name"]; exists {
			return fmt.Sprintf("%v", name)
		}
		if value, exists := v["value"]; exists {
			return fmt.Sprintf("%v", value)
		}
		if displayName, exists := v["displayName"]; exists {
			return fmt.Sprintf("%v", displayName)
		}
		// その他のオブジェクトは全体を表示
		return fmt.Sprintf("%v", v)

	default:
		// リフレクションを使って型を確認
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Map {
			// マップ型に変換を試みる
			m := make(map[string]interface{})
			for _, key := range rv.MapKeys() {
				m[key.String()] = rv.MapIndex(key).Interface()
			}

			// 開発統合フィールドの特別処理
			if isDevelopmentField(m) {
				return formatDevelopmentField(m)
			}

			// その他のマップ処理
			if name, exists := m["name"]; exists {
				return fmt.Sprintf("%v", name)
			}
			if value, exists := m["value"]; exists {
				return fmt.Sprintf("%v", value)
			}
			if displayName, exists := m["displayName"]; exists {
				return fmt.Sprintf("%v", displayName)
			}
		}

		return fmt.Sprintf("%v", v)
	}
}

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

// FormatDevelopmentFieldWithDetails は開発フィールドを詳細情報付きで整形する
func FormatDevelopmentFieldWithDetails(fieldValue map[string]interface{}, devStatus *DevStatusDetail) string {
	// サマリー情報を取得（既存ロジック）
	summary := extractDevelopmentSummary(fieldValue)

	// Dev-Status詳細情報がある場合
	if devStatus != nil && len(devStatus.Detail) > 0 {
		var parts []string

		// サマリー
		if summary.Count > 0 {
			parts = append(parts, fmt.Sprintf("Pull Request: %d %s", summary.Count, strings.ToLower(summary.State)))
		}

		// 最初のPRの詳細を追加
		for _, detail := range devStatus.Detail {
			if len(detail.PullRequests) > 0 {
				pr := detail.PullRequests[0]
				if pr.Name != "" {
					parts = append(parts, fmt.Sprintf("(%s)", pr.Name))
				}
				if pr.Source.Branch != "" {
					parts = append(parts, fmt.Sprintf("[%s]", pr.Source.Branch))
				}
				break
			}
		}

		if len(parts) > 0 {
			return strings.Join(parts, " ")
		}
	}

	// Dev-Status詳細がない場合は既存ロジック
	return formatDevelopmentField(fieldValue)
}

// extractDevelopmentSummary はサマリー情報を抽出（ヘルパー関数）
func extractDevelopmentSummary(v map[string]interface{}) struct{ Count int; State string } {
	result := struct{ Count int; State string }{}

	if pr, ok := v["pullrequest"].(map[string]interface{}); ok {
		if state, ok := pr["state"].(string); ok {
			result.State = state
		}
		if count, ok := pr["stateCount"].(float64); ok {
			result.Count = int(count)
		}
	}

	// jsonキーからのフォールバック（既存ロジック）
	if result.Count == 0 {
		if jsonData, ok := v["json"].(map[string]interface{}); ok {
			if cached, ok := jsonData["cachedValue"].(map[string]interface{}); ok {
				if summary, ok := cached["summary"].(map[string]interface{}); ok {
					if prSummary, ok := summary["pullrequest"].(map[string]interface{}); ok {
						if overall, ok := prSummary["overall"].(map[string]interface{}); ok {
							if count, ok := overall["count"].(float64); ok {
								result.Count = int(count)
							}
							if state, ok := overall["state"].(string); ok {
								result.State = state
							}
						}
					}
				}
			}
		}
	}

	return result
}
