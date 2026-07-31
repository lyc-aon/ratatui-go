package style

import "math"

// FromHSL converts hue in degrees and saturation/lightness in [0,1] to RGB.
// Hue wraps; saturation and lightness clamp, matching Ratatui's palette-backed
// Color::from_hsl behavior.
func FromHSL(hue, saturation, lightness float64) Color {
	h := normalizeHue(hue) / 360
	s := clampFloat(saturation, 0, 1)
	l := clampFloat(lightness, 0, 1)

	if s == 0 {
		v := floatToByte(l)
		return RGB(v, v, v)
	}
	q := l * (1 + s)
	if l >= 0.5 {
		q = l + s - l*s
	}
	p := 2*l - q
	return RGB(
		floatToByte(hueChannel(p, q, h+1.0/3.0)),
		floatToByte(hueChannel(p, q, h)),
		floatToByte(hueChannel(p, q, h-1.0/3.0)),
	)
}

func hueChannel(p, q, t float64) float64 {
	if t < 0 {
		t++
	}
	if t > 1 {
		t--
	}
	switch {
	case t < 1.0/6.0:
		return p + (q-p)*6*t
	case t < 0.5:
		return q
	case t < 2.0/3.0:
		return p + (q-p)*(2.0/3.0-t)*6
	default:
		return p
	}
}

// FromHSLuv converts hue in degrees and saturation/lightness in [0,100] to
// sRGB. Hue wraps; saturation and lightness clamp. HSLuv is perceptually
// uniform and matches Ratatui's palette-backed Color::from_hsluv behavior.
func FromHSLuv(hue, saturation, lightness float64) Color {
	h := normalizeHue(hue)
	s := clampFloat(saturation, 0, 100)
	l := clampFloat(lightness, 0, 100)

	chroma := 0.0
	if l > 0.00000001 && l < 99.9999999 {
		chroma = maxChromaForLH(l, h) * s / 100
	}
	hr := h * math.Pi / 180
	u := math.Cos(hr) * chroma
	v := math.Sin(hr) * chroma
	x, y, z := luvToXYZ(l, u, v)
	return xyzToRGB(x, y, z)
}

const (
	hsluvRefU    = 0.19783000664283
	hsluvRefV    = 0.46831999493879
	hsluvKappa   = 903.2962962
	hsluvEpsilon = 0.0088564516
)

var hsluvMatrix = [3][3]float64{
	{3.240969941904521, -1.537383177570093, -0.498610760293},
	{-0.96924363628087, 1.87596750150772, 0.041555057407175},
	{0.055630079696993, -0.20397695888897, 1.056971514242878},
}

type hsluvLine struct{ slope, intercept float64 }

func hsluvBounds(l float64) [6]hsluvLine {
	sub1 := math.Pow(l+16, 3) / 1560896
	sub2 := l / hsluvKappa
	if sub1 > hsluvEpsilon {
		sub2 = sub1
	}
	var out [6]hsluvLine
	n := 0
	for row := range hsluvMatrix {
		m1, m2, m3 := hsluvMatrix[row][0], hsluvMatrix[row][1], hsluvMatrix[row][2]
		for t := 0; t < 2; t++ {
			top1 := (284517*m1 - 94839*m3) * sub2
			top2 := (838422*m3+769860*m2+731718*m1)*l*sub2 - 769860*float64(t)*l
			bottom := (632260*m3-126452*m2)*sub2 + 126452*float64(t)
			out[n] = hsluvLine{slope: top1 / bottom, intercept: top2 / bottom}
			n++
		}
	}
	return out
}

func maxChromaForLH(l, h float64) float64 {
	hr := h * math.Pi / 180
	minLength := math.Inf(1)
	for _, line := range hsluvBounds(l) {
		length := line.intercept / (math.Sin(hr) - line.slope*math.Cos(hr))
		if length >= 0 && length < minLength {
			minLength = length
		}
	}
	return minLength
}

func luvToXYZ(l, u, v float64) (x, y, z float64) {
	if l == 0 {
		return 0, 0, 0
	}
	varU := u/(13*l) + hsluvRefU
	varV := v/(13*l) + hsluvRefV
	if l > 8 {
		y = math.Pow((l+16)/116, 3)
	} else {
		y = l / hsluvKappa
	}
	x = -(9 * y * varU) / ((varU-4)*varV - varU*varV)
	z = (9*y - 15*varV*y - varV*x) / (3 * varV)
	return x, y, z
}

func xyzToRGB(x, y, z float64) Color {
	linear := [3]float64{
		hsluvMatrix[0][0]*x + hsluvMatrix[0][1]*y + hsluvMatrix[0][2]*z,
		hsluvMatrix[1][0]*x + hsluvMatrix[1][1]*y + hsluvMatrix[1][2]*z,
		hsluvMatrix[2][0]*x + hsluvMatrix[2][1]*y + hsluvMatrix[2][2]*z,
	}
	for i := range linear {
		if linear[i] <= 0.0031308 {
			linear[i] *= 12.92
		} else {
			linear[i] = 1.055*math.Pow(linear[i], 1/2.4) - 0.055
		}
	}
	return RGB(floatToByte(linear[0]), floatToByte(linear[1]), floatToByte(linear[2]))
}

func normalizeHue(h float64) float64 {
	if math.IsNaN(h) || math.IsInf(h, 0) {
		return 0
	}
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	return h
}

func clampFloat(v, low, high float64) float64 {
	if math.IsNaN(v) || v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func floatToByte(v float64) uint8 {
	v = clampFloat(v, 0, 1)
	return uint8(math.Round(v * 255))
}
