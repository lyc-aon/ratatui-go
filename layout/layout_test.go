package layout

import (
	"testing"
)

func TestStandardLengthFill(t *testing.T) {
	tests := []struct {
		name        string
		direction   Direction
		constraints []Constraint
		area        Rect
		want        []Rect
	}{
		{
			name:        "horizontal length and fill",
			direction:   HorizontalDir,
			constraints: []Constraint{Length(10), Fill(1)},
			area:        NewRect(0, 0, 30, 10),
			want: []Rect{
				NewRect(0, 0, 10, 10),
				NewRect(10, 0, 20, 10),
			},
		},
		{
			name:        "vertical multiple fills with weights",
			direction:   VerticalDir,
			constraints: []Constraint{Fill(1), Fill(2)},
			area:        NewRect(0, 0, 10, 30),
			want: []Rect{
				NewRect(0, 0, 10, 10),
				NewRect(0, 10, 10, 20),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.direction, tt.constraints...).Split(tt.area)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d segments, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("segment %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCoreReviewTwoMinFillCounterexample(t *testing.T) {
	// Under FlexLegacy, Min does not grow (grow=0); leftover goes to Fill.
	// Under non-legacy flex, Min and Fill equalize TOTAL sizes by weight
	// (Min weight 1), with Min as a floor only.
	area := NewRect(0, 0, 30, 10)
	constraints := []Constraint{Min(10), Fill(1)}

	tests := []struct {
		name      string
		flex      Flex
		wantMinW  int
		wantFillW int
	}{
		{
			name:      "flex legacy min stays at min size",
			flex:      FlexLegacy,
			wantMinW:  10,
			wantFillW: 20,
		},
		{
			name:      "flex start min equalizes total size with fill",
			flex:      FlexStart,
			wantMinW:  15,
			wantFillW: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segs := Horizontal(constraints...).Flex(tt.flex).Split(area)
			if len(segs) != 2 {
				t.Fatalf("expected 2 segments, got %d", len(segs))
			}
			if segs[0].Width != tt.wantMinW {
				t.Errorf("Min segment width = %d, want %d", segs[0].Width, tt.wantMinW)
			}
			if segs[1].Width != tt.wantFillW {
				t.Errorf("Fill segment width = %d, want %d", segs[1].Width, tt.wantFillW)
			}
		})
	}
}

func TestPositiveSpacingFloors(t *testing.T) {
	area := NewRect(0, 0, 20, 10)
	constraints := []Constraint{Length(5), Length(5)}

	tests := []struct {
		name          string
		flex          Flex
		spacing       int
		wantMinSpacer int
	}{
		{
			name:          "space between respects positive spacing floor",
			flex:          FlexSpaceBetween,
			spacing:       4,
			wantMinSpacer: 4,
		},
		{
			name:          "space evenly respects positive spacing floor",
			flex:          FlexSpaceEvenly,
			spacing:       2,
			wantMinSpacer: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, spacers := Horizontal(constraints...).Flex(tt.flex).Spacing(tt.spacing).SplitWithSpacers(area)
			if len(spacers) < 3 {
				t.Fatalf("expected at least 3 spacers, got %d", len(spacers))
			}
			// Interior spacer
			if spacers[1].Width < tt.wantMinSpacer {
				t.Errorf("interior spacer width = %d, want >= %d", spacers[1].Width, tt.wantMinSpacer)
			}
		})
	}
}

func TestLegacyMaxZero(t *testing.T) {
	area := NewRect(0, 0, 10, 10)

	tests := []struct {
		name      string
		flex      Flex
		wantWidth int
	}{
		{
			name:      "flex legacy expands Max(0) to consume leftover",
			flex:      FlexLegacy,
			wantWidth: 10,
		},
		{
			name:      "flex start keeps Max(0) at size 0",
			flex:      FlexStart,
			wantWidth: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segs := Horizontal(Max(0)).Flex(tt.flex).Split(area)
			if len(segs) != 1 {
				t.Fatalf("expected 1 segment, got %d", len(segs))
			}
			if segs[0].Width != tt.wantWidth {
				t.Errorf("Max(0) segment width = %d, want %d", segs[0].Width, tt.wantWidth)
			}
		})
	}
}

func TestZeroConstraintSpacer(t *testing.T) {
	area := NewRect(5, 5, 20, 10)
	segs, spacers := New(HorizontalDir).SplitWithSpacers(area)

	if len(segs) != 0 {
		t.Errorf("expected 0 segments for zero constraints, got %d", len(segs))
	}
	if len(spacers) != 1 {
		t.Fatalf("expected 1 spacer for zero constraints, got %d", len(spacers))
	}
	if spacers[0] != area {
		t.Errorf("spacer = %+v, want full area %+v", spacers[0], area)
	}
}

func TestProbeCase558SpaceAroundOverlap(t *testing.T) {
	// Stable overconstrained Ratio tie under FlexSpaceAround + negative spacing.
	// Upstream 12/12: widths 0,61,4,5,4,0 starting at x=18.
	area := NewRect(18, 7, 74, 18)
	cons := []Constraint{Max(41), Percentage(82), Ratio(54, 11), Ratio(51, 5), Ratio(161, 18), Fill(29)}
	segs, sps := Horizontal(cons...).Flex(FlexSpaceAround).Spacing(-5).SplitWithSpacers(area)
	wantW := []int{0, 61, 4, 5, 4, 0}
	wantX := []int{18, 18, 79, 83, 88, 92}
	if len(segs) != len(wantW) {
		t.Fatalf("seg len %d", len(segs))
	}
	for i := range wantW {
		if segs[i].X != wantX[i] || segs[i].Width != wantW[i] {
			t.Fatalf("seg %d got (%d,%d) want (%d,%d) full=%v", i, segs[i].X, segs[i].Width, wantX[i], wantW[i], segs)
		}
	}
	// spacers all width 0 at segment boundaries
	wantSX := []int{18, 18, 79, 83, 88, 92, 92}
	if len(sps) != len(wantSX) {
		t.Fatalf("spacer len %d", len(sps))
	}
	for i := range wantSX {
		if sps[i].X != wantSX[i] || sps[i].Width != 0 {
			t.Fatalf("sp %d got (%d,%d) want (%d,0)", i, sps[i].X, sps[i].Width, wantSX[i])
		}
	}
}
