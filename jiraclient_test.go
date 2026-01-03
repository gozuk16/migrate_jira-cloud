package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andygrunwald/go-jira/v2/cloud"
)

// TestNewJIRAClient はNewJIRAClient関数のテスト
func TestNewJIRAClient(t *testing.T) {
	tests := []struct {
		name    string
		config  *JIRAConfig
		wantErr bool
	}{
		{
			name: "正常系: 有効な設定",
			config: &JIRAConfig{
				URL:      "https://test.atlassian.net",
				Email:    "test@example.com",
				APIToken: "test-token",
			},
			wantErr: false,
		},
		{
			name: "異常系: 無効なURL",
			config: &JIRAConfig{
				URL:      "://invalid-url",
				Email:    "test@example.com",
				APIToken: "test-token",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewJIRAClient(tt.config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("エラーが期待されましたが、nilが返されました")
				}
				return
			}

			if err != nil {
				t.Errorf("予期しないエラー: %v", err)
				return
			}

			if client == nil {
				t.Error("クライアントがnilです")
				return
			}

			if client.baseURL != tt.config.URL {
				t.Errorf("baseURL = %q, want %q", client.baseURL, tt.config.URL)
			}
			if client.email != tt.config.Email {
				t.Errorf("email = %q, want %q", client.email, tt.config.Email)
			}
			if client.apiToken != tt.config.APIToken {
				t.Errorf("apiToken = %q, want %q", client.apiToken, tt.config.APIToken)
			}
		})
	}
}

// TestSearchJQLV3 はSearchJQLV3メソッドのテスト
func TestSearchJQLV3(t *testing.T) {
	tests := []struct {
		name           string
		jql            string
		maxResults     int
		mockResponses  []JQLSearchResponse
		wantErr        bool
		wantIssueCount int
	}{
		{
			name:       "正常系: 1ページの結果",
			jql:        "project = TEST",
			maxResults: 50,
			mockResponses: []JQLSearchResponse{
				{
					IsLast: true,
					Issues: []cloud.Issue{
						{Key: "TEST-1"},
						{Key: "TEST-2"},
						{Key: "TEST-3"},
					},
				},
			},
			wantErr:        false,
			wantIssueCount: 3,
		},
		{
			name:       "正常系: 複数ページの結果",
			jql:        "project = TEST",
			maxResults: 2,
			mockResponses: []JQLSearchResponse{
				{
					IsLast:        false,
					NextPageToken: "page2",
					Issues: []cloud.Issue{
						{Key: "TEST-1"},
						{Key: "TEST-2"},
					},
				},
				{
					IsLast: true,
					Issues: []cloud.Issue{
						{Key: "TEST-3"},
					},
				},
			},
			wantErr:        false,
			wantIssueCount: 3,
		},
		{
			name:       "正常系: 結果が0件",
			jql:        "project = EMPTY",
			maxResults: 50,
			mockResponses: []JQLSearchResponse{
				{
					IsLast: true,
					Issues: []cloud.Issue{},
				},
			},
			wantErr:        false,
			wantIssueCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// モックサーバーのリクエストカウンター
			requestCount := 0

			// モックHTTPサーバーの作成
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Basic認証のチェック
				username, password, ok := r.BasicAuth()
				if !ok || username != "test@example.com" || password != "test-token" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				// リクエストメソッドの確認
				if r.Method != "GET" {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}

				// レスポンスを返す
				if requestCount < len(tt.mockResponses) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(tt.mockResponses[requestCount])
					requestCount++
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			// テスト用のJIRAクライアントを作成
			client := &JIRAClient{
				ctx:        context.Background(),
				httpClient: server.Client(),
				baseURL:    server.URL,
				email:      "test@example.com",
				apiToken:   "test-token",
			}

			// SearchJQLV3の実行
			issueKeys, err := client.SearchJQLV3(tt.jql, tt.maxResults)

			// エラーチェック
			if tt.wantErr {
				if err == nil {
					t.Errorf("エラーが期待されましたが、nilが返されました")
				}
				return
			}

			if err != nil {
				t.Errorf("予期しないエラー: %v", err)
				return
			}

			// 課題数のチェック
			if len(issueKeys) != tt.wantIssueCount {
				t.Errorf("課題数 = %d, want %d", len(issueKeys), tt.wantIssueCount)
			}

			// 課題キーの検証（最初のページのみ）
			if len(tt.mockResponses) > 0 && len(tt.mockResponses[0].Issues) > 0 {
				for i, issue := range tt.mockResponses[0].Issues {
					if i < len(issueKeys) && issueKeys[i] != issue.Key {
						t.Errorf("issueKeys[%d] = %q, want %q", i, issueKeys[i], issue.Key)
					}
				}
			}
		})
	}
}

