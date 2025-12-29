package main

import (
	"strings"
	"testing"
	"time"

	"github.com/andygrunwald/go-jira/v2/cloud"
)

func TestExtractJIRATables(t *testing.T) {
	mw := NewMarkdownWriter("", "", nil)

	tests := []struct {
		name           string
		input          string
		expectedText   string
		expectedTables []string
	}{
		{
			name:           "ヘッダー付きテーブル（基本）",
			input:          "||Header 1||Header 2||\n|Data 1|Data 2|",
			expectedText:   "__TABLE_0__",
			expectedTables: []string{"||Header 1||Header 2||\n|Data 1|Data 2|"},
		},
		{
			name:           "ヘッダー無しテーブル（1行）",
			input:          "|Data 1|Data 2|",
			expectedText:   "__TABLE_0__",
			expectedTables: []string{"|Data 1|Data 2|"},
		},
		{
			name:           "ヘッダー無しテーブル（複数行）",
			input:          "|Data 1|Data 2|\n|Data 3|Data 4|",
			expectedText:   "__TABLE_0__",
			expectedTables: []string{"|Data 1|Data 2|\n|Data 3|Data 4|"},
		},
		{
			name:           "セル内改行を含むヘッダー付きテーブル",
			input:          "||Header||\n|Line1\nLine2|",
			expectedText:   "__TABLE_0__",
			expectedTables: []string{"||Header||\n|Line1\nLine2|"},
		},
		{
			name:           "セル内改行を含むヘッダー無しテーブル",
			input:          "|Line1\nLine2|",
			expectedText:   "__TABLE_0__",
			expectedTables: []string{"|Line1\nLine2|"},
		},
		{
			name:           "ヘッダー付きとヘッダー無しが混在",
			input:          "||Header||\n|Data 1|\n\n|Data 2|\n|Data 3|",
			expectedText:   "__TABLE_0__\n\n__TABLE_1__",
			expectedTables: []string{"||Header||\n|Data 1|", "|Data 2|\n|Data 3|"},
		},
		{
			name:           "テーブルが無い場合",
			input:          "This is normal text",
			expectedText:   "This is normal text",
			expectedTables: []string{},
		},
		{
			name:           "空の入力",
			input:          "",
			expectedText:   "",
			expectedTables: []string{},
		},
		{
			name:           "テーブルの前後にテキストがある場合",
			input:          "Text before\n|Data|\nText after",
			expectedText:   "Text before\n__TABLE_0__\nText after",
			expectedTables: []string{"|Data|"},
		},
		{
			name:           "空行で区切られた複数のテーブル",
			input:          "|Table 1|\n\n|Table 2|",
			expectedText:   "__TABLE_0__\n\n__TABLE_1__",
			expectedTables: []string{"|Table 1|", "|Table 2|"},
		},
		{
			name:           "ヘッダー付きテーブル（複数行）",
			input:          "||H1||H2||\n|A1|A2|\n|B1|B2|",
			expectedText:   "__TABLE_0__",
			expectedTables: []string{"||H1||H2||\n|A1|A2|\n|B1|B2|"},
		},
		{
			name:           "複数のヘッダー無しテーブル",
			input:          "|T1 R1|\n|T1 R2|\n\n|T2 R1|\n|T2 R2|",
			expectedText:   "__TABLE_0__\n\n__TABLE_1__",
			expectedTables: []string{"|T1 R1|\n|T1 R2|", "|T2 R1|\n|T2 R2|"},
		},
		{
			name:           "テーブルとテキストが混在",
			input:          "Start\n||Header||\n|Data|\nMiddle\n|Row1|\n|Row2|\nEnd",
			expectedText:   "Start\n__TABLE_0__\nMiddle\n__TABLE_1__\nEnd",
			expectedTables: []string{"||Header||\n|Data|", "|Row1|\n|Row2|"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, tables := mw.extractJIRATables(tt.input)

			if text != tt.expectedText {
				t.Errorf("expected text:\n%q\ngot:\n%q", tt.expectedText, text)
			}

			if len(tables) != len(tt.expectedTables) {
				t.Errorf("expected %d tables, got %d", len(tt.expectedTables), len(tables))
				t.Errorf("expected tables: %v", tt.expectedTables)
				t.Errorf("got tables: %v", tables)
				return
			}

			for i, expectedTable := range tt.expectedTables {
				if tables[i] != expectedTable {
					t.Errorf("table[%d]:\nexpected:\n%q\ngot:\n%q", i, expectedTable, tables[i])
				}
			}
		})
	}
}

