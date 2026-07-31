package layout

import (
	"fmt"
	"testing"
)

// Ported compact upstream vectors from ratatui-core 0.30.2 layout.rs tests.

func letters(flex Flex, cons []Constraint, width int) string {
	area := NewRect(0, 0, width, 1)
	segs := Horizontal(cons...).Flex(flex).Split(area)
	b := make([]byte, width)
	for i := range b {
		b[i] = ' '
	}
	for i, s := range segs {
		c := byte('a' + i)
		for x := s.X; x < s.X+s.Width && x < width; x++ {
			if x >= 0 {
				b[x] = c
			}
		}
	}
	return string(b)
}

func pairsOf(flex Flex, cons []Constraint, width, spacing int) [][2]int {
	segs := Horizontal(cons...).Flex(flex).Spacing(spacing).Split(NewRect(0, 0, width, 1))
	out := make([][2]int, len(segs))
	for i, s := range segs {
		out[i] = [2]int{s.X, s.Width}
	}
	return out
}

func rangesOf(flex Flex, cons []Constraint, width, spacing int) [][2]int {
	segs := Horizontal(cons...).Flex(flex).Spacing(spacing).Split(NewRect(0, 0, width, 1))
	out := make([][2]int, len(segs))
	for i, s := range segs {
		out[i] = [2]int{s.X, s.X + s.Width}
	}
	return out
}

func widthsOf(flex Flex, cons []Constraint, width, spacing int) []int {
	segs := Horizontal(cons...).Flex(flex).Spacing(spacing).Split(NewRect(0, 0, width, 1))
	out := make([]int, len(segs))
	for i, s := range segs {
		out[i] = s.Width
	}
	return out
}

func spacerPairs(flex Flex, cons []Constraint, width, spacing int) [][2]int {
	_, sp := Horizontal(cons...).Flex(flex).Spacing(spacing).SplitWithSpacers(NewRect(0, 0, width, 1))
	out := make([][2]int, len(sp))
	for i, s := range sp {
		out[i] = [2]int{s.X, s.Width}
	}
	return out
}

func eqPairs(t *testing.T, name string, got, want [][2]int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: len got %d want %d; got=%v want=%v", name, len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s: idx %d got %v want %v; full got=%v want=%v", name, i, got[i], want[i], got, want)
		}
	}
}

func eqInts(t *testing.T, name string, got, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: len got %d want %d; got=%v want=%v", name, len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s: idx %d got %d want %d; full got=%v want=%v", name, i, got[i], want[i], got, want)
		}
	}
}

func TestUpstreamLetters(t *testing.T) {
	for i, tt := range letterCorpus() {
		t.Run(fmt.Sprintf("L%d", i), func(t *testing.T) {
			if got := letters(tt.flex, tt.cons, tt.width); got != tt.want {
				t.Fatalf("got %q want %q cons=%v flex=%v w=%d", got, tt.want, tt.cons, tt.flex, tt.width)
			}
		})
	}
}

type letterCase struct {
	flex  Flex
	width int
	cons  []Constraint
	want  string
}

// letterCorpus holds the full upstream letter-encoded cases (layout.rs split::*).
func letterCorpus() []letterCase {
	// Generated once from ratatui 0.30.2; kept inline for offline tests.
	return letterCorpusData
}

