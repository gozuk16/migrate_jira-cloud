package main

import (
	"os"
	"path/filepath"
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
				SavedAt: time.Now().Format(time.RFC3339),
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
