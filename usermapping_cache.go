package main

import (
	"encoding/json"
	"os"
)

// LoadUserMapping はキャッシュファイルからUserMappingを読み込む
// ファイルが存在しない場合は空のUserMappingを返す
func LoadUserMapping(path string) (UserMapping, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(UserMapping), nil
		}
		return nil, err
	}

	var mapping UserMapping
	if err := json.Unmarshal(data, &mapping); err != nil {
		return nil, err
	}

	return mapping, nil
}

// SaveUserMapping はUserMappingをJSONファイルに保存する
func SaveUserMapping(path string, mapping UserMapping) error {
	data, err := json.MarshalIndent(mapping, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// MergeUserMapping は src の内容を dst にマージする（srcの値でdstを上書き）
func MergeUserMapping(dst, src UserMapping) {
	for k, v := range src {
		dst[k] = v
	}
}
