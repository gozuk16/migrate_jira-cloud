package main

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// ImageAttributes は画像の属性を保持する構造体
type ImageAttributes struct {
	Width string // 例: "300px"
	Alt   string // 例: "説明文"
}

// splitAttributeString は属性文字列をカンマで分割する（引用符内は除外）
func splitAttributeString(s string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, ch := range s {
		switch ch {
		case '"', '\'':
			if !inQuote {
				inQuote = true
				quoteChar = ch
			} else if ch == quoteChar {
				inQuote = false
			}
			current.WriteRune(ch)
		case ',':
			if inQuote {
				current.WriteRune(ch)
			} else {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// parseImageAttributes は属性文字列をパースする
// 入力例: "width=300,alt=\"スクリーンショット\""
func parseImageAttributes(attrStr string) ImageAttributes {
	attrs := ImageAttributes{}

	// カンマで分割（ただし引用符内のカンマは除外）
	parts := splitAttributeString(attrStr)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// key=value 形式を分割
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		// 引用符を除去
		value = strings.Trim(value, "\"'")

		switch key {
		case "width":
			// 数値のみの場合は "px" を追加
			if matched, _ := regexp.MatchString(`^\d+$`, value); matched {
				attrs.Width = value + "px"
			} else {
				attrs.Width = value
			}
		case "alt":
			attrs.Alt = value
		}
	}

	return attrs
}

// replaceImageReferencesWithAttributes はJIRA形式の属性付き画像参照を変換する
// パターン: !$filename.png|width=300,alt="説明"!
func (mw *MarkdownWriter) replaceImageReferencesWithAttributes(text string, attachmentMap map[string]string) string {
	// JIRA形式の属性付き画像参照パターン: !filename.png|属性! または !$filename.png|属性!
	pattern := regexp.MustCompile(`!(?:\$)?([^!|]+(?:\.[a-zA-Z0-9]+))\|([^!]+)!`)

	result := pattern.ReplaceAllStringFunc(text, func(match string) string {
		// マッチからファイル名と属性を抽出
		submatches := pattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		originalFilename := submatches[1]
		attrStr := submatches[2]

		// 添付ファイルマップから保存されたファイル名を取得
		savedFilename, exists := attachmentMap[originalFilename]
		if !exists {
			return match // 見つからない場合は元のまま
		}

		// ファイル名をURLエンコーディング（スペース→%20）
		encodedFilename := url.PathEscape(savedFilename)

		// 属性をパース
		attrs := parseImageAttributes(attrStr)

		// 画像ファイルの場合のみ処理
		if !IsImageFile(originalFilename) {
			return match
		}

		// alt が指定されていない場合はファイル名を使用
		alt := attrs.Alt
		if alt == "" {
			alt = originalFilename
		}

		// Markdown形式に変換（相対パス）
		// title属性を使って幅を指定: ![alt](path "width=250")
		if attrs.Width != "" {
			return fmt.Sprintf("![%s](%s \"%s\")", alt, encodedFilename, "width="+attrs.Width)
		}
		return fmt.Sprintf("![%s](%s)", alt, encodedFilename)
	})

	return result
}

// replaceAttachmentReferences はJIRA形式の添付ファイル参照 [^filename.ext] をMarkdownリンクに変換する
func (mw *MarkdownWriter) replaceAttachmentReferences(text string, attachmentMap map[string]string) string {
	// パターン1: [表示テキスト|^filename.ext]（テキスト指定版を先に処理）
	textPattern := regexp.MustCompile(`\[([^\]|]+)\|\^([^\]]+)\]`)
	text = textPattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := textPattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		displayText := submatches[1]
		filename := submatches[2]
		savedFilename, exists := attachmentMap[filename]
		if !exists {
			return match
		}
		encodedFilename := url.PathEscape(savedFilename)
		return fmt.Sprintf("[%s](%s)", displayText, encodedFilename)
	})

	// パターン2: [^filename.ext]
	simplePattern := regexp.MustCompile(`\[\^([^\]]+)\]`)
	text = simplePattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := simplePattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		filename := submatches[1]
		savedFilename, exists := attachmentMap[filename]
		if !exists {
			return match
		}
		encodedFilename := url.PathEscape(savedFilename)
		return fmt.Sprintf("[%s](%s)", filename, encodedFilename)
	})

	return text
}

