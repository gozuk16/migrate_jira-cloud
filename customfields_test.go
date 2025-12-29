package main

import (
	"reflect"
	"testing"

	"github.com/andygrunwald/go-jira/v2/cloud"
)

// TestGetSortedCustomFieldKeys はカスタムフィールドのキーをソートするテスト
func TestGetSortedCustomFieldKeys(t *testing.T) {
	tests := []struct {
		name         string
		customFields map[string]interface{}
		expected     []string
	}{
		{
			name: "複数のカスタムフィールドをソート",
			customFields: map[string]interface{}{
				"customfield_10003": "value3",
				"customfield_10001": "value1",
				"customfield_10002": "value2",
			},
			expected: []string{"customfield_10001", "customfield_10002", "customfield_10003"},
		},
		{
			name: "単一のカスタムフィールド",
			customFields: map[string]interface{}{
				"customfield_10001": "value1",
			},
			expected: []string{"customfield_10001"},
		},
		{
			name:         "空のマップ",
			customFields: map[string]interface{}{},
			expected:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetSortedCustomFieldKeys(tt.customFields)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("GetSortedCustomFieldKeys() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestBuildFieldNameCache はフィールド名キャッシュの構築をテスト
func TestBuildFieldNameCache(t *testing.T) {
	tests := []struct {
		name     string
		fields   []cloud.Field
		expected FieldNameCache
	}{
		{
			name: "複数のフィールド",
			fields: []cloud.Field{
				{ID: "customfield_10001", Name: "スプリント"},
				{ID: "customfield_10002", Name: "ストーリーポイント"},
				{ID: "customfield_10003", Name: "エピックリンク"},
			},
			expected: FieldNameCache{
				"customfield_10001": "スプリント",
				"customfield_10002": "ストーリーポイント",
				"customfield_10003": "エピックリンク",
			},
		},
		{
			name:     "空のフィールドリスト",
			fields:   []cloud.Field{},
			expected: FieldNameCache{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildFieldNameCache(tt.fields)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("BuildFieldNameCache() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestGetFieldName はキャッシュからフィールド名を取得するテスト
func TestGetFieldName(t *testing.T) {
	cache := FieldNameCache{
		"customfield_10001": "スプリント",
		"customfield_10002": "ストーリーポイント",
	}

	tests := []struct {
		name     string
		fieldID  string
		expected string
	}{
		{
			name:     "存在するフィールドID",
			fieldID:  "customfield_10001",
			expected: "スプリント",
		},
		{
			name:     "存在しないフィールドID（フォールバック）",
			fieldID:  "customfield_10099",
			expected: "カスタムフィールド 10099",
		},
		{
			name:     "空のフィールドID",
			fieldID:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cache.GetFieldName(tt.fieldID)
			if result != tt.expected {
				t.Errorf("GetFieldName(%q) = %q, expected %q", tt.fieldID, result, tt.expected)
			}
		})
	}
}

// TestFormatCustomFieldName はカスタムフィールドIDを読みやすい名前に変換するテスト
func TestFormatCustomFieldName(t *testing.T) {
	tests := []struct {
		name     string
		fieldID  string
		expected string
	}{
		{
			name:     "customfield_プレフィックス付き",
			fieldID:  "customfield_10001",
			expected: "カスタムフィールド 10001",
		},
		{
			name:     "customfield_プレフィックス無し",
			fieldID:  "other_field",
			expected: "other_field",
		},
		{
			name:     "空の文字列",
			fieldID:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCustomFieldName(tt.fieldID)
			if result != tt.expected {
				t.Errorf("FormatCustomFieldName(%q) = %q, expected %q", tt.fieldID, result, tt.expected)
			}
		})
	}
}

// TestIsCustomFieldEmpty はカスタムフィールドが空かどうかを判定するテスト
func TestIsCustomFieldEmpty(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		{
			name:     "nilの場合",
			value:    nil,
			expected: true,
		},
		{
			name:     "空文字列の場合",
			value:    "",
			expected: true,
		},
		{
			name:     "空の配列の場合",
			value:    []interface{}{},
			expected: true,
		},
		{
			name:     "空のマップの場合",
			value:    map[string]interface{}{},
			expected: true,
		},
		{
			name:     "文字列が設定されている場合",
			value:    "値",
			expected: false,
		},
		{
			name:     "配列に要素がある場合",
			value:    []interface{}{"値"},
			expected: false,
		},
		{
			name:     "マップに要素がある場合",
			value:    map[string]interface{}{"key": "value"},
			expected: false,
		},
		{
			name:     "数値の場合",
			value:    123,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCustomFieldEmpty(tt.value)
			if result != tt.expected {
				t.Errorf("IsCustomFieldEmpty(%v) = %v, expected %v", tt.value, result, tt.expected)
			}
		})
	}
}

// TestFormatCustomFieldValue はカスタムフィールドの値を文字列に変換するテスト
func TestFormatCustomFieldValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{
			name:     "nilの場合",
			value:    nil,
			expected: "未設定",
		},
		{
			name:     "空文字列の場合",
			value:    "",
			expected: "未設定",
		},
		{
			name:     "文字列の場合",
			value:    "テスト値",
			expected: "テスト値",
		},
		{
			name:     "float64の場合",
			value:    3.14,
			expected: "3.14",
		},
		{
			name:     "intの場合",
			value:    42,
			expected: "42",
		},
		{
			name:     "boolの場合（true）",
			value:    true,
			expected: "はい",
		},
		{
			name:     "boolの場合（false）",
			value:    false,
			expected: "いいえ",
		},
		{
			name:     "空の配列の場合",
			value:    []interface{}{},
			expected: "未設定",
		},
		{
			name: "nameフィールド付きオブジェクトの配列",
			value: []interface{}{
				map[string]interface{}{"name": "値1"},
				map[string]interface{}{"name": "値2"},
			},
			expected: "値1, 値2",
		},
		{
			name: "valueフィールド付きオブジェクトの配列",
			value: []interface{}{
				map[string]interface{}{"value": "値A"},
				map[string]interface{}{"value": "値B"},
			},
			expected: "値A, 値B",
		},
		{
			name:     "文字列の配列",
			value:    []interface{}{"値1", "値2", "値3"},
			expected: "値1, 値2, 値3",
		},
		{
			name:     "nameフィールド付きオブジェクト",
			value:    map[string]interface{}{"name": "オブジェクト名"},
			expected: "オブジェクト名",
		},
		{
			name:     "valueフィールド付きオブジェクト",
			value:    map[string]interface{}{"value": "オブジェクト値"},
			expected: "オブジェクト値",
		},
		{
			name:     "displayNameフィールド付きオブジェクト",
			value:    map[string]interface{}{"displayName": "表示名"},
			expected: "表示名",
		},
		{
			name: "開発フィールド（pullrequest）",
			value: map[string]interface{}{
				"pullrequest": map[string]interface{}{
					"state":      "OPEN",
					"stateCount": float64(1),
				},
			},
			expected: "Pull Request: 1 open",
		},
		{
			name: "開発フィールド（情報なし）",
			value: map[string]interface{}{
				"pullrequest": map[string]interface{}{},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCustomFieldValue(tt.value)
			if result != tt.expected {
				t.Errorf("FormatCustomFieldValue(%v) = %q, expected %q", tt.value, result, tt.expected)
			}
		})
	}
}

// TestBuildUserMappingFromIssue は課題からユーザーマッピングを構築するテスト
func TestBuildUserMappingFromIssue(t *testing.T) {
	tests := []struct {
		name     string
		issue    *cloud.Issue
		expected UserMapping
	}{
		{
			name: "Reporter、Assignee、Commentが設定されている場合",
			issue: &cloud.Issue{
				Fields: &cloud.IssueFields{
					Reporter: &cloud.User{
						AccountID:   "reporter123",
						DisplayName: "報告者",
					},
					Assignee: &cloud.User{
						AccountID:   "assignee456",
						DisplayName: "担当者",
					},
					Comments: &cloud.Comments{
						Comments: []*cloud.Comment{
							{
								Author: &cloud.User{
									AccountID:   "commenter789",
									DisplayName: "コメント者",
								},
							},
						},
					},
				},
			},
			expected: UserMapping{
				"reporter123":   "報告者",
				"assignee456":   "担当者",
				"commenter789":  "コメント者",
			},
		},
		{
			name:     "issueがnilの場合",
			issue:    nil,
			expected: UserMapping{},
		},
		{
			name: "Fieldsがnilの場合",
			issue: &cloud.Issue{
				Fields: nil,
			},
			expected: UserMapping{},
		},
		{
			name: "ReporterとAssigneeがnilの場合",
			issue: &cloud.Issue{
				Fields: &cloud.IssueFields{
					Reporter: nil,
					Assignee: nil,
				},
			},
			expected: UserMapping{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make(UserMapping)
			BuildUserMappingFromIssue(tt.issue, result)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("BuildUserMappingFromIssue() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestBuildUserMapping は複数の課題からユーザーマッピングを構築するテスト
func TestBuildUserMapping(t *testing.T) {
	tests := []struct {
		name     string
		issues   []*cloud.Issue
		expected UserMapping
	}{
		{
			name: "複数の課題",
			issues: []*cloud.Issue{
				{
					Fields: &cloud.IssueFields{
						Reporter: &cloud.User{
							AccountID:   "user1",
							DisplayName: "ユーザー1",
						},
					},
				},
				{
					Fields: &cloud.IssueFields{
						Assignee: &cloud.User{
							AccountID:   "user2",
							DisplayName: "ユーザー2",
						},
					},
				},
			},
			expected: UserMapping{
				"user1": "ユーザー1",
				"user2": "ユーザー2",
			},
		},
		{
			name:     "空の配列",
			issues:   []*cloud.Issue{},
			expected: UserMapping{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildUserMapping(tt.issues)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("BuildUserMapping() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestIsDevelopmentField は開発フィールド検出のテスト
func TestIsDevelopmentField(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected bool
	}{
		{
			name: "Bitbucket pullrequestフィールド",
			input: map[string]interface{}{
				"pullrequest": map[string]interface{}{
					"state": "OPEN",
				},
			},
			expected: true,
		},
		{
			name: "jsonキーを持つフィールド",
			input: map[string]interface{}{
				"json": map[string]interface{}{},
			},
			expected: true,
		},
		{
			name: "両方のキーを持つフィールド",
			input: map[string]interface{}{
				"pullrequest": map[string]interface{}{},
				"json":        map[string]interface{}{},
			},
			expected: true,
		},
		{
			name: "通常のフィールド",
			input: map[string]interface{}{
				"name": "test",
			},
			expected: false,
		},
		{
			name:     "空のマップ",
			input:    map[string]interface{}{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDevelopmentField(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestFormatDevelopmentField は開発フィールド整形のテスト
func TestFormatDevelopmentField(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected string
	}{
		{
			name: "pullrequestキーから情報抽出",
			input: map[string]interface{}{
				"pullrequest": map[string]interface{}{
					"dataType":   "pullrequest",
					"state":      "OPEN",
					"stateCount": float64(1),
				},
			},
			expected: "Pull Request: 1 open",
		},
		{
			name: "jsonキーから詳細情報抽出",
			input: map[string]interface{}{
				"json": map[string]interface{}{
					"cachedValue": map[string]interface{}{
						"summary": map[string]interface{}{
							"pullrequest": map[string]interface{}{
								"overall": map[string]interface{}{
									"count": float64(2),
									"state": "MERGED",
								},
							},
						},
					},
				},
			},
			expected: "Pull Request: 2 merged",
		},
		{
			name: "SCRUM-2の実際のデータ",
			input: map[string]interface{}{
				"pullrequest": map[string]interface{}{
					"dataType":   "pullrequest",
					"state":      "OPEN",
					"stateCount": float64(1),
				},
				"json": map[string]interface{}{
					"cachedValue": map[string]interface{}{
						"errors": []interface{}{},
						"summary": map[string]interface{}{
							"pullrequest": map[string]interface{}{
								"overall": map[string]interface{}{
									"count":       float64(1),
									"state":       "OPEN",
									"dataType":    "pullrequest",
									"open":        true,
									"lastUpdated": "2025-12-28T13:10:08.957+0900",
								},
							},
						},
					},
					"isStale": true,
				},
			},
			expected: "Pull Request: 1 open",
		},
		{
			name: "情報が抽出できない場合",
			input: map[string]interface{}{
				"pullrequest": map[string]interface{}{},
			},
			expected: "",
		},
		{
			name:     "空のマップ",
			input:    map[string]interface{}{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDevelopmentField(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
