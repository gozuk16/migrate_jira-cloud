package main

import (
	"encoding/json"
	"os"
	"time"
)

// ProjectKeyCache はプロジェクトキーのキャッシュデータ
type ProjectKeyCache struct {
	Keys      []string  `json:"keys"`
	FetchedAt time.Time `json:"fetchedAt"`
}

// LoadProjectKeys はキャッシュファイルからプロジェクトキー一覧を読み込む
func LoadProjectKeys(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cache ProjectKeyCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	return cache.Keys, nil
}

// SaveProjectKeys はプロジェクトキー一覧をキャッシュファイルに保存する
func SaveProjectKeys(path string, keys []string) error {
	cache := ProjectKeyCache{
		Keys:      keys,
		FetchedAt: time.Now(),
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