func TestUpstreamPriorityAndFill(t *testing.T) {
	tests := []struct {
		name  string
		flex  Flex
		width int
		cons  []Constraint
		want  [][2]int // x,width
	}{
		// length_is_higher_priority (legacy ranges as x,width)
		{"min_len_max", FlexLegacy, 100, []Constraint{Min(25), Length(25), Max(25)}, [][2]int{{0, 50}, {50, 25}, {75, 25}}},
		{"max_len_min", FlexLegacy, 100, []Constraint{Max(25), Length(25), Min(25)}, [][2]int{{0, 25}, {25, 25}, {50, 50}}},
		{"len_len_len", FlexLegacy, 100, []Constraint{Length(33), Length(33), Length(33)}, [][2]int{{0, 33}, {33, 33}, {66, 34}}},
		{"perc_len_ratio", FlexLegacy, 100, []Constraint{Percentage(25), Length(25), Ratio(1, 4)}, [][2]int{{0, 25}, {25, 25}, {50, 50}}},
		{"len_len_min", FlexLegacy, 100, []Constraint{Length(100), Length(1), Min(20)}, [][2]int{{0, 80}, {80, 0}, {80, 20}}},
		{"min_len_len", FlexLegacy, 100, []Constraint{Min(20), Length(1), Length(100)}, [][2]int{{0, 20}, {20, 1}, {21, 79}}},
		{"fill_len_fill", FlexLegacy, 100, []Constraint{Fill(1), Length(10), Fill(1)}, [][2]int{{0, 45}, {45, 10}, {55, 45}}},
		{"fill_len_fill2", FlexLegacy, 100, []Constraint{Fill(1), Length(10), Fill(2)}, [][2]int{{0, 30}, {30, 10}, {40, 60}}},
		{"fill_len_fill4", FlexLegacy, 100, []Constraint{Fill(1), Length(10), Fill(4)}, [][2]int{{0, 18}, {18, 10}, {28, 72}}},
		{"fill_len_fill5", FlexLegacy, 100, []Constraint{Fill(1), Length(10), Fill(5)}, [][2]int{{0, 15}, {15, 10}, {25, 75}}},
		// fixed_with_50_width
		{"f50_a", FlexLegacy, 50, []Constraint{Fill(1), Length(10), Fill(2)}, [][2]int{{0, 13}, {13, 10}, {23, 27}}},
		{"f50_b", FlexLegacy, 50, []Constraint{Length(10), Fill(2), Fill(1)}, [][2]int{{0, 10}, {10, 27}, {37, 13}}},
		// fill weights / zero / collapse
		{"same_fill", FlexLegacy, 100, []Constraint{Fill(1), Fill(2), Fill(1), Fill(1)}, [][2]int{{0, 20}, {20, 40}, {60, 20}, {80, 20}}},
		{"inc_fill", FlexLegacy, 100, []Constraint{Fill(1), Fill(2), Fill(3), Fill(4)}, [][2]int{{0, 10}, {10, 20}, {30, 30}, {60, 40}}},
		{"zero_fill1", FlexLegacy, 100, []Constraint{Fill(0), Fill(1), Fill(0)}, [][2]int{{0, 0}, {0, 100}, {100, 0}}},
		{"zero_fill2", FlexLegacy, 100, []Constraint{Fill(0), Length(1), Fill(0)}, [][2]int{{0, 50}, {50, 1}, {51, 49}}},
		{"space_fill6", FlexLegacy, 100, []Constraint{Fill(0), Percentage(20)}, [][2]int{{0, 80}, {80, 20}}},
		{"fill_collapse1", FlexLegacy, 100, []Constraint{Fill(1), Fill(1), Fill(1), Min(30), Length(50)}, [][2]int{{0, 7}, {7, 6}, {13, 7}, {20, 30}, {50, 50}}},
		{"fill_collapse2", FlexLegacy, 100, []Constraint{Fill(1), Fill(1), Fill(1), Length(50), Length(50)}, [][2]int{{0, 0}, {0, 0}, {0, 0}, {0, 50}, {50, 50}}},
		// min/max contests
		{"max_min", FlexLegacy, 100, []Constraint{Max(100), Min(0)}, [][2]int{{0, 100}, {100, 0}}},
		{"min_max", FlexLegacy, 100, []Constraint{Min(0), Max(100)}, [][2]int{{0, 0}, {0, 100}}},
		{"length_min", FlexLegacy, 100, []Constraint{Length(65535), Min(10)}, [][2]int{{0, 90}, {90, 10}}},
		{"min_length", FlexLegacy, 100, []Constraint{Min(10), Length(65535)}, [][2]int{{0, 10}, {10, 90}}},
		{"length_max", FlexLegacy, 100, []Constraint{Length(0), Max(10)}, [][2]int{{0, 90}, {90, 10}}},
		{"max_length", FlexLegacy, 100, []Constraint{Max(10), Length(0)}, [][2]int{{0, 10}, {10, 90}}},
		{"min_percentage", FlexLegacy, 100, []Constraint{Min(0), Percentage(20)}, [][2]int{{0, 80}, {80, 20}}},
		{"max_percentage", FlexLegacy, 100, []Constraint{Max(0), Percentage(20)}, [][2]int{{0, 0}, {0, 100}}},
		// constraint_length legacy stretch
		{"len_min1", FlexLegacy, 100, []Constraint{Length(25), Min(100)}, [][2]int{{0, 0}, {0, 100}}},
		{"len_min2", FlexLegacy, 100, []Constraint{Length(25), Min(0)}, [][2]int{{0, 25}, {25, 75}}},
		{"len_max1", FlexLegacy, 100, []Constraint{Length(25), Max(0)}, [][2]int{{0, 100}, {100, 0}}},
		{"perc_len", FlexLegacy, 100, []Constraint{Percentage(25), Length(25)}, [][2]int{{0, 75}, {75, 25}}},
		{"len_perc", FlexLegacy, 100, []Constraint{Length(25), Percentage(25)}, [][2]int{{0, 25}, {25, 75}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eqPairs(t, tt.name, pairsOf(tt.flex, tt.cons, tt.width, 0), tt.want)
		})
	}
}

