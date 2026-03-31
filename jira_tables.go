package main

import (
	"fmt"
	"regexp"
	"strings"
)

// extractJIRATables はJIRAテーブルを抽出してプレースホルダーに置き換える
// セル内改行を保持したままテーブル全体を抽出する
func (mw *MarkdownWriter) extractJIRATables(text string) (string, []string) {
	lines := strings.Split(text, "\n")
	tables := []string{}
	result := []string{}

	i := 0
	for i < len(lines) {
		line := lines[i]

		// ヘッダー行を検出（||で始まる行）
		if strings.HasPrefix(line, "||") {
			// セル内改行対応: ||で終わるまで次の行と結合
			completedHeader := line
			i++
			for !strings.HasSuffix(completedHeader, "||") && i < len(lines) {
				completedHeader += "\n" + lines[i]
				i++
			}

			if !strings.HasSuffix(completedHeader, "||") {
				// ||で終わらない場合はテーブルとして扱わない
				result = append(result, completedHeader)
				continue
			}

			tableLines := []string{completedHeader}

			// データ行を収集
			for i < len(lines) {
				dataLine := lines[i]

				// 次のテーブルヘッダーをチェック（セル内改行対応）
				if strings.HasPrefix(dataLine, "||") {
					// 次のテーブル開始 → 現在のテーブル終了
					break
				} else if strings.HasPrefix(dataLine, "|") && !strings.HasPrefix(dataLine, "||") {
					// データ行の開始
					completeLine := dataLine
					i++

					// |で終わるまで次の行と結合（セル内改行対応）
					for !strings.HasSuffix(completeLine, "|") && i < len(lines) {
						nextLine := lines[i]
						// 次のテーブルヘッダーが来たら結合を中止
						if strings.HasPrefix(nextLine, "||") {
							break
						}
						completeLine += "\n" + nextLine
						i++
					}

					if strings.HasSuffix(completeLine, "|") {
						tableLines = append(tableLines, completeLine)
					}
				} else if dataLine == "" {
					// 空行 → テーブル終了
					break
				} else {
					// テーブル外の行（|で始まらない） → テーブル終了
					break
				}
			}

			// テーブルをプレースホルダー化
			tables = append(tables, strings.Join(tableLines, "\n"))
			result = append(result, fmt.Sprintf("__TABLE_%d__", len(tables)-1))
		} else if strings.HasPrefix(line, "|") && !strings.HasPrefix(line, "||") {
			// ヘッダー無しテーブルを検出
			tableLines := []string{}

			// データ行を収集（セル内改行対応）
			for i < len(lines) {
				dataLine := lines[i]

				if strings.HasPrefix(dataLine, "|") && !strings.HasPrefix(dataLine, "||") {
					// データ行の開始
					completeLine := dataLine
					i++

					// |で終わるまで次の行と結合（セル内改行対応）
					for !strings.HasSuffix(completeLine, "|") && i < len(lines) {
						nextLine := lines[i]
						// 次のテーブルヘッダーが来たら結合を中止
						if strings.HasPrefix(nextLine, "||") && strings.HasSuffix(nextLine, "||") {
							break
						}
						// 次のデータ行が来たら結合を中止
						if strings.HasPrefix(nextLine, "|") {
							break
						}
						// 空行が来たら結合を中止
						if nextLine == "" {
							break
						}
						completeLine += "\n" + nextLine
						i++
					}

					if strings.HasSuffix(completeLine, "|") {
						tableLines = append(tableLines, completeLine)
					}
				} else if dataLine == "" {
					// 空行 → テーブル終了
					break
				} else {
					// テーブル外の行 → テーブル終了
					break
				}
			}

			// テーブルをプレースホルダー化
			if len(tableLines) > 0 {
				tables = append(tables, strings.Join(tableLines, "\n"))
				result = append(result, fmt.Sprintf("__TABLE_%d__", len(tables)-1))
			}
		} else {
			result = append(result, line)
			i++
		}
	}

	return strings.Join(result, "\n"), tables
}

