package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/urfave/cli/v3"
	"golang.org/x/text/unicode/norm"
)

// convertFromJSON はJSONファイルからMarkdownを生成する
func convertFromJSON(ctx context.Context, cmd *cli.Command) error {
	inputPath := cmd.String("input")
	outputDir := cmd.String("output")
	configPath := cmd.Root().String("config")

	// 設定読み込み（Markdown出力設定用）
	config, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("設定ファイルの読み込みに失敗しました: %w", err)
	}

	if outputDir == "" {
		outputDir = config.Output.MarkdownDir
	}

	jsonSaver := NewJSONSaver("")

	// 入力パスがファイルかディレクトリか判定
	fileInfo, err := os.Stat(inputPath)
	if err != nil {
		return fmt.Errorf("入力パスエラー: %w", err)
	}

	var jsonFiles []string
	if fileInfo.IsDir() {
		// ディレクトリの場合、再帰的にJSONファイルを収集
		err := filepath.Walk(inputPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && filepath.Ext(path) == ".json" {
				jsonFiles = append(jsonFiles, path)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("ディレクトリ走査エラー: %w", err)
		}
	} else {
		jsonFiles = []string{inputPath}
	}

	if len(jsonFiles) == 0 {
		return fmt.Errorf("JSONファイルが見つかりませんでした: %s", inputPath)
	}

	fmt.Printf("%d 件のJSONファイルを処理します\n", len(jsonFiles))

	// プロジェクトキーキャッシュを読み込む（APIアクセスなし）
	var cachedProjectKeys []string
	if keys, err := LoadProjectKeys(config.ProjectKeyCachePath()); err == nil {
		cachedProjectKeys = keys
		fmt.Printf("プロジェクトキーキャッシュを読み込みました（%d件）\n", len(keys))
	}

	// UserMappingキャッシュを読み込む（goroutine間で共有）
	sharedUserMapping, err := LoadUserMapping(config.UserMappingCachePath())
	if err != nil {
		fmt.Printf("警告: UserMappingキャッシュの読み込みに失敗しました: %v\n", err)
		sharedUserMapping = make(UserMapping)
	} else if len(sharedUserMapping) > 0 {
		fmt.Printf("UserMappingキャッシュを読み込みました（%d件）\n", len(sharedUserMapping))
	}
	var userMappingMu sync.Mutex

	// workers数の決定: CLIフラグ明示指定 > config.toml > デフォルト(4)
	workers := config.Convert.Workers
	if cmd.IsSet("workers") {
		workers = int(cmd.Int("workers"))
	}
	if workers < 1 {
		workers = 1
	}
	if workers > 1 {
		fmt.Printf("並行実行: %d workers\n", workers)
	}

	// 各JSONファイルを処理
	var successCount int64
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	total := len(jsonFiles)

	for i, jsonFile := range jsonFiles {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, file string) {
			defer wg.Done()
			defer func() { <-sem }()

			fmt.Printf("[%d/%d] 変換中: %s\n", idx+1, total, file)

			data, err := jsonSaver.LoadIssue(file)
			if err != nil {
				fmt.Printf("  エラー: JSON読み込みに失敗しました: %v\n", err)
				return
			}

			// フィールド名キャッシュを構築
			fieldNameCache := BuildFieldNameCache(data.Fields)

			// ユーザーマッピング構築（課題データ + キャッシュをマージ）
			issueMapping := make(UserMapping)
			BuildUserMappingFromIssue(data.Issue, issueMapping)
			userMappingMu.Lock()
			MergeUserMapping(sharedUserMapping, issueMapping)
			mergedMapping := make(UserMapping)
			MergeUserMapping(mergedMapping, sharedUserMapping)
			userMappingMu.Unlock()

			// Markdown生成
			mdWriter := NewMarkdownWriter(outputDir, mergedMapping, config)

			// 事前解決済みConfluenceスペース名を設定（APIアクセスなし）
			if len(data.ConfluenceSpaces) > 0 {
				mdWriter.SetConfluenceSpaces(data.ConfluenceSpaces)
			}

			// プロジェクトキーキャッシュがあれば設定
			if len(cachedProjectKeys) > 0 {
				mdWriter.SetProjectKeys(cachedProjectKeys)
			}

			// 課題ディレクトリの作成
			projectKey := data.Issue.Fields.Project.Key
			issueDir := filepath.Join(outputDir, projectKey, data.Issue.Key)
			if err := os.MkdirAll(issueDir, 0755); err != nil {
				fmt.Printf("  エラー: 課題ディレクトリの作成に失敗しました: %v\n", err)
				return
			}

			// 添付ファイルの処理
			attachDir := attachmentTargetDir(config, projectKey, data.Issue.Key)
			attachDirCreated := false
			var attachmentFiles []string
			if data.Issue.Fields.Attachments != nil {
				for _, att := range data.Issue.Fields.Attachments {
					safeFilename := sanitizeFilenameForConvert(att.Filename)
					attachmentFiles = append(attachmentFiles, safeFilename)

					// 添付ファイルがある場合のみディレクトリを作成
					if !attachDirCreated {
						if err := os.MkdirAll(attachDir, 0755); err != nil {
							fmt.Printf("  エラー: 添付ファイルディレクトリの作成に失敗しました: %v\n", err)
							return
						}
						attachDirCreated = true
					}

					newPath := filepath.Join(attachDir, safeFilename)

					// 旧attachmentsディレクトリが設定されている場合、ファイルをコピー
					if config.Output.AttachmentsDir != "" {
						oldPath := filepath.Join(config.Output.AttachmentsDir, fmt.Sprintf("%s_%s", data.Issue.Key, safeFilename))
						if err := copyFileIfExists(oldPath, newPath); err != nil {
							fmt.Printf("  警告: 添付ファイルのコピーに失敗しました (%s): %v\n", att.Filename, err)
						}
					}

					// static_dir設定時: markdown側（issueDir）に既存の添付ファイルがあればstaticへ移動
					if config.Output.StaticDir != "" {
						existingPath := filepath.Join(issueDir, safeFilename)
						if _, err := os.Stat(existingPath); err == nil {
							if err := os.Rename(existingPath, newPath); err != nil {
								fmt.Printf("  警告: 添付ファイルの移動に失敗しました (%s): %v\n", safeFilename, err)
							} else {
								slog.Info("添付ファイルをstaticに移動しました", "file", safeFilename, "from", existingPath, "to", newPath)
							}
						}
					}

					// .mdファイルの場合はCP932→UTF-8変換とフロントマター整理を行う
					if strings.HasSuffix(strings.ToLower(safeFilename), ".md") {
						if err := convertCP932ToUTF8(newPath); err != nil {
							slog.Warn("エンコーディング変換に失敗しました", "file", safeFilename, "error", err)
						}
						if err := sanitizeMarkdownFrontMatter(newPath); err != nil {
							slog.Warn("フロントマターの整理に失敗しました", "file", safeFilename, "error", err)
						}
					}
				}
			}

			if err := mdWriter.WriteIssue(data.Issue, attachmentFiles, fieldNameCache, data.DevStatus, data.ParentInfo, data.ChildIssues, data.RemoteLinks); err != nil {
				fmt.Printf("  エラー: Markdown生成に失敗しました: %v\n", err)
				return
			}

			fmt.Printf("  完了: %s\n", data.Issue.Key)
			atomic.AddInt64(&successCount, 1)
		}(i, jsonFile)
	}
	wg.Wait()

	// UserMappingキャッシュを保存（全goroutine完了後）
	userMappingMu.Lock()
	if err := SaveUserMapping(config.UserMappingCachePath(), sharedUserMapping); err != nil {
		fmt.Printf("警告: UserMappingキャッシュの保存に失敗しました: %v\n", err)
	}
	userMappingMu.Unlock()

	fmt.Printf("\n処理が完了しました\n")
	fmt.Printf("- 成功: %d 件\n", atomic.LoadInt64(&successCount))
	fmt.Printf("- 失敗: %d 件\n", int64(total)-atomic.LoadInt64(&successCount))
	fmt.Printf("- 出力先: %s\n", outputDir)

	return nil
}

// sanitizeFilenameForConvert はファイル名を安全な形式にサニタイズする（Downloader.sanitizeFilenameと同じロジック）
func sanitizeFilenameForConvert(filename string) string {
	// Unicode正規化（NFC）: macOSのHFS+/APFSがNFD形式に変換する問題を防ぎ、
	// Linuxホスト環境でのファイル名とMarkdownリンクの一致を保証する
	filename = norm.NFC.String(filename)
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		"..", "_",
		":", "_",
	)
	return replacer.Replace(filename)
}

// copyFileIfExists はファイルをコピーする（コピー先が存在する場合は上書き）
func copyFileIfExists(src, dst string) error {
	// コピー元が存在しない場合はスキップ
	srcFile, err := os.Open(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
