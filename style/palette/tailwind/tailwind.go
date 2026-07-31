package tailwind

import "github.com/michaelkelly/ratatui-go/style"

// Palette contains variants C50..C950.
type Palette struct {
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
	C950 style.Color
}

var (
	Black = style.FromU32(0x000000)
	White = style.FromU32(0xFFFFFF)
)

var (
	Slate = Palette{
		C50:  style.FromU32(0xf8fafc),
		C100: style.FromU32(0xf1f5f9),
		C200: style.FromU32(0xe2e8f0),
		C300: style.FromU32(0xcbd5e1),
		C400: style.FromU32(0x94a3b8),
		C500: style.FromU32(0x64748b),
		C600: style.FromU32(0x475569),
		C700: style.FromU32(0x334155),
		C800: style.FromU32(0x1e293b),
		C900: style.FromU32(0x0f172a),
		C950: style.FromU32(0x020617),
	}
	Gray = Palette{
		C50:  style.FromU32(0xf9fafb),
		C100: style.FromU32(0xf3f4f6),
		C200: style.FromU32(0xe5e7eb),
		C300: style.FromU32(0xd1d5db),
		C400: style.FromU32(0x9ca3af),
		C500: style.FromU32(0x6b7280),
		C600: style.FromU32(0x4b5563),
		C700: style.FromU32(0x374151),
		C800: style.FromU32(0x1f2937),
		C900: style.FromU32(0x111827),
		C950: style.FromU32(0x030712),
	}
	Zinc = Palette{
		C50:  style.FromU32(0xfafafa),
		C100: style.FromU32(0xf4f4f5),
		C200: style.FromU32(0xe4e4e7),
		C300: style.FromU32(0xd4d4d8),
		C400: style.FromU32(0xa1a1aa),
		C500: style.FromU32(0x71717a),
		C600: style.FromU32(0x52525b),
		C700: style.FromU32(0x3f3f46),
		C800: style.FromU32(0x27272a),
		C900: style.FromU32(0x18181b),
		C950: style.FromU32(0x09090b),
	}
	Neutral = Palette{
		C50:  style.FromU32(0xfafafa),
		C100: style.FromU32(0xf5f5f5),
		C200: style.FromU32(0xe5e5e5),
		C300: style.FromU32(0xd4d4d4),
		C400: style.FromU32(0xa3a3a3),
		C500: style.FromU32(0x737373),
		C600: style.FromU32(0x525252),
		C700: style.FromU32(0x404040),
		C800: style.FromU32(0x262626),
		C900: style.FromU32(0x171717),
		C950: style.FromU32(0x0a0a0a),
	}
	Stone = Palette{
		C50:  style.FromU32(0xfafaf9),
		C100: style.FromU32(0xf5f5f4),
		C200: style.FromU32(0xe7e5e4),
		C300: style.FromU32(0xd6d3d1),
		C400: style.FromU32(0xa8a29e),
		C500: style.FromU32(0x78716c),
		C600: style.FromU32(0x57534e),
		C700: style.FromU32(0x44403c),
		C800: style.FromU32(0x292524),
		C900: style.FromU32(0x1c1917),
		C950: style.FromU32(0x0c0a09),
	}
	Red = Palette{
		C50:  style.FromU32(0xfef2f2),
		C100: style.FromU32(0xfee2e2),
		C200: style.FromU32(0xfecaca),
		C300: style.FromU32(0xfca5a5),
		C400: style.FromU32(0xf87171),
		C500: style.FromU32(0xef4444),
		C600: style.FromU32(0xdc2626),
		C700: style.FromU32(0xb91c1c),
		C800: style.FromU32(0x991b1b),
		C900: style.FromU32(0x7f1d1d),
		C950: style.FromU32(0x450a0a),
	}
	Orange = Palette{
		C50:  style.FromU32(0xfff7ed),
		C100: style.FromU32(0xffedd5),
		C200: style.FromU32(0xfed7aa),
		C300: style.FromU32(0xfdba74),
		C400: style.FromU32(0xfb923c),
		C500: style.FromU32(0xf97316),
		C600: style.FromU32(0xea580c),
		C700: style.FromU32(0xc2410c),
		C800: style.FromU32(0x9a3412),
		C900: style.FromU32(0x7c2d12),
		C950: style.FromU32(0x431407),
	}
	Amber = Palette{
		C50:  style.FromU32(0xfffbeb),
		C100: style.FromU32(0xfef3c7),
		C200: style.FromU32(0xfde68a),
		C300: style.FromU32(0xfcd34d),
		C400: style.FromU32(0xfbbf24),
		C500: style.FromU32(0xf59e0b),
		C600: style.FromU32(0xd97706),
		C700: style.FromU32(0xb45309),
		C800: style.FromU32(0x92400e),
		C900: style.FromU32(0x78350f),
		C950: style.FromU32(0x451a03),
	}
	Yellow = Palette{
		C50:  style.FromU32(0xfefce8),
		C100: style.FromU32(0xfef9c3),
		C200: style.FromU32(0xfef08a),
		C300: style.FromU32(0xfde047),
		C400: style.FromU32(0xfacc15),
		C500: style.FromU32(0xeab308),
		C600: style.FromU32(0xca8a04),
		C700: style.FromU32(0xa16207),
		C800: style.FromU32(0x854d0e),
		C900: style.FromU32(0x713f12),
		C950: style.FromU32(0x422006),
	}
	Lime = Palette{
		C50:  style.FromU32(0xf7fee7),
		C100: style.FromU32(0xecfccb),
		C200: style.FromU32(0xd9f99d),
		C300: style.FromU32(0xbef264),
		C400: style.FromU32(0xa3e635),
		C500: style.FromU32(0x84cc16),
		C600: style.FromU32(0x65a30d),
		C700: style.FromU32(0x4d7c0f),
		C800: style.FromU32(0x3f6212),
		C900: style.FromU32(0x365314),
		C950: style.FromU32(0x1a2e05),
	}
	Green = Palette{
		C50:  style.FromU32(0xf0fdf4),
		C100: style.FromU32(0xdcfce7),
		C200: style.FromU32(0xbbf7d0),
		C300: style.FromU32(0x86efac),
		C400: style.FromU32(0x4ade80),
		C500: style.FromU32(0x22c55e),
		C600: style.FromU32(0x16a34a),
		C700: style.FromU32(0x15803d),
		C800: style.FromU32(0x166534),
		C900: style.FromU32(0x14532d),
		C950: style.FromU32(0x052e16),
	}
	Emerald = Palette{
		C50:  style.FromU32(0xecfdf5),
		C100: style.FromU32(0xd1fae5),
		C200: style.FromU32(0xa7f3d0),
		C300: style.FromU32(0x6ee7b7),
		C400: style.FromU32(0x34d399),
		C500: style.FromU32(0x10b981),
		C600: style.FromU32(0x059669),
		C700: style.FromU32(0x047857),
		C800: style.FromU32(0x065f46),
		C900: style.FromU32(0x064e3b),
		C950: style.FromU32(0x022c22),
	}
	Teal = Palette{
		C50:  style.FromU32(0xf0fdfa),
		C100: style.FromU32(0xccfbf1),
		C200: style.FromU32(0x99f6e4),
		C300: style.FromU32(0x5eead4),
		C400: style.FromU32(0x2dd4bf),
		C500: style.FromU32(0x14b8a6),
		C600: style.FromU32(0x0d9488),
		C700: style.FromU32(0x0f766e),
		C800: style.FromU32(0x115e59),
		C900: style.FromU32(0x134e4a),
		C950: style.FromU32(0x042f2e),
	}
	Cyan = Palette{
		C50:  style.FromU32(0xecfeff),
		C100: style.FromU32(0xcffafe),
		C200: style.FromU32(0xa5f3fc),
		C300: style.FromU32(0x67e8f9),
		C400: style.FromU32(0x22d3ee),
		C500: style.FromU32(0x06b6d4),
		C600: style.FromU32(0x0891b2),
		C700: style.FromU32(0x0e7490),
		C800: style.FromU32(0x155e75),
		C900: style.FromU32(0x164e63),
		C950: style.FromU32(0x083344),
	}
	Sky = Palette{
		C50:  style.FromU32(0xf0f9ff),
		C100: style.FromU32(0xe0f2fe),
		C200: style.FromU32(0xbae6fd),
		C300: style.FromU32(0x7dd3fc),
		C400: style.FromU32(0x38bdf8),
		C500: style.FromU32(0x0ea5e9),
		C600: style.FromU32(0x0284c7),
		C700: style.FromU32(0x0369a1),
		C800: style.FromU32(0x075985),
		C900: style.FromU32(0x0c4a6e),
		C950: style.FromU32(0x082f49),
	}
	Blue = Palette{
		C50:  style.FromU32(0xeff6ff),
		C100: style.FromU32(0xdbeafe),
		C200: style.FromU32(0xbfdbfe),
		C300: style.FromU32(0x93c5fd),
		C400: style.FromU32(0x60a5fa),
		C500: style.FromU32(0x3b82f6),
		C600: style.FromU32(0x2563eb),
		C700: style.FromU32(0x1d4ed8),
		C800: style.FromU32(0x1e40af),
		C900: style.FromU32(0x1e3a8a),
		C950: style.FromU32(0x172554),
	}
	Indigo = Palette{
		C50:  style.FromU32(0xeef2ff),
		C100: style.FromU32(0xe0e7ff),
		C200: style.FromU32(0xc7d2fe),
		C300: style.FromU32(0xa5b4fc),
		C400: style.FromU32(0x818cf8),
		C500: style.FromU32(0x6366f1),
		C600: style.FromU32(0x4f46e5),
		C700: style.FromU32(0x4338ca),
		C800: style.FromU32(0x3730a3),
		C900: style.FromU32(0x312e81),
		C950: style.FromU32(0x1e1b4b),
	}
	Violet = Palette{
		C50:  style.FromU32(0xf5f3ff),
		C100: style.FromU32(0xede9fe),
		C200: style.FromU32(0xddd6fe),
		C300: style.FromU32(0xc4b5fd),
		C400: style.FromU32(0xa78bfa),
		C500: style.FromU32(0x8b5cf6),
		C600: style.FromU32(0x7c3aed),
		C700: style.FromU32(0x6d28d9),
		C800: style.FromU32(0x5b21b6),
		C900: style.FromU32(0x4c1d95),
		C950: style.FromU32(0x2e1065),
	}
	Purple = Palette{
		C50:  style.FromU32(0xfaf5ff),
		C100: style.FromU32(0xf3e8ff),
		C200: style.FromU32(0xe9d5ff),
		C300: style.FromU32(0xd8b4fe),
		C400: style.FromU32(0xc084fc),
		C500: style.FromU32(0xa855f7),
		C600: style.FromU32(0x9333ea),
		C700: style.FromU32(0x7e22ce),
		C800: style.FromU32(0x6b21a8),
		C900: style.FromU32(0x581c87),
		C950: style.FromU32(0x3b0764),
	}
	Fuchsia = Palette{
		C50:  style.FromU32(0xfdf4ff),
		C100: style.FromU32(0xfae8ff),
		C200: style.FromU32(0xf5d0fe),
		C300: style.FromU32(0xf0abfc),
		C400: style.FromU32(0xe879f9),
		C500: style.FromU32(0xd946ef),
		C600: style.FromU32(0xc026d3),
		C700: style.FromU32(0xa21caf),
		C800: style.FromU32(0x86198f),
		C900: style.FromU32(0x701a75),
		C950: style.FromU32(0x4a044e),
	}
	Pink = Palette{
		C50:  style.FromU32(0xfdf2f8),
		C100: style.FromU32(0xfce7f3),
		C200: style.FromU32(0xfbcfe8),
		C300: style.FromU32(0xf9a8d4),
		C400: style.FromU32(0xf472b6),
		C500: style.FromU32(0xec4899),
		C600: style.FromU32(0xdb2777),
		C700: style.FromU32(0xbe185d),
		C800: style.FromU32(0x9d174d),
		C900: style.FromU32(0x831843),
		C950: style.FromU32(0x500724),
	}
	Rose = Palette{
		C50:  style.FromU32(0xfff1f2),
		C100: style.FromU32(0xffe4e6),
		C200: style.FromU32(0xfecdd3),
		C300: style.FromU32(0xfda4af),
		C400: style.FromU32(0xfb7185),
		C500: style.FromU32(0xf43f5e),
		C600: style.FromU32(0xe11d48),
		C700: style.FromU32(0xbe123c),
		C800: style.FromU32(0x9f1239),
		C900: style.FromU32(0x881337),
		C950: style.FromU32(0x4c0519),
	}
)