func TestUpstreamLengthPriorityFlex(t *testing.T) {
	flexes := []Flex{FlexStart, FlexEnd, FlexCenter, FlexSpaceAround, FlexSpaceEvenly, FlexSpaceBetween}
	tests := []struct {
		name string
		cons []Constraint
		want []int
	}{
		{"min_len_max", []Constraint{Min(25), Length(25), Max(25)}, []int{50, 25, 25}},
		{"max_len_min", []Constraint{Max(25), Length(25), Min(25)}, []int{25, 25, 50}},
		{"len_len_len1", []Constraint{Length(33), Length(33), Length(33)}, []int{33, 33, 33}},
		{"len_len_min", []Constraint{Length(100), Length(1), Min(20)}, []int{79, 1, 20}},
		{"min_len_len", []Constraint{Min(20), Length(1), Length(100)}, []int{20, 1, 79}},
		{"fill_len_fill1", []Constraint{Fill(1), Length(10), Fill(1)}, []int{45, 10, 45}},
		{"fill_len_fill2", []Constraint{Fill(1), Length(10), Fill(2)}, []int{30, 10, 60}},
		{"fill_len_fill5", []Constraint{Fill(1), Length(10), Fill(5)}, []int{15, 10, 75}},
	}
	for _, tt := range tests {
		for _, flex := range flexes {
			name := fmt.Sprintf("%s_%v", tt.name, flex)
			t.Run(name, func(t *testing.T) {
				eqInts(t, name, widthsOf(flex, tt.cons, 100, 0), tt.want)
			})
		}
	}
}

