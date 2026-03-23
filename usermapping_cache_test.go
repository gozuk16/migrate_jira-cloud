package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUserMapping_FileNotExist(t *testing.T) {
	mapping, err := LoadUserMapping("/nonexistent/path/user_mapping.json")
	if err != nil {
		t.Errorf("ファイルが存在しない場合はエラーなしで空mapを返すべきです: %v", err)
	}
	if len(mapping) != 0 {
		t.Errorf("空のmapを返すべきですが %d 件ありました", len(mapping))
	}
}

func TestSaveAndLoadUserMapping(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "user_mapping.json")

	original := UserMapping{
		"account1": "Alice",
		"account2": "Bob",
	}

	if err := SaveUserMapping(path, original); err != nil {
		t.Fatalf("SaveUserMapping失敗: %v", err)
	}

	loaded, err := LoadUserMapping(path)
	if err != nil {
		t.Fatalf("LoadUserMapping失敗: %v", err)
	}

	if len(loaded) != len(original) {
		t.Errorf("件数が一致しません: got %d, want %d", len(loaded), len(original))
	}
	for k, v := range original {
		if loaded[k] != v {
			t.Errorf("mapping[%s] = %q, want %q", k, loaded[k], v)
		}
	}
}

func TestMergeUserMapping(t *testing.T) {
	dst := UserMapping{
		"account1": "Alice",
		"account2": "OldBob",
	}
	src := UserMapping{
		"account2": "NewBob",
		"account3": "Carol",
	}

	MergeUserMapping(dst, src)

	if dst["account1"] != "Alice" {
		t.Errorf("account1 = %q, want %q", dst["account1"], "Alice")
	}
	if dst["account2"] != "NewBob" {
		t.Errorf("account2 = %q, want %q (srcで上書きされるべき)", dst["account2"], "NewBob")
	}
	if dst["account3"] != "Carol" {
		t.Errorf("account3 = %q, want %q (srcから追加されるべき)", dst["account3"], "Carol")
	}
}

func TestSaveUserMapping_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "user_mapping.json")

	mapping := UserMapping{"account1": "Alice"}
	if err := SaveUserMapping(path, mapping); err != nil {
		t.Fatalf("SaveUserMapping失敗: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("キャッシュファイルが作成されていません")
	}
}

func TestLoadUserMapping_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "user_mapping.json")

	// 空のmapを保存
	if err := SaveUserMapping(path, make(UserMapping)); err != nil {
		t.Fatalf("SaveUserMapping失敗: %v", err)
	}

	loaded, err := LoadUserMapping(path)
	if err != nil {
		t.Fatalf("LoadUserMapping失敗: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("空のmapを返すべきですが %d 件ありました", len(loaded))
	}
}