func TestConvertJIRATableToMarkdown(t *testing.T) {
	mw := NewMarkdownWriter("", "", nil)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:  "ヘッダー付きテーブル",
			input: "||Header 1||Header 2||\n|Data 1|Data 2|",
			expected: "| Header 1 | Header 2 |\n" +
				"| ------ | ------ |\n" +
				"| Data 1 | Data 2 |",
		},
		{
			name:  "ヘッダー無しテーブル（1行）",
			input: "|Data 1|Data 2|",
			expected: "|   |   |\n" +
				"| ------ | ------ |\n" +
				"| Data 1 | Data 2 |",
		},
		{
			name:  "ヘッダー無しテーブル（複数行）",
			input: "|Data 1|Data 2|\n|Data 3|Data 4|",
			expected: "|   |   |\n" +
				"| ------ | ------ |\n" +
				"| Data 1 | Data 2 |\n" +
				"| Data 3 | Data 4 |",
		},
		{
			name:  "ヘッダー無しテーブル（3セル）",
			input: "|A|B|C|\n|D|E|F|",
			expected: "|   |   |   |\n" +
				"| ------ | ------ | ------ |\n" +
				"| A | B | C |\n" +
				"| D | E | F |",
		},
		{
			name:  "セル内改行を含むヘッダー付きテーブル",
			input: "||Header||\n|Line1\nLine2|",
			expected: "| Header |\n" +
				"| ------ |\n" +
				"| Line1<br>Line2 |",
		},
		{
			name:  "セル内改行を含むヘッダー無しテーブル",
			input: "|Line1\nLine2|",
			expected: "|   |\n" +
				"| ------ |\n" +
				"| Line1<br>Line2 |",
		},
		{
			name:  "複数のセル内改行",
			input: "|Line1\nLine2\nLine3|",
			expected: "|   |\n" +
				"| ------ |\n" +
				"| Line1<br>Line2<br>Line3 |",
		},
		{
			name:  "複数セルにそれぞれセル内改行",
			input: "|Cell1Line1\nCell1Line2|Cell2Line1\nCell2Line2|",
			expected: "|   |   |\n" +
				"| ------ | ------ |\n" +
				"| Cell1Line1<br>Cell1Line2 | Cell2Line1<br>Cell2Line2 |",
		},
		{
			name: "ヘッダー付きテーブルでヘッダーとデータ両方にセル内改行",
			input: "||Header1\nLine2||Header2\nLine2||\n|Data1\nLine2|Data2\nLine2|",
			expected: "| Header1<br>Line2 | Header2<br>Line2 |\n" +
				"| ------ | ------ |\n" +
				"| Data1<br>Line2 | Data2<br>Line2 |",
		},
		{
			name:  "ヘッダー無し・複数行・各セルにセル内改行",
			input: "|R1C1L1\nR1C1L2|R1C2L1\nR1C2L2|\n|R2C1L1\nR2C1L2|R2C2L1\nR2C2L2|",
			expected: "|   |   |\n" +
				"| ------ | ------ |\n" +
				"| R1C1L1<br>R1C1L2 | R1C2L1<br>R1C2L2 |\n" +
				"| R2C1L1<br>R2C1L2 | R2C2L1<br>R2C2L2 |",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mw.convertJIRATableToMarkdown(tt.input)

			if result != tt.expected {
				t.Errorf("expected:\n%s\n\ngot:\n%s", tt.expected, result)
			}
		})
	}
}

