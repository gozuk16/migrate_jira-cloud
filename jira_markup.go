package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// convertJIRAMarkupToMarkdown はJIRAマークアップをMarkdown形式に変換する
func (mw *MarkdownWriter) convertJIRAMarkupToMarkdown(text string, currentProjectKey string) string {
	// コントロールコードを除去（^H等のJIRA Serverデータに含まれる不正文字）
	text = removeControlCharacters(text)

	// プレースホルダーでコードブロックとインラインコードを保護
	codeBlocks := []string{}
	placeholderIndex := 0
	inlineCodes := []string{}
	inlineCodeIndex := 0

	// 1. コードブロック（言語指定付き）を抽出して保護
	codeWithLangPattern := regexp.MustCompile(`(?s)\{code:([^}]+)\}(.*?)\{code\}`)
	text = codeWithLangPattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := codeWithLangPattern.FindStringSubmatch(match)
		if len(submatches) >= 3 {
			lang := submatches[1]
			code := strings.ReplaceAll(submatches[2], "\r", "")
			// Markdownのコードブロック形式に変換（中身にフェンスが含まれる場合は長いフェンスを使用）
			fence := fenceForContent(code)
			mdCodeBlock := fmt.Sprintf("%s%s\n%s\n%s", fence, lang, code, fence)
			placeholder := fmt.Sprintf("__CODE_BLOCK_%d__", placeholderIndex)
			codeBlocks = append(codeBlocks, mdCodeBlock)
			placeholderIndex++
			return placeholder
		}
		return match
	})

	// 2. コードブロック（言語指定なし）を抽出して保護
	codePattern := regexp.MustCompile(`(?s)\{code\}(.*?)\{code\}`)
	text = codePattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := codePattern.FindStringSubmatch(match)
		if len(submatches) >= 2 {
			code := strings.ReplaceAll(submatches[1], "\r", "")
			// Markdownのコードブロック形式に変換（中身にフェンスが含まれる場合は長いフェンスを使用）
			fence := fenceForContent(code)
			mdCodeBlock := fmt.Sprintf("%s\n%s\n%s", fence, code, fence)
			placeholder := fmt.Sprintf("__CODE_BLOCK_%d__", placeholderIndex)
			codeBlocks = append(codeBlocks, mdCodeBlock)
			placeholderIndex++
			return placeholder
		}
		return match
	})

	// 3. フォーマット済みテキストを抽出して保護
	noformatPattern := regexp.MustCompile(`(?s)\{noformat\}(.*?)\{noformat\}`)
	text = noformatPattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := noformatPattern.FindStringSubmatch(match)
		if len(submatches) >= 2 {
			content := strings.ReplaceAll(submatches[1], "\r", "")
			// Markdownのコードブロック形式に変換（中身にフェンスが含まれる場合は長いフェンスを使用）
			fence := fenceForContent(content)
			mdCodeBlock := fmt.Sprintf("%s\n%s\n%s", fence, content, fence)
			placeholder := fmt.Sprintf("__CODE_BLOCK_%d__", placeholderIndex)
			codeBlocks = append(codeBlocks, mdCodeBlock)
			placeholderIndex++
			return placeholder
		}
		return match
	})

	// 4. インラインコード: {{text}} → `text`
	// (?:[^}]|\}[^}])* で「}以外の文字」または「非}}の単独}（}の後に}以外）」にマッチし、
	// ${page_path} のような } を含む内容にも対応する（GoのREGEXはlookahead非対応のため）
	inlineCodePattern := regexp.MustCompile(`\{\{((?:[^}]|\}[^}])*)\}\}`)
	text = inlineCodePattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := inlineCodePattern.FindStringSubmatch(match)
		if len(submatches) >= 2 {
			code := submatches[1]
			inlineCode := fmt.Sprintf("`%s`", code)
			placeholder := fmt.Sprintf("__INLINE_CODE_%d__", inlineCodeIndex)
			inlineCodes = append(inlineCodes, inlineCode)
			inlineCodeIndex++
			return placeholder
		}
		return match
	})

	// 4a. バッククォートで囲まれたインラインコードも保護
	// JIRAエディタがバッククォートを直接使用する場合がある
	// コードブロック（{code}、{noformat}）はstep 1-3で既に抽出済みのため残存するバッククォートはインラインコード
	backtickCodePattern := regexp.MustCompile("`([^`]+)`")
	text = backtickCodePattern.ReplaceAllStringFunc(text, func(match string) string {
		placeholder := fmt.Sprintf("__INLINE_CODE_%d__", inlineCodeIndex)
		inlineCodes = append(inlineCodes, match) // match は `FOO_BAR_` の形式（既にMarkdown形式）
		inlineCodeIndex++
		return placeholder
	})

	// 4-0. テーブル抽出（改行正規化前に実行し、\n\nによるテーブル境界を正しく検出する）
	// step 4-1で\n\n→\nに変換される前にテーブルをプレースホルダー化する
	text, tables := mw.extractJIRATables(text)

	// 4-1. JIRA改行の正規化（コードブロック・インラインコード・テーブル保護後、マークアップ変換前に実行）
	// JIRAのWikiマークアップでは改行1つが\n\nとして保存されるため、Markdownの改行に合わせる
	// 4連続以上の改行はユーザーが意図的に入れた空行として<br>タグに変換
	// 前後の\n\nは\n{2,3}正規化で潰されるためプレースホルダーで保護する
	// br数はJIRA改行単位（\n\n=1単位）で計算: brs = n/2 - 1
	const brNewline = "__BR_NL__"
	text = regexp.MustCompile(`\n{4,}`).ReplaceAllStringFunc(text, func(match string) string {
		brs := len(match)/2 - 1
		result := brNewline
		for i := 0; i < brs; i++ {
			if i > 0 {
				result += "\n"
			}
			result += "<br>"
		}
		return result + brNewline
	})
	// 残りの2-3連続改行は1改行に正規化
	text = regexp.MustCompile(`\n{2,3}`).ReplaceAllString(text, "\n")
	// プレースホルダーを\n\nに戻す
	text = strings.ReplaceAll(text, brNewline, "\n\n")
	// 4-1a. テーブルプレースホルダー周囲の空行を確保（step 4-1で\n\n→\nに変換されたため再挿入）
	text = regexp.MustCompile(`([^\n])\n(__TABLE_\d+__)`).ReplaceAllString(text, "$1\n\n$2")
	text = regexp.MustCompile(`(__TABLE_\d+__)\n([^\n])`).ReplaceAllString(text, "$1\n\n$2")
	// 4-1b. コードブロックプレースホルダー周囲の空行を確保（step 4-1で\n\n→\nに変換されたため再挿入）
	text = regexp.MustCompile(`([^\n])\n(__CODE_BLOCK_\d+__)`).ReplaceAllString(text, "$1\n\n$2")
	text = regexp.MustCompile(`(__CODE_BLOCK_\d+__)\n([^\n])`).ReplaceAllString(text, "$1\n\n$2")

	// 5. バックスラッシュをエスケープ（コードブロック・インラインコード保護後に実行）
	// UNCパスなどの \ がMarkdownのエスケープ文字として解釈されるのを防ぐ
	text = strings.ReplaceAll(text, `\`, `\\`)

	// 6. ブレース記法の変換（{quote}, {color}, {status}, {panel}, {note}等）
	// コードブロック保護後、テーブル変換前に処理する
	text = mw.convertQuoteMarkup(text)
	// blockquote前の空行確保（引用ブロックの最初の行の前にのみ空行を挿入）
	// quoteEndBlankLineと同様にマルチラインモードで行頭を判定し、>で始まらない行の後にのみマッチさせる
	quoteStartBlankLine := regexp.MustCompile(`(?m)(^[^>\n][^\n]*)\n(>)`)
	text = quoteStartBlankLine.ReplaceAllString(text, "$1\n\n$2")
	// blockquote後の空行確保（CommonMarkの遅延継続行防止）
	// >で始まる行の直後に非>行が続く場合、lazy continuationで引用ブロックに吸い込まれるため空行を追加
	quoteEndBlankLine := regexp.MustCompile(`(?m)(^>[^\n]*)\n([^>\n])`)
	text = quoteEndBlankLine.ReplaceAllString(text, "$1\n\n$2")
	// 水平線前の空行確保（Setext見出しとして解釈されないようにする）
	// CommonMarkでは非空行の直後に----が続く場合、Setext H2見出しとして解釈されるため空行を追加
	hrBeforeBlankLine := regexp.MustCompile(`([^\n])\n(----+\n)`)
	text = hrBeforeBlankLine.ReplaceAllString(text, "$1\n\n$2")
	// 水平線後の空行確保
	hrAfterBlankLine := regexp.MustCompile(`(----+)\n([^\n])`)
	text = hrAfterBlankLine.ReplaceAllString(text, "$1\n\n$2")
	text = mw.convertStatusLabelMarkup(text) // カスタムステータスラベルを先に変換（より具体的なパターン）
	text = mw.convertColorMarkup(text)
	text = mw.convertStatusMarkup(text)
	text = mw.convertPanelMarkup(text)
	text = mw.convertAdmonitionMarkup(text)
	// パネル後の空行確保（Goldmarkのタイプ6 HTMLブロックを正しく終了させるため）
	// CommonMarkでは<div>等のHTMLブロック(タイプ6)は空行でのみ終了する
	// 改行正規化で</div></div>後の空行が失われた場合、続くMarkdownが生テキストになるため空行を追加する
	panelEndBlankLine := regexp.MustCompile(`(</div></div>)\n([^\n])`)
	text = panelEndBlankLine.ReplaceAllString(text, "$1\n\n$2")

	// 6. テーブルMarkdown変換・置換（抽出は step 4-0 で実施済み）
	for i, table := range tables {
		placeholder := fmt.Sprintf("__TABLE_%d__", i)
		markdownTable := mw.convertJIRATableToMarkdown(table)
		text = strings.ReplaceAll(text, placeholder, markdownTable)
	}

	// 7. メンション変換: [~accountid:xxx] → <span class="mention">@ユーザー名</span>
	mentionPattern := regexp.MustCompile(`\[~accountid:([^\]]+)\]`)
	text = mentionPattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := mentionPattern.FindStringSubmatch(match)
		if len(submatches) >= 2 {
			accountID := submatches[1]

			// account IDからユーザー名を取得
			if userName, exists := mw.userMapping[accountID]; exists && userName != "" {
				return `<span class="mention">@` + userName + `</span>`
			}

			// マッピングが見つからない場合はaccount IDを表示
			return `<span class="mention">@` + accountID + `</span>`
		}
		return match
	})

	// 6-1.5. URLプレフィックス置換（設定ファイルの url_replacements を適用）
	text = mw.applyURLReplacements(text)

	// 6-2. JIRA課題URLを相対パスリンクに変換（リンク変換の前に実行）
	text = mw.convertJIRAIssueLinksToRelative(text, currentProjectKey)

	// 7-0.5. [URL] → [URL](URL)（パイプなしの単純URLリンク、Jira旧仕様）
	// [^\]|\s] で | と空白を除外し、[URL|text] 形式への誤マッチを防ぐ
	// linkPattern の前に適用することで変換済みの [URL](URL) への再マッチを回避する
	simpleLinkPattern := regexp.MustCompile(`\[(https?://[^\]|\s]+)\]`)
	text = simpleLinkPattern.ReplaceAllString(text, `[$1]($1)`)

	// 7. リンク変換: [text|url] → [text](url)、[text|url|smart-link] → [text](url)
	linkPattern := regexp.MustCompile(`\[([^\]|]+)\|([^\]|]+)(?:\|[^\]]+)?\]`)
	text = linkPattern.ReplaceAllString(text, `[$1]($2)`)

	// 7-2. プレーンテキストの課題キー（SCRUM-1等）をMarkdownリンクに変換（プロジェクトキー一覧がある場合のみ）
	if len(mw.projectKeys) > 0 {
		text = mw.convertPlainTextIssueKeysToLinks(text, currentProjectKey)
	}

	// 7-1. Markdownリンクを保護（装飾変換でURL内の~等が誤変換されないようにする）
	mdLinkPattern := regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)
	var protectedLinks []string
	linkProtectIndex := 0
	text = mdLinkPattern.ReplaceAllStringFunc(text, func(match string) string {
		placeholder := fmt.Sprintf("___LINK_PROTECT_%d___", linkProtectIndex)
		protectedLinks = append(protectedLinks, match)
		linkProtectIndex++
		return placeholder
	})

	// 8-1. 見出し変換: h1. - h6. → # - ######（行単位処理）
	// 見出しをプレースホルダーで保護してからリスト変換を実行
	headings := []string{}
	headingIndex := 0
	headingPattern := regexp.MustCompile(`^h([1-6])\.\s+(.+)$`)
	lines := strings.Split(text, "\n")
	var processedLines []string
	for _, line := range lines {
		if matches := headingPattern.FindStringSubmatch(line); matches != nil {
			levelStr := matches[1]
			title := matches[2]
			level, _ := strconv.Atoi(levelStr)
			hashes := strings.Repeat("#", level)
			heading := hashes + " " + title
			placeholder := fmt.Sprintf("__HEADING_%d__", headingIndex)
			headings = append(headings, heading)
			processedLines = append(processedLines, placeholder)
			headingIndex++
		} else {
			processedLines = append(processedLines, line)
		}
	}
	text = strings.Join(processedLines, "\n")

	// 8-2. リスト変換: * → -、# → 1.（行単位処理）
	text = mw.convertJIRAListsToMarkdown(text)

	// 8-3. 見出しプレースホルダーを復元
	for i, heading := range headings {
		placeholder := fmt.Sprintf("__HEADING_%d__", i)
		text = strings.ReplaceAll(text, placeholder, heading)
	}
	// 8-3a. 見出し前後の空行を確保（見出し行の前後に非空行が隣接している場合に空行を挿入）
	text = regexp.MustCompile(`([^\n])\n(#{1,6} )`).ReplaceAllString(text, "$1\n\n$2")
	text = regexp.MustCompile(`(#{1,6} [^\n]*)\n([^\n#])`).ReplaceAllString(text, "$1\n\n$2")
	// 8-3b. リストブロック前後の空行を確保
	text = ensureBlankLinesAroundLists(text)

	// 8-4. リスト行を保護（装飾変換時の衝突回避）
	text, protectedLists := mw.protectListLines(text)

	// 9. 太字: *text* → **text**（日本語対応版）
	// Go の regexp は negative lookahead/lookbehind を サポートしないため、簡略版を使用
	// 単語境界の厳密な要件を緩和し、行頭・行末の * をサポート
	text = convertBoldMarkup(text)

	// 9-1. 太字+イタリック: **_text_** → ***text***
	// convertBoldMarkup 後に **_..._** パターンを ***...*** に変換する
	boldItalicPattern := regexp.MustCompile(`\*\*_([\s\S]*?)_\*\*`)
	text = boldItalicPattern.ReplaceAllString(text, `***$1***`)

	// 10. イタリック: _text_ → *text*（日本語対応版）
	text = convertItalicMarkup(text)

	// 10-1. イタリック変換後に残った _ は本文の記号なので \_ にエスケープ
	text = escapeRemainingUnderscores(text)

	// 11. 削除線: -text- → ~~text~~（日付・URL対応版）
	text = convertStrikethroughMarkup(text)

	// 12. 上付き: ^text^ → <sup>text</sup>
	supPattern := regexp.MustCompile(`\^([^\^]+)\^`)
	text = supPattern.ReplaceAllString(text, `<sup>$1</sup>`)

	// 13. 下付き: ~text~ → <sub>text</sub>
	// ~~は取り消し線なので除外する必要がある
	// ~~で囲まれた部分を一時的に保護する
	strikeProtectPattern := regexp.MustCompile(`~~[^~]*~~`)
	strikes := strikeProtectPattern.FindAllString(text, -1)
	strikeProtectIndex := 0
	text = strikeProtectPattern.ReplaceAllStringFunc(text, func(match string) string {
		placeholder := fmt.Sprintf("___STRIKE_PROTECT_%d___", strikeProtectIndex)
		strikeProtectIndex++
		return placeholder
	})

	// 下付き処理
	subPattern := regexp.MustCompile(`~([^~]+?)~`)
	text = subPattern.ReplaceAllString(text, `<sub>$1</sub>`)

	// 取り消し線を復元
	for i, strike := range strikes {
		placeholder := fmt.Sprintf("___STRIKE_PROTECT_%d___", i)
		text = strings.Replace(text, placeholder, strike, 1)
	}

	// 14. 下線: +text+ → <u>text</u>
	underlinePattern := regexp.MustCompile(`\+([^\+\n]+?)\+`)
	text = underlinePattern.ReplaceAllString(text, `<u>$1</u>`)

	// 7-2. Markdownリンクを復元
	for i, link := range protectedLinks {
		placeholder := fmt.Sprintf("___LINK_PROTECT_%d___", i)
		text = strings.Replace(text, placeholder, link, 1)
	}

	// 8-5. リスト行を復元
	text = mw.restoreListLines(text, protectedLists)

	// 13-9. 三重以上の改行を二重改行に正規化（複数の空行挿入処理の重複を防止）
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")

	// 14. プレースホルダーを元のコードブロックとインラインコードに戻す
	// リスト内のコードブロックは2行目以降に同じインデントを追加してネストを正しく表現する
	for i, codeBlock := range codeBlocks {
		placeholder := fmt.Sprintf("__CODE_BLOCK_%d__", i)
		text = restoreCodeBlockWithIndent(text, placeholder, codeBlock)
	}
	for i, inlineCode := range inlineCodes {
		placeholder := fmt.Sprintf("__INLINE_CODE_%d__", i)
		text = strings.ReplaceAll(text, placeholder, inlineCode)
	}

	return text
}