// replaceImageReferences はJIRA形式の画像参照 !filename.png! をMarkdown形式に変換する
func (mw *MarkdownWriter) replaceImageReferences(text string, attachmentMap map[string]string) string {
	// JIRA形式の画像参照パターン: !filename.png! または !filename.png|属性!
	// 例: !screenshot.png!, !image.jpg|width=300!
	pattern := regexp.MustCompile(`!([^!|]+(?:\.[a-zA-Z0-9]+))(?:\|[^!]*)?!`)

	result := pattern.ReplaceAllStringFunc(text, func(match string) string {
		// マッチからファイル名を抽出
		submatches := pattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		originalFilename := submatches[1]

		// 添付ファイルマップから保存されたファイル名を取得
		savedFilename, exists := attachmentMap[originalFilename]
		if !exists {
			return match // 見つからない場合は元のまま
		}

		// ファイル名をURLエンコーディング（スペース→%20）
		encodedFilename := url.PathEscape(savedFilename)
		// 画像ファイルの場合は画像形式、それ以外はリンク形式（同じディレクトリ内の相対パス）
		if IsImageFile(originalFilename) {
			return fmt.Sprintf("![%s](%s)", originalFilename, encodedFilename)
		}
		return fmt.Sprintf("[%s](%s)", originalFilename, encodedFilename)
	})

	return result
}

// mapStatusColor はJIRAの色名をCSSクラス名にマッピング
func mapStatusColor(color string) string {
	colorMap := map[string]string{
		"green":     "status-green",
		"yellow":    "status-yellow",
		"red":       "status-red",
		"blue":      "status-blue",
		"blue-gray": "status-blue",
		"grey":      "status-gray",
		"gray":      "status-gray",
	}
	return colorMap[color]
}

// statusLabelColorMap はカスタムステータスラベルの16進数カラーコードをCSSクラス名にマッピング
var statusLabelColorMap = map[string]string{
	"#ff991f": "status-label-warning", // オレンジ/警告
	"#00b8d9": "status-label-teal",    // ティール/OK
	"#36b37e": "status-label-success", // 緑/成功
	"#ff5630": "status-label-danger",  // 赤/危険
	"#6554c0": "status-label-purple",  // 紫
	"#97a0af": "status-label-gray",    // グレー
}

// convertStatusLabelMarkup はカスタムステータスラベルをHTMLスパンに変換
// パターン: {color:#XXX}*[ text ]*{color}
func (mw *MarkdownWriter) convertStatusLabelMarkup(text string) string {
	// 正規表現: {color:#HEXCODE}*[ text ]*{color}
	pattern := regexp.MustCompile(`(?i)\{color:(#[0-9a-fA-F]{6})\}\*\[\s*([^\]]+?)\s*\]\*\{color\}`)

	return pattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := pattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}

		colorCode := strings.ToLower(submatches[1])
		labelText := submatches[2]

		if className, ok := statusLabelColorMap[colorCode]; ok {
			return fmt.Sprintf(`<span class="status-label %s">%s</span>`, className, labelText)
		}
		// 未知の色はデフォルトクラス
		return fmt.Sprintf(`<span class="status-label">%s</span>`, labelText)
	})
}

// convertStatusMarkup は{status}マクロをHTMLスパンに変換
func (mw *MarkdownWriter) convertStatusMarkup(content string) string {
	// パターン: {status:colour=Green}text{status} または {status:color=Green}text{status}
	pattern := regexp.MustCompile(`(?i)\{status(?::colou?r=([^}]+))?\}([^{]*)\{status\}`)

	return pattern.ReplaceAllStringFunc(content, func(match string) string {
		submatches := pattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}

		color := strings.ToLower(submatches[1])
		text := submatches[2]

		// 色をCSSクラスにマッピング
		colorClass := mapStatusColor(color)

		if colorClass != "" {
			return fmt.Sprintf(`<span class="status %s">%s</span>`, colorClass, text)
		}
		return fmt.Sprintf(`<span class="status">%s</span>`, text)
	})
}