func TestUpstreamFlexConstraint(t *testing.T) {
	tests := []struct {
		name string
		flex Flex
		cons []Constraint
		want [][2]int // ranges x..x+w as x,w
	}{
		{"length_legacy", FlexLegacy, []Constraint{Length(50)}, [][2]int{{0, 100}}},
		{"length_start", FlexStart, []Constraint{Length(50)}, [][2]int{{0, 50}}},
		{"length_end", FlexEnd, []Constraint{Length(50)}, [][2]int{{50, 50}}},
		{"length_center", FlexCenter, []Constraint{Length(50)}, [][2]int{{25, 50}}},
		{"min_start", FlexStart, []Constraint{Min(50)}, [][2]int{{0, 100}}},
		{"max_start", FlexStart, []Constraint{Max(50)}, [][2]int{{0, 50}}},
		{"spacebetween_min", FlexSpaceBetween, []Constraint{Min(1)}, [][2]int{{0, 100}}},
		{"spacebetween_max", FlexSpaceBetween, []Constraint{Max(20)}, [][2]int{{0, 100}}},
		{"spacebetween_len", FlexSpaceBetween, []Constraint{Length(20)}, [][2]int{{0, 100}}},
		{"length_start2", FlexStart, []Constraint{Length(25), Length(25)}, [][2]int{{0, 25}, {25, 25}}},
		{"length_center2", FlexCenter, []Constraint{Length(25), Length(25)}, [][2]int{{25, 25}, {50, 25}}},
		{"length_end2", FlexEnd, []Constraint{Length(25), Length(25)}, [][2]int{{50, 25}, {75, 25}}},
		{"length_spacebetween", FlexSpaceBetween, []Constraint{Length(25), Length(25)}, [][2]int{{0, 25}, {75, 25}}},
		{"length_spaceevenly", FlexSpaceEvenly, []Constraint{Length(25), Length(25)}, [][2]int{{17, 25}, {58, 25}}},
		{"length_spacearound", FlexSpaceAround, []Constraint{Length(25), Length(25)}, [][2]int{{13, 25}, {63, 25}}},
		{"min_start2", FlexStart, []Constraint{Min(25), Min(25)}, [][2]int{{0, 50}, {50, 50}}},
		{"min_spacebetween", FlexSpaceBetween, []Constraint{Min(25), Min(25)}, [][2]int{{0, 50}, {50, 50}}},
		{"max_spaceevenly", FlexSpaceEvenly, []Constraint{Max(25), Max(25)}, [][2]int{{17, 25}, {58, 25}}},
		{"max_spacearound", FlexSpaceAround, []Constraint{Max(25), Max(25)}, [][2]int{{13, 25}, {63, 25}}},
		{"length_spaced_around", FlexSpaceBetween, []Constraint{Length(25), Length(25), Length(25)}, [][2]int{{0, 25}, {38, 25}, {75, 25}}},
		{"one_segment_spaceevenly", FlexSpaceEvenly, []Constraint{Length(50)}, [][2]int{{25, 50}}},
		{"one_segment_spacearound", FlexSpaceAround, []Constraint{Length(50)}, [][2]int{{25, 50}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eqPairs(t, tt.name, pairsOf(tt.flex, tt.cons, 100, 0), tt.want)
		})
	}
}

func TestUpstreamFlexSpacingOverlap(t *testing.T) {
	tests := []struct {
		name    string
		flex    Flex
		spacing int
		cons    []Constraint
		want    [][2]int
	}{
		{"overlap0", FlexStart, 0, []Constraint{Length(20), Length(20), Length(20)}, [][2]int{{0, 20}, {20, 20}, {40, 20}}},
		{"overlap-1-start", FlexStart, -1, []Constraint{Length(20), Length(20), Length(20)}, [][2]int{{0, 20}, {19, 20}, {38, 20}}},
		{"overlap-1-center", FlexCenter, -1, []Constraint{Length(20), Length(20), Length(20)}, [][2]int{{21, 20}, {40, 20}, {59, 20}}},
		{"overlap-1-end", FlexEnd, -1, []Constraint{Length(20), Length(20), Length(20)}, [][2]int{{42, 20}, {61, 20}, {80, 20}}},
		{"overlap-1-legacy", FlexLegacy, -1, []Constraint{Length(20), Length(20), Length(20)}, [][2]int{{0, 20}, {19, 20}, {38, 62}}},
		{"overlap-1-sb", FlexSpaceBetween, -1, []Constraint{Length(20), Length(20), Length(20)}, [][2]int{{0, 20}, {40, 20}, {80, 20}}},
		{"overlap-1-se", FlexSpaceEvenly, -1, []Constraint{Length(20), Length(20), Length(20)}, [][2]int{{10, 20}, {40, 20}, {70, 20}}},
		{"overlap-1-sa", FlexSpaceAround, -1, []Constraint{Length(20), Length(20), Length(20)}, [][2]int{{7, 20}, {40, 20}, {73, 20}}},
		{"sp2-start", FlexStart, 2, []Constraint{Length(20), Length(20), Length(20)}, [][2]int{{0, 20}, {22, 20}, {44, 20}}},
		{"sp2-center", FlexCenter, 2, []Constraint{Length(20), Length(20), Length(20)}, [][2]int{{18, 20}, {40, 20}, {62, 20}}},
		{"sp2-end", FlexEnd, 2, []Constraint{Length(20), Length(20), Length(20)}, [][2]int{{36, 20}, {58, 20}, {80, 20}}},
		{"sp2-legacy", FlexLegacy, 2, []Constraint{Length(20), Length(20), Length(20)}, [][2]int{{0, 20}, {22, 20}, {44, 56}}},
		{"sp2-sb", FlexSpaceBetween, 2, []Constraint{Length(20), Length(20), Length(20)}, [][2]int{{0, 20}, {40, 20}, {80, 20}}},
		{"sp2-se", FlexSpaceEvenly, 2, []Constraint{Length(20), Length(20), Length(20)}, [][2]int{{10, 20}, {40, 20}, {70, 20}}},
		{"sp2-sa", FlexSpaceAround, 2, []Constraint{Length(20), Length(20), Length(20)}, [][2]int{{7, 20}, {40, 20}, {73, 20}}},
		{"user-sp-center", FlexCenter, 80, []Constraint{Length(10), Length(10)}, [][2]int{{0, 10}, {90, 10}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eqPairs(t, tt.name, pairsOf(tt.flex, tt.cons, 100, tt.spacing), tt.want)
		})
	}
}