// ensureBlankLinesAroundLists はリストブロックの前後に空行を挿入する
// リスト行（- や 1. で始まる行）のかたまりの先頭前と末尾後に空行を追加する
// リスト継続行（2スペース以上のインデントを持つ非リスト行）はリストブロックの一部として扱う
func ensureBlankLinesAroundLists(text string) string {
	lines := strings.Split(text, "\n")
	listLinePattern := regexp.MustCompile(`^(\s*)(- |1\. )`)
	var result []string

	for i, line := range lines {
		isList := listLinePattern.MatchString(line)

		// リストブロックの先頭行の前に空行を挿入（前行が非リスト・非継続・非空行の場合）
		if isList && i > 0 {
			prevLine := lines[i-1]
			if !listLinePattern.MatchString(prevLine) && prevLine != "" && !isListContinuationLine(prevLine) {
				result = append(result, "")
			}
		}

		result = append(result, line)

		// リストブロックの最終行の後に空行を挿入（次行が非リスト・非継続・非空行の場合）
		if isList && i < len(lines)-1 {
			nextLine := lines[i+1]
			if !listLinePattern.MatchString(nextLine) && nextLine != "" && !isListContinuationLine(nextLine) {
				result = append(result, "")
			}
		}
	}

	return strings.Join(result, "\n")
}

