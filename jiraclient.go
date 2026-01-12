package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/andygrunwald/go-jira/v2/cloud"
)

// JIRAClient はJIRA APIクライアントのラッパー
type JIRAClient struct {
	client     *cloud.Client
	ctx        context.Context
	httpClient *http.Client
	baseURL    string
	email      string
	apiToken   string
}

// JQLSearchRequest は新しい /rest/api/3/search/jql エンドポイント用のリクエスト構造体
type JQLSearchRequest struct {
	JQL           string   `json:"jql"`
	MaxResults    int      `json:"maxResults"`
	Fields        []string `json:"fields,omitempty"`
	NextPageToken string   `json:"nextPageToken,omitempty"`
}

// JQLSearchResponse は新しい /rest/api/3/search/jql エンドポイント用のレスポンス構造体
type JQLSearchResponse struct {
	Expand        string        `json:"expand"`
	IsLast        bool          `json:"isLast"`
	Issues        []cloud.Issue `json:"issues"` // レスポンスのキーは "issues"
	NextPageToken string        `json:"nextPageToken,omitempty"`
}

// NewJIRAClient は新しいJIRAクライアントを作成する
func NewJIRAClient(config *JIRAConfig) (*JIRAClient, error) {
	// Basic認証用のトランスポート設定
	tp := cloud.BasicAuthTransport{
		Username: config.Email,
		APIToken: config.APIToken,
	}

	// JIRAクライアントの作成
	client, err := cloud.NewClient(config.URL, tp.Client())
	if err != nil {
		return nil, fmt.Errorf("JIRAクライアントの作成に失敗しました: %w", err)
	}

	return &JIRAClient{
		client:     client,
		ctx:        context.Background(),
		httpClient: tp.Client(),
		baseURL:    config.URL,
		email:      config.Email,
		apiToken:   config.APIToken,
	}, nil
}

// GetIssue は指定された課題キーまたはIDの詳細情報を取得する
func (jc *JIRAClient) GetIssue(issueKey string) (*cloud.Issue, error) {
	// expandパラメータで追加情報を取得
	// - renderedFields: HTMLレンダリング済みの項目値
	issue, resp, err := jc.client.Issue.Get(jc.ctx, issueKey, &cloud.GetQueryOptions{
		Expand: "renderedFields",
	})

	// リクエストヘッダー情報をログ出力（go-jiraライブラリ経由のため、実際のヘッダーは取得できない）
	slog.Info("課題取得リクエスト",
		"issueKey", issueKey,
		"expand", "renderedFields",
		"note", "ヘッダーはgo-jiraライブラリが自動設定")
	if err != nil {
		slog.Error("課題取得エラー",
			"issueKey", issueKey,
			"error", err)
		return nil, fmt.Errorf("課題 %s の取得に失敗しました: %w", issueKey, err)
	}

	slog.Info("課題取得成功",
		"issueKey", issue.Key,
		"summary", issue.Fields.Summary,
		"status", resp.StatusCode,
		"headers", resp.Header)

	return issue, nil
}

