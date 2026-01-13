package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/andygrunwald/go-jira/v2/cloud"
)

// createTestConfig はテスト用のConfigを作成する
func createTestConfig() *Config {
	return &Config{
		Display: DisplayConfig{
			HiddenCustomFields: []string{
				"customfield_10015", // Start date
				"customfield_10019", // Rank
			},
		},
	}
}

func TestExtractJIRATables(t *testing.T) {
	mw := NewMarkdownWriter("", "", nil, createTestConfig())

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
	mw := NewMarkdownWriter("", "", nil, createTestConfig())

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
			expected: `<span class="mention">@牛頭</span>さん、こんにちは`,
		},
		{
			name:     "複数のメンション",
			input:    "[~accountid:557058:6eed56ba-9b9b-4a87-ad74-18b7086f1063]と[~accountid:123456:abcdef]",
			expected: `<span class="mention">@牛頭</span>と<span class="mention">@太郎</span>`,
		},
		{
			name:     "マッピングが存在しない場合",
			input:    "[~accountid:unknown]",
			expected: `<span class="mention">@unknown</span>`,
		},
		{
			name:     "メンション無し",
			input:    "通常のテキストです",
			expected: "通常のテキストです",
		},
		{
			name:     "メンションが文章中に混在",
			input:    "こんにちは、[~accountid:557058:6eed56ba-9b9b-4a87-ad74-18b7086f1063]さん。レビューをお願いします。",
			expected: `こんにちは、<span class="mention">@牛頭</span>さん。レビューをお願いします。`,
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
	mw := NewMarkdownWriter("", "", nil, createTestConfig())

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
			result := mw.generateMarkdown(issue, []string{}, make(FieldNameCache), nil, nil, []ChildIssueInfo{})

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
	mw := NewMarkdownWriter("", "", nil, createTestConfig())

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
			result := mw.generateMarkdown(issue, []string{}, make(FieldNameCache), nil, nil, []ChildIssueInfo{})

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
	mw := NewMarkdownWriter("test_output", "test_attachments", nil, createTestConfig())

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
	mw := NewMarkdownWriter("test_output", "test_attachments", nil, createTestConfig())

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
			result := mw.generateMarkdown(issue, []string{}, make(FieldNameCache), nil, nil, []ChildIssueInfo{})

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
	mw := NewMarkdownWriter("test_output", "test_attachments", nil, createTestConfig())

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
			result := mw.generateMarkdown(issue, []string{}, make(FieldNameCache), nil, nil, []ChildIssueInfo{})

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
	mw := NewMarkdownWriter("test_output", "test_attachments", nil, createTestConfig())

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
			result := mw.generateMarkdown(issue, []string{}, make(FieldNameCache), nil, nil, []ChildIssueInfo{})

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

// TestGenerateMarkdown_Golden は generateMarkdown() の出力をゴールデンファイルと比較するテスト
// このテストは、リファクタリング後も同じ出力が生成されることを保証する
func TestGenerateMarkdown_Golden(t *testing.T) {
	// テスト用のMarkdownWriterを作成
	mw := NewMarkdownWriter("", "", nil, createTestConfig())

	// 完全な課題データを作成（すべてのフィールドを含む）
	issue := &cloud.Issue{
		ID:  "10001",
		Key: "SCRUM-2",
		Fields: &cloud.IssueFields{
			Type: cloud.IssueType{
				Name: "タスク",
			},
			Status: &cloud.Status{
				Name: "完了",
			},
			Priority: &cloud.Priority{
				Name: "中",
			},
			Reporter: &cloud.User{
				DisplayName:  "テスト報告者",
				EmailAddress: "reporter@example.com",
			},
			Assignee: &cloud.User{
				DisplayName:  "テスト担当者",
				EmailAddress: "assignee@example.com",
			},
			Summary:     "ゴールデンファイルテスト用の課題",
			Description: "これはテスト用の説明です。\n\n*太字*と_斜体_のテキストを含みます。\n\nコードブロック:\n{code:java}\npublic static void main(String[] args) {\n    System.out.println(\"Hello, World!\");\n}\n{code}\n\nリスト:\n* 項目1\n* 項目2\n** 項目2-1",
			Created:     cloud.Time(time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)),
			Updated:     cloud.Time(time.Date(2025, 1, 15, 14, 30, 0, 0, time.UTC)),
			Duedate:     cloud.Date(time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)),
			Labels:      []string{"テスト", "ゴールデンファイル"},
			Project: cloud.Project{
				Key:  "SCRUM",
				Name: "スクラムプロジェクト",
			},
			Resolution: &cloud.Resolution{
				Name: "完了",
			},
			Parent: &cloud.Parent{
				Key: "SCRUM-1",
			},
			TimeTracking: &cloud.TimeTracking{
				OriginalEstimateSeconds:  28800, // 8時間
				RemainingEstimateSeconds: 0,
				TimeSpentSeconds:         25200, // 7時間
			},
			Comments: &cloud.Comments{
				Comments: []*cloud.Comment{
					{
						ID: "10000",
						Author: &cloud.User{
							DisplayName: "コメント投稿者1",
						},
						Body:    "最初のコメントです。",
						Created: "2025-01-02T10:00:00.000+0900",
					},
					{
						ID: "10001",
						Author: &cloud.User{
							DisplayName: "コメント投稿者2",
						},
						Body:    "2番目のコメントです。\n\n複数行のコメント。",
						Created: "2025-01-03T11:00:00.000+0900",
					},
				},
			},
			Subtasks: []*cloud.Subtasks{
				{
					ID:  "10002",
					Key: "SCRUM-3",
					Fields: cloud.IssueFields{
						Summary: "サブタスク1",
						Status: &cloud.Status{
							Name: "進行中",
						},
					},
				},
				{
					ID:  "10003",
					Key: "SCRUM-4",
					Fields: cloud.IssueFields{
						Summary: "サブタスク2",
						Status: &cloud.Status{
							Name: "完了",
						},
					},
				},
			},
			IssueLinks: []*cloud.IssueLink{
				{
					ID: "10000",
					Type: cloud.IssueLinkType{
						Name:    "関連",
						Inward:  "関連している",
						Outward: "関連する",
					},
					OutwardIssue: &cloud.Issue{
						ID:  "10004",
						Key: "SCRUM-5",
						Fields: &cloud.IssueFields{
							Summary: "関連課題1",
							Status: &cloud.Status{
								Name: "未着手",
							},
						},
					},
				},
				{
					ID: "10001",
					Type: cloud.IssueLinkType{
						Name:    "ブロック",
						Inward:  "ブロックされている",
						Outward: "ブロックする",
					},
					InwardIssue: &cloud.Issue{
						ID:  "10005",
						Key: "SCRUM-6",
						Fields: &cloud.IssueFields{
							Summary: "ブロック元課題",
							Status: &cloud.Status{
								Name: "完了",
							},
						},
					},
				},
			},
		},
		Changelog: &cloud.Changelog{
			Histories: []cloud.ChangelogHistory{
				{
					Id: "10000",
					Author: cloud.User{
						DisplayName: "変更者1",
					},
					Created: "2025-01-05T12:00:00.000+0900",
					Items: []cloud.ChangelogItems{
						{
							Field:      "status",
							FromString: "未着手",
							ToString:   "進行中",
						},
					},
				},
				{
					Id: "10001",
					Author: cloud.User{
						DisplayName: "変更者2",
					},
					Created: "2025-01-10T15:00:00.000+0900",
					Items: []cloud.ChangelogItems{
						{
							Field:      "status",
							FromString: "進行中",
							ToString:   "完了",
						},
						{
							Field:      "assignee",
							FromString: "前任者",
							ToString:   "テスト担当者",
						},
					},
				},
			},
		},
	}

	// 添付ファイルリスト
	attachmentFiles := []string{
		"SCRUM-2_screenshot.png",
		"SCRUM-2_document.pdf",
	}

	// フィールド名キャッシュ
	fieldNameCache := make(FieldNameCache)

	// 開発情報（プルリクエストとブランチ）
	devStatus := &DevStatusDetail{
		Detail: []DevStatusDetailItem{
			{
				PullRequests: []DevPullRequest{
					{
						ID:   "1",
						Name: "Feature: Add golden file test",
						Author: DevAuthor{
							Name: "developer1",
						},
						Status: "MERGED",
						Source: DevPullRequestBranch{
							Branch: "feature/golden-file-test",
							URL:    "https://github.com/test/repo/tree/feature/golden-file-test",
						},
						URL: "https://github.com/test/repo/pull/1",
					},
				},
				Branches: []DevBranch{
					{
						Name: "feature/golden-file-test",
						URL:  "https://github.com/test/repo/tree/feature/golden-file-test",
					},
				},
			},
		},
	}

	// generateMarkdownを実行
	got := mw.generateMarkdown(issue, attachmentFiles, fieldNameCache, devStatus, nil, []ChildIssueInfo{})

	// ゴールデンファイルのパス
	goldenFile := "testdata/generate-markdown.golden"

	// ゴールデンファイルの内容を読み込み
	want, err := os.ReadFile(goldenFile)
	if err != nil {
		// ゴールデンファイルが存在しない場合は作成
		if os.IsNotExist(err) {
			t.Logf("ゴールデンファイルが存在しないため作成します: %s", goldenFile)
			if err := os.WriteFile(goldenFile, []byte(got), 0644); err != nil {
				t.Fatalf("ゴールデンファイルの作成に失敗しました: %v", err)
			}
			t.Logf("ゴールデンファイルを作成しました。次回のテスト実行で比較が行われます。")
			return
		}
		t.Fatalf("ゴールデンファイルの読み込みに失敗しました: %v", err)
	}

	// 出力を比較
	if got != string(want) {
		t.Errorf("generateMarkdown()の出力がゴールデンファイルと一致しません\n")
		t.Logf("差分を確認するには以下のコマンドを実行してください:\n")
		t.Logf("  diff -u %s <(echo %q)\n", goldenFile, got)

		// 実際の出力をファイルに保存（デバッグ用）
		actualFile := "testdata/generate-markdown.actual"
		if err := os.WriteFile(actualFile, []byte(got), 0644); err != nil {
			t.Logf("実際の出力の保存に失敗しました: %v", err)
		} else {
			t.Logf("実際の出力を保存しました: %s", actualFile)
			t.Logf("差分を確認するには: diff -u %s %s", goldenFile, actualFile)
		}
	}
}

