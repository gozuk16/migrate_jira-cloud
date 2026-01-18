.PHONY: build run test coverage lint clean

# ビルド
build:
	go build -o migJira .

# 実行（サンプル: 課題取得）
run1:
	LOG_LEVEL=DEBUG go run . issue SCRUM-2 -c config.toml
	cp -pf output/markdown/SCRUM/SCRUM-2.md hugo-jira/content/SCRUM/.
	cp -pf output/markdown/SCRUM/_index.md hugo-jira/content/SCRUM/.

# 実行（サンプル: エピック課題取得）
run2:
	LOG_LEVEL=DEBUG go run . issue SCRUM-5 -c config.toml
	cp -pf output/markdown/SCRUM/SCRUM-5.md hugo-jira/content/SCRUM/.
	cp -pf output/markdown/SCRUM/_index.md hugo-jira/content/SCRUM/.

# 実行（サンプル: JQL）
run3:
	LOG_LEVEL=DEBUG go run . search -c config.toml
	rm -rf hugo-jira/content/SCRUM
	cp -pr output/markdown/SCRUM hugo-jira/content/.

# 実行（サンプル: バグ課題取得）
run4:
	LOG_LEVEL=DEBUG go run . issue KT-3 -c config.toml
	cp -pf output/markdown/KT/KT-3.md hugo-jira/content/KT/.
	cp -pf output/markdown/KT/_index.md hugo-jira/content/KT/.

# 実行（サンプル: JQL）
run5:
	LOG_LEVEL=DEBUG go run . search "project = kanban-test" -c config.toml
	rm -rf hugo-jira/content/KT
	cp -pr output/markdown/KT hugo-jira/content/.

# テスト
test:
	go test -v ./...

# テストカバレッジ
coverage:
	go test -cover ./...
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# リント
lint:
	golangci-lint run
