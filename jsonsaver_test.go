package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cloud "github.com/andygrunwald/go-jira/v2/cloud"
)

func TestJSONSaver_SaveAndLoad(t *testing.T) {
	// 一時ディレクトリを作成
	tempDir, err := os.MkdirTemp("", "jsonsaver_test")
	if err != nil {
		t.Fatalf("一時ディレクトリの作成に失敗: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name      string
		issueData *IssueData
		wantErr   bool
	}{
		{
			name: "正常系: 基本的な課題データの保存と読み込み",
			issueData: &IssueData{
				Issue: &cloud.Issue{
					ID:  "10001",
					Key: "TEST-1",
					Fields: &cloud.IssueFields{
						Summary: "テスト課題",
						Project: cloud.Project{
							Key:  "TEST",
							Name: "テストプロジェクト",
						},
						Type: cloud.IssueType{
							Name: "タスク",
						},
						Status: &cloud.Status{
							Name: "進行中",
						},
						Created: cloud.Time(time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)),
						Updated: cloud.Time(time.Date(2025, 1, 15, 14, 30, 0, 0, time.UTC)),
					},
				},
				ParentInfo: &ParentIssueInfo{
					Key:  "TEST-0",
					Type: "エピック",
				},
				ChildIssues: []ChildIssueInfo{
					{
						Key:     "TEST-2",
						Summary: "子課題1",
						Status:  "完了",
						Type:    "サブタスク",
					},
				},
				SavedAt: time.Now().Format(time.RFC3339),
			},
			wantErr: false,
		},
		{
			name: "正常系: 最小限のデータ",
			issueData: &IssueData{
				Issue: &cloud.Issue{
					ID:  "10002",
					Key: "MIN-1",
					Fields: &cloud.IssueFields{
						Summary: "最小限の課題",
						Project: cloud.Project{
							Key: "MIN",
						},
					},
				},
				SavedAt: time.Now().Format(time.RFC3339),
			},
			wantErr: false,
		},
		{
			name: "正常系: 開発情報を含むデータ",
			issueData: &IssueData{
				Issue: &cloud.Issue{
					ID:  "10003",
					Key: "DEV-1",
					Fields: &cloud.IssueFields{
						Summary: "開発情報付き課題",
						Project: cloud.Project{
							Key: "DEV",
						},
					},
				},
				DevStatus: &DevStatusDetail{
					Detail: []DevStatusDetailItem{
						{
							Branches: []DevBranch{
								{
									Name: "feature/test",
									URL:  "https://github.com/test/repo/tree/feature/test",
								},
							},
							PullRequests: []DevPullRequest{
								{
									Name:   "Test PR",
									Status: "MERGED",
									URL:    "https://github.com/test/repo/pull/1",
								},
							},
						},
					},
				},
				DevStatusRawJSON: json.RawMessage(`{"detail":[{"branches":[{"name":"feature/test"}],"pullRequests":[{"name":"Test PR"}]}]}`),
				SavedAt:          time.Now().Format(time.RFC3339),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saver := NewJSONSaver(tempDir)

			// 保存
			savedPath, err := saver.SaveIssue(tt.issueData)
			if (err != nil) != tt.wantErr {
				t.Errorf("SaveIssue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// ファイルが作成されたことを確認
			expectedPath := filepath.Join(tempDir, tt.issueData.Issue.Fields.Project.Key, tt.issueData.Issue.Key+".json")
			if savedPath != expectedPath {
				t.Errorf("SaveIssue() path = %v, want %v", savedPath, expectedPath)
			}

			if _, err := os.Stat(savedPath); os.IsNotExist(err) {
				t.Errorf("JSONファイルが作成されていない: %s", savedPath)
				return
			}

			// 読み込み
			loaded, err := saver.LoadIssue(savedPath)
			if err != nil {
				t.Errorf("LoadIssue() error = %v", err)
				return
			}

			// データの検証
			if loaded.Issue.Key != tt.issueData.Issue.Key {
				t.Errorf("LoadIssue() Issue.Key = %v, want %v", loaded.Issue.Key, tt.issueData.Issue.Key)
			}
			if loaded.Issue.Fields.Summary != tt.issueData.Issue.Fields.Summary {
				t.Errorf("LoadIssue() Issue.Fields.Summary = %v, want %v", loaded.Issue.Fields.Summary, tt.issueData.Issue.Fields.Summary)
			}

			// ParentInfo の検証
			if tt.issueData.ParentInfo != nil {
				if loaded.ParentInfo == nil {
					t.Error("LoadIssue() ParentInfo is nil, expected non-nil")
				} else if loaded.ParentInfo.Key != tt.issueData.ParentInfo.Key {
					t.Errorf("LoadIssue() ParentInfo.Key = %v, want %v", loaded.ParentInfo.Key, tt.issueData.ParentInfo.Key)
				}
			}

			// ChildIssues の検証
			if len(tt.issueData.ChildIssues) > 0 {
				if len(loaded.ChildIssues) != len(tt.issueData.ChildIssues) {
					t.Errorf("LoadIssue() len(ChildIssues) = %v, want %v", len(loaded.ChildIssues), len(tt.issueData.ChildIssues))
				}
			}

			// DevStatus の検証
			if tt.issueData.DevStatus != nil {
				if loaded.DevStatus == nil {
					t.Error("LoadIssue() DevStatus is nil, expected non-nil")
				}
			}

			// DevStatusRawJSON の検証
			if tt.issueData.DevStatusRawJSON != nil {
				if loaded.DevStatusRawJSON == nil {
					t.Error("LoadIssue() DevStatusRawJSON is nil, expected non-nil")
				} else if len(loaded.DevStatusRawJSON) == 0 {
					t.Error("LoadIssue() DevStatusRawJSON is empty, expected non-empty")
				}
			}
		})
	}
}

func TestJSONSaver_LoadIssue_Errors(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "jsonsaver_error_test")
	if err != nil {
		t.Fatalf("一時ディレクトリの作成に失敗: %v", err)
	}
	defer os.RemoveAll(tempDir)

	saver := NewJSONSaver(tempDir)

	t.Run("存在しないファイル", func(t *testing.T) {
		_, err := saver.LoadIssue(filepath.Join(tempDir, "nonexistent.json"))
		if err == nil {
			t.Error("LoadIssue() should return error for nonexistent file")
		}
	})

	t.Run("不正なJSON", func(t *testing.T) {
		invalidJSONPath := filepath.Join(tempDir, "invalid.json")
		err := os.WriteFile(invalidJSONPath, []byte("invalid json content"), 0644)
		if err != nil {
			t.Fatalf("テストファイルの作成に失敗: %v", err)
		}

		_, err = saver.LoadIssue(invalidJSONPath)
		if err == nil {
			t.Error("LoadIssue() should return error for invalid JSON")
		}
	})
}

// TestIssueData_ConfluenceSpaces_SerializeDeserialize はConfluenceSpacesフィールドが
// JSON serialize/deserialize で正しく保存・復元されることをテストする
func TestIssueData_ConfluenceSpaces_SerializeDeserialize(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "jsonsaver_confluence_test")
	if err != nil {
		t.Fatalf("一時ディレクトリの作成に失敗: %v", err)
	}
	defer os.RemoveAll(tempDir)

	saver := NewJSONSaver(tempDir)

	tests := []struct {
		name             string
		confluenceSpaces map[string]string
	}{
		{
			name: "ConfluenceSpacesが設定されている場合",
			confluenceSpaces: map[string]string{
				"11111": "エンジニアスペース",
				"22222": "デザインスペース",
				"33333": "マーケティングスペース",
			},
		},
		{
			name:             "ConfluenceSpacesがnilの場合（omitempty）",
			confluenceSpaces: nil,
		},
		{
			name:             "ConfluenceSpacesが空マップの場合",
			confluenceSpaces: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			issueData := &IssueData{
				Issue: &cloud.Issue{
					ID:  "10001",
					Key: "TEST-100",
					Fields: &cloud.IssueFields{
						Summary: "ConfluenceSpacesテスト課題",
						Project: cloud.Project{
							Key:  "TEST",
							Name: "テストプロジェクト",
						},
					},
				},
				ConfluenceSpaces: tt.confluenceSpaces,
				SavedAt:          time.Now().Format(time.RFC3339),
			}

			// Act: 保存
			savedPath, err := saver.SaveIssue(issueData)
			if err != nil {
				t.Fatalf("SaveIssue() error = %v", err)
			}

			// Act: 読み込み
			loaded, err := saver.LoadIssue(savedPath)
			if err != nil {
				t.Fatalf("LoadIssue() error = %v", err)
			}

			// Assert
			if tt.confluenceSpaces == nil || len(tt.confluenceSpaces) == 0 {
				// nilや空マップはomitemptyによりnilで復元される
				if len(loaded.ConfluenceSpaces) != 0 {
					t.Errorf("LoadIssue() ConfluenceSpaces = %v, want nil or empty", loaded.ConfluenceSpaces)
				}
			} else {
				if loaded.ConfluenceSpaces == nil {
					t.Fatal("LoadIssue() ConfluenceSpaces is nil, expected non-nil")
				}
				if len(loaded.ConfluenceSpaces) != len(tt.confluenceSpaces) {
					t.Errorf("LoadIssue() len(ConfluenceSpaces) = %d, want %d",
						len(loaded.ConfluenceSpaces), len(tt.confluenceSpaces))
				}
				for pageID, spaceName := range tt.confluenceSpaces {
					if loaded.ConfluenceSpaces[pageID] != spaceName {
						t.Errorf("LoadIssue() ConfluenceSpaces[%s] = %v, want %v",
							pageID, loaded.ConfluenceSpaces[pageID], spaceName)
					}
				}
			}
		})
	}
}

