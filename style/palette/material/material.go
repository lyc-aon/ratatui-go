package material

import "github.com/michaelkelly/ratatui-go/style"

// AccentedPalette contains variants C50..C900 and A100/A200/A400/A700.
type AccentedPalette struct {
	C50  style.Color
	C100 style.Color
	C200 style.Color
	C300 style.Color
	C400 style.Color
	C500 style.Color
	C600 style.Color
	C700 style.Color
	C800 style.Color
	C900 style.Color
	A100 style.Color
	A200 style.Color
	A400 style.Color
	A700 style.Color
}

// NonAccentedPalette contains variants C50..C900.
type NonAccentedPalette struct {
	C50  style.Color
	C100 style.Color
	C200 style.Color
	C300 style.Color
	C400 style.Color
	C500 style.Color
	C600 style.Color
	C700 style.Color
	C800 style.Color
	C900 style.Color
}

var (
	Black = style.FromU32(0x000000)
	White = style.FromU32(0xFFFFFF)
)

var (
	Red = AccentedPalette{
		C50:  style.FromU32(0xFFEBEE),
		C100: style.FromU32(0xFFCDD2),
		C200: style.FromU32(0xEF9A9A),
		C300: style.FromU32(0xE57373),
		C400: style.FromU32(0xEF5350),
		C500: style.FromU32(0xF44336),
		C600: style.FromU32(0xE53935),
		C700: style.FromU32(0xD32F2F),
		C800: style.FromU32(0xC62828),
		C900: style.FromU32(0xB71C1C),
		A100: style.FromU32(0xFF8A80),
		A200: style.FromU32(0xFF5252),
		A400: style.FromU32(0xFF1744),
		A700: style.FromU32(0xD50000),
	}
	Pink = AccentedPalette{
		C50:  style.FromU32(0xFCE4EC),
		C100: style.FromU32(0xF8BBD0),
		C200: style.FromU32(0xF48FB1),
		C300: style.FromU32(0xF06292),
		C400: style.FromU32(0xEC407A),
		C500: style.FromU32(0xE91E63),
		C600: style.FromU32(0xD81B60),
		C700: style.FromU32(0xC2185B),
		C800: style.FromU32(0xAD1457),
		C900: style.FromU32(0x880E4F),
		A100: style.FromU32(0xFF80AB),
		A200: style.FromU32(0xFF4081),
		A400: style.FromU32(0xF50057),
		A700: style.FromU32(0xC51162),
	}
	Purple = AccentedPalette{
		C50:  style.FromU32(0xF3E5F5),
		C100: style.FromU32(0xE1BEE7),
		C200: style.FromU32(0xCE93D8),
		C300: style.FromU32(0xBA68C8),
		C400: style.FromU32(0xAB47BC),
		C500: style.FromU32(0x9C27B0),
		C600: style.FromU32(0x8E24AA),
		C700: style.FromU32(0x7B1FA2),
		C800: style.FromU32(0x6A1B9A),
		C900: style.FromU32(0x4A148C),
		A100: style.FromU32(0xEA80FC),
		A200: style.FromU32(0xE040FB),
		A400: style.FromU32(0xD500F9),
		A700: style.FromU32(0xAA00FF),
	}
	DeepPurple = AccentedPalette{
		C50:  style.FromU32(0xEDE7F6),
		C100: style.FromU32(0xD1C4E9),
		C200: style.FromU32(0xB39DDB),
		C300: style.FromU32(0x9575CD),
		C400: style.FromU32(0x7E57C2),
		C500: style.FromU32(0x673AB7),
		C600: style.FromU32(0x5E35B1),
		C700: style.FromU32(0x512DA8),
		C800: style.FromU32(0x4527A0),
		C900: style.FromU32(0x311B92),
		A100: style.FromU32(0xB388FF),
		A200: style.FromU32(0x7C4DFF),
		A400: style.FromU32(0x651FFF),
		A700: style.FromU32(0x6200EA),
	}
	Indigo = AccentedPalette{
		C50:  style.FromU32(0xE8EAF6),
		C100: style.FromU32(0xC5CAE9),
		C200: style.FromU32(0x9FA8DA),
		C300: style.FromU32(0x7986CB),
		C400: style.FromU32(0x5C6BC0),
		C500: style.FromU32(0x3F51B5),
		C600: style.FromU32(0x3949AB),
		C700: style.FromU32(0x303F9F),
		C800: style.FromU32(0x283593),
		C900: style.FromU32(0x1A237E),
		A100: style.FromU32(0x8C9EFF),
		A200: style.FromU32(0x536DFE),
		A400: style.FromU32(0x3D5AFE),
		A700: style.FromU32(0x304FFE),
	}
	Blue = AccentedPalette{
		C50:  style.FromU32(0xE3F2FD),
		C100: style.FromU32(0xBBDEFB),
		C200: style.FromU32(0x90CAF9),
		C300: style.FromU32(0x64B5F6),
		C400: style.FromU32(0x42A5F5),
		C500: style.FromU32(0x2196F3),
		C600: style.FromU32(0x1E88E5),
		C700: style.FromU32(0x1976D2),
		C800: style.FromU32(0x1565C0),
		C900: style.FromU32(0x0D47A1),
		A100: style.FromU32(0x82B1FF),
		A200: style.FromU32(0x448AFF),
		A400: style.FromU32(0x2979FF),
		A700: style.FromU32(0x2962FF),
	}
	LightBlue = AccentedPalette{
		C50:  style.FromU32(0xE1F5FE),
		C100: style.FromU32(0xB3E5FC),
		C200: style.FromU32(0x81D4FA),
		C300: style.FromU32(0x4FC3F7),
		C400: style.FromU32(0x29B6F6),
		C500: style.FromU32(0x03A9F4),
		C600: style.FromU32(0x039BE5),
		C700: style.FromU32(0x0288D1),
		C800: style.FromU32(0x0277BD),
		C900: style.FromU32(0x01579B),
		A100: style.FromU32(0x80D8FF),
		A200: style.FromU32(0x40C4FF),
		A400: style.FromU32(0x00B0FF),
		A700: style.FromU32(0x0091EA),
	}
	Cyan = AccentedPalette{
		C50:  style.FromU32(0xE0F7FA),
		C100: style.FromU32(0xB2EBF2),
		C200: style.FromU32(0x80DEEA),
		C300: style.FromU32(0x4DD0E1),
		C400: style.FromU32(0x26C6DA),
		C500: style.FromU32(0x00BCD4),
		C600: style.FromU32(0x00ACC1),
		C700: style.FromU32(0x0097A7),
		C800: style.FromU32(0x00838F),
		C900: style.FromU32(0x006064),
		A100: style.FromU32(0x84FFFF),
		A200: style.FromU32(0x18FFFF),
		A400: style.FromU32(0x00E5FF),
		A700: style.FromU32(0x00B8D4),
	}
	Teal = AccentedPalette{
		C50:  style.FromU32(0xE0F2F1),
		C100: style.FromU32(0xB2DFDB),
		C200: style.FromU32(0x80CBC4),
		C300: style.FromU32(0x4DB6AC),
		C400: style.FromU32(0x26A69A),
		C500: style.FromU32(0x009688),
		C600: style.FromU32(0x00897B),
		C700: style.FromU32(0x00796B),
		C800: style.FromU32(0x00695C),
		C900: style.FromU32(0x004D40),
		A100: style.FromU32(0xA7FFEB),
		A200: style.FromU32(0x64FFDA),
		A400: style.FromU32(0x1DE9B6),
		A700: style.FromU32(0x00BFA5),
	}
	Green = AccentedPalette{
		C50:  style.FromU32(0xE8F5E9),
		C100: style.FromU32(0xC8E6C9),
		C200: style.FromU32(0xA5D6A7),
		C300: style.FromU32(0x81C784),
		C400: style.FromU32(0x66BB6A),
		C500: style.FromU32(0x4CAF50),
		C600: style.FromU32(0x43A047),
		C700: style.FromU32(0x388E3C),
		C800: style.FromU32(0x2E7D32),
		C900: style.FromU32(0x1B5E20),
		A100: style.FromU32(0xB9F6CA),
		A200: style.FromU32(0x69F0AE),
		A400: style.FromU32(0x00E676),
		A700: style.FromU32(0x00C853),
	}
	LightGreen = AccentedPalette{
		C50:  style.FromU32(0xF1F8E9),
		C100: style.FromU32(0xDCEDC8),
		C200: style.FromU32(0xC5E1A5),
		C300: style.FromU32(0xAED581),
		C400: style.FromU32(0x9CCC65),
		C500: style.FromU32(0x8BC34A),
		C600: style.FromU32(0x7CB342),
		C700: style.FromU32(0x689F38),
		C800: style.FromU32(0x558B2F),
		C900: style.FromU32(0x33691E),
		A100: style.FromU32(0xCCFF90),
		A200: style.FromU32(0xB2FF59),
		A400: style.FromU32(0x76FF03),
		A700: style.FromU32(0x64DD17),
	}
	Lime = AccentedPalette{
		C50:  style.FromU32(0xF9FBE7),
		C100: style.FromU32(0xF0F4C3),
		C200: style.FromU32(0xE6EE9C),
		C300: style.FromU32(0xDCE775),
		C400: style.FromU32(0xD4E157),
		C500: style.FromU32(0xCDDC39),
		C600: style.FromU32(0xC0CA33),
		C700: style.FromU32(0xAFB42B),
		C800: style.FromU32(0x9E9D24),
		C900: style.FromU32(0x827717),
		A100: style.FromU32(0xF4FF81),
		A200: style.FromU32(0xEEFF41),
		A400: style.FromU32(0xC6FF00),
		A700: style.FromU32(0xAEEA00),
	}
	Yellow = AccentedPalette{
		C50:  style.FromU32(0xFFFDE7),
		C100: style.FromU32(0xFFF9C4),
		C200: style.FromU32(0xFFF59D),
		C300: style.FromU32(0xFFF176),
		C400: style.FromU32(0xFFEE58),
		C500: style.FromU32(0xFFEB3B),
		C600: style.FromU32(0xFDD835),
		C700: style.FromU32(0xFBC02D),
		C800: style.FromU32(0xF9A825),
		C900: style.FromU32(0xF57F17),
		A100: style.FromU32(0xFFFF8D),
		A200: style.FromU32(0xFFFF00),
		A400: style.FromU32(0xFFEA00),
		A700: style.FromU32(0xFFD600),
	}
	Amber = AccentedPalette{
		C50:  style.FromU32(0xFFF8E1),
		C100: style.FromU32(0xFFECB3),
		C200: style.FromU32(0xFFE082),
		C300: style.FromU32(0xFFD54F),
		C400: style.FromU32(0xFFCA28),
		C500: style.FromU32(0xFFC107),
		C600: style.FromU32(0xFFB300),
		C700: style.FromU32(0xFFA000),
		C800: style.FromU32(0xFF8F00),
		C900: style.FromU32(0xFF6F00),
		A100: style.FromU32(0xFFE57F),
		A200: style.FromU32(0xFFD740),
		A400: style.FromU32(0xFFC400),
		A700: style.FromU32(0xFFAB00),
	}
	Orange = AccentedPalette{
		C50:  style.FromU32(0xFFF3E0),
		C100: style.FromU32(0xFFE0B2),
		C200: style.FromU32(0xFFCC80),
		C300: style.FromU32(0xFFB74D),
		C400: style.FromU32(0xFFA726),
		C500: style.FromU32(0xFF9800),
		C600: style.FromU32(0xFB8C00),
		C700: style.FromU32(0xF57C00),
		C800: style.FromU32(0xEF6C00),
		C900: style.FromU32(0xE65100),
		A100: style.FromU32(0xFFD180),
		A200: style.FromU32(0xFFAB40),
		A400: style.FromU32(0xFF9100),
		A700: style.FromU32(0xFF6D00),
	}
	DeepOrange = AccentedPalette{
		C50:  style.FromU32(0xFBE9E7),
		C100: style.FromU32(0xFFCCBC),
		C200: style.FromU32(0xFFAB91),
		C300: style.FromU32(0xFF8A65),
		C400: style.FromU32(0xFF7043),
		C500: style.FromU32(0xFF5722),
		C600: style.FromU32(0xF4511E),
		C700: style.FromU32(0xE64A19),
		C800: style.FromU32(0xD84315),
		C900: style.FromU32(0xBF360C),
		A100: style.FromU32(0xFF9E80),
		A200: style.FromU32(0xFF6E40),
		A400: style.FromU32(0xFF3D00),
		A700: style.FromU32(0xDD2C00),
	}
	Brown = NonAccentedPalette{
		C50:  style.FromU32(0xEFEBE9),
		C100: style.FromU32(0xD7CCC8),
		C200: style.FromU32(0xBCAAA4),
		C300: style.FromU32(0xA1887F),
		C400: style.FromU32(0x8D6E63),
		C500: style.FromU32(0x795548),
		C600: style.FromU32(0x6D4C41),
		C700: style.FromU32(0x5D4037),
		C800: style.FromU32(0x4E342E),
		C900: style.FromU32(0x3E2723),
	}
	Gray = NonAccentedPalette{
		C50:  style.FromU32(0xFAFAFA),
		C100: style.FromU32(0xF5F5F5),
		C200: style.FromU32(0xEEEEEE),
		C300: style.FromU32(0xE0E0E0),
		C400: style.FromU32(0xBDBDBD),
		C500: style.FromU32(0x9E9E9E),
		C600: style.FromU32(0x757575),
		C700: style.FromU32(0x616161),
		C800: style.FromU32(0x424242),
		C900: style.FromU32(0x212121),
	}
	BlueGray = NonAccentedPalette{
		C50:  style.FromU32(0xECEFF1),
		C100: style.FromU32(0xCFD8DC),
		C200: style.FromU32(0xB0BEC5),
		C300: style.FromU32(0x90A4AE),
		C400: style.FromU32(0x78909C),
		C500: style.FromU32(0x607D8B),
		C600: style.FromU32(0x546E7A),
		C700: style.FromU32(0x455A64),
		C800: style.FromU32(0x37474F),
		C900: style.FromU32(0x263238),
	}
)