func TestUpstreamPrioritySpec(t *testing.T) {
	tests := []struct {
		name string
		cons []Constraint
		want [][2]int
	}{
		{"a", []Constraint{Length(25), Length(25)}, [][2]int{{0, 25}, {25, 75}}},
		{"b", []Constraint{Length(25), Percentage(25)}, [][2]int{{0, 25}, {25, 75}}},
		{"c", []Constraint{Percentage(25), Length(25)}, [][2]int{{0, 75}, {75, 25}}},
		{"d", []Constraint{Min(25), Percentage(25)}, [][2]int{{0, 75}, {75, 25}}},
		{"e", []Constraint{Percentage(25), Min(25)}, [][2]int{{0, 25}, {25, 75}}},
		{"f", []Constraint{Min(25), Percentage(100)}, [][2]int{{0, 25}, {25, 75}}},
		{"g", []Constraint{Percentage(100), Min(25)}, [][2]int{{0, 75}, {75, 25}}},
		{"h", []Constraint{Max(75), Percentage(75)}, [][2]int{{0, 25}, {25, 75}}},
		{"i", []Constraint{Percentage(75), Max(75)}, [][2]int{{0, 75}, {75, 25}}},
		{"j", []Constraint{Max(25), Percentage(25)}, [][2]int{{0, 25}, {25, 75}}},
		{"k", []Constraint{Percentage(25), Max(25)}, [][2]int{{0, 75}, {75, 25}}},
		{"l", []Constraint{Length(25), Ratio(1, 4)}, [][2]int{{0, 25}, {25, 75}}},
		{"m", []Constraint{Ratio(1, 4), Length(25)}, [][2]int{{0, 75}, {75, 25}}},
		{"n", []Constraint{Percentage(25), Ratio(1, 4)}, [][2]int{{0, 25}, {25, 75}}},
		{"o", []Constraint{Ratio(1, 4), Percentage(25)}, [][2]int{{0, 75}, {75, 25}}},
		{"p", []Constraint{Ratio(1, 4), Fill(25)}, [][2]int{{0, 25}, {25, 75}}},
		{"q", []Constraint{Fill(25), Ratio(1, 4)}, [][2]int{{0, 75}, {75, 25}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eqPairs(t, tt.name, pairsOf(FlexLegacy, tt.cons, 100, 0), tt.want)
		})
	}
}