func TestConvertJIRAMention(t *testing.T) {
	userMapping := UserMapping{
		"557058:6eed56ba-9b9b-4a87-ad74-18b7086f1063": "牛頭",
		"123456:abcdef": "太郎",
	}
	mw := &MarkdownWriter{
		outputDir:      "",
		attachmentsDir: "",
		userMapping:    userMapping,
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "単一のメンション",
			input:    "[~accountid:557058:6eed56ba-9b9b-4a87-ad74-18b7086f1063]さん、こんにちは",
			expected: "@牛頭さん、こんにちは",
		},
		{
			name:     "複数のメンション",
			input:    "[~accountid:557058:6eed56ba-9b9b-4a87-ad74-18b7086f1063]と[~accountid:123456:abcdef]",
			expected: "@牛頭と@太郎",
		},
		{
			name:     "マッピングが存在しない場合",
			input:    "[~accountid:unknown]",
			expected: "@unknown",
		},
		{
			name:     "メンション無し",
			input:    "通常のテキストです",
			expected: "通常のテキストです",
		},
		{
			name:     "メンションが文章中に混在",
			input:    "こんにちは、[~accountid:557058:6eed56ba-9b9b-4a87-ad74-18b7086f1063]さん。レビューをお願いします。",
			expected: "こんにちは、@牛頭さん。レビューをお願いします。",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mw.convertJIRAMarkupToMarkdown(tt.input)

			if result != tt.expected {
				t.Errorf("expected:\n%q\n\ngot:\n%q", tt.expected, result)
			}
		})
	}
}

func TestDuedateField(t *testing.T) {
	mw := NewMarkdownWriter("", "", nil)

	tests := []struct {
		name           string
		duedate        cloud.Date
		expectDuedate  bool
		expectedString string
	}{
		{
			name:           "期限が設定されている場合",
			duedate:        cloud.Date(time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)),
			expectDuedate:  true,
			expectedString: "- **期限**: 2025-01-31",
		},
		{
			name:           "期限が設定されていない場合（ゼロ値）",
			duedate:        cloud.Date{},
			expectDuedate:  false,
			expectedString: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// モックのIssueを作成
			issue := &cloud.Issue{
				Key: "TEST-123",
				Fields: &cloud.IssueFields{
					Type: cloud.IssueType{
						Name: "タスク",
					},
					Status: &cloud.Status{
						Name: "未着手",
					},
					Priority: &cloud.Priority{
						Name: "中",
					},
					Reporter: &cloud.User{
						DisplayName: "テスト報告者",
					},
					Assignee: &cloud.User{
						DisplayName: "テスト担当者",
					},
					Summary:     "テスト課題",
					Description: "テスト説明",
					Created:     cloud.Time(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
					Updated:     cloud.Time(time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)),
					Duedate:     tt.duedate,
					Project: cloud.Project{
						Key:  "TEST",
						Name: "テストプロジェクト",
					},
				},
			}

			// generateMarkdownを呼び出し
			result := mw.generateMarkdown(issue, []string{}, nil)

			// 期限フィールドの有無を確認
			if tt.expectDuedate {
				if !strings.Contains(result, tt.expectedString) {
					t.Errorf("期限フィールドが出力されていません\n期待する文字列: %q\n実際の出力:\n%s", tt.expectedString, result)
				}
			} else {
				if strings.Contains(result, "**期限**") {
					t.Errorf("期限フィールドが出力されるべきではありません\n実際の出力:\n%s", result)
				}
			}
		})
	}
}