// convertQuoteListsToMarkdown は引用内のJIRAリストをMarkdownリストに変換
func convertQuoteListsToMarkdown(content string) string {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))

	// TrimSpace後に末尾スペースが除去された空項目（例: "** "→"**"）にも対応するため (?:\s+(.*)|$) を使用
	bulletListPattern := regexp.MustCompile(`^(\*+)(?:\s+(.*)|$)`)
	numberedListPattern := regexp.MustCompile(`^(#+)(?:\s+(.*)|$)`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 箇条書きリスト (*) の処理
		if matches := bulletListPattern.FindStringSubmatch(trimmed); len(matches) == 3 {
			asterisks := matches[1]
			itemContent := matches[2]
			if itemContent == "" {
				itemContent = "&nbsp;"
			}
			level := len(asterisks) - 1
			indent := strings.Repeat("    ", level)
			result = append(result, indent+"- "+itemContent)
			continue
		}

		// 番号付きリスト (#) の処理
		if matches := numberedListPattern.FindStringSubmatch(trimmed); len(matches) == 3 {
			hashes := matches[1]
			itemContent := matches[2]
			if itemContent == "" {
				itemContent = "&nbsp;"
			}
			level := len(hashes) - 1
			indent := strings.Repeat("    ", level)
			result = append(result, indent+"1. "+itemContent)
			continue
		}

		// リストではない通常の行
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// convertQuoteMarkup は{quote}...{quote}をMarkdownの引用に変換
func (mw *MarkdownWriter) convertQuoteMarkup(text string) string {
	quotePattern := regexp.MustCompile(`(?s)\{quote\}(.*?)\{quote\}`)
	return quotePattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := quotePattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		content := submatches[1]

		// 引用内のJIRAリストをMarkdownリストに変換
		content = convertQuoteListsToMarkdown(content)

		// 各行の先頭に引用記号を追加
		lines := strings.Split(content, "\n")
		var result []string

		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				result = append(result, "> "+line)
			} else {
				result = append(result, ">")
			}
		}

		return strings.Join(result, "\n")
	})
}

// convertColorMarkup は{color:...}...{color}をHTMLのspanタグに変換
// JIRAの色指定をそのままインラインスタイルとして出力
func (mw *MarkdownWriter) convertColorMarkup(text string) string {
	tokenPattern := regexp.MustCompile(`\{color(?::([^}]+))?\}`)
	matches := tokenPattern.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text
	}

	var colorStack []string
	var result strings.Builder
	lastPos := 0

	for _, m := range matches {
		matchStart, matchEnd := m[0], m[1]
		result.WriteString(text[lastPos:matchStart])

		captureStart := m[2]
		if captureStart >= 0 {
			// 開始タグ {color:XXX}
			colorValue := text[m[2]:m[3]]
			if len(colorStack) > 0 {
				result.WriteString("</span>")
			}
			colorStack = append(colorStack, colorValue)
			result.WriteString(fmt.Sprintf(`<span style="color:%s">`, colorValue))
		} else {
			// 閉じタグ {color}
			if len(colorStack) > 0 {
				result.WriteString("</span>")
				colorStack = colorStack[:len(colorStack)-1]
				if len(colorStack) > 0 {
					result.WriteString(fmt.Sprintf(`<span style="color:%s">`, colorStack[len(colorStack)-1]))
				}
			}
		}

		lastPos = matchEnd
	}

	result.WriteString(text[lastPos:])

	for range colorStack {
		result.WriteString("</span>")
	}

	return result.String()
}

// getPanelClass はbgColorからCSSクラスを判別
func getPanelClass(bgColor string) string {
	bgColor = strings.ToLower(strings.TrimSpace(bgColor))
	if !strings.HasPrefix(bgColor, "#") {
		bgColor = "#" + bgColor
	}

	switch bgColor {
	case "#ffebe6":
		return "panel-error"
	case "#e3fcef":
		return "panel-success"
	case "#fffae6":
		return "panel-warning"
	case "#deebff":
		return "panel-info"
	default:
		return "panel-info"
	}
}

// parsePanelParams はpanelのパラメータ文字列を解析
func parsePanelParams(paramStr string) map[string]string {
	params := make(map[string]string)
	paramPattern := regexp.MustCompile(`(\w+)=([^|]+)`)
	matches := paramPattern.FindAllStringSubmatch(paramStr, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			key := strings.TrimSpace(match[1])
			value := strings.TrimSpace(match[2])
			params[key] = value
		}
	}

	return params
}

