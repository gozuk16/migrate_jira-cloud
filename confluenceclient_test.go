package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestExtractPageIDFromGlobalID(t *testing.T) {
	tests := []struct {
		name      string
		globalID  string
		want      string
		wantError bool
	}{
		{
			name:      "正常系",
			globalID:  "appId=0c89e1b8-ac25-318c-b359-03c7f3bc2b18&pageId=98412",
			want:      "98412",
			wantError: false,
		},
		{
			name:      "pageIdなし",
			globalID:  "appId=0c89e1b8-ac25-318c-b359-03c7f3bc2b18",
			want:      "",
			wantError: true,
		},
		{
			name:      "複数のpageId",
			globalID:  "appId=test&pageId=12345&extra=value",
			want:      "12345",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractPageIDFromGlobalID(tt.globalID)
			if (err != nil) != tt.wantError {
				t.Errorf("ExtractPageIDFromGlobalID() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if got != tt.want {
				t.Errorf("ExtractPageIDFromGlobalID() = %v, want %v", got, tt.want)
			}
		})
	}
}

// newTestConfluenceServer はテスト用のHTTPサーバーを起動する。
// pageID -> spaceID のマップと spaceID -> spaceName のマップを受け取り、
// /wiki/api/v2/pages/:id と /wiki/api/v2/spaces/:id のエンドポイントを提供する。
func newTestConfluenceServer(t *testing.T, pageToSpace map[string]string, spaceNames map[string]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/wiki/api/v2/pages/", func(w http.ResponseWriter, r *http.Request) {
		// /wiki/api/v2/pages/{pageID} からpageIDを取り出す
		pageID := r.URL.Path[len("/wiki/api/v2/pages/"):]
		spaceID, ok := pageToSpace[pageID]
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ConfluencePageInfo{
			ID:      pageID,
			SpaceID: spaceID,
			Title:   "Page " + pageID,
		})
	})

	mux.HandleFunc("/wiki/api/v2/spaces/", func(w http.ResponseWriter, r *http.Request) {
		spaceID := r.URL.Path[len("/wiki/api/v2/spaces/"):]
		name, ok := spaceNames[spaceID]
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ConfluenceSpaceInfo{
			ID:   spaceID,
			Key:  spaceID,
			Name: name,
		})
	})

	return httptest.NewServer(mux)
}

// TestGetSpaceInfo_Cache はSpaceCacheのキャッシュ機能をテストする
func TestGetSpaceInfo_Cache(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wiki/api/v2/spaces/SP1" {
			callCount++
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ConfluenceSpaceInfo{
				ID:   "SP1",
				Key:  "SPACE",
				Name: "テストスペース",
			})
		} else {
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewConfluenceClient(server.URL, "user@example.com", "token")

	// 初回はAPIを呼ぶ
	info, err := client.GetSpaceInfo("SP1")
	if err != nil {
		t.Fatalf("GetSpaceInfo() error = %v", err)
	}
	if info.Name != "テストスペース" {
		t.Errorf("GetSpaceInfo() Name = %v, want テストスペース", info.Name)
	}
	if callCount != 1 {
		t.Errorf("API呼び出し回数 = %d, want 1", callCount)
	}

	// 2回目はキャッシュから返すのでAPIを呼ばない
	info2, err := client.GetSpaceInfo("SP1")
	if err != nil {
		t.Fatalf("GetSpaceInfo() 2nd error = %v", err)
	}
	if info2.Name != "テストスペース" {
		t.Errorf("GetSpaceInfo() 2nd Name = %v, want テストスペース", info2.Name)
	}
	if callCount != 1 {
		t.Errorf("API呼び出し回数（キャッシュ後）= %d, want 1（キャッシュから返すべき）", callCount)
	}
}

// TestSpaceCache_ConcurrentAccess はSpaceCacheのRWMutexが並行アクセスを安全に扱えることを確認する
func TestSpaceCache_ConcurrentAccess(t *testing.T) {
	// 複数のスペースIDを用意
	spaceNames := map[string]string{
		"SP1": "スペース1",
		"SP2": "スペース2",
		"SP3": "スペース3",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /wiki/api/v2/spaces/{spaceID}
		path := r.URL.Path
		prefix := "/wiki/api/v2/spaces/"
		if len(path) <= len(prefix) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		spaceID := path[len(prefix):]
		name, ok := spaceNames[spaceID]
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ConfluenceSpaceInfo{
			ID:   spaceID,
			Key:  spaceID,
			Name: name,
		})
	}))
	defer server.Close()

	client := NewConfluenceClient(server.URL, "user@example.com", "token")

	// 複数goroutineから同時にGetSpaceInfoを呼び出す
	const goroutines = 20
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	spaceIDs := []string{"SP1", "SP2", "SP3"}

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		spaceID := spaceIDs[i%len(spaceIDs)]
		go func(sid string) {
			defer wg.Done()
			info, err := client.GetSpaceInfo(sid)
			if err != nil {
				errs <- fmt.Errorf("GetSpaceInfo(%s) error: %w", sid, err)
				return
			}
			expected := spaceNames[sid]
			if info.Name != expected {
				errs <- fmt.Errorf("GetSpaceInfo(%s) Name = %v, want %v", sid, info.Name, expected)
			}
		}(spaceID)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}

// TestGetSpaceName_WithHTTP はGetSpaceNameがGetPageInfoとGetSpaceInfoを連携して動作することをテストする
func TestGetSpaceName_WithHTTP(t *testing.T) {
	pageToSpace := map[string]string{
		"11111": "SP1",
	}
	spaceNames := map[string]string{
		"SP1": "マイスペース",
	}
	server := newTestConfluenceServer(t, pageToSpace, spaceNames)
	defer server.Close()

	client := NewConfluenceClient(server.URL, "user@example.com", "token")

	name, err := client.GetSpaceName("11111")
	if err != nil {
		t.Fatalf("GetSpaceName() error = %v", err)
	}
	if name != "マイスペース" {
		t.Errorf("GetSpaceName() = %v, want マイスペース", name)
	}
}

// TestGetSpaceName_PageNotFound はページが存在しない場合にエラーを返すことをテストする
func TestGetSpaceName_PageNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := NewConfluenceClient(server.URL, "user@example.com", "token")
	_, err := client.GetSpaceName("99999")
	if err == nil {
		t.Error("GetSpaceName() should return error when page not found")
	}
}

// TestSpaceCache_ConcurrentReadWrite はキャッシュへの同時Read/Writeがレースコンディションを起こさないことを確認する
func TestSpaceCache_ConcurrentReadWrite(t *testing.T) {
	// 1つのスペースIDに対してキャッシュ書き込みと読み込みを同時実行
	const spaceID = "SP_RACE"
	const spaceName = "レーステストスペース"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ConfluenceSpaceInfo{
			ID:   spaceID,
			Key:  spaceID,
			Name: spaceName,
		})
	}))
	defer server.Close()

	client := NewConfluenceClient(server.URL, "user@example.com", "token")

	// 多数のgoroutineから同時にアクセス（レースディテクターで検出）
	const goroutines = 50
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// すべてのgoroutineが同じspaceIDにアクセス（キャッシュのWrite/Read競合を誘発）
			_, _ = client.GetSpaceInfo(spaceID)
		}()
	}
	wg.Wait()
}