// SearchJQLV3 は新しい /rest/api/3/search/jql エンドポイントを使用してJQL検索を実行する（GETメソッド）
// 課題キーのリストのみを返す（軽量な検索）
func (jc *JIRAClient) SearchJQLV3(jql string, maxResults int) ([]string, error) {
	allIssueKeys := []string{}
	nextPageToken := ""
	seenTokens := make(map[string]bool)
	maxPages := 100 // 無限ループ防止

	for page := 0; page < maxPages; page++ {
		// URLクエリパラメータの構築
		apiURL := fmt.Sprintf("%s/rest/api/3/search/jql", jc.baseURL)

		// JQLの値だけをURLエンコード
		encodedJQL := url.QueryEscape(jql)

		// クエリ文字列を手動で構築
		// id,keyのみを取得して軽量なレスポンスにする
		params := fmt.Sprintf("jql=%s&maxResults=%d&fields=id,key",
			encodedJQL, maxResults)

		// NextPageTokenがある場合は追加（値だけエンコード）
		if nextPageToken != "" {
			encodedToken := url.QueryEscape(nextPageToken)
			params += fmt.Sprintf("&nextPageToken=%s", encodedToken)
		}

		// URL形式: ?jql=project%3DSCRUM&maxResults=50&fields=*all&expand=renderedFields
		requestURL := fmt.Sprintf("%s?%s", apiURL, params)

		// HTTPリクエストの作成（GETメソッド）
		req, err := http.NewRequestWithContext(jc.ctx, "GET", requestURL, nil)
		if err != nil {
			return nil, fmt.Errorf("HTTPリクエストの作成に失敗しました: %w", err)
		}

		// ヘッダーの設定
		req.Header.Set("Accept", "application/json")
		req.SetBasicAuth(jc.email, jc.apiToken)

		slog.Info("JQL検索リクエスト",
			"method", "GET",
			"url", requestURL,
			"page", page+1,
			"maxResults", maxResults,
			"headers", req.Header)

		// リクエストの実行
		resp, err := jc.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("HTTPリクエストの実行に失敗しました: %w", err)
		}
		defer resp.Body.Close()

		// ステータスコードの確認
		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			slog.Error("JQL検索エラー",
				"status", resp.StatusCode,
				"body", string(bodyBytes))
			return nil, fmt.Errorf("JQL検索に失敗しました。ステータスコード: %d, レスポンス: %s", resp.StatusCode, string(bodyBytes))
		}

		// レスポンスボディを読み取り
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("レスポンスボディ読み取りエラー",
				"error", err)
			return nil, fmt.Errorf("レスポンスボディの読み取りに失敗しました: %w", err)
		}

		// デバッグ用：レスポンスボディをログ出力
		slog.Debug("レスポンスボディ",
			"body", string(bodyBytes))

		// レスポンスのパース
		var searchResp JQLSearchResponse
		if err := json.Unmarshal(bodyBytes, &searchResp); err != nil {
			slog.Error("レスポンスパースエラー",
				"error", err,
				"bodyPreview", string(bodyBytes[:min(500, len(bodyBytes))]))
			return nil, fmt.Errorf("レスポンスのパースに失敗しました: %w", err)
		}

		slog.Info("JQL検索レスポンス",
			"status", resp.StatusCode,
			"headers", resp.Header,
			"count", len(searchResp.Issues),
			"isLast", searchResp.IsLast,
			"hasNextToken", searchResp.NextPageToken != "",
			"totalIssues", len(allIssueKeys)+len(searchResp.Issues))

		// 取得した課題キーを追加
		for _, issue := range searchResp.Issues {
			allIssueKeys = append(allIssueKeys, issue.Key)
		}

		// 終了条件の判定
		if searchResp.IsLast || len(searchResp.Issues) == 0 {
			break
		}

		// NextPageTokenの重複チェック（無限ループ防止）
		if searchResp.NextPageToken != "" {
			if seenTokens[searchResp.NextPageToken] {
				// 同じトークンが返された場合は終了
				break
			}
			seenTokens[searchResp.NextPageToken] = true
			nextPageToken = searchResp.NextPageToken
		} else {
			// NextPageTokenがない場合は終了
			break
		}
	}

	return allIssueKeys, nil
}

// GetIssuesByJQL はJQLクエリに基づいて課題キーのリストを取得する（新しいAPIエンドポイントを使用）
func (jc *JIRAClient) GetIssuesByJQL(jql string, maxResults int) ([]string, error) {
	// 新しい /rest/api/3/search/jql エンドポイントを使用して課題キーを取得
	issueKeys, err := jc.SearchJQLV3(jql, maxResults)
	if err != nil {
		return nil, fmt.Errorf("JQL検索に失敗しました: %w", err)
	}

	return issueKeys, nil
}

