package main

import (
	"testing"
)

func TestExtractPageIDFromGlobalID(t *testing.T) {
	tests := []struct {
		name      string
		globalID  string
		want      string
		wantError bool
	}{
		{
			name:      "正常系",
			globalID:  "appId=0c89e1b8-ac25-318c-b359-03c7f3bc2b18&pageId=98412",
			want:      "98412",
			wantError: false,
		},
		{
			name:      "pageIdなし",
			globalID:  "appId=0c89e1b8-ac25-318c-b359-03c7f3bc2b18",
			want:      "",
			wantError: true,
		},
		{
			name:      "複数のpageId",
			globalID:  "appId=test&pageId=12345&extra=value",
			want:      "12345",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractPageIDFromGlobalID(tt.globalID)
			if (err != nil) != tt.wantError {
				t.Errorf("ExtractPageIDFromGlobalID() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if got != tt.want {
				t.Errorf("ExtractPageIDFromGlobalID() = %v, want %v", got, tt.want)
			}
		})
	}
}