func TestTimeTrackingFields(t *testing.T) {
	mw := NewMarkdownWriter("", "", nil)

	tests := []struct {
		name          string
		timeTracking  *cloud.TimeTracking
		expectStrings []string
		notExpect     []string
	}{
		{
			name: "すべての時間管理フィールドが設定されている場合",
			timeTracking: &cloud.TimeTracking{
				OriginalEstimateSeconds:  26100, // 7.25h
				RemainingEstimateSeconds: 5400,  // 1.50h
				TimeSpentSeconds:         3600,  // 1.00h
			},
			expectStrings: []string{
				"- **初期見積り**: 7.25h",
				"- **残り時間**: 1.50h",
				"- **作業時間**: 1.00h",
			},
			notExpect: []string{},
		},
		{
			name: "一部のフィールドのみ設定されている場合",
			timeTracking: &cloud.TimeTracking{
				OriginalEstimateSeconds: 10800, // 3.00h
				TimeSpentSeconds:        5400,  // 1.50h
			},
			expectStrings: []string{
				"- **初期見積り**: 3.00h",
				"- **作業時間**: 1.50h",
			},
			notExpect: []string{
				"- **残り時間**:",
			},
		},
		{
			name:          "TimeTrackingがnilの場合",
			timeTracking:  nil,
			expectStrings: []string{},
			notExpect: []string{
				"- **初期見積り**:",
				"- **残り時間**:",
				"- **作業時間**:",
			},
		},
		{
			name: "TimeTrackingは存在するが全フィールドが空の場合",
			timeTracking: &cloud.TimeTracking{
				OriginalEstimateSeconds:  0,
				RemainingEstimateSeconds: 0,
				TimeSpentSeconds:         0,
			},
			expectStrings: []string{},
			notExpect: []string{
				"- **初期見積り**:",
				"- **残り時間**:",
				"- **作業時間**:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// モックのIssueを作成
			issue := &cloud.Issue{
				Key: "TEST-123",
				Fields: &cloud.IssueFields{
					Type: cloud.IssueType{
						Name: "タスク",
					},
					Status: &cloud.Status{
						Name: "未着手",
					},
					Priority: &cloud.Priority{
						Name: "中",
					},
					Reporter: &cloud.User{
						DisplayName: "テスト報告者",
					},
					Assignee: &cloud.User{
						DisplayName: "テスト担当者",
					},
					Summary:      "テスト課題",
					Description:  "テスト説明",
					Created:      cloud.Time(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
					Updated:      cloud.Time(time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)),
					TimeTracking: tt.timeTracking,
					Project: cloud.Project{
						Key:  "TEST",
						Name: "テストプロジェクト",
					},
				},
			}

			// generateMarkdownを呼び出し
			result := mw.generateMarkdown(issue, []string{}, nil)

			// 期待される文字列が含まれているか確認
			for _, expected := range tt.expectStrings {
				if !strings.Contains(result, expected) {
					t.Errorf("期待する文字列が出力されていません\n期待: %q\n実際の出力:\n%s", expected, result)
				}
			}

			// 含まれてはいけない文字列が含まれていないか確認
			for _, notExpected := range tt.notExpect {
				if strings.Contains(result, notExpected) {
					t.Errorf("出力されるべきでない文字列が含まれています\n含まれてはいけない: %q\n実際の出力:\n%s", notExpected, result)
				}
			}
		})
	}
}

// TestFormatTimeSeconds はformatTimeSecondsメソッドのテスト
func TestFormatTimeSeconds(t *testing.T) {
	tests := []struct {
		name     string
		seconds  int
		expected string
	}{
		{
			name:     "0秒の場合は空文字列を返す",
			seconds:  0,
			expected: "",
		},
		{
			name:     "3600秒（1時間）",
			seconds:  3600,
			expected: "1.00h",
		},
		{
			name:     "1800秒（30分）",
			seconds:  1800,
			expected: "0.50h",
		},
		{
			name:     "26100秒（7時間15分）",
			seconds:  26100,
			expected: "7.25h",
		},
		{
			name:     "5400秒（1時間30分）",
			seconds:  5400,
			expected: "1.50h",
		},
		{
			name:     "9000秒（2時間30分）",
			seconds:  9000,
			expected: "2.50h",
		},
		{
			name:     "900秒（15分）",
			seconds:  900,
			expected: "0.25h",
		},
		{
			name:     "7200秒（2時間）",
			seconds:  7200,
			expected: "2.00h",
		},
		{
			name:     "28800秒（8時間・1日の標準作業時間）",
			seconds:  28800,
			expected: "8.00h",
		},
		{
			name:     "36000秒（10時間）",
			seconds:  36000,
			expected: "10.00h",
		},
	}

	// MarkdownWriterのインスタンスを作成
	mw := NewMarkdownWriter("test_output", "test_attachments", nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mw.formatTimeSeconds(tt.seconds)
			if result != tt.expected {
				t.Errorf("formatTimeSeconds(%d) = %q, expected %q", tt.seconds, result, tt.expected)
			}
		})
	}
}

