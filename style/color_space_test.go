package style

import "testing"

func TestFromHSL(t *testing.T) {
	tests := []struct {
		name    string
		h, s, l float64
		want    Color
	}{
		{"black", 0, 0, 0, RGB(0, 0, 0)},
		{"white", 0, 0, 1, RGB(255, 255, 255)},
		{"gray", 0, 0, 0.5, RGB(128, 128, 128)},
		{"red", 0, 1, 0.5, RGB(255, 0, 0)},
		{"blue wrapped negative", -120, 1, 0.5, RGB(0, 0, 255)},
		{"clamped", 360, 2, 0.5, RGB(255, 0, 0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FromHSL(tt.h, tt.s, tt.l); got != tt.want {
				t.Fatalf("FromHSL(%v,%v,%v) = %v, want %v", tt.h, tt.s, tt.l, got, tt.want)
			}
		})
	}
}

func TestFromHSLuv(t *testing.T) {
	tests := []struct {
		name    string
		h, s, l float64
		want    Color
	}{
		{"black", 0, 100, 0, RGB(0, 0, 0)},
		{"white", 0, 0, 100, RGB(255, 255, 255)},
		{"gray", 0, 0, 50, RGB(119, 119, 119)},
		{"red", 12.18, 100, 53.2, RGB(255, 0, 0)},
		{"blue", -94.13, 100, 32.3, RGB(0, 0, 255)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FromHSLuv(tt.h, tt.s, tt.l); got != tt.want {
				t.Fatalf("FromHSLuv(%v,%v,%v) = %v, want %v", tt.h, tt.s, tt.l, got, tt.want)
			}
		})
	}
}
