package text

import (
	"testing"
)

func TestLoneCRAndCRLF(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		wantLines []string
	}{
		{
			name:      "CRLF splits line and strips CR",
			input:     "hello\r\nworld",
			wantCount: 2,
			wantLines: []string{"hello", "world"},
		},
		{
			name:      "lone CR remains as line content",
			input:     "hello\rworld",
			wantCount: 1,
			wantLines: []string{"hello\rworld"},
		},
		{
			name:      "multiple CRLFs with empty line",
			input:     "line1\r\n\r\nline2",
			wantCount: 3,
			wantLines: []string{"line1", "", "line2"},
		},
		{
			name:      "lone CR at end of string",
			input:     "hello\r",
			wantCount: 1,
			wantLines: []string{"hello\r"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txt := RawText(tt.input)
			if txt.Height() != tt.wantCount {
				t.Fatalf("height = %d, want %d", txt.Height(), tt.wantCount)
			}
			for i, want := range tt.wantLines {
				got := txt.Lines[i].Spans[0].Content
				if got != want {
					t.Errorf("line %d = %q, want %q", i, got, want)
				}
			}
		})
	}
}

func TestWideGraphemeTruncationGap(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxWidth  int
		wantStr   string
		wantWidth int
	}{
		{
			name:      "wide grapheme truncated leaves cell gap",
			input:     "😀😀",
			maxWidth:  3,
			wantStr:   "😀",
			wantWidth: 2,
		},
		{
			name:      "ASCII prefix plus wide grapheme gap",
			input:     "hello😀world",
			maxWidth:  6,
			wantStr:   "hello",
			wantWidth: 5,
		},
		{
			name:      "single wide grapheme exceeding maxWidth 1",
			input:     "界",
			maxWidth:  1,
			wantStr:   "",
			wantWidth: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStr, gotWidth := Truncate(tt.input, tt.maxWidth)
			if gotStr != tt.wantStr {
				t.Errorf("Truncate string = %q, want %q", gotStr, tt.wantStr)
			}
			if gotWidth != tt.wantWidth {
				t.Errorf("Truncate width = %d, want %d", gotWidth, tt.wantWidth)
			}
		})
	}
}