// TestLabelsAndParentFields はラベルと親課題フィールドのテスト
func TestLabelsAndParentFields(t *testing.T) {
	tests := []struct {
		name          string
		labels        []string
		parent        *cloud.Parent
		expectStrings []string
		notExpect     []string
	}{
		{
			name:   "ラベルが1つ設定されている場合",
			labels: []string{"バグ"},
			parent: nil,
			expectStrings: []string{
				"- **ラベル**: バグ",
			},
			notExpect: []string{
				"- **親課題**:",
			},
		},
		{
			name:   "ラベルが複数設定されている場合",
			labels: []string{"バグ", "緊急", "セキュリティ"},
			parent: nil,
			expectStrings: []string{
				"- **ラベル**: バグ, 緊急, セキュリティ",
			},
			notExpect: []string{
				"- **親課題**:",
			},
		},
		{
			name:   "ラベルが設定されていない場合",
			labels: []string{},
			parent: nil,
			expectStrings: []string{},
			notExpect: []string{
				"- **ラベル**:",
				"- **親課題**:",
			},
		},
		{
			name:   "親課題が設定されている場合",
			labels: []string{},
			parent: &cloud.Parent{
				Key: "PROJ-100",
			},
			expectStrings: []string{
				"- **親課題**: [PROJ-100](../PROJ-100/)",
			},
			notExpect: []string{
				"- **ラベル**:",
			},
		},
		{
			name:   "親課題がnilの場合",
			labels: []string{},
			parent: nil,
			expectStrings: []string{},
			notExpect: []string{
				"- **ラベル**:",
				"- **親課題**:",
			},
		},
		{
			name:   "ラベルと親課題の両方が設定されている場合",
			labels: []string{"改善", "UIデザイン"},
			parent: &cloud.Parent{
				Key: "PROJ-200",
			},
			expectStrings: []string{
				"- **ラベル**: 改善, UIデザイン",
				"- **親課題**: [PROJ-200](../PROJ-200/)",
			},
			notExpect: []string{},
		},
	}

	// MarkdownWriterのインスタンスを作成
	mw := NewMarkdownWriter("test_output", "test_attachments", nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// モックのIssueを作成
			issue := &cloud.Issue{
				Key: "TEST-123",
				Fields: &cloud.IssueFields{
					Type: cloud.IssueType{
						Name: "タスク",
					},
					Status: &cloud.Status{
						Name: "未着手",
					},
					Priority: &cloud.Priority{
						Name: "中",
					},
					Reporter: &cloud.User{
						DisplayName: "テスト報告者",
					},
					Assignee: &cloud.User{
						DisplayName: "テスト担当者",
					},
					Summary:     "テスト課題",
					Description: "テスト説明",
					Created:     cloud.Time(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
					Updated:     cloud.Time(time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)),
					Labels:      tt.labels,
					Parent:      tt.parent,
					Project: cloud.Project{
						Key:  "TEST",
						Name: "テストプロジェクト",
					},
				},
			}

			// generateMarkdownを呼び出し
			result := mw.generateMarkdown(issue, []string{}, nil)

			// 期待される文字列が含まれているか確認
			for _, expected := range tt.expectStrings {
				if !strings.Contains(result, expected) {
					t.Errorf("期待する文字列が出力されていません\n期待: %q\n実際の出力:\n%s", expected, result)
				}
			}

			// 含まれてはいけない文字列が含まれていないか確認
			for _, notExpected := range tt.notExpect {
				if strings.Contains(result, notExpected) {
					t.Errorf("出力されるべきでない文字列が含まれています\n含まれてはいけない: %q\n実際の出力:\n%s", notExpected, result)
				}
			}
		})
	}
}