// convertPanelMarkup は{panel:...}...{panel}をHTMLのdivタグに変換
func (mw *MarkdownWriter) convertPanelMarkup(text string) string {
	// パラメータ付きpanel
	panelWithParamsPattern := regexp.MustCompile(`(?s)\{panel:([^}]+)\}(.*?)\{panel\}`)
	text = panelWithParamsPattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := panelWithParamsPattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}

		paramStr := submatches[1]
		content := submatches[2]
		params := parsePanelParams(paramStr)

		bgColor := params["bgColor"]
		title := params["title"]
		panelClass := getPanelClass(bgColor)

		content = strings.TrimSpace(content)
		content = strings.ReplaceAll(content, "\n", "<br>\n")

		var result string
		if title != "" {
			result = fmt.Sprintf(`<div class="panel %s"><div class="panel-title">%s</div><div class="panel-body">%s</div></div>`,
				panelClass, title, content)
		} else {
			result = fmt.Sprintf(`<div class="panel %s"><div class="panel-body">%s</div></div>`,
				panelClass, content)
		}

		return result
	})

	// パラメータなしpanel
	panelSimplePattern := regexp.MustCompile(`(?s)\{panel\}(.*?)\{panel\}`)
	text = panelSimplePattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := panelSimplePattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		content := submatches[1]
		content = strings.TrimSpace(content)
		content = strings.ReplaceAll(content, "\n", "<br>\n")
		return fmt.Sprintf(`<div class="panel panel-info"><div class="panel-body">%s</div></div>`, content)
	})

	return text
}

// convertHTMLJIRAIssueMacroToRelative は HTML形式のJIRA課題マクロを相対パスリンクに変換
func (mw *MarkdownWriter) convertHTMLJIRAIssueMacroToRelative(text string, currentProjectKey string) string {
	// ステップ1: 全てのJIRA issue macro spanを見つける
	// パターン: <span class="jira-issue-macro ..." data-jira-key="ISSUE-KEY">...
	macroPattern := regexp.MustCompile(
		`<span\s+class="jira-issue-macro[^"]*"\s+data-jira-key="([A-Z][A-Z0-9_]*-[0-9]+)"[^>]*>`,
	)

	// 各マクロを処理
	for {
		match := macroPattern.FindStringSubmatchIndex(text)
		if match == nil {
			break
		}

		// マクロの開始位置とissue keyを取得
		macroStart := match[0]
		macroEnd := match[1]
		issueKeyStart := match[2]
		issueKeyEnd := match[3]

		if issueKeyStart < 0 {
			break
		}

		issueKey := text[issueKeyStart:issueKeyEnd]
		projectKey := strings.ToLower(strings.Split(issueKey, "-")[0])
		issueKeyLower := strings.ToLower(issueKey)

		// ステップ2: マクロの内容を探して、対応する</span>とステータス情報を抽出
		// マクロのspan深度を追跡
		spanDepth := 1
		pos := macroEnd
		macroContent := ""
		statusText := ""

		for pos < len(text) && spanDepth > 0 {
			// 次の<span または </span> を見つける
			nextOpen := strings.Index(text[pos:], "<span")
			nextClose := strings.Index(text[pos:], "</span>")

			// どちらが先か判定
			if nextClose == -1 {
				// </span> がもう見つからない
				break
			}

			if nextOpen != -1 && nextOpen < nextClose {
				// <span が先
				spanDepth++
				macroContent += text[pos : pos+nextOpen+5]
				pos += nextOpen + 5
			} else {
				// </span> が先
				spanDepth--
				if spanDepth == 0 {
					macroContent += text[pos : pos+nextClose]
					macroEnd = pos + nextClose + 7
					break
				}
				macroContent += text[pos : pos+nextClose+7]
				pos += nextClose + 7
			}
		}

		// ステップ3: マクロの内容からステータスを抽出
		if strings.Contains(macroContent, "aui-lozenge") {
			// aui-lozenge を含むspan内のテキストを抽出
			statusPattern := regexp.MustCompile(`<span\s+class="[^"]*aui-lozenge[^"]*"[^>]*>([^<]+)</span>`)
			if matches := statusPattern.FindStringSubmatch(macroContent); len(matches) > 1 {
				statusText = strings.TrimSpace(matches[1])
			}
		}

		// ステップ4: Markdown形式に変換（同プロジェクト: ../key/、別プロジェクト: ../../project/key/）
		currentProject := strings.ToLower(currentProjectKey)
		var result string
		if projectKey == currentProject {
			result = fmt.Sprintf("[%s](../%s/)", issueKey, issueKeyLower)
		} else {
			result = fmt.Sprintf("[%s](../../%s/%s/)", issueKey, projectKey, issueKeyLower)
		}
		if statusText != "" {
			result += fmt.Sprintf(" (%s)", statusText)
		}

		// ステップ5: テキストを置き換え
		text = text[:macroStart] + result + text[macroEnd:]
	}

	return text
}

