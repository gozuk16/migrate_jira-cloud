package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"
)

// ConfluenceClient はConfluence REST API v2クライアント
type ConfluenceClient struct {
	baseURL    string
	email      string
	apiToken   string
	httpClient *http.Client
	spaceCache *SpaceCache
}

// SpaceCache はスペース情報のキャッシュ
type SpaceCache struct {
	mu     sync.RWMutex
	spaces map[string]string // spaceId -> spaceName
}

// ConfluencePageInfo はページ情報
type ConfluencePageInfo struct {
	ID      string `json:"id"`
	SpaceID string `json:"spaceId"`
	Title   string `json:"title"`
}

// ConfluenceSpaceInfo はスペース情報
type ConfluenceSpaceInfo struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// NewConfluenceClient は新しいConfluenceクライアントを作成
func NewConfluenceClient(baseURL, email, apiToken string) *ConfluenceClient {
	return &ConfluenceClient{
		baseURL:  baseURL,
		email:    email,
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		spaceCache: &SpaceCache{
			spaces: make(map[string]string),
		},
	}
}

// ExtractPageIDFromGlobalID はglobalIdからページIDを抽出
func ExtractPageIDFromGlobalID(globalID string) (string, error) {
	re := regexp.MustCompile(`pageId=(\d+)`)
	matches := re.FindStringSubmatch(globalID)
	if len(matches) < 2 {
		return "", fmt.Errorf("pageId not found in globalId: %s", globalID)
	}
	return matches[1], nil
}

// GetPageInfo はページ情報を取得
func (cc *ConfluenceClient) GetPageInfo(pageID string) (*ConfluencePageInfo, error) {
	url := fmt.Sprintf("%s/wiki/api/v2/pages/%s", cc.baseURL, pageID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(cc.email, cc.apiToken)
	req.Header.Set("Accept", "application/json")

	resp, err := cc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("confluence API error: %d %s", resp.StatusCode, string(body))
	}

	var pageInfo ConfluencePageInfo
	if err := json.NewDecoder(resp.Body).Decode(&pageInfo); err != nil {
		return nil, err
	}

	return &pageInfo, nil
}

// GetSpaceInfo はスペース情報を取得（キャッシュ使用）
func (cc *ConfluenceClient) GetSpaceInfo(spaceID string) (*ConfluenceSpaceInfo, error) {
	// キャッシュチェック
	cc.spaceCache.mu.RLock()
	if spaceName, ok := cc.spaceCache.spaces[spaceID]; ok {
		cc.spaceCache.mu.RUnlock()
		return &ConfluenceSpaceInfo{
			ID:   spaceID,
			Name: spaceName,
		}, nil
	}
	cc.spaceCache.mu.RUnlock()

	// APIから取得
	url := fmt.Sprintf("%s/wiki/api/v2/spaces/%s", cc.baseURL, spaceID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(cc.email, cc.apiToken)
	req.Header.Set("Accept", "application/json")

	resp, err := cc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("confluence API error: %d %s", resp.StatusCode, string(body))
	}

	var spaceInfo ConfluenceSpaceInfo
	if err := json.NewDecoder(resp.Body).Decode(&spaceInfo); err != nil {
		return nil, err
	}

	// キャッシュに保存
	cc.spaceCache.mu.Lock()
	cc.spaceCache.spaces[spaceID] = spaceInfo.Name
	cc.spaceCache.mu.Unlock()

	return &spaceInfo, nil
}

// GetSpaceName はページIDからスペース名を取得
func (cc *ConfluenceClient) GetSpaceName(pageID string) (string, error) {
	pageInfo, err := cc.GetPageInfo(pageID)
	if err != nil {
		return "", err
	}

	spaceInfo, err := cc.GetSpaceInfo(pageInfo.SpaceID)
	if err != nil {
		return "", err
	}

	return spaceInfo.Name, nil
}
