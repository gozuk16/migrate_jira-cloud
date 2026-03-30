package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andygrunwald/go-jira/v2/cloud"
)

// ---- resolveConfluenceSpaces テスト ----

// TestResolveConfluenceSpaces_NilClient はconfluenceClientがnilの場合にnilを返すことをテストする
func TestResolveConfluenceSpaces_NilClient(t *testing.T) {
	remoteLinks := []cloud.RemoteLink{
		{
			GlobalID: "appId=test&pageId=12345",
			Application: &cloud.RemoteLinkApplication{
				Type: "com.atlassian.confluence",
				Name: "Confluence",
			},
		},
	}
	result := resolveConfluenceSpaces(remoteLinks, nil)
	if result != nil {
		t.Errorf("resolveConfluenceSpaces() with nil client = %v, want nil", result)
	}
}

// TestResolveConfluenceSpaces_EmptyLinks はリモートリンクが空の場合にnilを返すことをテストする
func TestResolveConfluenceSpaces_EmptyLinks(t *testing.T) {
	client := NewConfluenceClient("https://example.atlassian.net", "user@example.com", "token")
	result := resolveConfluenceSpaces([]cloud.RemoteLink{}, client)
	if result != nil {
		t.Errorf("resolveConfluenceSpaces() with empty links = %v, want nil", result)
	}
}

// TestResolveConfluenceSpaces_OnlyNonConfluenceLinks はConfluence以外のリンクのみの場合にnilを返すことをテストする
func TestResolveConfluenceSpaces_OnlyNonConfluenceLinks(t *testing.T) {
	client := NewConfluenceClient("https://example.atlassian.net", "user@example.com", "token")
	remoteLinks := []cloud.RemoteLink{
		{
			GlobalID: "appId=github&some=data",
			Application: &cloud.RemoteLinkApplication{
				Type: "com.github",
				Name: "GitHub",
			},
		},
		{
			// Applicationがnilのリンク
			GlobalID: "appId=test&pageId=99999",
		},
	}
	result := resolveConfluenceSpaces(remoteLinks, client)
	if result != nil {
		t.Errorf("resolveConfluenceSpaces() with non-confluence links = %v, want nil", result)
	}
}

// TestResolveConfluenceSpaces_WithHTTP はHTTPモックサーバーを使ってConfluenceスペース名を解決することをテストする
func TestResolveConfluenceSpaces_WithHTTP(t *testing.T) {
	// Arrange: モックサーバーを起動
	pageToSpace := map[string]string{
		"11111": "SP1",
		"22222": "SP2",
	}
	spaceNames := map[string]string{
		"SP1": "エンジニアスペース",
		"SP2": "デザインスペース",
	}
	server := newTestConfluenceServer(t, pageToSpace, spaceNames)
	defer server.Close()

	client := NewConfluenceClient(server.URL, "user@example.com", "token")

	remoteLinks := []cloud.RemoteLink{
		{
			GlobalID: "appId=test&pageId=11111",
			Application: &cloud.RemoteLinkApplication{
				Type: "com.atlassian.confluence",
				Name: "Confluence",
			},
		},
		{
			GlobalID: "appId=test&pageId=22222",
			Application: &cloud.RemoteLinkApplication{
				Type: "com.atlassian.confluence",
				Name: "Confluence",
			},
		},
	}

	// Act
	result := resolveConfluenceSpaces(remoteLinks, client)

	// Assert
	if result == nil {
		t.Fatal("resolveConfluenceSpaces() = nil, want non-nil")
	}
	if got, want := result["11111"], "エンジニアスペース"; got != want {
		t.Errorf("result[\"11111\"] = %v, want %v", got, want)
	}
	if got, want := result["22222"], "デザインスペース"; got != want {
		t.Errorf("result[\"22222\"] = %v, want %v", got, want)
	}
}

