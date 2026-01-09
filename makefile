.PHONY: build run test lint clean

# ビルド
build:
	go build -o migJira .

# 実行（サンプル: 課題取得）
run1:
	LOG_LEVEL=DEBUG go run . issue SCRUM-2 -c config.toml
	rm -rf hugo-jira/content/page/SCRUM/SCRUM-2.md
	cp -pr output/markdown/SCRUM/SCRUM-2.md hugo-jira/content/page/SCRUM/.

# 実行（サンプル: JQL）
run2:
	LOG_LEVEL=DEBUG go run . search -c config.toml
	rm -rf hugo-jira/content/page/*
	cp -pr output/markdown/* hugo-jira/content/page/.

# 実行（サンプル: エピック課題取得）
run3:
	LOG_LEVEL=DEBUG go run . issue SCRUM-5 -c config.toml
	rm -rf hugo-jira/content/page/SCRUM/SCRUM-5.md
	cp -pr output/markdown/SCRUM/SCRUM-5.md hugo-jira/content/page/SCRUM/.
# テスト
test:
	go test -v ./...

# リント
lint:
	golangci-lint run