// getAdmonitionClass はadmonitionタイプからCSSクラスを取得
func getAdmonitionClass(admonitionType string) string {
	switch strings.ToLower(admonitionType) {
	case "note":
		return "panel-note"
	case "info":
		return "panel-info"
	case "warning":
		return "panel-warning"
	case "tip":
		return "panel-success"
	default:
		return "panel-info"
	}
}

// convertAdmonitionMarkup は{note}等のadmonitionをpanelに変換
func (mw *MarkdownWriter) convertAdmonitionMarkup(text string) string {
	// Goのregexpはバックリファレンスをサポートしないため、各タイプ別に処理
	admonitionTypes := []string{"note", "info", "warning", "tip"}

	for _, adType := range admonitionTypes {
		pattern := regexp.MustCompile(`(?s)\{` + adType + `\}(.*?)\{` + adType + `\}`)
		text = pattern.ReplaceAllStringFunc(text, func(match string) string {
			submatches := pattern.FindStringSubmatch(match)
			if len(submatches) < 2 {
				return match
			}

			content := submatches[1]
			panelClass := getAdmonitionClass(adType)

			content = strings.TrimSpace(content)
			content = strings.ReplaceAll(content, "\n", "<br>\n")
			return fmt.Sprintf(`<div class="panel %s"><div class="panel-body">%s</div></div>`,
				panelClass, content)
		})
	}

	return text
}

// applyURLReplacements は設定ファイルの url_replacements に従いURLプレフィックスを置換する
// パス部分はそのまま保持し、先頭のURLベースのみを置換する
func (mw *MarkdownWriter) applyURLReplacements(text string) string {
	if mw.config == nil || len(mw.config.URLReplacements) == 0 {
		return text
	}
	for from, to := range mw.config.URLReplacements {
		if from == "" || to == "" {
			continue
		}
		from = strings.TrimSuffix(from, "/")
		to = strings.TrimSuffix(to, "/")
		text = strings.ReplaceAll(text, from, to)
	}
	return text
}

// convertJIRAIssueLinksToRelative はJIRA課題URLを相対パスリンクに変換する
// config.tomlで設定されたJIRAインスタンスのURL（CloudおよびServerURL）と一致するリンクを変換する
func (mw *MarkdownWriter) convertJIRAIssueLinksToRelative(text string, currentProjectKey string) string {
	if mw.config == nil || mw.config.JIRA.URL == "" {
		return text // 設定がない場合は変換しない
	}

	// 変換対象のベースURLリスト（Cloud URL必須、Server URLはオプション）
	baseURLs := []string{strings.TrimSuffix(mw.config.JIRA.URL, "/")}
	if mw.config.JIRA.ServerURL != "" {
		baseURLs = append(baseURLs, strings.TrimSuffix(mw.config.JIRA.ServerURL, "/"))
	}

	currentProject := strings.ToLower(currentProjectKey)

	for _, baseURL := range baseURLs {
		escapedURL := regexp.QuoteMeta(baseURL)

		// パターン1: JIRA形式 [URL|smart-link] を変換
		// 同プロジェクト: [SCRUM-6](../scrum-6/)、別プロジェクト: [KT-3](../../kt/kt-3/)
		pattern1 := regexp.MustCompile(
			`\[` + escapedURL + `/browse/([A-Z][A-Z0-9_]*)-([0-9]+)\|[^\]]*\]`,
		)
		text = pattern1.ReplaceAllStringFunc(text, func(match string) string {
			submatches := pattern1.FindStringSubmatch(match)
			if len(submatches) < 3 {
				return match
			}
			targetProject := strings.ToLower(submatches[1])
			issueKey := strings.ToLower(submatches[1] + "-" + submatches[2])
			linkText := submatches[1] + "-" + submatches[2]
			if targetProject == currentProject {
				return "[" + linkText + "](../" + issueKey + "/)"
			}
			return "[" + linkText + "](../../" + targetProject + "/" + issueKey + "/)"
		})

		// パターン2: Markdown形式 [URL](URL) を変換（フォールバック）
		// 例: [https://...browse/SCRUM-6](https://...browse/SCRUM-6) → [SCRUM-6](../scrum-6/)
		pattern2 := regexp.MustCompile(
			`\[` + escapedURL + `/browse/([A-Z][A-Z0-9_]*)-([0-9]+)\]\(` +
				escapedURL + `/browse/[A-Z][A-Z0-9_]*-[0-9]+\)`,
		)
		text = pattern2.ReplaceAllStringFunc(text, func(match string) string {
			submatches := pattern2.FindStringSubmatch(match)
			if len(submatches) < 3 {
				return match
			}
			targetProject := strings.ToLower(submatches[1])
			issueKey := strings.ToLower(submatches[1] + "-" + submatches[2])
			linkText := submatches[1] + "-" + submatches[2]
			if targetProject == currentProject {
				return "[" + linkText + "](../" + issueKey + "/)"
			}
			return "[" + linkText + "](../../" + targetProject + "/" + issueKey + "/)"
		})

		// パターン3: プレーンURL形式 http://server/browse/SCRUM-1 を変換（リンクなし）
		pattern3 := regexp.MustCompile(
			escapedURL + `/browse/([A-Z][A-Z0-9_]*)-([0-9]+)`,
		)
		text = pattern3.ReplaceAllStringFunc(text, func(match string) string {
			submatches := pattern3.FindStringSubmatch(match)
			if len(submatches) < 3 {
				return match
			}
			targetProject := strings.ToLower(submatches[1])
			issueKey := strings.ToLower(submatches[1] + "-" + submatches[2])
			linkText := submatches[1] + "-" + submatches[2]
			if targetProject == currentProject {
				return "[" + linkText + "](../" + issueKey + "/)"
			}
			return "[" + linkText + "](../../" + targetProject + "/" + issueKey + "/)"
		})
	}

	return text
}