// listInfo represents information about an open list
type listInfo struct {
	listType string // "ul" or "ol"
	level    int
}

// convertCellListsToHTML converts JIRA list elements within a table cell to HTML list tags
func convertCellListsToHTML(cell string) string {
	lines := strings.Split(cell, "\n")
	var listStack []listInfo // tracks open lists with type and level
	var output strings.Builder

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for unordered list (* ** ***)
		// TrimSpace後に末尾スペースが除去された空項目（例: "** "→"**"）にも対応するため (?:\s+(.*)|$) を使用
		if match := regexp.MustCompile(`^(\*+)(?:\s+(.*)|$)`).FindStringSubmatch(trimmed); match != nil {
			level := len(match[1])
			content := match[2]
			items := processListItem("ul", level, content, &listStack)
			for _, item := range items {
				output.WriteString(item)
			}
			continue
		}

		// Check for ordered list (# ## ###)
		if match := regexp.MustCompile(`^(#+)(?:\s+(.*)|$)`).FindStringSubmatch(trimmed); match != nil {
			level := len(match[1])
			content := match[2]
			items := processListItem("ol", level, content, &listStack)
			for _, item := range items {
				output.WriteString(item)
			}
			continue
		}

		// Non-list line: close all open lists
		hadOpenLists := len(listStack) > 0
		items := closeAllLists(&listStack)
		for _, item := range items {
			output.WriteString(item)
		}

		if trimmed != "" {
			// Add newline after list closing tags if there were open lists
			if hadOpenLists && len(items) > 0 {
				output.WriteString("\n")
			}
			output.WriteString(line)
			// Add newline between lines (except after the last line)
			if i < len(lines)-1 {
				output.WriteString("\n")
			}
		} else if i < len(lines)-1 {
			// Empty line: still add newline if not the last line
			output.WriteString("\n")
		}
	}

	// Close any remaining open lists
	items := closeAllLists(&listStack)
	for _, item := range items {
		output.WriteString(item)
	}

	return output.String()
}

// processListItem handles a single list item, managing list opening/closing
func processListItem(listType string, level int, content string, stack *[]listInfo) []string {
	var result []string

	// Close lists that are deeper or different type at same level
	for len(*stack) > 0 {
		top := (*stack)[len(*stack)-1]
		if top.level > level || (top.level == level && top.listType != listType) {
			result = append(result, closeList(top.listType))
			*stack = (*stack)[:len(*stack)-1]
		} else {
			break
		}
	}

	// Open new list if needed
	if len(*stack) == 0 || (*stack)[len(*stack)-1].level < level {
		result = append(result, openList(listType))
		*stack = append(*stack, listInfo{listType: listType, level: level})
	}

	result = append(result, fmt.Sprintf("<li>%s</li>", content))
	return result
}

// openList returns the opening tag for a list
func openList(listType string) string {
	return fmt.Sprintf("<%s>", listType)
}

// closeList returns the closing tag for a list
func closeList(listType string) string {
	return fmt.Sprintf("</%s>", listType)
}

// closeAllLists closes all open lists in the stack
func closeAllLists(stack *[]listInfo) []string {
	var result []string
	for i := len(*stack) - 1; i >= 0; i-- {
		result = append(result, closeList((*stack)[i].listType))
	}
	*stack = (*stack)[:0]
	return result
}