// TestGenerateBasicInfo_StartDatePosition はStart dateが期限の上に表示されることを確認
func TestGenerateBasicInfo_StartDatePosition(t *testing.T) {
	// Start dateと期限の両方が設定された課題を作成
	issue := &cloud.Issue{
		Key: "TEST-1",
		Fields: &cloud.IssueFields{
			Type:    cloud.IssueType{Name: "タスク"},
			Status:  &cloud.Status{Name: "進行中"},
			Created: cloud.Time(time.Now()),
			Updated: cloud.Time(time.Now()),
			Duedate: cloud.Date(time.Now().AddDate(0, 0, 7)),
			Unknowns: map[string]interface{}{
				"customfield_10015": "2025-01-10", // Start date
			},
		},
	}

	cache := make(FieldNameCache)
	cache["customfield_10015"] = "Start date"

	userMapping := make(UserMapping)
	mw := NewMarkdownWriter("", "", userMapping, createTestConfig())
	var sb strings.Builder
	mw.generateBasicInfo(&sb, issue, cache, nil)

	result := sb.String()

	// Start dateが期限の前に表示されることを確認
	startDatePos := strings.Index(result, "Start date")
	dueDatePos := strings.Index(result, "期限")

	if startDatePos == -1 {
		t.Error("Start dateが表示されていません")
	}
	if dueDatePos == -1 {
		t.Error("期限が表示されていません")
	}
	if startDatePos > dueDatePos {
		t.Errorf("Start dateが期限の後に表示されています。Start date位置=%d, 期限位置=%d", startDatePos, dueDatePos)
	}
}