// TestResolveConfluenceSpaces_InvalidGlobalID はpageIdが取得できないglobalIdを持つリンクをスキップすることをテストする
func TestResolveConfluenceSpaces_InvalidGlobalID(t *testing.T) {
	// ページが存在しないためAPIは呼ばれるがエラーになる
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := NewConfluenceClient(server.URL, "user@example.com", "token")
	remoteLinks := []cloud.RemoteLink{
		{
			// pageIdを含まない不正なglobalId
			GlobalID: "appId=test&spaceId=SP1",
			Application: &cloud.RemoteLinkApplication{
				Type: "com.atlassian.confluence",
				Name: "Confluence",
			},
		},
		{
			// globalIdが空
			GlobalID: "",
			Application: &cloud.RemoteLinkApplication{
				Type: "com.atlassian.confluence",
				Name: "Confluence",
			},
		},
	}

	result := resolveConfluenceSpaces(remoteLinks, client)
	// 有効なエントリが0件なのでnilになる
	if result != nil {
		t.Errorf("resolveConfluenceSpaces() = %v, want nil (all links invalid)", result)
	}
}

// TestResolveConfluenceSpaces_APIError はAPIがエラーを返した場合に該当ページをスキップすることをテストする
func TestResolveConfluenceSpaces_APIError(t *testing.T) {
	// 1つ目は成功、2つ目はAPIエラー
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.URL.Path == "/wiki/api/v2/pages/11111" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ConfluencePageInfo{
				ID:      "11111",
				SpaceID: "SP1",
			})
		} else if r.URL.Path == "/wiki/api/v2/spaces/SP1" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ConfluenceSpaceInfo{
				ID:   "SP1",
				Key:  "SP1",
				Name: "成功スペース",
			})
		} else {
			// 2つ目のページはAPIエラー
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := NewConfluenceClient(server.URL, "user@example.com", "token")
	remoteLinks := []cloud.RemoteLink{
		{
			GlobalID: "appId=test&pageId=11111",
			Application: &cloud.RemoteLinkApplication{
				Type: "com.atlassian.confluence",
			},
		},
		{
			GlobalID: "appId=test&pageId=22222",
			Application: &cloud.RemoteLinkApplication{
				Type: "com.atlassian.confluence",
			},
		},
	}

	result := resolveConfluenceSpaces(remoteLinks, client)
	// 成功した1件のみ結果に含まれる
	if result == nil {
		t.Fatal("resolveConfluenceSpaces() = nil, want non-nil (one success)")
	}
	if _, ok := result["11111"]; !ok {
		t.Error("result should contain pageID 11111")
	}
	if _, ok := result["22222"]; ok {
		t.Error("result should NOT contain pageID 22222 (API error)")
	}
}

// ---- convertFromJSON 並行実行 successCount テスト ----

// createMinimalIssueJSON はconvertFromJSONで読み込める最小限のIssueDataをJSONファイルとして作成する
func createMinimalIssueJSON(t *testing.T, dir, projectKey, issueKey string) string {
	t.Helper()

	issueData := IssueData{
		Issue: &cloud.Issue{
			ID:  issueKey,
			Key: issueKey,
			Fields: &cloud.IssueFields{
				Summary: fmt.Sprintf("テスト課題 %s", issueKey),
				Project: cloud.Project{
					Key:  projectKey,
					Name: fmt.Sprintf("%s プロジェクト", projectKey),
				},
				Type: cloud.IssueType{Name: "Task"},
				Status: &cloud.Status{
					Name: "進行中",
				},
				Created: cloud.Time(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
				Updated: cloud.Time(time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)),
			},
		},
		SavedAt: time.Now().Format(time.RFC3339),
	}

	jsonData, err := json.MarshalIndent(issueData, "", "  ")
	if err != nil {
		t.Fatalf("IssueDataのJSON変換に失敗: %v", err)
	}

	jsonPath := filepath.Join(dir, fmt.Sprintf("%s.json", issueKey))
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		t.Fatalf("JSONファイルの書き込みに失敗: %v", err)
	}
	return jsonPath
}

// TestSuccessCountConcurrency はsuccessCountの並行カウントが正確であることをテストする
// convertFromJSONのatomic.AddInt64相当のロジックを直接テストする
func TestSuccessCountConcurrency(t *testing.T) {
	// Arrange
	const total = 100
	const workers = 8
	var successCount int64

	// convertFromJSONと同じセマフォ + WaitGroup パターン
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	// Act: すべてのgoroutineが成功するケース
	for i := 0; i < total; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			// 処理成功を模擬
			atomic.AddInt64(&successCount, 1)
		}()
	}
	wg.Wait()

	// Assert
	got := atomic.LoadInt64(&successCount)
	if got != total {
		t.Errorf("successCount = %d, want %d（レースコンディションが発生している可能性がある）", got, total)
	}
}

