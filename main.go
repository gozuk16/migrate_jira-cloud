package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andygrunwald/go-jira/v2/cloud"
	"github.com/urfave/cli/v3"
)

func main() {
	// ログレベルの設定（環境変数 LOG_LEVEL で制御）
	logLevel := slog.LevelInfo
	if level := os.Getenv("LOG_LEVEL"); level == "DEBUG" {
		logLevel = slog.LevelDebug
	}

	// ログ出力先の設定
	var logWriter io.Writer = os.Stderr

	// DEBUG レベルの場合はファイルにも出力
	if logLevel == slog.LevelDebug {
		logFile, err := os.OpenFile("debug.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			log.Printf("警告: ログファイルの作成に失敗しました: %v\n", err)
		} else {
			defer logFile.Close()
			// Stderrとファイルの両方に出力
			logWriter = io.MultiWriter(os.Stderr, logFile)
		}
	}

	// slog のセットアップ
	logger := slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// DEBUGモードの場合、ログファイルの場所を通知
	if logLevel == slog.LevelDebug {
		fmt.Println("デバッグモード: ログを debug.log に保存します")
	}

	app := &cli.Command{
		Name:  "migJira",
		Usage: "JIRA課題を取得してMarkdownで出力する",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Value:   "config.toml",
				Usage:   "設定ファイルのパス",
			},
		},
		Commands: []*cli.Command{
			{
				Name:    "issue",
				Aliases: []string{"i"},
				Usage:   "課題を取得して出力する(例: PROJ-123)",
				Action:  fetchIssue,
			},
			{
				Name:    "search",
				Aliases: []string{"s"},
				Usage:   "JQLで課題を検索して出力する。省略時は設定ファイルのdefault_jqlを使用",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:    "max",
						Aliases: []string{"m"},
						Value:   100,
						Usage:   "最大取得件数",
					},
				},
				Action: searchIssues,
			},
			{
				Name:    "convert",
				Aliases: []string{"conv"},
				Usage:   "JSONファイルからMarkdownを生成する（APIアクセス不要）",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "input",
						Aliases:  []string{"i"},
						Usage:    "入力JSONファイルまたはディレクトリのパス",
						Required: true,
					},
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "出力先ディレクトリ（省略時は設定ファイルのmarkdown_dir）",
					},
					&cli.IntFlag{
						Name:    "workers",
						Aliases: []string{"w"},
						Value:   1,
						Usage:   "並行実行するworker数（省略時はconfig.tomlのconvert.workersを使用、デフォルト: 4）",
					},
				},
				Action: convertFromJSON,
			},
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