func TestUpstreamFillVsFlexAndSpacing(t *testing.T) {
	tests := []struct {
		name    string
		flex    Flex
		spacing int
		cons    []Constraint
		want    [][2]int
	}{
		{"prop1", FlexLegacy, 0, []Constraint{Length(10), Fill(1), Length(10)}, [][2]int{{0, 10}, {10, 80}, {90, 10}}},
		{"flex_sb", FlexSpaceBetween, 0, []Constraint{Length(10), Length(10)}, [][2]int{{0, 10}, {90, 10}}},
		{"prop2", FlexLegacy, 0, []Constraint{Fill(1), Length(10), Fill(1), Length(10), Fill(1)}, [][2]int{{0, 27}, {27, 10}, {37, 26}, {63, 10}, {73, 27}}},
		{"flex_se", FlexSpaceEvenly, 0, []Constraint{Length(10), Length(10)}, [][2]int{{27, 10}, {63, 10}}},
		{"prop3", FlexLegacy, 0, []Constraint{Length(10), Length(10), Fill(1)}, [][2]int{{0, 10}, {10, 10}, {20, 80}}},
		{"flex_start", FlexStart, 0, []Constraint{Length(10), Length(10)}, [][2]int{{0, 10}, {10, 10}}},
		{"prop4", FlexLegacy, 0, []Constraint{Fill(1), Length(10), Length(10)}, [][2]int{{0, 80}, {80, 10}, {90, 10}}},
		{"flex_end", FlexEnd, 0, []Constraint{Length(10), Length(10)}, [][2]int{{80, 10}, {90, 10}}},
		{"prop5", FlexLegacy, 0, []Constraint{Fill(1), Length(10), Length(10), Fill(1)}, [][2]int{{0, 40}, {40, 10}, {50, 10}, {60, 40}}},
		{"flex_center", FlexCenter, 0, []Constraint{Length(10), Length(10)}, [][2]int{{40, 10}, {50, 10}}},
		{"flex_sa", FlexSpaceAround, 0, []Constraint{Length(10), Length(10)}, [][2]int{{20, 10}, {70, 10}}},
		// fill spacing
		{"fill_sp0", FlexStart, 0, []Constraint{Fill(1), Fill(1)}, [][2]int{{0, 50}, {50, 50}}},
		{"fill_sp10", FlexStart, 10, []Constraint{Fill(1), Fill(1)}, [][2]int{{0, 45}, {55, 45}}},
		{"fill_sp10_se", FlexSpaceEvenly, 10, []Constraint{Fill(1), Fill(1)}, [][2]int{{10, 35}, {55, 35}}},
		{"fill_sp10_sa", FlexSpaceAround, 10, []Constraint{Fill(1), Fill(1)}, [][2]int{{10, 30}, {60, 30}}},
		{"fill_len_sp10", FlexStart, 10, []Constraint{Fill(1), Length(10), Fill(1)}, [][2]int{{0, 35}, {45, 10}, {65, 35}}},
		{"fill_len_sp10_se", FlexSpaceEvenly, 10, []Constraint{Fill(1), Length(10), Fill(1)}, [][2]int{{10, 25}, {45, 10}, {65, 25}}},
		{"fill_len_sp10_sa", FlexSpaceAround, 10, []Constraint{Fill(1), Length(10), Fill(1)}, [][2]int{{10, 15}, {45, 10}, {75, 15}}},
		// fill overlap
		{"fill_ov-10", FlexStart, -10, []Constraint{Fill(1), Fill(1)}, [][2]int{{0, 55}, {45, 55}}},
		{"fill_ov-10_sa", FlexSpaceAround, -10, []Constraint{Fill(1), Fill(1)}, [][2]int{{0, 50}, {50, 50}}},
		{"fill_len_ov-10", FlexStart, -10, []Constraint{Fill(1), Length(10), Fill(1)}, [][2]int{{0, 55}, {45, 10}, {45, 55}}},
		{"fill_len_ov-1", FlexStart, -1, []Constraint{Fill(1), Length(10), Fill(1)}, [][2]int{{0, 46}, {45, 10}, {54, 46}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eqPairs(t, tt.name, pairsOf(tt.flex, tt.cons, 100, tt.spacing), tt.want)
		})
	}
}