// TestSuccessCountConcurrency_PartialFailure は一部が失敗するケースのsuccessCountをテストする
func TestSuccessCountConcurrency_PartialFailure(t *testing.T) {
	// Arrange
	const total = 50
	const failures = 10
	const workers = 4
	var successCount int64

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	// Act: 一部のgoroutineが失敗する（returnで抜けてAddInt64しない）
	for i := 0; i < total; i++ {
		wg.Add(1)
		sem <- struct{}{}
		idx := i
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			// idx < failures の場合は失敗（returnして成功カウントしない）
			if idx < failures {
				return
			}
			atomic.AddInt64(&successCount, 1)
		}()
	}
	wg.Wait()

	// Assert
	got := atomic.LoadInt64(&successCount)
	want := int64(total - failures)
	if got != want {
		t.Errorf("successCount = %d, want %d", got, want)
	}
}

// TestConvertFromJSON_ConcurrentMarkdownGeneration は複数JSONを並行変換してMarkdownが正確に生成されることをテストする
func TestConvertFromJSON_ConcurrentMarkdownGeneration(t *testing.T) {
	// Arrange: テンポラリディレクトリを作成
	inputDir, err := os.MkdirTemp("", "convert_input_test")
	if err != nil {
		t.Fatalf("入力ディレクトリの作成に失敗: %v", err)
	}
	defer os.RemoveAll(inputDir)

	outputDir, err := os.MkdirTemp("", "convert_output_test")
	if err != nil {
		t.Fatalf("出力ディレクトリの作成に失敗: %v", err)
	}
	defer os.RemoveAll(outputDir)

	// 複数のJSONファイルを作成
	const issueCount = 20
	projectKey := "PARA"
	for i := 1; i <= issueCount; i++ {
		issueKey := fmt.Sprintf("%s-%d", projectKey, i)
		createMinimalIssueJSON(t, inputDir, projectKey, issueKey)
	}

	// Config を作成（テスト用）
	config := createTestConfig()
	config.Output.MarkdownDir = outputDir

	// convertFromJSONと同じ並行実行ロジックを実行
	const workers = 4
	var successCount int64
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	jsonSaver := NewJSONSaver("")
	// ディレクトリ内のJSONファイルを収集
	var jsonFiles []string
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		t.Fatalf("ディレクトリの読み込みに失敗: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			jsonFiles = append(jsonFiles, filepath.Join(inputDir, e.Name()))
		}
	}

	if len(jsonFiles) != issueCount {
		t.Fatalf("期待するJSONファイル数 = %d, got %d", issueCount, len(jsonFiles))
	}

	total := len(jsonFiles)
	for _, jsonFile := range jsonFiles {
		wg.Add(1)
		sem <- struct{}{}
		file := jsonFile
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			data, err := jsonSaver.LoadIssue(file)
			if err != nil {
				t.Errorf("JSON読み込みエラー: %v", err)
				return
			}

			fieldNameCache := BuildFieldNameCache(data.Fields)
			userMapping := make(UserMapping)
			BuildUserMappingFromIssue(data.Issue, userMapping)

			// 各goroutineで独立したmdWriterを作成（レースコンディション回避）
			mdWriter := NewMarkdownWriter(outputDir, userMapping, config)
			if len(data.ConfluenceSpaces) > 0 {
				mdWriter.SetConfluenceSpaces(data.ConfluenceSpaces)
			}

			pKey := data.Issue.Fields.Project.Key
			issueDir := filepath.Join(outputDir, pKey, data.Issue.Key)
			if err := os.MkdirAll(issueDir, 0755); err != nil {
				t.Errorf("課題ディレクトリの作成に失敗: %v", err)
				return
			}

			if err := mdWriter.WriteIssue(data.Issue, nil, fieldNameCache, data.DevStatus, data.ParentInfo, data.ChildIssues, data.RemoteLinks); err != nil {
				t.Errorf("Markdown生成エラー: %v", err)
				return
			}

			atomic.AddInt64(&successCount, 1)
		}()
	}
	wg.Wait()

	// Assert: 全件成功していること
	got := atomic.LoadInt64(&successCount)
	if got != int64(total) {
		t.Errorf("successCount = %d, want %d", got, total)
	}

	// 生成されたMarkdownファイルが存在することを確認
	for i := 1; i <= issueCount; i++ {
		issueKey := fmt.Sprintf("%s-%d", projectKey, i)
		mdPath := filepath.Join(outputDir, projectKey, issueKey, "index.md")
		if _, err := os.Stat(mdPath); os.IsNotExist(err) {
			t.Errorf("Markdownファイルが存在しない: %s", mdPath)
		}
	}
}