// TestSearchJQLV3_HTTPError はHTTPエラーのテスト
func TestSearchJQLV3_HTTPError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "異常系: 404 Not Found",
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name:       "異常系: 401 Unauthorized",
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
		},
		{
			name:       "異常系: 500 Internal Server Error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// モックHTTPサーバーの作成
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(`{"errorMessages":["Test error"]}`))
			}))
			defer server.Close()

			// テスト用のJIRAクライアントを作成
			client := &JIRAClient{
				ctx:        context.Background(),
				httpClient: server.Client(),
				baseURL:    server.URL,
				email:      "test@example.com",
				apiToken:   "test-token",
			}

			// SearchJQLV3の実行
			_, err := client.SearchJQLV3("project = TEST", 50)

			// エラーチェック
			if tt.wantErr {
				if err == nil {
					t.Errorf("エラーが期待されましたが、nilが返されました")
				}
				return
			}

			if err != nil {
				t.Errorf("予期しないエラー: %v", err)
			}
		})
	}
}

// TestSearchJQLV3_InvalidJSON は無効なJSONレスポンスのテスト
func TestSearchJQLV3_InvalidJSON(t *testing.T) {
	// モックHTTPサーバーの作成
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	// テスト用のJIRAクライアントを作成
	client := &JIRAClient{
		ctx:        nil,
		httpClient: server.Client(),
		baseURL:    server.URL,
		email:      "test@example.com",
		apiToken:   "test-token",
	}

	// SearchJQLV3の実行
	_, err := client.SearchJQLV3("project = TEST", 50)

	// エラーが返されることを確認
	if err == nil {
		t.Error("無効なJSONに対してエラーが返されませんでした")
	}
}

