package symbols

type BorderSet struct {
	TopLeft          string
	TopRight         string
	BottomLeft       string
	BottomRight      string
	VerticalLeft     string
	VerticalRight    string
	HorizontalTop    string
	HorizontalBottom string
}

func BorderSetFromLineSet(ls LineSet) BorderSet {
	return BorderSet{
		TopLeft:          ls.TopLeft,
		TopRight:         ls.TopRight,
		BottomLeft:       ls.BottomLeft,
		BottomRight:      ls.BottomRight,
		VerticalLeft:     ls.Vertical,
		VerticalRight:    ls.Vertical,
		HorizontalTop:    ls.Horizontal,
		HorizontalBottom: ls.Horizontal,
	}
}

var BorderPlain = BorderSetFromLineSet(LineNormal)
var BorderRounded = BorderSetFromLineSet(LineRounded)
var BorderDouble = BorderSetFromLineSet(LineDouble)
var BorderThick = BorderSetFromLineSet(LineThick)
var BorderLightDoubleDashed = BorderSetFromLineSet(LineLightDoubleDashed)
var BorderHeavyDoubleDashed = BorderSetFromLineSet(LineHeavyDoubleDashed)
var BorderLightTripleDashed = BorderSetFromLineSet(LineLightTripleDashed)
var BorderHeavyTripleDashed = BorderSetFromLineSet(LineHeavyTripleDashed)
var BorderLightQuadrupleDashed = BorderSetFromLineSet(LineLightQuadrupleDashed)
var BorderHeavyQuadrupleDashed = BorderSetFromLineSet(LineHeavyQuadrupleDashed)

const (
	QuadrantTopLeft                       = "▘"
	QuadrantTopRight                      = "▝"
	QuadrantBottomLeft                    = "▖"
	QuadrantBottomRight                   = "▗"
	QuadrantTopHalf                       = "▀"
	QuadrantBottomHalf                    = "▄"
	QuadrantLeftHalf                      = "▌"
	QuadrantRightHalf                     = "▐"
	QuadrantTopLeftBottomLeftBottomRight  = "▙"
	QuadrantTopLeftTopRightBottomLeft     = "▛"
	QuadrantTopLeftTopRightBottomRight    = "▜"
	QuadrantTopRightBottomLeftBottomRight = "▟"
	QuadrantTopLeftBottomRight            = "▚"
	QuadrantTopRightBottomLeft            = "▞"
	QuadrantBlock                         = "█"
)

var BorderQuadrantOutside = BorderSet{
	TopLeft:          QuadrantTopLeftTopRightBottomLeft,
	TopRight:         QuadrantTopLeftTopRightBottomRight,
	BottomLeft:       QuadrantTopLeftBottomLeftBottomRight,
	BottomRight:      QuadrantTopRightBottomLeftBottomRight,
	VerticalLeft:     QuadrantLeftHalf,
	VerticalRight:    QuadrantRightHalf,
	HorizontalTop:    QuadrantTopHalf,
	HorizontalBottom: QuadrantBottomHalf,
}

var BorderQuadrantInside = BorderSet{
	TopLeft:          QuadrantBottomRight,
	TopRight:         QuadrantBottomLeft,
	BottomLeft:       QuadrantTopRight,
	BottomRight:      QuadrantTopLeft,
	VerticalLeft:     QuadrantRightHalf,
	VerticalRight:    QuadrantLeftHalf,
	HorizontalTop:    QuadrantBottomHalf,
	HorizontalBottom: QuadrantTopHalf,
}

const (
	OneEighthTopEight    = "▔"
	OneEighthBottomEight = "▁"
	OneEighthLeftEight   = "▏"
	OneEighthRightEight  = "▕"
)

var BorderOneEighthWide = BorderSet{
	TopLeft:          OneEighthBottomEight,
	TopRight:         OneEighthBottomEight,
	BottomLeft:       OneEighthTopEight,
	BottomRight:      OneEighthTopEight,
	VerticalLeft:     OneEighthLeftEight,
	VerticalRight:    OneEighthRightEight,
	HorizontalTop:    OneEighthBottomEight,
	HorizontalBottom: OneEighthTopEight,
}

var BorderOneEighthTall = BorderSet{
	TopLeft:          OneEighthRightEight,
	TopRight:         OneEighthLeftEight,
	BottomLeft:       OneEighthRightEight,
	BottomRight:      OneEighthLeftEight,
	VerticalLeft:     OneEighthRightEight,
	VerticalRight:    OneEighthLeftEight,
	HorizontalTop:    OneEighthTopEight,
	HorizontalBottom: OneEighthBottomEight,
}

var BorderProportionalWide = BorderSet{
	TopLeft:          QuadrantBottomHalf,
	TopRight:         QuadrantBottomHalf,
	BottomLeft:       QuadrantTopHalf,
	BottomRight:      QuadrantTopHalf,
	VerticalLeft:     QuadrantBlock,
	VerticalRight:    QuadrantBlock,
	HorizontalTop:    QuadrantBottomHalf,
	HorizontalBottom: QuadrantTopHalf,
}

var BorderProportionalTall = BorderSet{
	TopLeft:          QuadrantBlock,
	TopRight:         QuadrantBlock,
	BottomLeft:       QuadrantBlock,
	BottomRight:      QuadrantBlock,
	VerticalLeft:     QuadrantBlock,
	VerticalRight:    QuadrantBlock,
	HorizontalTop:    QuadrantTopHalf,
	HorizontalBottom: QuadrantBottomHalf,
}

var BorderFull = BorderSet{
	TopLeft:          BlockFull,
	TopRight:         BlockFull,
	BottomLeft:       BlockFull,
	BottomRight:      BlockFull,
	VerticalLeft:     BlockFull,
	VerticalRight:    BlockFull,
	HorizontalTop:    BlockFull,
	HorizontalBottom: BlockFull,
}

var BorderEmpty = BorderSet{
	TopLeft:          " ",
	TopRight:         " ",
	BottomLeft:       " ",
	BottomRight:      " ",
	VerticalLeft:     " ",
	VerticalRight:    " ",
	HorizontalTop:    " ",
	HorizontalBottom: " ",
}