// TestConvertFromJSON_IndependentMdWriter は各goroutineが独立したmdWriterを持つことを確認する
// (SetConfluenceSpacesが並行呼び出しされても問題ないこと)
func TestConvertFromJSON_IndependentMdWriter(t *testing.T) {
	config := createTestConfig()
	outputDir, err := os.MkdirTemp("", "mdwriter_independent_test")
	if err != nil {
		t.Fatalf("出力ディレクトリの作成に失敗: %v", err)
	}
	defer os.RemoveAll(outputDir)
	config.Output.MarkdownDir = outputDir

	const goroutines = 20
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()

			// 各goroutineが独立したmdWriterを作成
			userMapping := make(UserMapping)
			mdWriter := NewMarkdownWriter(outputDir, userMapping, config)

			// 異なるConfluenceSpacesマップを設定（並行アクセスをシミュレート）
			spaces := map[string]string{
				fmt.Sprintf("page%d", idx): fmt.Sprintf("スペース%d", idx),
			}
			mdWriter.SetConfluenceSpaces(spaces)

			// SetConfluenceSpacesが正しく設定されたことを確認
			if mdWriter.confluenceSpaces == nil {
				errs <- fmt.Errorf("goroutine %d: confluenceSpaces is nil", idx)
				return
			}
			expected := fmt.Sprintf("スペース%d", idx)
			key := fmt.Sprintf("page%d", idx)
			if mdWriter.confluenceSpaces[key] != expected {
				errs <- fmt.Errorf("goroutine %d: confluenceSpaces[%s] = %v, want %v", idx, key, mdWriter.confluenceSpaces[key], expected)
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}

// ---- attachmentTargetDir テスト ----

func TestAttachmentTargetDir(t *testing.T) {
	tests := []struct {
		name       string
		staticDir  string
		markdownDir string
		projectKey string
		issueKey   string
		wantSuffix string // filepath.Join の末尾部分で検証
		wantLower  bool   // 小文字化されているか
	}{
		{
			name:        "StaticDir未設定: MarkdownDir下に保存",
			staticDir:   "",
			markdownDir: "output/markdown",
			projectKey:  "SCRUM",
			issueKey:    "SCRUM-2",
			wantSuffix:  filepath.Join("output", "markdown", "SCRUM", "SCRUM-2"),
			wantLower:   false,
		},
		{
			name:        "StaticDir設定: static下の小文字パス",
			staticDir:   "hugo-jira/static",
			markdownDir: "output/markdown",
			projectKey:  "SCRUM",
			issueKey:    "SCRUM-2",
			wantSuffix:  filepath.Join("hugo-jira", "static", "scrum", "scrum-2"),
			wantLower:   true,
		},
		{
			name:        "StaticDir設定: 大文字プロジェクトキーが小文字化される",
			staticDir:   "/tmp/static",
			markdownDir: "output/markdown",
			projectKey:  "MYPROJECT",
			issueKey:    "MYPROJECT-123",
			wantSuffix:  filepath.Join("tmp", "static", "myproject", "myproject-123"),
			wantLower:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Output: OutputConfig{
					MarkdownDir: tt.markdownDir,
					StaticDir:   tt.staticDir,
				},
			}
			got := attachmentTargetDir(config, tt.projectKey, tt.issueKey)

			if !strings.HasSuffix(got, tt.wantSuffix) {
				t.Errorf("attachmentTargetDir() = %q, want suffix %q", got, tt.wantSuffix)
			}
			if tt.wantLower {
				if strings.Contains(got, tt.projectKey) && tt.projectKey != strings.ToLower(tt.projectKey) {
					t.Errorf("attachmentTargetDir() = %q, should be lowercased", got)
				}
			}
		})
	}
}