// fetchIssue は単一の課題を取得して出力する
func fetchIssue(ctx context.Context, cmd *cli.Command) error {
	configPath := cmd.String("config")

	// 位置引数からチケット番号を取得
	if cmd.Args().Len() == 0 {
		return fmt.Errorf("チケット番号を指定してください（例: PROJ-123）")
	}
	issueKey := cmd.Args().First()

	// 設定ファイルの読み込み
	config, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("設定ファイルの読み込みに失敗しました: %w", err)
	}

	// JIRAクライアントの作成
	jiraClient, err := NewJIRAClient(&config.JIRA)
	if err != nil {
		return fmt.Errorf("JIRAクライアントの作成に失敗しました: %w", err)
	}

	// フィールドリストを取得してキャッシュを作成
	fields, err := jiraClient.GetFieldList()
	if err != nil {
		fmt.Printf("警告: フィールドリストの取得に失敗しました: %v\n", err)
		fields = nil
	}
	fieldNameCache := BuildFieldNameCache(fields)

	fmt.Printf("課題 %s を取得中...\n", issueKey)

	// 課題の取得
	issue, err := jiraClient.GetIssue(issueKey)
	if err != nil {
		return fmt.Errorf("課題の取得に失敗しました: %w", err)
	}
	slog.Debug("JIRA課題データ (説明)",
		"issueKey", issue.Key,
		"json", string(issue.Fields.Description))

	fmt.Printf("課題を取得しました: %s - %s\n---\n", issue.Key, issue.Fields.Summary)

	// 開発情報の詳細を取得（設定で有効な場合のみ）
	var devStatus *DevStatusDetail
	var devStatusRawJSON []byte
	if config.Development.Enabled && issue.ID != "" {
		apiType := config.Development.APIType
		if apiType == "" {
			apiType = "rest" // デフォルトはREST API
		}

		if apiType == "graphql" {
			// GraphQL APIを使用
			devStatus, devStatusRawJSON, err = jiraClient.GetDevStatusGraphQL(issue.ID)
			if err != nil {
				slog.Debug("GraphQL API 開発情報取得失敗",
					"issueKey", issueKey,
					"issueID", issue.ID,
					"error", err)
				slog.Warn("開発情報の詳細取得に失敗（スキップして継続）",
					"issueKey", issueKey,
					"error", err)
				devStatus = nil
				devStatusRawJSON = nil
			}
		} else {
			// REST APIを使用
			appType := config.Development.ApplicationType
			if appType == "" {
				appType = "bitbucket" // デフォルト
			}

			devStatus, devStatusRawJSON, err = jiraClient.GetDevStatusDetails(issue.ID, appType, "pullrequest")
			if err != nil {
				slog.Debug("REST API 開発情報取得失敗",
					"issueKey", issueKey,
					"issueID", issue.ID,
					"appType", appType,
					"error", err)
				slog.Warn("開発情報の詳細取得に失敗（スキップして継続）",
					"issueKey", issueKey,
					"error", err)
				devStatus = nil
				devStatusRawJSON = nil
			}
		}
	}

	// 添付ファイルのダウンロード（課題ディレクトリに直接保存）
	issueDir := filepath.Join(config.Output.MarkdownDir, issue.Fields.Project.Key, issue.Key)
	downloader := NewDownloader(config.JIRA.Email, config.JIRA.APIToken)
	attachmentFiles, err := downloader.DownloadAttachments(issue, issueDir)
	if err != nil {
		return fmt.Errorf("添付ファイルのダウンロードに失敗しました: %w", err)
	}

	if len(attachmentFiles) > 0 {
		fmt.Printf("添付ファイルを %d 件ダウンロードしました\n", len(attachmentFiles))
	}

	// ユーザーマッピングの構築
	userMapping := make(UserMapping)
	BuildUserMappingFromIssue(issue, userMapping)

	// 親課題情報の取得
	var parentInfo *ParentIssueInfo
	if issue.Fields.Parent != nil && issue.Fields.Parent.Key != "" {
		parentIssue, err := jiraClient.GetIssue(issue.Fields.Parent.Key)
		if err != nil {
			fmt.Printf("警告: 親課題 %s の取得に失敗しました（スキップして継続）: %v\n", issue.Fields.Parent.Key, err)
		} else {
			parentInfo = &ParentIssueInfo{
				Key:  parentIssue.Key,
				Type: parentIssue.Fields.Type.Name,
			}
		}
	}

	// 子課題情報の取得（すべての課題に対して実行）
	var childIssues []ChildIssueInfo
	childKeys, err := jiraClient.GetChildIssues(issue.Key, 100)
	if err != nil {
		fmt.Printf("警告: 子課題の取得に失敗しました（課題: %s）: %v\n", issue.Key, err)
	} else if len(childKeys) > 0 {
		childIssues = make([]ChildIssueInfo, 0, len(childKeys))
		for _, childKey := range childKeys {
			childIssue, err := jiraClient.GetIssue(childKey)
			if err != nil {
				fmt.Printf("警告: 子課題 %s の取得に失敗しました: %v\n", childKey, err)
				continue
			}
			// Sub-task課題タイプは除外
			issueType := childIssue.Fields.Type.Name
			if issueType == "Sub-task" || issueType == "Subtask" || issueType == "サブタスク" {
				continue
			}

			// Rankフィールドを取得
			rankValue := ""
			if rank, exists := childIssue.Fields.Unknowns[config.Display.RankFieldId]; exists {
				if rankStr, ok := rank.(string); ok {
					rankValue = rankStr
				}
			}
			childIssues = append(childIssues, ChildIssueInfo{
				Key:     childIssue.Key,
				Summary: childIssue.Fields.Summary,
				Status:  childIssue.Fields.Status.Name,
				Type:    childIssue.Fields.Type.Name,
				Rank:    rankValue,
			})
		}
		// 子課題をRankフィールドでソート
		if len(childIssues) > 0 {
			sort.Slice(childIssues, func(i, j int) bool {
				// Rankが空の場合は後ろに配置
				if childIssues[i].Rank == "" && childIssues[j].Rank != "" {
					return false
				}
				if childIssues[i].Rank != "" && childIssues[j].Rank == "" {
					return true
				}
				// 両方とも空でない場合は辞書順でソート
				return childIssues[i].Rank < childIssues[j].Rank
			})
		}
	}

	// リモートリンク（Confluenceコンテンツなど）の取得
	var remoteLinks []cloud.RemoteLink
	remoteLinksResult, err := jiraClient.GetRemoteLinks(issueKey)
	if err != nil {
		slog.Warn("リモートリンクの取得に失敗（スキップして継続）",
			"issueKey", issueKey,
			"error", err)
		remoteLinks = []cloud.RemoteLink{}
	} else {
		remoteLinks = remoteLinksResult
	}

	// Confluenceスペース名を事前解決してJSONに保存
	var confluenceSpaces map[string]string
	if config.JIRA.URL != "" && config.JIRA.Email != "" && config.JIRA.APIToken != "" {
		confluenceClient := NewConfluenceClient(
			config.JIRA.URL,
			config.JIRA.Email,
			config.JIRA.APIToken,
		)
		confluenceSpaces = resolveConfluenceSpaces(remoteLinks, confluenceClient)
	}

	// Markdown出力
	mdWriter := NewMarkdownWriter(config.Output.MarkdownDir, userMapping, config)

	// 事前解決済みConfluenceスペース名を設定
	if len(confluenceSpaces) > 0 {
		mdWriter.SetConfluenceSpaces(confluenceSpaces)
	}

	// プロジェクトキー一覧を取得してキャッシュし、MarkdownWriterに設定
	if projectKeys, err := jiraClient.GetAllProjects(); err != nil {
		fmt.Printf("警告: プロジェクトキー一覧の取得に失敗しました: %v\n", err)
	} else {
		cachePath := config.ProjectKeyCachePath()
		if err := SaveProjectKeys(cachePath, projectKeys); err != nil {
			fmt.Printf("警告: プロジェクトキーキャッシュの保存に失敗しました: %v\n", err)
		}
		mdWriter.SetProjectKeys(projectKeys)
	}

	// プロジェクトの_index.md生成
	// issueコマンドではチケット一覧なしで_index.md生成
	projectKey := issue.Fields.Project.Key
	project, err := jiraClient.GetProject(projectKey)
	if err != nil {
		slog.Warn("警告: プロジェクト %s の取得に失敗しました",
			"project", projectKey,
			"error", err)
	} else {
		if err := mdWriter.WriteProjectIndex(project); err != nil {
			slog.Warn("警告: _index.md の生成に失敗しました",
				"project", projectKey,
				"error", err)
		}
	}

	// JSON保存（設定されている場合）
	if config.Output.JSONDir != "" {
		jsonSaver := NewJSONSaver(config.Output.JSONDir)
		issueData := &IssueData{
			Issue:            issue,
			DevStatus:        devStatus,
			DevStatusRawJSON: devStatusRawJSON,
			ParentInfo:       parentInfo,
			ChildIssues:      childIssues,
			RemoteLinks:      remoteLinks,
			Fields:           fields,
			ConfluenceSpaces: confluenceSpaces,
			SavedAt:          time.Now().Format(time.RFC3339),
		}
		jsonPath, err := jsonSaver.SaveIssue(issueData)
		if err != nil {
			slog.Warn("JSON保存エラー", "error", err)
		} else {
			fmt.Printf("JSONファイルを出力しました: %s\n", jsonPath)
		}
	}

	if err := mdWriter.WriteIssue(issue, attachmentFiles, fieldNameCache, devStatus, parentInfo, childIssues, remoteLinks); err != nil {
		return fmt.Errorf("Markdownファイルの出力に失敗しました: %w", err)
	}

	fmt.Printf("Markdownファイルを出力しました: %s/%s/%s/index.md\n", config.Output.MarkdownDir, projectKey, issue.Key)

	return nil
}