func TestUpstreamSpacers(t *testing.T) {
	tests := []struct {
		name    string
		flex    Flex
		spacing int
		cons    []Constraint
		want    [][2]int
	}{
		{"no_legacy", FlexLegacy, 0, []Constraint{Length(10), Length(10)}, [][2]int{{0, 0}, {10, 0}, {100, 0}}},
		{"no_sb", FlexSpaceBetween, 0, []Constraint{Length(10), Length(10)}, [][2]int{{0, 0}, {10, 80}, {100, 0}}},
		{"no_se", FlexSpaceEvenly, 0, []Constraint{Length(10), Length(10)}, [][2]int{{0, 27}, {37, 26}, {73, 27}}},
		{"no_sa", FlexSpaceAround, 0, []Constraint{Length(10), Length(10)}, [][2]int{{0, 20}, {30, 40}, {80, 20}}},
		{"no_start", FlexStart, 0, []Constraint{Length(10), Length(10)}, [][2]int{{0, 0}, {10, 0}, {20, 80}}},
		{"no_center", FlexCenter, 0, []Constraint{Length(10), Length(10)}, [][2]int{{0, 40}, {50, 0}, {60, 40}}},
		{"no_end", FlexEnd, 0, []Constraint{Length(10), Length(10)}, [][2]int{{0, 80}, {90, 0}, {100, 0}}},
		{"sp5_legacy", FlexLegacy, 5, []Constraint{Length(10), Length(10)}, [][2]int{{0, 0}, {10, 5}, {100, 0}}},
		{"sp5_start", FlexStart, 5, []Constraint{Length(10), Length(10)}, [][2]int{{0, 0}, {10, 5}, {25, 75}}},
		{"sp5_center", FlexCenter, 5, []Constraint{Length(10), Length(10)}, [][2]int{{0, 38}, {48, 5}, {63, 37}}},
		{"sp5_end", FlexEnd, 5, []Constraint{Length(10), Length(10)}, [][2]int{{0, 75}, {85, 5}, {100, 0}}},
		{"ov_legacy", FlexLegacy, -1, []Constraint{Length(10), Length(10)}, [][2]int{{0, 0}, {10, 0}, {100, 0}}},
		{"ov_start", FlexStart, -1, []Constraint{Length(10), Length(10)}, [][2]int{{0, 0}, {10, 0}, {19, 81}}},
		{"ov_center", FlexCenter, -1, []Constraint{Length(10), Length(10)}, [][2]int{{0, 41}, {51, 0}, {60, 40}}},
		{"ov_end", FlexEnd, -1, []Constraint{Length(10), Length(10)}, [][2]int{{0, 81}, {91, 0}, {100, 0}}},
		{"too_legacy", FlexLegacy, 200, []Constraint{Length(10), Length(10)}, [][2]int{{0, 0}, {0, 100}, {100, 0}}},
		{"too_se", FlexSpaceEvenly, 200, []Constraint{Length(10), Length(10)}, [][2]int{{0, 33}, {33, 34}, {67, 33}}},
		{"too_sa", FlexSpaceAround, 200, []Constraint{Length(10), Length(10)}, [][2]int{{0, 25}, {25, 50}, {75, 25}}},
		{"too_center", FlexCenter, 200, []Constraint{Length(10), Length(10)}, [][2]int{{0, 0}, {0, 100}, {100, 0}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eqPairs(t, tt.name, spacerPairs(tt.flex, tt.cons, 100, tt.spacing), tt.want)
		})
	}
}

func TestUpstreamTableLength(t *testing.T) {
	eqPairs(t, "w7", rangesOf(FlexStart, []Constraint{Length(4), Length(4)}, 7, 1), [][2]int{{0, 3}, {4, 7}})
	eqPairs(t, "w4", rangesOf(FlexStart, []Constraint{Length(4), Length(4)}, 4, 1), [][2]int{{0, 2}, {3, 4}})
}

