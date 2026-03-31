package main

import (
	"encoding/json"

	cloud "github.com/andygrunwald/go-jira/v2/cloud"
)

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

// IssueData はJSONファイルに保存する課題データの構造
type IssueData struct {
	Issue            *cloud.Issue       `json:"issue"`
	DevStatus        *DevStatusDetail   `json:"devStatus,omitempty"`
	DevStatusRawJSON json.RawMessage    `json:"devStatusRawJSON,omitempty"`
	ParentInfo       *ParentIssueInfo   `json:"parentInfo,omitempty"`
	ChildIssues      []ChildIssueInfo   `json:"childIssues,omitempty"`
	RemoteLinks      []cloud.RemoteLink `json:"remoteLinks,omitempty"`
	Fields           []cloud.Field      `json:"fields,omitempty"`
	ConfluenceSpaces map[string]string  `json:"confluenceSpaces,omitempty"` // pageID -> spaceName
	SavedAt          string             `json:"savedAt"`
}