// TestGenerateBasicInfo_RankHidden はRankが非表示になることを確認
func TestGenerateBasicInfo_RankHidden(t *testing.T) {
	issue := &cloud.Issue{
		Key: "TEST-2",
		Fields: &cloud.IssueFields{
			Type:    cloud.IssueType{Name: "タスク"},
			Status:  &cloud.Status{Name: "進行中"},
			Created: cloud.Time(time.Now()),
			Updated: cloud.Time(time.Now()),
			Unknowns: map[string]interface{}{
				"customfield_10019": "0|i00007:", // Rank
			},
		},
	}

	cache := make(FieldNameCache)
	cache["customfield_10019"] = "Rank"

	userMapping := make(UserMapping)
	mw := NewMarkdownWriter("", "", userMapping, createTestConfig())
	var sb strings.Builder
	mw.generateBasicInfo(&sb, issue, cache, nil)

	result := sb.String()

	// Rankが表示されていないことを確認
	if strings.Contains(result, "Rank") {
		t.Error("Rankが表示されています（非表示にする必要があります）")
	}
}

func TestConvertJIRAListsToMarkdown(t *testing.T) {
	userMapping := make(UserMapping)
	mw := NewMarkdownWriter("", "", userMapping, createTestConfig())

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "基本的なリスト",
			input:    "* リスト1\n** リスト2\n*** リスト3",
			expected: "- リスト1\n  - リスト2\n    - リスト3",
		},
		{
			name:     "最大ネストレベル（6レベル）",
			input:    "* レベル1\n****** レベル6",
			expected: "- レベル1\n          - レベル6",
		},
		{
			name:     "リストと通常テキストの混在",
			input:    "通常のテキスト\n* リスト1\n* リスト2\n通常のテキスト2",
			expected: "通常のテキスト\n- リスト1\n- リスト2\n通常のテキスト2",
		},
		{
			name:     "複数レベルのリスト",
			input:    "* アイテム1\n** サブアイテム1\n*** サブサブアイテム1\n** サブアイテム2\n* アイテム2",
			expected: "- アイテム1\n  - サブアイテム1\n    - サブサブアイテム1\n  - サブアイテム2\n- アイテム2",
		},
		{
			name:     "空行を含むリスト",
			input:    "* リスト1\n\n* リスト2",
			expected: "- リスト1\n\n- リスト2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mw.convertJIRAListsToMarkdown(tt.input)
			if result != tt.expected {
				t.Errorf("期待値と異なります\n期待: %q\n結果: %q", tt.expected, result)
			}
		})
	}
}

