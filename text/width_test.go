package text

import "testing"

func TestHalfwidthKatakanaSoundMarkWidths(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  int
	}{
		{"ﾞ", 1},
		{"ﾟ", 1},
		{"ｶﾞ", 2},
		{"ﾊﾟ", 2},
		{"aﾞ", 2},
		{"あﾞ", 3},
		{"ｶ\u3099", 1},
		{"ガ", 2},
		{"aｶﾞb", 4},
		{"あｶﾞ", 4},
	} {
		if got := GraphemeWidth(tc.value); got != tc.want {
			t.Errorf("GraphemeWidth(%q) = %d, want %d", tc.value, got, tc.want)
		}
	}
}

func TestTruncateNeverSplitsWideGrapheme(t *testing.T) {
	if got, width := Truncate("a界b", 2); got != "a" || width != 1 {
		t.Fatalf("Truncate = %q, %d; want a, 1", got, width)
	}
	if got, width := TruncateStart("a界b", 2); got != "b" || width != 1 {
		t.Fatalf("TruncateStart = %q, %d; want b, 1", got, width)
	}
}