// TestGetDevStatusDetails はGetDevStatusDetailsメソッドのテスト
func TestGetDevStatusDetails(t *testing.T) {
	tests := []struct {
		name             string
		issueID          string
		applicationType  string
		dataType         string
		mockResponse     DevStatusDetail
		mockStatusCode   int
		wantErr          bool
		wantPRCount      int
		wantBranchCount  int
	}{
		{
			name:            "正常系: プルリクエスト情報取得",
			issueID:         "10001",
			applicationType: "github",
			dataType:        "pullrequest",
			mockResponse: DevStatusDetail{
				Detail: []DevStatusDetailItem{
					{
						PullRequests: []DevPullRequest{
							{
								ID:     "1",
								Name:   "Fix bug",
								Author: DevAuthor{Name: "test-user"},
								Status: "OPEN",
								Source: DevPullRequestBranch{Branch: "fix-bug"},
								URL:    "https://github.com/test/repo/pull/1",
							},
						},
					},
				},
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			wantPRCount:    1,
		},
		{
			name:            "正常系: ブランチ情報取得",
			issueID:         "10001",
			applicationType: "github",
			dataType:        "branch",
			mockResponse: DevStatusDetail{
				Detail: []DevStatusDetailItem{
					{
						Branches: []DevBranch{
							{
								Name: "feature-branch",
								URL:  "https://github.com/test/repo/tree/feature-branch",
							},
						},
					},
				},
			},
			mockStatusCode:  http.StatusOK,
			wantErr:         false,
			wantBranchCount: 1,
		},
		{
			name:            "正常系: 開発情報が存在しない",
			issueID:         "10001",
			applicationType: "github",
			dataType:        "pullrequest",
			mockResponse: DevStatusDetail{
				Detail: []DevStatusDetailItem{},
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			wantPRCount:    0,
		},
		{
			name:            "異常系: HTTPエラー（404）",
			issueID:         "10001",
			applicationType: "github",
			dataType:        "pullrequest",
			mockStatusCode:  http.StatusNotFound,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// モックHTTPサーバーの作成
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Basic認証のチェック
				username, password, ok := r.BasicAuth()
				if !ok || username != "test@example.com" || password != "test-token" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				// クエリパラメータの確認
				if r.URL.Query().Get("issueId") != tt.issueID {
					t.Errorf("issueId = %q, want %q", r.URL.Query().Get("issueId"), tt.issueID)
				}
				if r.URL.Query().Get("applicationType") != tt.applicationType {
					t.Errorf("applicationType = %q, want %q", r.URL.Query().Get("applicationType"), tt.applicationType)
				}
				if r.URL.Query().Get("dataType") != tt.dataType {
					t.Errorf("dataType = %q, want %q", r.URL.Query().Get("dataType"), tt.dataType)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.mockStatusCode)
				if tt.mockStatusCode == http.StatusOK {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			// テスト用のJIRAクライアントを作成
			client := &JIRAClient{
				ctx:        context.Background(),
				httpClient: server.Client(),
				baseURL:    server.URL,
				email:      "test@example.com",
				apiToken:   "test-token",
			}

			// GetDevStatusDetailsの実行
			detail, err := client.GetDevStatusDetails(tt.issueID, tt.applicationType, tt.dataType)

			// エラーチェック
			if tt.wantErr {
				if err == nil {
					t.Errorf("エラーが期待されましたが、nilが返されました")
				}
				return
			}

			if err != nil {
				t.Errorf("予期しないエラー: %v", err)
				return
			}

			if detail == nil {
				t.Error("detailがnilです")
				return
			}

			// プルリクエスト数のチェック
			if len(detail.Detail) > 0 {
				actualPRCount := len(detail.Detail[0].PullRequests)
				if actualPRCount != tt.wantPRCount {
					t.Errorf("PRCount = %d, want %d", actualPRCount, tt.wantPRCount)
				}

				actualBranchCount := len(detail.Detail[0].Branches)
				if actualBranchCount != tt.wantBranchCount {
					t.Errorf("BranchCount = %d, want %d", actualBranchCount, tt.wantBranchCount)
				}
			}
		})
	}
}

// TestGetIssuesByJQL はGetIssuesByJQLメソッドのテスト
func TestGetIssuesByJQL(t *testing.T) {
	// モックHTTPサーバーの作成
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(JQLSearchResponse{
			IsLast: true,
			Issues: []cloud.Issue{
				{Key: "TEST-1"},
				{Key: "TEST-2"},
			},
		})
	}))
	defer server.Close()

	// テスト用のJIRAクライアントを作成
	client := &JIRAClient{
		ctx:        context.Background(),
		httpClient: server.Client(),
		baseURL:    server.URL,
		email:      "test@example.com",
		apiToken:   "test-token",
	}

	// GetIssuesByJQLの実行
	issueKeys, err := client.GetIssuesByJQL("project = TEST", 50)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	// 課題数のチェック
	if len(issueKeys) != 2 {
		t.Errorf("課題数 = %d, want 2", len(issueKeys))
	}

	// 課題キーの検証
	expectedKeys := []string{"TEST-1", "TEST-2"}
	for i, key := range expectedKeys {
		if i < len(issueKeys) && issueKeys[i] != key {
			t.Errorf("issueKeys[%d] = %q, want %q", i, issueKeys[i], key)
		}
	}
}