func TestConvertJIRAMarkupToMarkdown_Headings(t *testing.T) {
	userMapping := make(UserMapping)
	mw := NewMarkdownWriter("", "", userMapping, createTestConfig())

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "見出しレベル1",
			input:    "h1. 見出し1",
			expected: "# 見出し1",
		},
		{
			name:     "見出しレベル2-6",
			input:    "h2. 見出し2\nh3. 見出し3\nh6. 見出し6",
			expected: "## 見出し2  \n### 見出し3  \n###### 見出し6",
		},
		{
			name:     "見出しとリストの混在",
			input:    "h2. タイトル\n* リスト1\n* リスト2",
			expected: "## タイトル  \n- リスト1  \n- リスト2",
		},
		{
			name:     "見出し後に通常テキスト",
			input:    "h1. タイトル\n\n通常のテキスト",
			expected: "# タイトル  \n\n通常のテキスト",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mw.convertJIRAMarkupToMarkdown(tt.input)
			if result != tt.expected {
				t.Errorf("期待値と異なります\n期待: %q\n結果: %q", tt.expected, result)
			}
		})
	}
}

func TestConvertJIRAMarkupToMarkdown_ListAndHeadingIntegration(t *testing.T) {
	userMapping := make(UserMapping)
	mw := NewMarkdownWriter("", "", userMapping, createTestConfig())

	// リストと見出しが正しく変換されることを確認
	input := "h2. リストの例\n* リスト1\n** サブリスト1\n* リスト2"
	result := mw.convertJIRAMarkupToMarkdown(input)

	// 見出しが変換されているか確認
	if !strings.Contains(result, "## リストの例") {
		t.Errorf("見出しが変換されていません: %q", result)
	}

	// リストが変換されているか確認
	if !strings.Contains(result, "- リスト1") {
		t.Errorf("リストが変換されていません: %q", result)
	}

	if !strings.Contains(result, "  - サブリスト1") {
		t.Errorf("ネストされたリストが変換されていません: %q", result)
	}
}