// isListContinuationLine はリスト項目の継続行かどうかを判定する
// convertJIRAListsToMarkdown が継続行に付与するインデント（最低2スペース）で識別する
func isListContinuationLine(line string) bool {
	return len(line) >= 2 && line[0] == ' ' && line[1] == ' '
}

// ensureBlankLinesAroundImages は画像リンク `![alt](path)` の前後に空行を挿入する
func ensureBlankLinesAroundImages(text string) string {
	imgPattern := `!\[[^\]]*\]\([^)]*\)`
	// インライン画像をそれぞれ独立した行に分離（前後に\nを挿入）
	text = regexp.MustCompile(`([^\n])(`+imgPattern+`)`).ReplaceAllString(text, "$1\n$2")
	text = regexp.MustCompile(`(`+imgPattern+`)([^\n])`).ReplaceAllString(text, "$1\n$2")
	// 行ベースで画像行の前後に空行を挿入
	imgLinePattern := regexp.MustCompile(`^` + imgPattern + `$`)
	lines := strings.Split(text, "\n")
	var result []string
	for i, line := range lines {
		isImage := imgLinePattern.MatchString(line)
		if isImage && i > 0 && lines[i-1] != "" {
			result = append(result, "")
		}
		result = append(result, line)
		// 次行が画像の場合は、次画像の「前への空行挿入」に任せる
		if isImage && i < len(lines)-1 && lines[i+1] != "" && !imgLinePattern.MatchString(lines[i+1]) {
			result = append(result, "")
		}
	}
	return strings.Join(result, "\n")
}

