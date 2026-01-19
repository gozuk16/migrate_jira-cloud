package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	cloud "github.com/andygrunwald/go-jira/v2/cloud"
)

// IssueData はJSONファイルに保存する課題データの構造
type IssueData struct {
	Issue       *cloud.Issue     `json:"issue"`
	DevStatus   *DevStatusDetail `json:"devStatus,omitempty"`
	ParentInfo  *ParentIssueInfo `json:"parentInfo,omitempty"`
	ChildIssues []ChildIssueInfo `json:"childIssues,omitempty"`
	Fields      []cloud.Field    `json:"fields,omitempty"`
	SavedAt     string           `json:"savedAt"`
}

// JSONSaver はJSON保存を管理する構造体
type JSONSaver struct {
	outputDir string
}

// NewJSONSaver は新しいJSONSaverを作成
func NewJSONSaver(outputDir string) *JSONSaver {
	return &JSONSaver{outputDir: outputDir}
}

// SaveIssue は課題データをJSONファイルとして保存
func (js *JSONSaver) SaveIssue(data *IssueData) (string, error) {
	// ディレクトリ作成
	projectDir := filepath.Join(js.outputDir, data.Issue.Fields.Project.Key)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return "", fmt.Errorf("JSONディレクトリ作成エラー: %w", err)
	}

	// JSON生成
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("JSONマーシャリングエラー: %w", err)
	}

	// ファイル保存
	outputPath := filepath.Join(projectDir, fmt.Sprintf("%s.json", data.Issue.Key))
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return "", fmt.Errorf("JSONファイル書き込みエラー: %w", err)
	}

	return outputPath, nil
}

// LoadIssue はJSONファイルから課題データを読み込み
func (js *JSONSaver) LoadIssue(jsonPath string) (*IssueData, error) {
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("JSONファイル読み込みエラー: %w", err)
	}

	var data IssueData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("JSONパースエラー: %w", err)
	}

	return &data, nil
}