// convertJIRATableToMarkdown 1つのJIRAテーブルをMarkdownテーブルに変換する
func (mw *MarkdownWriter) convertJIRATableToMarkdown(table string) string {
	lines := strings.Split(table, "\n")
	var result []string

	// ヘッダーの有無を判定
	hasHeader := false
	if len(lines) > 0 {
		firstLine := lines[0]
		hasHeader = strings.HasPrefix(firstLine, "||") && strings.HasSuffix(firstLine, "||")
	}

	// ヘッダー無しの場合、最初のデータ行からセル数を取得して空ヘッダーを生成
	if !hasHeader && len(lines) > 0 {
		// 最初のデータ行を取得（セル内改行対応）
		firstLine := lines[0]
		if strings.HasPrefix(firstLine, "|") && !strings.HasPrefix(firstLine, "||") {
			// セル内改行を考慮して完全な行を取得
			completeLine := firstLine
			j := 1
			for !strings.HasSuffix(completeLine, "|") && j < len(lines) {
				nextLine := lines[j]
				completeLine += "\n" + nextLine
				j++
			}

			if strings.HasSuffix(completeLine, "|") {
				content := strings.Trim(completeLine, "|")
				cells := strings.Split(content, "|")
				cellCount := len(cells)

				// 空ヘッダー行を生成
				emptyHeaders := make([]string, cellCount)
				for k := range emptyHeaders {
					emptyHeaders[k] = " "
				}
				header := "| " + strings.Join(emptyHeaders, " | ") + " |"
				result = append(result, header)

				// セパレーター行を生成
				separators := make([]string, cellCount)
				for k := range separators {
					separators[k] = "------"
				}
				separator := "| " + strings.Join(separators, " | ") + " |"
				result = append(result, separator)
			}
		}
	}

	i := 0
	for i < len(lines) {
		line := lines[i]

		// ヘッダー行を変換（セル内改行対応）
		if strings.HasPrefix(line, "||") {
			completeLine := line
			i++

			// ||で終わるまで次の行と結合（セル内改行対応）
			for !strings.HasSuffix(completeLine, "||") && i < len(lines) {
				nextLine := lines[i]
				completeLine += "\n" + nextLine
				i++
			}

			if strings.HasSuffix(completeLine, "||") {
				content := strings.Trim(completeLine, "|")
				cells := strings.Split(content, "||")
				// セル内のリスト要素をHTMLに変換（<br>置換前）
				for j, cell := range cells {
					cells[j] = convertCellListsToHTML(cell)
				}
				// セル内の色マークアップを変換
				for j, cell := range cells {
					cells[j] = mw.convertStatusLabelMarkup(cell)
					cells[j] = mw.convertColorMarkup(cells[j])
				}
				// セル内の残りの改行を <br> に置換
				for j, cell := range cells {
					cells[j] = strings.ReplaceAll(cell, "\n", "<br>")
				}
				// Markdownテーブルヘッダー
				header := "| " + strings.Join(cells, " | ") + " |"
				result = append(result, header)
				// セパレーター行
				separators := make([]string, len(cells))
				for j := range separators {
					separators[j] = "------"
				}
				separator := "| " + strings.Join(separators, " | ") + " |"
				result = append(result, separator)
			}
		} else if strings.HasPrefix(line, "|") && !strings.HasPrefix(line, "||") {
			// データ行を変換（セル内改行対応）
			completeLine := line
			i++

			// |で終わるまで次の行と結合（セル内改行対応）
			for !strings.HasSuffix(completeLine, "|") && i < len(lines) {
				nextLine := lines[i]
				completeLine += "\n" + nextLine
				i++
			}

			if strings.HasSuffix(completeLine, "|") {
				content := strings.Trim(completeLine, "|")
				cells := strings.Split(content, "|")
				// セル内のリスト要素をHTMLに変換（<br>置換前）
				for j, cell := range cells {
					cells[j] = convertCellListsToHTML(cell)
				}
				// セル内の色マークアップを変換
				for j, cell := range cells {
					cells[j] = mw.convertStatusLabelMarkup(cell)
					cells[j] = mw.convertColorMarkup(cells[j])
				}
				// セル内の残りの改行を <br> に置換
				for j, cell := range cells {
					cells[j] = strings.ReplaceAll(cell, "\n", "<br>")
				}
				// Markdownテーブルデータ行
				row := "| " + strings.Join(cells, " | ") + " |"
				result = append(result, row)
			}
		} else {
			i++
		}
	}

	return strings.Join(result, "\n")
}