func TestUpstreamLegacyVsDefault(t *testing.T) {
	tests := []struct {
		name string
		flex Flex
		cons []Constraint
		want [][2]int
	}{
		{"min_len_leg", FlexLegacy, []Constraint{Min(10), Length(10)}, [][2]int{{0, 90}, {90, 10}}},
		{"min_len_start", FlexStart, []Constraint{Min(10), Length(10)}, [][2]int{{0, 90}, {90, 10}}},
		{"min_pct_leg", FlexLegacy, []Constraint{Min(10), Percentage(100)}, [][2]int{{0, 10}, {10, 90}}},
		{"min_pct_start", FlexStart, []Constraint{Min(10), Percentage(100)}, [][2]int{{0, 10}, {10, 90}}},
		{"pct_leg", FlexLegacy, []Constraint{Percentage(50), Percentage(50)}, [][2]int{{0, 50}, {50, 50}}},
		{"pct_start", FlexStart, []Constraint{Percentage(50), Percentage(50)}, [][2]int{{0, 50}, {50, 50}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eqPairs(t, tt.name, pairsOf(tt.flex, tt.cons, 100, 0), tt.want)
		})
	}
}

func TestUpstreamEdgeCases(t *testing.T) {
	{
		got := Vertical(Percentage(50), Percentage(50), Min(0)).Split(NewRect(0, 0, 1, 1))
		want := []Rect{NewRect(0, 0, 1, 1), NewRect(0, 1, 1, 0), NewRect(0, 1, 1, 0)}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("edge1 %d got %+v want %+v full=%v", i, got[i], want[i], got)
			}
		}
	}
	{
		got := Vertical(Max(1), Percentage(99), Min(0)).Split(NewRect(0, 0, 1, 1))
		want := []Rect{NewRect(0, 0, 1, 0), NewRect(0, 0, 1, 1), NewRect(0, 1, 1, 0)}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("edge2 %d got %+v want %+v full=%v", i, got[i], want[i], got)
			}
		}
	}
	{
		got := Horizontal(Min(1), Length(0), Min(1)).Split(NewRect(0, 0, 1, 1))
		want := []Rect{NewRect(0, 0, 1, 1), NewRect(1, 0, 0, 1), NewRect(1, 0, 0, 1)}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("edge3 %d got %+v want %+v full=%v", i, got[i], want[i], got)
			}
		}
	}
	{
		got := Horizontal(Length(3), Min(4), Length(1), Min(4)).Split(NewRect(0, 0, 7, 1))
		want := []Rect{NewRect(0, 0, 0, 1), NewRect(0, 0, 4, 1), NewRect(4, 0, 0, 1), NewRect(4, 0, 3, 1)}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("edge4 %d got %+v want %+v full=%v", i, got[i], want[i], got)
			}
		}
	}
}

func TestConstraintApply(t *testing.T) {
	// Rust Apply uses f32 then truncates toward zero via `as u16`.
	cases := []struct {
		c            Constraint
		length, want int
	}{
		{Percentage(50), 10, 5},
		{Percentage(33), 10, 3},
		{Percentage(200), 10, 10},
		{Ratio(1, 3), 10, 3},
		{Ratio(1, 0), 10, 10},
		{Ratio(0, 0), 10, 0},
		{Length(4), 10, 4},
		{Length(20), 10, 10},
		{Fill(4), 10, 4},
		{Max(4), 10, 4},
		{Min(4), 10, 10},
		{Min(20), 10, 20},
	}
	for _, tt := range cases {
		if got := tt.c.Apply(tt.length); got != tt.want {
			t.Fatalf("Apply(%v,%d)=%d want %d", tt.c, tt.length, got, tt.want)
		}
	}
}

func TestFromHelpers(t *testing.T) {
	if g := FromLengths(1, 2); g[0] != Length(1) || g[1] != Length(2) {
		t.Fatal(g)
	}
	if g := FromMins(1, 2); g[0] != Min(1) || g[1] != Min(2) {
		t.Fatal(g)
	}
	if g := FromMaxes(1, 2); g[0] != Max(1) || g[1] != Max(2) {
		t.Fatal(g)
	}
	if g := FromPercentages(1, 2); g[0] != Percentage(1) || g[1] != Percentage(2) {
		t.Fatal(g)
	}
	if g := FromFills(1, 2); g[0] != Fill(1) || g[1] != Fill(2) {
		t.Fatal(g)
	}
	if g := FromRatios(RatioPair{1, 2}, RatioPair{3, 4}); g[0] != Ratio(1, 2) || g[1] != Ratio(3, 4) {
		t.Fatal(g)
	}
}