// TestSubtasksField はサブタスクフィールドのテスト
func TestSubtasksField(t *testing.T) {
	tests := []struct {
		name          string
		subtasks      []*cloud.Subtasks
		expectStrings []string
		notExpect     []string
	}{
		{
			name: "サブタスクが設定されている場合",
			subtasks: []*cloud.Subtasks{
				{
					Key: "PROJ-124",
					Fields: cloud.IssueFields{
						Summary: "サブタスク1",
						Status:  &cloud.Status{Name: "完了"},
					},
				},
				{
					Key: "PROJ-125",
					Fields: cloud.IssueFields{
						Summary: "サブタスク2",
						Status:  &cloud.Status{Name: "対応中"},
					},
				},
			},
			expectStrings: []string{
				"## サブタスク",
				"- **[PROJ-124](../PROJ-124/)**: サブタスク1 [完了]",
				"- **[PROJ-125](../PROJ-125/)**: サブタスク2 [対応中]",
			},
			notExpect: []string{},
		},
		{
			name:          "サブタスクが設定されていない場合",
			subtasks:      []*cloud.Subtasks{},
			expectStrings: []string{},
			notExpect:     []string{"## サブタスク"},
		},
		{
			name: "サブタスクのステータスがnilの場合",
			subtasks: []*cloud.Subtasks{
				{
					Key: "PROJ-126",
					Fields: cloud.IssueFields{
						Summary: "ステータス無しサブタスク",
						Status:  nil,
					},
				},
			},
			expectStrings: []string{
				"## サブタスク",
				"- **[PROJ-126](../PROJ-126/)**: ステータス無しサブタスク",
			},
			notExpect: []string{},
		},
	}

	// MarkdownWriterのインスタンスを作成
	mw := NewMarkdownWriter("test_output", "test_attachments", nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// モックのIssueを作成
			issue := &cloud.Issue{
				Key: "PROJ-123",
				Fields: &cloud.IssueFields{
					Type: cloud.IssueType{
						Name: "タスク",
					},
					Status: &cloud.Status{
						Name: "未着手",
					},
					Priority: &cloud.Priority{
						Name: "中",
					},
					Reporter: &cloud.User{
						DisplayName: "テスト報告者",
					},
					Assignee: &cloud.User{
						DisplayName: "テスト担当者",
					},
					Summary:     "テスト課題",
					Description: "テスト説明",
					Created:     cloud.Time(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
					Updated:     cloud.Time(time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)),
					Subtasks:    tt.subtasks,
					Project: cloud.Project{
						Key:  "PROJ",
						Name: "テストプロジェクト",
					},
				},
			}

			// generateMarkdownを呼び出し
			result := mw.generateMarkdown(issue, []string{}, nil)

			// 期待される文字列が含まれているか確認
			for _, expected := range tt.expectStrings {
				if !strings.Contains(result, expected) {
					t.Errorf("期待する文字列が出力されていません\n期待: %q\n実際の出力:\n%s", expected, result)
				}
			}

			// 含まれてはいけない文字列が含まれていないか確認
			for _, notExpected := range tt.notExpect {
				if strings.Contains(result, notExpected) {
					t.Errorf("出力されるべきでない文字列が含まれています\n含まれてはいけない: %q\n実際の出力:\n%s", notExpected, result)
				}
			}
		})
	}
}

