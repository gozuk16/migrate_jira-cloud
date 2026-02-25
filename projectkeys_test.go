package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadProjectKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project_keys.json")

	keys := []string{"SCRUM", "KT", "OTHER"}

	if err := SaveProjectKeys(path, keys); err != nil {
		t.Fatalf("SaveProjectKeys() error = %v", err)
	}

	loaded, err := LoadProjectKeys(path)
	if err != nil {
		t.Fatalf("LoadProjectKeys() error = %v", err)
	}

	if len(loaded) != len(keys) {
		t.Fatalf("LoadProjectKeys() len = %d, want %d", len(loaded), len(keys))
	}
	for i, k := range keys {
		if loaded[i] != k {
			t.Errorf("LoadProjectKeys()[%d] = %q, want %q", i, loaded[i], k)
		}
	}
}

func TestLoadProjectKeys_FileNotFound(t *testing.T) {
	_, err := LoadProjectKeys("/nonexistent/project_keys.json")
	if err == nil {
		t.Error("LoadProjectKeys() expected error for missing file, got nil")
	}
}

func TestLoadProjectKeys_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project_keys.json")
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadProjectKeys(path)
	if err == nil {
		t.Error("LoadProjectKeys() expected error for invalid JSON, got nil")
	}
}