// convertJIRAHeadingsToMarkdown は JIRA の見出しマークアップを Markdown に変換する
// h1. 見出し → # 見出し
// h2. 見出し → ## 見出し
func (mw *MarkdownWriter) convertJIRAHeadingsToMarkdown(text string) string {
	lines := strings.Split(text, "\n")
	result := make([]string, 0, len(lines))

	headingPattern := regexp.MustCompile(`^h([1-6])\.\s+(.+)$`)

	for _, line := range lines {
		matches := headingPattern.FindStringSubmatch(line)
		if len(matches) == 3 {
			levelStr := matches[1]
			title := matches[2]
			level, _ := strconv.Atoi(levelStr)
			hashes := strings.Repeat("#", level)
			converted := hashes + " " + title
			result = append(result, converted)
		} else {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// convertJIRAListsToMarkdown は JIRA のリストマークアップを Markdown に変換する
// * リスト → - リスト
// ** りすと2 → (4スペース)- りすと2
// # リスト → 1. リスト
// ## りすと2 → (4スペース)1. りすと2
// #* 混在 → (4スペース)- 混在  （連番の子として箇条書き）
// *# 混在 → (4スペース)1. 混在 （箇条書きの子として連番）
// リスト項目間の非リスト行（垂直タブ等で挿入されたテキスト）は継続行として扱い、
// 直前のリスト項目の末尾に <br> で結合して改行を保持する
func (mw *MarkdownWriter) convertJIRAListsToMarkdown(text string) string {
	lines := strings.Split(text, "\n")
	result := make([]string, 0, len(lines))

	// 古いJIRAでは先頭にスペースが入ることがあるため、^\s* で先頭の空白を許容
	// [*#]{1,6} で * と # の混在プレフィックスにも対応
	listPattern := regexp.MustCompile(`^\s*([*#]{1,6})\s+(.*)$`)

	inListContext := false
	baseLevel := 0                   // 現在のリストブロックの基準レベル
	listWasSeen := false             // ドキュメント内でリスト項目が1つ以上出現したか
	nonListContentAfterList := false // リスト出現後に非空・非リスト行が出現したか

	for i, line := range lines {
		matches := listPattern.FindStringSubmatch(line)
		if len(matches) == 3 {
			prefix := matches[1]
			content := matches[2]
			level := len(prefix) - 1

			// リストがコードブロック等（空行を挟んでも）で中断された後の再開:
			// 前のリストブロックのベースレベルを維持せず、再開ポイントのレベルを新しい基準とする
			if nonListContentAfterList {
				baseLevel = level
				nonListContentAfterList = false
			}
			// listWasSeen == false（ドキュメント先頭から始まる単独の深いリストなど）は
			// baseLevel = 0 のまま絶対レベルを使用（既存の動作を維持）

			// ベースレベルからの相対レベルでインデントを計算
			effectiveLevel := level - baseLevel
			if effectiveLevel < 0 {
				effectiveLevel = 0
			}
			indent := strings.Repeat("    ", effectiveLevel)
			// プレフィックスの最後の文字でリストマーカーを決定
			var marker string
			if prefix[len(prefix)-1] == '*' {
				marker = "- "
			} else {
				marker = "1. "
			}
			if content == "" {
				content = "&nbsp;"
			}
			result = append(result, indent+marker+content)
			inListContext = true
			listWasSeen = true
		} else if line == "" {
			// 空行はリストコンテキストをリセット（nonListContentAfterListはクリアしない）
			result = append(result, line)
			inListContext = false
		} else if inListContext && hasFollowingListLine(lines, i+1, listPattern) {
			// 継続行: 後続にリスト行があるため、直前のリスト項目に <br> で結合して改行を保持
			if len(result) > 0 {
				result[len(result)-1] = result[len(result)-1] + "<br>" + line
			} else {
				result = append(result, line)
			}
		} else if listWasSeen && !nonListContentAfterList && isCodeBlockPlaceholder(line) {
			// コードブロックプレースホルダーがリスト出現後に現れた場合:
			// 次のリスト項目のレベルを前方走査して、コードブロックをリスト内にインデントする
			nextLevel := findNextListLevel(lines, i+1, listPattern)
			if nextLevel >= 0 {
				nextEffective := nextLevel - baseLevel
				if nextEffective < 0 {
					nextEffective = 0
				}
				// コードブロックはリスト項目のコンテント列 = (nextEffective+1)*4 スペースに配置
				codeIndent := strings.Repeat("    ", nextEffective+1)
				result = append(result, codeIndent+line)
			} else {
				// 後続にリストなし → インデントなし（リスト外コードブロック）
				result = append(result, line)
			}
			inListContext = false
			// nonListContentAfterList はセットしない（baseLevelを維持してネストを保持）
		} else {
			result = append(result, line)
			// リストが出現した後に非空・非リスト行が現れた場合、次のリストブロックの基準レベルをリセットする
			// （inListContextがfalseでも空行を挟んだコンテンツ後のリスト再開に対応）
			if listWasSeen {
				nonListContentAfterList = true
			}
			inListContext = false
		}
	}

	return strings.Join(result, "\n")
}

// hasFollowingListLine は lines[startIdx:] の中に JIRA リスト行が存在するか判定する
// 空行が現れた時点で探索を打ち切る
func hasFollowingListLine(lines []string, startIdx int, listPattern *regexp.Regexp) bool {
	for i := startIdx; i < len(lines); i++ {
		if listPattern.MatchString(lines[i]) {
			return true
		}
		if lines[i] == "" {
			return false
		}
	}
	return false
}

// isCodeBlockPlaceholder はコードブロックプレースホルダー行かどうかを判定する
func isCodeBlockPlaceholder(line string) bool {
	matched, _ := regexp.MatchString(`^__CODE_BLOCK_\d+__$`, strings.TrimSpace(line))
	return matched
}

// findNextListLevel は lines[startIdx:] の中で最初に現れる JIRA リスト行のレベル（prefix長-1）を返す
// 空行や非リスト行はスキップし、見つからなければ -1 を返す
func findNextListLevel(lines []string, startIdx int, listPattern *regexp.Regexp) int {
	for i := startIdx; i < len(lines); i++ {
		if matches := listPattern.FindStringSubmatch(lines[i]); len(matches) == 3 {
			return len(matches[1]) - 1
		}
	}
	return -1
}

// protectListLines はリスト行を一時的にプレースホルダーに置き換えて保護します
// 装飾記号の変換時にリストマーカー（*）との衝突を防ぐために使用します
func (mw *MarkdownWriter) protectListLines(text string) (string, []string) {
	lines := strings.Split(text, "\n")
	var result []string
	var protected []string

	// JIRA リストパターン（* と # の混在プレフィックスに対応）
	// 古いJIRAでは先頭にスペースが入ることがあるため、^\s* で先頭の空白を許容
	listPattern := regexp.MustCompile(`^\s*[*#]{1,6}\s+.*$`)
	// Markdownの見出しパターン（行頭から#が始まる、スペースなし）
	markdownHeadingPattern := regexp.MustCompile(`^#{1,6}\s+.+$`)

	for _, line := range lines {
		// Markdownの見出しは保護対象から除外
		if markdownHeadingPattern.MatchString(line) {
			result = append(result, line)
			continue
		}

		if listPattern.MatchString(line) {
			// リスト行をプレースホルダーに置き換え
			// 修正: 元の行番号iではなく、protected配列のインデックスを使用
			placeholder := fmt.Sprintf("___LIST_PLACEHOLDER_%d___", len(protected))
			result = append(result, placeholder)
			protected = append(protected, line)
		} else {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n"), protected
}

// restoreListLines はプレースホルダーを元のリスト行に戻します
func (mw *MarkdownWriter) restoreListLines(text string, protected []string) string {
	result := text
	for i, line := range protected {
		placeholder := fmt.Sprintf("___LIST_PLACEHOLDER_%d___", i)
		result = strings.Replace(result, placeholder, line, 1)
	}
	return result
}

// restoreCodeBlockWithIndent はコードブロックプレースホルダーを復元する。
// プレースホルダーがリスト項目内にある場合（行の先頭にスペースあり）、
// コードブロックの2行目以降にリスト項目と同じインデントを追加して
// Markdownのネストを正しく表現する。
func restoreCodeBlockWithIndent(text, placeholder, codeBlock string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if !strings.Contains(line, placeholder) {
			continue
		}
		// 行の先頭スペースを取得（リスト項目のインデントレベル）
		indent := ""
		for _, ch := range line {
			if ch == ' ' || ch == '\t' {
				indent += string(ch)
			} else {
				break
			}
		}
		// リスト項目内の場合（第1階層も含む）、2行目以降にインデント+4スペースを追加
		// インデントの有無ではなくリストマーカーの有無で判定することで第1階層も対応する
		trimmed := strings.TrimLeft(line, " \t")
		isList := strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "1. ")
		// インデント付きスタンドアロンプレースホルダー: リスト変換でインデントされた __CODE_BLOCK_N__
		isIndentedPlaceholder := !isList && len(indent) > 0 && strings.TrimSpace(line) == placeholder
		if isList {
			contentIndent := indent + "    "
			codeLines := strings.Split(codeBlock, "\n")
			for j := 1; j < len(codeLines); j++ {
				if codeLines[j] != "" {
					codeLines[j] = contentIndent + codeLines[j]
				}
			}
			lines[i] = strings.ReplaceAll(line, placeholder, strings.Join(codeLines, "\n"))
		} else if isIndentedPlaceholder {
			// 行全体をインデント付きコードブロックで置換（全行にindentを付与）
			codeLines := strings.Split(codeBlock, "\n")
			for j := range codeLines {
				if codeLines[j] != "" {
					codeLines[j] = indent + codeLines[j]
				}
			}
			lines[i] = strings.Join(codeLines, "\n")
		} else {
			lines[i] = strings.ReplaceAll(line, placeholder, codeBlock)
		}
	}
	return strings.Join(lines, "\n")
}

// convertBoldMarkup は*text*を**text**に変換します（日本語対応）
// 既に**で囲まれている場合は誤変換を避けます
func convertBoldMarkup(text string) string {
	lines := strings.Split(text, "\n")
	var result []string

	for _, line := range lines {
		// 既に**で囲まれている部分を保護するため、複数回のマッチングを試行
		// パターン：*text*（**textではない）
		converted := line

		// 簡単なパターン：*text*の形式（*の間に0個以上の非*文字）
		pattern := regexp.MustCompile(`\*([^\*\n]+?)\*`)

		for {
			prev := converted
			// マッチする部分を検出
			matches := pattern.FindAllStringSubmatchIndex(converted, -1)
			if len(matches) == 0 {
				break
			}

			// 後ろから処理（インデックスを保つため）
			for i := len(matches) - 1; i >= 0; i-- {
				match := matches[i]
				// マッチ位置から、既に**で囲まれていないかチェック
				start := match[0]
				end := match[1]

				// 前後の文字をチェック
				isBold := false
				if start > 0 && converted[start-1] == '*' {
					// 既に**で囲まれている可能性
					isBold = true
				}
				if end < len(converted) && converted[end] == '*' {
					// 既に**で囲まれている
					isBold = true
				}

				if !isBold {
					// *text* → **text**に変換
					matchText := converted[match[2]:match[3]]
					replacement := fmt.Sprintf("**%s**", matchText)
					converted = converted[:start] + replacement + converted[end:]
					break
				}
			}

			if converted == prev {
				break // 変更がなければ終了
			}
		}

		result = append(result, converted)
	}

	return strings.Join(result, "\n")
}

// fenceForContent はコードブロックの内容に応じて適切なフェンス文字列を返します。
// 内容に連続バッククオートが含まれる場合、その最長より1つ多いバッククオート数のフェンスを返します。
// 最低3つのバッククオートを保証します。
func fenceForContent(content string) string {
	maxRun := 0
	currentRun := 0
	for _, ch := range content {
		if ch == '`' {
			currentRun++
			if currentRun > maxRun {
				maxRun = currentRun
			}
		} else {
			currentRun = 0
		}
	}
	fenceLen := maxRun + 1
	if fenceLen < 3 {
		fenceLen = 3
	}
	return strings.Repeat("`", fenceLen)
}

// isInvalidItalicBoundary は、文字がイタリック記法の無効な境界文字かどうかを判定します。
// 正規表現の記号（%, \, [, ]など）に隣接した _ はイタリック記法として無効です。
func isInvalidItalicBoundary(r rune) bool {
	switch r {
	case '%', '\\', '[', ']', '$', '(', ')', '{', '}', '+', '^', '|', '?', '.':
		return true
	}
	return false
}

// isAlphanumeric は、文字が英数字かどうかを判定します。
func isAlphanumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || (r >= '０' && r <= '９')
}

// convertItalicMarkup は_text_を*text*に変換します（日本語対応）
func convertItalicMarkup(text string) string {
	lines := strings.Split(text, "\n")
	var result []string

	for _, line := range lines {
		converted := line

		// パターン：_text_の形式（_の間に1個以上の非_文字）
		pattern := regexp.MustCompile(`_([^_\n]+?)_`)

		for {
			prev := converted
			matches := pattern.FindAllStringSubmatchIndex(converted, -1)
			if len(matches) == 0 {
				break
			}

			// 後ろから処理
			for i := len(matches) - 1; i >= 0; i-- {
				match := matches[i]
				start := match[0]
				end := match[1]

				// 前後の文字をチェック（既に*で囲まれているかチェック）
				isItalic := false
				if start > 0 && converted[start-1] == '_' {
					isItalic = true
				}
				if end < len(converted) && converted[end] == '_' {
					isItalic = true
				}

				// 前の文字をチェック
				var prevChar, nextChar rune
				prevIsAlphanumeric := false
				nextIsAlphanumeric := false

				if !isItalic && start > 0 {
					prevRunes := []rune(converted[:start])
					if len(prevRunes) > 0 {
						prevChar = prevRunes[len(prevRunes)-1]
						prevIsAlphanumeric = isAlphanumeric(prevChar)
						// 無効な境界記号かチェック
						if isInvalidItalicBoundary(prevChar) {
							isItalic = true
						}
					}
				}

				// 後の文字をチェック
				if !isItalic && end < len(converted) {
					nextRunes := []rune(converted[end:])
					if len(nextRunes) > 0 {
						nextChar = nextRunes[0]
						nextIsAlphanumeric = isAlphanumeric(nextChar)
						// 無効な境界記号かチェック
						if isInvalidItalicBoundary(nextChar) {
							isItalic = true
						}
					}
				}

				// スネークケース判定：前後どちらかが英数字なら無効（FOO_BAR_ や Japanese_Bushu_Kakusu_140_CI_SA など）
				if !isItalic && (prevIsAlphanumeric || nextIsAlphanumeric) {
					isItalic = true
				}

				if !isItalic {
					// _text_ → *text*に変換
					matchText := converted[match[2]:match[3]]
					replacement := fmt.Sprintf("*%s*", matchText)
					converted = converted[:start] + replacement + converted[end:]
					break
				}
			}

			if converted == prev {
				break
			}
		}

		result = append(result, converted)
	}

	return strings.Join(result, "\n")
}

// escapeRemainingUnderscores はイタリック変換後に残った _ を \_ にエスケープする。
// プレースホルダーやMarkdownリンクのURLの _ はエスケープしない。
func escapeRemainingUnderscores(text string) string {
	if !strings.Contains(text, "_") {
		return text
	}

	const markerStart = "\uE000"
	const markerEnd = "\uE001"

	// 1. プレースホルダーを保護: __XXX__ や ___XXX___ 形式
	phPattern := regexp.MustCompile(`_{2,}[A-Z][A-Z_0-9]+_{2,}`)
	var phs []string
	text = phPattern.ReplaceAllStringFunc(text, func(match string) string {
		idx := len(phs)
		phs = append(phs, match)
		return fmt.Sprintf("%sPH%d%s", markerStart, idx, markerEnd)
	})

	// 2. MarkdownリンクのURL部分を保護: ](url)
	urlPattern := regexp.MustCompile(`\]\([^)]*\)`)
	var urls []string
	text = urlPattern.ReplaceAllStringFunc(text, func(match string) string {
		idx := len(urls)
		urls = append(urls, match)
		return fmt.Sprintf("%sURL%d%s", markerStart, idx, markerEnd)
	})

	// 2-1. ベアURL（裸のURL）を保護: https://... または http://...
	// autolinkでリンク化される際にURLが壊れないよう、_ をエスケープしない
	bareURLPattern := regexp.MustCompile(`https?://[^\s\)>]+`)
	var bareURLs []string
	text = bareURLPattern.ReplaceAllStringFunc(text, func(match string) string {
		idx := len(bareURLs)
		bareURLs = append(bareURLs, match)
		return fmt.Sprintf("%sBURL%d%s", markerStart, idx, markerEnd)
	})

	// 3. 全ての _ を \_ にエスケープ
	text = strings.ReplaceAll(text, "_", `\_`)

	// 3-1. ベアURLを復元
	for i, burl := range bareURLs {
		text = strings.Replace(text, fmt.Sprintf("%sBURL%d%s", markerStart, i, markerEnd), burl, 1)
	}

	// 4. URLを復元
	for i, url := range urls {
		text = strings.Replace(text, fmt.Sprintf("%sURL%d%s", markerStart, i, markerEnd), url, 1)
	}

	// 5. プレースホルダーを復元
	for i, ph := range phs {
		text = strings.Replace(text, fmt.Sprintf("%sPH%d%s", markerStart, i, markerEnd), ph, 1)
	}

	return text
}

// convertStrikethroughMarkup は-text-を~~text~~に変換します（日付・URL・リストアイテム対応）
func convertStrikethroughMarkup(text string) string {
	lines := strings.Split(text, "\n")
	var result []string

	for _, line := range lines {
		converted := line

		// パターン：-text-の形式（-の間に1個以上の非-文字、空白のみは除外、リスト要素（-空白）も除去）
		pattern := regexp.MustCompile(`-([^- \n]+?)-`)

		for {
			prev := converted
			matches := pattern.FindAllStringSubmatchIndex(converted, -1)
			if len(matches) == 0 {
				break
			}

			// 後ろから処理
			for i := len(matches) - 1; i >= 0; i-- {
				match := matches[i]
				start := match[0]
				end := match[1]

				// 前後の文字をチェック（マルチバイト文字対応）
				shouldSkip := false

				// キャプチャグループの内容をチェック（空白のみは変換しない）
				matchContent := converted[match[2]:match[3]]
				if strings.TrimSpace(matchContent) == "" {
					shouldSkip = true
				}

				// リストアイテムのマーカー（行頭の "- "）は変換しない
				if start == 0 && len(matchContent) > 0 && matchContent[0] == ' ' {
					shouldSkip = true
				}

				// 前の文字をチェック
				if !shouldSkip && start > 0 {
					prevRune, _ := utf8.DecodeLastRuneInString(converted[:start])
					if prevRune != utf8.RuneError {
						// ASCII英数字または記号(-/:)の場合のみスキップ
						// 日本語などのマルチバイト文字は変換を許可
						if (prevRune >= '0' && prevRune <= '9') ||
							(prevRune >= 'a' && prevRune <= 'z') ||
							(prevRune >= 'A' && prevRune <= 'Z') ||
							prevRune == '-' || prevRune == '/' || prevRune == ':' {
							shouldSkip = true
						}
					}
				}

				// 後の文字をチェック
				if !shouldSkip && end < len(converted) {
					nextRune, _ := utf8.DecodeRuneInString(converted[end:])
					if nextRune != utf8.RuneError {
						// ASCII英数字または記号(-/:)の場合のみスキップ
						// 日本語などのマルチバイト文字は変換を許可
						if (nextRune >= '0' && nextRune <= '9') ||
							(nextRune >= 'a' && nextRune <= 'z') ||
							(nextRune >= 'A' && nextRune <= 'Z') ||
							nextRune == '-' || nextRune == '/' || nextRune == ':' {
							shouldSkip = true
						}
					}
				}

				// 既に~~で囲まれているかチェック
				if !shouldSkip && start > 1 && converted[start-1:start] == "~" && converted[start-2:start-1] == "~" {
					shouldSkip = true
				}
				if !shouldSkip && end+1 < len(converted) && converted[end:end+1] == "~" && end+2 < len(converted) && converted[end+1:end+2] == "~" {
					shouldSkip = true
				}

				if !shouldSkip {
					// -text- → ~~text~~に変換
					replacement := fmt.Sprintf("~~%s~~", matchContent)
					converted = converted[:start] + replacement + converted[end:]
					break
				}
			}

			if converted == prev {
				break
			}
		}

		result = append(result, converted)
	}

	return strings.Join(result, "\n")
}