// TestIssueLinksField は関連リンクフィールドのテスト
func TestIssueLinksField(t *testing.T) {
	tests := []struct {
		name          string
		issueLinks    []*cloud.IssueLink
		expectStrings []string
		notExpect     []string
	}{
		{
			name: "Outward関連リンクが設定されている場合",
			issueLinks: []*cloud.IssueLink{
				{
					Type: cloud.IssueLinkType{
						Outward: "blocks",
					},
					OutwardIssue: &cloud.Issue{
						Key: "PROJ-130",
						Fields: &cloud.IssueFields{
							Summary: "ブロック課題",
							Status:  &cloud.Status{Name: "対応中"},
						},
					},
				},
			},
			expectStrings: []string{
				"## 関連リンク",
				"- **blocks**: [PROJ-130](../PROJ-130/) - ブロック課題 [対応中]",
			},
			notExpect: []string{},
		},
		{
			name: "Inward関連リンクが設定されている場合",
			issueLinks: []*cloud.IssueLink{
				{
					Type: cloud.IssueLinkType{
						Inward: "is blocked by",
					},
					InwardIssue: &cloud.Issue{
						Key: "PROJ-140",
						Fields: &cloud.IssueFields{
							Summary: "ブロッカー課題",
							Status:  &cloud.Status{Name: "完了"},
						},
					},
				},
			},
			expectStrings: []string{
				"## 関連リンク",
				"- **is blocked by**: [PROJ-140](../PROJ-140/) - ブロッカー課題 [完了]",
			},
			notExpect: []string{},
		},
		{
			name: "OutwardとInward両方の関連リンクが設定されている場合",
			issueLinks: []*cloud.IssueLink{
				{
					Type: cloud.IssueLinkType{
						Outward: "relates to",
					},
					OutwardIssue: &cloud.Issue{
						Key: "PROJ-150",
						Fields: &cloud.IssueFields{
							Summary: "関連タスク",
							Status:  &cloud.Status{Name: "未着手"},
						},
					},
				},
				{
					Type: cloud.IssueLinkType{
						Inward: "duplicates",
					},
					InwardIssue: &cloud.Issue{
						Key: "PROJ-160",
						Fields: &cloud.IssueFields{
							Summary: "重複課題",
							Status:  &cloud.Status{Name: "完了"},
						},
					},
				},
			},
			expectStrings: []string{
				"## 関連リンク",
				"- **relates to**: [PROJ-150](../PROJ-150/) - 関連タスク [未着手]",
				"- **duplicates**: [PROJ-160](../PROJ-160/) - 重複課題 [完了]",
			},
			notExpect: []string{},
		},
		{
			name:          "関連リンクが設定されていない場合",
			issueLinks:    []*cloud.IssueLink{},
			expectStrings: []string{},
			notExpect:     []string{"## 関連リンク"},
		},
		{
			name: "関連課題のFieldsがnilの場合",
			issueLinks: []*cloud.IssueLink{
				{
					Type: cloud.IssueLinkType{
						Outward: "blocks",
					},
					OutwardIssue: &cloud.Issue{
						Key:    "PROJ-170",
						Fields: nil,
					},
				},
			},
			expectStrings: []string{
				"## 関連リンク",
				"- **blocks**: [PROJ-170](../PROJ-170/)",
			},
			notExpect: []string{" - "},
		},
	}

	// MarkdownWriterのインスタンスを作成
	mw := NewMarkdownWriter("test_output", "test_attachments", nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// モックのIssueを作成
			issue := &cloud.Issue{
				Key: "PROJ-123",
				Fields: &cloud.IssueFields{
					Type: cloud.IssueType{
						Name: "タスク",
					},
					Status: &cloud.Status{
						Name: "未着手",
					},
					Priority: &cloud.Priority{
						Name: "中",
					},
					Reporter: &cloud.User{
						DisplayName: "テスト報告者",
					},
					Assignee: &cloud.User{
						DisplayName: "テスト担当者",
					},
					Summary:     "テスト課題",
					Description: "テスト説明",
					Created:     cloud.Time(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
					Updated:     cloud.Time(time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)),
					IssueLinks:  tt.issueLinks,
					Project: cloud.Project{
						Key:  "PROJ",
						Name: "テストプロジェクト",
					},
				},
			}

			// generateMarkdownを呼び出し
			result := mw.generateMarkdown(issue, []string{}, nil)

			// 期待される文字列が含まれているか確認
			for _, expected := range tt.expectStrings {
				if !strings.Contains(result, expected) {
					t.Errorf("期待する文字列が出力されていません\n期待: %q\n実際の出力:\n%s", expected, result)
				}
			}

			// 含まれてはいけない文字列が含まれていないか確認
			for _, notExpected := range tt.notExpect {
				if strings.Contains(result, notExpected) {
					t.Errorf("出力されるべきでない文字列が含まれています\n含まれてはいけない: %q\n実際の出力:\n%s", notExpected, result)
				}
			}
		})
	}
}