// convertPlainTextIssueKeysToLinks はプレーンテキストの課題キー（SCRUM-1等）をMarkdownリンクに変換する
// プロジェクトキー一覧と照合し、既知のプロジェクトのみ変換する
func (mw *MarkdownWriter) convertPlainTextIssueKeysToLinks(text string, currentProjectKey string) string {
	currentProject := strings.ToLower(currentProjectKey)

	// 課題キーパターン: 大文字アルファベット・数字・アンダースコアで始まる1文字以上 + "-" + 数字1文字以上
	// 単語境界（\b相当）として前後に単語文字が来ないことを確認
	// GoのRE2は \b をサポートしているが、日本語等には効かないため前後の文字クラスで制御
	issueKeyPattern := regexp.MustCompile(`(?:^|([^A-Za-z0-9_\[]))([A-Z][A-Z0-9_]*)-(\d+)(?:[^A-Za-z0-9_\(]|$)`)

	// 既にMarkdownリンクの中にある課題キーを保護するため、先にプレースホルダー化する
	mdLinkPattern := regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)
	var protectedLinks []string
	protectIndex := 0
	text = mdLinkPattern.ReplaceAllStringFunc(text, func(match string) string {
		placeholder := fmt.Sprintf("___PTLINK_PROTECT_%d___", protectIndex)
		protectedLinks = append(protectedLinks, match)
		protectIndex++
		return placeholder
	})

	// 行単位で処理（行頭の課題キーも対象にするため）
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = issueKeyPattern.ReplaceAllStringFunc(line, func(match string) string {
			submatches := issueKeyPattern.FindStringSubmatch(match)
			if len(submatches) < 4 {
				return match
			}
			prefix := submatches[1]   // 課題キー前の文字（キャプチャグループ1）
			projPart := submatches[2] // プロジェクトキー部分
			numPart := submatches[3]  // 番号部分

			// プロジェクトキー一覧に含まれなければ変換しない
			if !mw.projectKeys[projPart] {
				return match
			}

			issueKey := strings.ToLower(projPart + "-" + numPart)
			linkText := projPart + "-" + numPart
			targetProject := strings.ToLower(projPart)

			var link string
			if targetProject == currentProject {
				link = "[" + linkText + "](../" + issueKey + "/)"
			} else {
				link = "[" + linkText + "](../../" + targetProject + "/" + issueKey + "/)"
			}

			// マッチ全体を置換するが、前後の文字を保持する
			// match の末尾文字（単語文字でない文字）を後ろに戻す
			suffix := match[len(prefix)+len(projPart)+1+len(numPart):]
			return prefix + link + suffix
		})
	}
	text = strings.Join(lines, "\n")

	// プレースホルダーを元に戻す
	for j, orig := range protectedLinks {
		text = strings.ReplaceAll(text, fmt.Sprintf("___PTLINK_PROTECT_%d___", j), orig)
	}

	return text
}