// GetChildIssues は指定された課題の子課題キーを取得する
func (jc *JIRAClient) GetChildIssues(parentKey string, maxResults int) ([]string, error) {
	// JQLクエリで親課題を指定して子課題を取得
	jql := fmt.Sprintf(`parent = "%s"`, parentKey)
	issueKeys, err := jc.GetIssuesByJQL(jql, maxResults)
	if err != nil {
		slog.Warn("子課題の取得に失敗しました",
			"parentKey", parentKey,
			"error", err)
		return []string{}, nil // 子課題が存在しない場合は空配列を返す
	}

	return issueKeys, nil
}

// GetFieldList は全フィールド情報を取得する
func (jc *JIRAClient) GetFieldList() ([]cloud.Field, error) {
	fields, _, err := jc.client.Field.GetList(jc.ctx)
	if err != nil {
		return nil, fmt.Errorf("フィールドリストの取得に失敗しました: %w", err)
	}
	return fields, nil
}

// GetProject はプロジェクトの詳細情報を取得する
func (jc *JIRAClient) GetProject(projectKey string) (*cloud.Project, error) {
	project, resp, err := jc.client.Project.Get(jc.ctx, projectKey)
	if err != nil {
		slog.Error("プロジェクト取得エラー",
			"projectKey", projectKey,
			"error", err)
		return nil, fmt.Errorf("プロジェクト %s の取得に失敗しました: %w", projectKey, err)
	}

	slog.Info("プロジェクト取得成功",
		"projectKey", project.Key,
		"name", project.Name,
		"status", resp.StatusCode)

	return project, nil
}

// DevStatusDetail はDev-Status APIのレスポンス構造
type DevStatusDetail struct {
	Detail []DevStatusDetailItem `json:"detail"`
}

type DevStatusDetailItem struct {
	Branches     []DevBranch      `json:"branches"`
	PullRequests []DevPullRequest `json:"pullRequests"`
}

type DevBranch struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type DevPullRequest struct {
	ID     string                `json:"id"`
	Name   string                `json:"name"`
	Author DevAuthor             `json:"author"`
	Status string                `json:"status"`
	Source DevPullRequestBranch  `json:"source"`
	URL    string                `json:"url"`
}

type DevPullRequestBranch struct {
	Branch string `json:"branch"`
	URL    string `json:"url"`
}

type DevAuthor struct {
	Name string `json:"name"`
}

// GetDevStatusDetails はDev-Status APIから開発情報の詳細を取得する
func (jc *JIRAClient) GetDevStatusDetails(issueID, applicationType, dataType string) (*DevStatusDetail, error) {
	apiURL := fmt.Sprintf("%s/rest/dev-status/1.0/issue/detail", jc.baseURL)

	// クエリパラメータ構築
	params := url.Values{}
	params.Set("issueId", issueID)
	params.Set("applicationType", applicationType)
	params.Set("dataType", dataType)

	requestURL := fmt.Sprintf("%s?%s", apiURL, params.Encode())

	req, err := http.NewRequestWithContext(jc.ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("HTTPリクエストの作成に失敗: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(jc.email, jc.apiToken)

	slog.Debug("Dev-Status API リクエスト",
		"url", requestURL,
		"issueID", issueID,
		"applicationType", applicationType,
		"dataType", dataType)

	resp, err := jc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTPリクエストの実行に失敗: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		slog.Warn("Dev-Status API エラー",
			"status", resp.StatusCode,
			"body", string(bodyBytes))
		return nil, fmt.Errorf("Dev-Status API エラー: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("レスポンスボディ読み取り失敗: %w", err)
	}

	slog.Debug("Dev-Status API レスポンス", "body", string(bodyBytes))

	var detail DevStatusDetail
	if err := json.Unmarshal(bodyBytes, &detail); err != nil {
		return nil, fmt.Errorf("レスポンスパース失敗: %w", err)
	}

	return &detail, nil
}