// TestIssueData_ConfluenceSpaces_JSONFormat はConfluenceSpacesのJSONフォーマットを検証する
func TestIssueData_ConfluenceSpaces_JSONFormat(t *testing.T) {
	// ConfluenceSpacesを持つIssueDataをシリアライズしてJSONキーを確認
	issueData := &IssueData{
		Issue: &cloud.Issue{
			ID:  "10001",
			Key: "TEST-200",
			Fields: &cloud.IssueFields{
				Summary: "JSONフォーマットテスト",
				Project: cloud.Project{Key: "TEST"},
			},
		},
		ConfluenceSpaces: map[string]string{
			"99999": "テストスペース",
		},
		SavedAt: "2025-01-01T00:00:00Z",
	}

	jsonData, err := json.MarshalIndent(issueData, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}

	jsonStr := string(jsonData)

	// JSONキーが正しいこと（"confluenceSpaces"）
	if !strings.Contains(jsonStr, `"confluenceSpaces"`) {
		t.Errorf("JSON should contain key \"confluenceSpaces\", got:\n%s", jsonStr)
	}

	// pageIDとspaceNameが含まれていること
	if !strings.Contains(jsonStr, `"99999"`) {
		t.Errorf("JSON should contain pageID \"99999\", got:\n%s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"テストスペース"`) {
		t.Errorf("JSON should contain spaceName \"テストスペース\", got:\n%s", jsonStr)
	}
}

// TestIssueData_ConfluenceSpaces_OmitemptyWhenNil はConfluenceSpacesがnilのとき
// omitemptyによりJSONに含まれないことをテストする
func TestIssueData_ConfluenceSpaces_OmitemptyWhenNil(t *testing.T) {
	issueData := &IssueData{
		Issue: &cloud.Issue{
			ID:  "10001",
			Key: "TEST-300",
			Fields: &cloud.IssueFields{
				Summary: "omitemptyテスト",
				Project: cloud.Project{Key: "TEST"},
			},
		},
		ConfluenceSpaces: nil, // nilの場合はJSONに含まれない
		SavedAt:          "2025-01-01T00:00:00Z",
	}

	jsonData, err := json.Marshal(issueData)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	jsonStr := string(jsonData)
	if strings.Contains(jsonStr, `"confluenceSpaces"`) {
		t.Errorf("JSON should NOT contain \"confluenceSpaces\" when nil (omitempty), got:\n%s", jsonStr)
	}
}