// searchIssues はJQLで課題を検索して出力する
func searchIssues(ctx context.Context, cmd *cli.Command) error {
	configPath := cmd.String("config")
	maxResults := cmd.Int("max")

	// 位置引数からJQLを取得（省略可能）
	var jql string
	if cmd.Args().Len() > 0 {
		jql = cmd.Args().First()
	}

	// 設定ファイルの読み込み
	config, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("設定ファイルの読み込みに失敗しました: %w", err)
	}

	// JQLが指定されていない場合は設定ファイルのデフォルト値を使用
	if jql == "" {
		jql = config.Search.DefaultJQL
		if jql == "" {
			return fmt.Errorf("JQLクエリが指定されていません。引数で指定するか、設定ファイルにdefault_jqlを設定してください")
		}
		fmt.Printf("設定ファイルのデフォルトJQLを使用: %s\n", jql)
	}

	// JIRAクライアントの作成
	jiraClient, err := NewJIRAClient(&config.JIRA)
	if err != nil {
		return fmt.Errorf("JIRAクライアントの作成に失敗しました: %w", err)
	}

	// フィールドリストを取得してキャッシュを作成
	fields, err := jiraClient.GetFieldList()
	if err != nil {
		fmt.Printf("警告: フィールドリストの取得に失敗しました: %v\n", err)
		fields = nil
	}
	fieldNameCache := BuildFieldNameCache(fields)

	fmt.Printf("JQLで検索中: %s\n", jql)

	// 課題キーの検索
	issueKeys, err := jiraClient.GetIssuesByJQL(jql, maxResults)
	if err != nil {
		return fmt.Errorf("課題の検索に失敗しました: %w", err)
	}

	fmt.Printf("%d 件の課題が見つかりました\n", len(issueKeys))

	// ユーザーマッピングの初期化
	userMapping := make(UserMapping)

	// 各課題を処理
	downloader := NewDownloader(config.JIRA.Email, config.JIRA.APIToken)
	mdWriter := NewMarkdownWriter(config.Output.MarkdownDir, userMapping, config)

	// Confluenceクライアントを準備（スペース名の事前解決用）
	var confluenceClient *ConfluenceClient
	if config.JIRA.URL != "" && config.JIRA.Email != "" && config.JIRA.APIToken != "" {
		confluenceClient = NewConfluenceClient(
			config.JIRA.URL,
			config.JIRA.Email,
			config.JIRA.APIToken,
		)
	}

	// プロジェクトキー一覧を取得してキャッシュし、MarkdownWriterに設定
	if projectKeys, err := jiraClient.GetAllProjects(); err != nil {
		fmt.Printf("警告: プロジェクトキー一覧の取得に失敗しました: %v\n", err)
	} else {
		cachePath := config.ProjectKeyCachePath()
		if err := SaveProjectKeys(cachePath, projectKeys); err != nil {
			fmt.Printf("警告: プロジェクトキーキャッシュの保存に失敗しました: %v\n", err)
		}
		mdWriter.SetProjectKeys(projectKeys)
	}

	// 親課題情報のキャッシュ
	parentInfoCache := make(map[string]*ParentIssueInfo)

	// 子課題キャッシュ
	childIssuesCache := make(map[string][]ChildIssueInfo)

	// プロジェクトごとの_index.md生成済みフラグ（重複防止）
	generatedProjects := make(map[string]bool)

	for i, issueKey := range issueKeys {
		fmt.Printf("[%d/%d] 処理中: %s\n", i+1, len(issueKeys), issueKey)

		// 課題の詳細情報を取得（descriptionを含む完全な情報）
		issue, err := jiraClient.GetIssue(issueKey)
		if err != nil {
			fmt.Printf("警告: 課題 %s の取得に失敗しました: %v\n", issueKey, err)
			continue
		}

		fmt.Printf("  取得完了: %s - %s\n", issue.Key, issue.Fields.Summary)

		// プロジェクトの_index.md生成（初回のみ）
		projectKey := issue.Fields.Project.Key
		if !generatedProjects[projectKey] {
			project, err := jiraClient.GetProject(projectKey)
			if err != nil {
				slog.Warn("プロジェクト取得に失敗",
					"project", projectKey,
					"error", err)
			} else {
				if err := mdWriter.WriteProjectIndex(project); err != nil {
					slog.Warn("_index.md生成に失敗",
						"project", projectKey,
						"error", err)
				} else {
					fmt.Printf("_index.mdを生成しました: %s\n", projectKey)
				}
			}
			generatedProjects[projectKey] = true
		}

		// ユーザーマッピングに追加
		BuildUserMappingFromIssue(issue, userMapping)

		// デバッグ用: 取得した課題データをJSON形式でログ出力
		if issueJSON, err := json.MarshalIndent(issue, "", "  "); err == nil {
			slog.Debug("JIRA課題データ (JSON)",
				"issueKey", issue.Key,
				"json", string(issueJSON))
		} else {
			slog.Warn("JSON変換に失敗しました", "issueKey", issue.Key, "error", err)
		}

		// 添付ファイルのダウンロード（課題ディレクトリに直接保存）
		issueDir := filepath.Join(config.Output.MarkdownDir, projectKey, issue.Key)
		attachmentFiles, err := downloader.DownloadAttachments(issue, issueDir)
		if err != nil {
			fmt.Printf("  警告: 添付ファイルのダウンロードに失敗しました: %v\n", err)
			attachmentFiles = []string{}
		}

		// 開発情報の詳細を取得（設定で有効な場合のみ）
		var devStatus *DevStatusDetail
		var devStatusRawJSON []byte
		if config.Development.Enabled && issue.ID != "" {
			apiType := config.Development.APIType
			if apiType == "" {
				apiType = "rest" // デフォルトはREST API
			}

			if apiType == "graphql" {
				// GraphQL APIを使用
				devStatus, devStatusRawJSON, err = jiraClient.GetDevStatusGraphQL(issue.ID)
				if err != nil {
					slog.Debug("GraphQL API 開発情報取得失敗",
						"issueKey", issueKey,
						"issueID", issue.ID,
						"error", err)
					slog.Warn("開発情報の詳細取得に失敗（スキップして継続）",
						"issueKey", issueKey,
						"error", err)
					devStatus = nil
					devStatusRawJSON = nil
				}
			} else {
				// REST APIを使用
				appType := config.Development.ApplicationType
				if appType == "" {
					appType = "bitbucket" // デフォルト
				}

				devStatus, devStatusRawJSON, err = jiraClient.GetDevStatusDetails(issue.ID, appType, "pullrequest")
				if err != nil {
					slog.Debug("REST API 開発情報取得失敗",
						"issueKey", issueKey,
						"issueID", issue.ID,
						"appType", appType,
						"error", err)
					slog.Warn("開発情報の詳細取得に失敗（スキップして継続）",
						"issueKey", issueKey,
						"error", err)
					devStatus = nil
					devStatusRawJSON = nil
				}
			}
		}

		// 親課題情報の取得（キャッシュを使用）
		var parentInfo *ParentIssueInfo
		if issue.Fields.Parent != nil && issue.Fields.Parent.Key != "" {
			parentKey := issue.Fields.Parent.Key
			if cachedInfo, exists := parentInfoCache[parentKey]; exists {
				parentInfo = cachedInfo
			} else {
				parentIssue, err := jiraClient.GetIssue(parentKey)
				if err != nil {
					fmt.Printf("  警告: 親課題 %s の取得に失敗しました: %v\n", parentKey, err)
				} else {
					parentInfo = &ParentIssueInfo{
						Key:  parentIssue.Key,
						Type: parentIssue.Fields.Type.Name,
					}
					parentInfoCache[parentKey] = parentInfo
				}
			}
		}

		// 子課題の取得（キャッシュ使用、すべての課題に対して実行）
		var childIssues []ChildIssueInfo
		if cachedChildren, exists := childIssuesCache[issue.Key]; exists {
			childIssues = cachedChildren
		} else {
			childKeys, err := jiraClient.GetChildIssues(issue.Key, 100)
			if err != nil {
				fmt.Printf("  警告: 子課題の取得に失敗しました（課題: %s）: %v\n", issue.Key, err)
			} else if len(childKeys) > 0 {
				childIssues = make([]ChildIssueInfo, 0, len(childKeys))
				for _, childKey := range childKeys {
					childIssue, err := jiraClient.GetIssue(childKey)
					if err != nil {
						fmt.Printf("  警告: 子課題 %s の取得に失敗しました: %v\n", childKey, err)
						continue
					}
					// Sub-task課題タイプは除外
					issueType := childIssue.Fields.Type.Name
					if issueType == "Sub-task" || issueType == "Subtask" || issueType == "サブタスク" {
						continue
					}

					// Rankフィールドを取得
					rankValue := ""
					if rank, exists := childIssue.Fields.Unknowns[config.Display.RankFieldId]; exists {
						if rankStr, ok := rank.(string); ok {
							rankValue = rankStr
						}
					}
					childIssues = append(childIssues, ChildIssueInfo{
						Key:     childIssue.Key,
						Summary: childIssue.Fields.Summary,
						Status:  childIssue.Fields.Status.Name,
						Type:    childIssue.Fields.Type.Name,
						Rank:    rankValue,
					})
				}
				// 子課題をRankフィールドでソート
				if len(childIssues) > 0 {
					sort.Slice(childIssues, func(i, j int) bool {
						// Rankが空の場合は後ろに配置
						if childIssues[i].Rank == "" && childIssues[j].Rank != "" {
							return false
						}
						if childIssues[i].Rank != "" && childIssues[j].Rank == "" {
							return true
						}
						// 両方とも空でない場合は辞書順でソート
						return childIssues[i].Rank < childIssues[j].Rank
					})
				}
				childIssuesCache[issue.Key] = childIssues
			}
		}

		// リモートリンク（Confluenceコンテンツなど）の取得
		var remoteLinks []cloud.RemoteLink
		remoteLinksResult, err := jiraClient.GetRemoteLinks(issue.Key)
		if err != nil {
			slog.Debug("リモートリンク取得エラー",
				"issueKey", issue.Key,
				"error", err)
			remoteLinks = []cloud.RemoteLink{}
		} else {
			remoteLinks = remoteLinksResult
		}

		// Confluenceスペース名を事前解決
		confluenceSpaces := resolveConfluenceSpaces(remoteLinks, confluenceClient)
		if len(confluenceSpaces) > 0 {
			mdWriter.SetConfluenceSpaces(confluenceSpaces)
		}

		// JSON保存（設定されている場合）
		if config.Output.JSONDir != "" {
			jsonSaver := NewJSONSaver(config.Output.JSONDir)
			issueData := &IssueData{
				Issue:            issue,
				DevStatus:        devStatus,
				DevStatusRawJSON: devStatusRawJSON,
				ParentInfo:       parentInfo,
				ChildIssues:      childIssues,
				RemoteLinks:      remoteLinks,
				Fields:           fields,
				ConfluenceSpaces: confluenceSpaces,
				SavedAt:          time.Now().Format(time.RFC3339),
			}
			jsonPath, err := jsonSaver.SaveIssue(issueData)
			if err != nil {
				slog.Warn("JSON保存エラー", "issueKey", issue.Key, "error", err)
			} else {
				fmt.Printf("  JSON出力: %s\n", jsonPath)
			}
		}

		// Markdown出力
		if err := mdWriter.WriteIssue(issue, attachmentFiles, fieldNameCache, devStatus, parentInfo, childIssues, remoteLinks); err != nil {
			fmt.Printf("  警告: Markdownファイルの出力に失敗しました: %v\n", err)
		}
	}

	fmt.Printf("\n処理が完了しました\n")
	fmt.Printf("- Markdown: %s\n", config.Output.MarkdownDir)
	if config.Output.JSONDir != "" {
		fmt.Printf("- JSON: %s\n", config.Output.JSONDir)
	}

	return nil
}

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

			// ユーザーマッピング構築
			userMapping := make(UserMapping)
			BuildUserMappingFromIssue(data.Issue, userMapping)

			// Markdown生成
			mdWriter := NewMarkdownWriter(outputDir, userMapping, config)

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

			// 旧attachmentsディレクトリから課題ディレクトリへ添付ファイルをコピー
			var attachmentFiles []string
			if data.Issue.Fields.Attachments != nil {
				for _, att := range data.Issue.Fields.Attachments {
					safeFilename := sanitizeFilenameForConvert(att.Filename)
					attachmentFiles = append(attachmentFiles, safeFilename)

					newPath := filepath.Join(issueDir, safeFilename)

					// 旧attachmentsディレクトリが設定されている場合、ファイルをコピー
					if config.Output.AttachmentsDir != "" {
						oldPath := filepath.Join(config.Output.AttachmentsDir, fmt.Sprintf("%s_%s", data.Issue.Key, safeFilename))
						if err := copyFileIfExists(oldPath, newPath); err != nil {
							fmt.Printf("  警告: 添付ファイルのコピーに失敗しました (%s): %v\n", att.Filename, err)
						}
					}

					// .mdファイルの場合はフロントマターのtagsからバッククオートを除去する
					if strings.HasSuffix(strings.ToLower(safeFilename), ".md") {
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

	fmt.Printf("\n処理が完了しました\n")
	fmt.Printf("- 成功: %d 件\n", atomic.LoadInt64(&successCount))
	fmt.Printf("- 失敗: %d 件\n", int64(total)-atomic.LoadInt64(&successCount))
	fmt.Printf("- 出力先: %s\n", outputDir)

	return nil
}

// resolveConfluenceSpaces はリモートリンクからConfluenceページIDとスペース名のマップを生成する
func resolveConfluenceSpaces(remoteLinks []cloud.RemoteLink, confluenceClient *ConfluenceClient) map[string]string {
	if confluenceClient == nil || len(remoteLinks) == 0 {
		return nil
	}

	spaces := make(map[string]string)
	for _, link := range remoteLinks {
		if link.Application == nil || !strings.Contains(strings.ToLower(link.Application.Type), "confluence") {
			continue
		}
		if link.GlobalID == "" {
			continue
		}
		pageID, err := ExtractPageIDFromGlobalID(link.GlobalID)
		if err != nil {
			continue
		}
		spaceName, err := confluenceClient.GetSpaceName(pageID)
		if err != nil {
			slog.Debug("Confluenceスペース名取得失敗", "pageID", pageID, "error", err)
			continue
		}
		spaces[pageID] = spaceName
	}

	if len(spaces) == 0 {
		return nil
	}
	return spaces
}

// sanitizeFilenameForConvert はファイル名を安全な形式にサニタイズする（Downloader.sanitizeFilenameと同じロジック）
func sanitizeFilenameForConvert(filename string) string {
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