func TestConvertJIRAListsToMarkdown_NumberedLists(t *testing.T) {
	userMapping := make(UserMapping)
	mw := NewMarkdownWriter("", "", userMapping, createTestConfig())

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "基本的な番号付きリスト",
			input:    "# Item 1\n# Item 2\n# Item 3",
			expected: "1. Item 1\n1. Item 2\n1. Item 3",
		},
		{
			name:     "ネストした番号付きリスト",
			input:    "# Level 1\n## Level 2\n### Level 3",
			expected: "1. Level 1\n  1. Level 2\n    1. Level 3",
		},
		{
			name:     "最大ネストレベル（6レベル）",
			input:    "# L1\n## L2\n### L3\n#### L4\n##### L5\n###### L6",
			expected: "1. L1\n  1. L2\n    1. L3\n      1. L4\n        1. L5\n          1. L6",
		},
		{
			name:     "番号付きリストと番号なしリストの混在",
			input:    "# Numbered 1\n* Bullet 1\n## Numbered 2\n** Bullet 2",
			expected: "1. Numbered 1\n- Bullet 1\n  1. Numbered 2\n  - Bullet 2",
		},
		{
			name:     "番号付きリストと通常テキストの混在",
			input:    "Normal text\n# Item 1\n# Item 2\nAnother text",
			expected: "Normal text\n1. Item 1\n1. Item 2\nAnother text",
		},
		{
			name:     "空行を含む番号付きリスト",
			input:    "# Item 1\n\n# Item 2",
			expected: "1. Item 1\n\n1. Item 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mw.convertJIRAListsToMarkdown(tt.input)
			if result != tt.expected {
				t.Errorf("convertJIRAListsToMarkdown() got:\n%s\n\nwant:\n%s", result, tt.expected)
			}
		})
	}
}

// TestChildIssuesField は子作業項目セクションのテスト
func TestChildIssuesField(t *testing.T) {
	tests := []struct {
		name           string
		childIssues    []ChildIssueInfo
		expectedOutput bool
		expectedText   string
	}{
		{
			name: "子課題が設定されている場合",
			childIssues: []ChildIssueInfo{
				{
					Key:     "STORY-1",
					Summary: "ユーザーストーリー1",
					Status:  "未着手",
					Type:    "Story",
					Rank:    "",
				},
				{
					Key:     "TASK-1",
					Summary: "実装タスク",
					Status:  "完了",
					Type:    "Task",
					Rank:    "",
				},
			},
			expectedOutput: true,
			expectedText:   "## 子作業項目",
		},
		{
			name:           "子課題が設定されていない場合",
			childIssues:    []ChildIssueInfo{},
			expectedOutput: false,
			expectedText:   "## 子作業項目",
		},
		{
			name: "複数の課題タイプが混在する場合",
			childIssues: []ChildIssueInfo{
				{
					Key:     "EPIC-1",
					Summary: "子エピック",
					Status:  "進行中",
					Type:    "Epic",
					Rank:    "",
				},
				{
					Key:     "STORY-1",
					Summary: "ストーリー",
					Status:  "未着手",
					Type:    "Story",
					Rank:    "",
				},
				{
					Key:     "BUG-1",
					Summary: "バグ",
					Status:  "完了",
					Type:    "Bug",
					Rank:    "",
				},
			},
			expectedOutput: true,
			expectedText:   "[EPIC-1](../EPIC-1/)",
		},
		{
			name: "ステータスが空文字列の場合",
			childIssues: []ChildIssueInfo{
				{
					Key:     "TASK-1",
					Summary: "タスク",
					Status:  "",
					Type:    "Task",
					Rank:    "",
				},
			},
			expectedOutput: true,
			expectedText:   "[TASK-1](../TASK-1/)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := NewMarkdownWriter("", "", nil, createTestConfig())
			var sb strings.Builder

			// generateChildIssuesを呼び出し
			mw.generateChildIssues(&sb, tt.childIssues)
			result := sb.String()

			// 出力の有無を確認
			if tt.expectedOutput {
				if !strings.Contains(result, tt.expectedText) {
					t.Errorf("期待するテキストが出力されていません\n期待: %q\n実際: %s", tt.expectedText, result)
				}
			} else {
				if strings.Contains(result, "## 子作業項目") {
					t.Errorf("子作業項目セクションが出力されるべきではありません\n実際: %s", result)
				}
			}

			// 複数ケースで詳細確認
			if tt.name == "複数の課題タイプが混在する場合" {
				if !strings.Contains(result, "📗") { // Story アイコン
					t.Errorf("ストーリーアイコン(📗)が表示されていません")
				}
				if !strings.Contains(result, "🐞") { // Bug アイコン
					t.Errorf("バグアイコン(🐞)が表示されていません")
				}
			}
		})
	}
}
