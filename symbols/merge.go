package symbols

type MergeStrategy int

const (
	MergeStrategyReplace MergeStrategy = iota
	MergeStrategyExact
	MergeStrategyFuzzy
)

type LineStyle int

const (
	LineStyleNothing LineStyle = iota
	LineStylePlain
	LineStyleRounded
	LineStyleDouble
	LineStyleThick
	LineStyleDoubleDash
	LineStyleDoubleDashThick
	LineStyleTripleDash
	LineStyleTripleDashThick
	LineStyleQuadrupleDash
	LineStyleQuadrupleDashThick
)

type borderSymbol struct {
	right LineStyle
	up    LineStyle
	left  LineStyle
	down  LineStyle
}

func (ls LineStyle) merge(other LineStyle) LineStyle {
	if other == LineStyleNothing {
		return ls
	}
	return other
}

func parseBorderSymbol(s string) (borderSymbol, bool) {
	switch s {
	case "─":
		return borderSymbol{LineStylePlain, LineStyleNothing, LineStylePlain, LineStyleNothing}, true
	case "━":
		return borderSymbol{LineStyleThick, LineStyleNothing, LineStyleThick, LineStyleNothing}, true
	case "│":
		return borderSymbol{LineStyleNothing, LineStylePlain, LineStyleNothing, LineStylePlain}, true
	case "┃":
		return borderSymbol{LineStyleNothing, LineStyleThick, LineStyleNothing, LineStyleThick}, true
	case "┄":
		return borderSymbol{LineStyleTripleDash, LineStyleNothing, LineStyleTripleDash, LineStyleNothing}, true
	case "┅":
		return borderSymbol{LineStyleTripleDashThick, LineStyleNothing, LineStyleTripleDashThick, LineStyleNothing}, true
	case "┆":
		return borderSymbol{LineStyleNothing, LineStyleTripleDash, LineStyleNothing, LineStyleTripleDash}, true
	case "┇":
		return borderSymbol{LineStyleNothing, LineStyleTripleDashThick, LineStyleNothing, LineStyleTripleDashThick}, true
	case "┈":
		return borderSymbol{LineStyleQuadrupleDash, LineStyleNothing, LineStyleQuadrupleDash, LineStyleNothing}, true
	case "┉":
		return borderSymbol{LineStyleQuadrupleDashThick, LineStyleNothing, LineStyleQuadrupleDashThick, LineStyleNothing}, true
	case "┊":
		return borderSymbol{LineStyleNothing, LineStyleQuadrupleDash, LineStyleNothing, LineStyleQuadrupleDash}, true
	case "┋":
		return borderSymbol{LineStyleNothing, LineStyleQuadrupleDashThick, LineStyleNothing, LineStyleQuadrupleDashThick}, true
	case "┌":
		return borderSymbol{LineStylePlain, LineStyleNothing, LineStyleNothing, LineStylePlain}, true
	case "┍":
		return borderSymbol{LineStyleThick, LineStyleNothing, LineStyleNothing, LineStylePlain}, true
	case "┎":
		return borderSymbol{LineStylePlain, LineStyleNothing, LineStyleNothing, LineStyleThick}, true
	case "┏":
		return borderSymbol{LineStyleThick, LineStyleNothing, LineStyleNothing, LineStyleThick}, true
	case "┐":
		return borderSymbol{LineStyleNothing, LineStyleNothing, LineStylePlain, LineStylePlain}, true
	case "┑":
		return borderSymbol{LineStyleNothing, LineStyleNothing, LineStyleThick, LineStylePlain}, true
	case "┒":
		return borderSymbol{LineStyleNothing, LineStyleNothing, LineStylePlain, LineStyleThick}, true
	case "┓":
		return borderSymbol{LineStyleNothing, LineStyleNothing, LineStyleThick, LineStyleThick}, true
	case "└":
		return borderSymbol{LineStylePlain, LineStylePlain, LineStyleNothing, LineStyleNothing}, true
	case "┕":
		return borderSymbol{LineStyleThick, LineStylePlain, LineStyleNothing, LineStyleNothing}, true
	case "┖":
		return borderSymbol{LineStylePlain, LineStyleThick, LineStyleNothing, LineStyleNothing}, true
	case "┗":
		return borderSymbol{LineStyleThick, LineStyleThick, LineStyleNothing, LineStyleNothing}, true
	case "┘":
		return borderSymbol{LineStyleNothing, LineStylePlain, LineStylePlain, LineStyleNothing}, true
	case "┙":
		return borderSymbol{LineStyleNothing, LineStylePlain, LineStyleThick, LineStyleNothing}, true
	case "┚":
		return borderSymbol{LineStyleNothing, LineStyleThick, LineStylePlain, LineStyleNothing}, true
	case "┛":
		return borderSymbol{LineStyleNothing, LineStyleThick, LineStyleThick, LineStyleNothing}, true
	case "├":
		return borderSymbol{LineStylePlain, LineStylePlain, LineStyleNothing, LineStylePlain}, true
	case "┝":
		return borderSymbol{LineStyleThick, LineStylePlain, LineStyleNothing, LineStylePlain}, true
	case "┞":
		return borderSymbol{LineStylePlain, LineStyleThick, LineStyleNothing, LineStylePlain}, true
	case "┟":
		return borderSymbol{LineStylePlain, LineStylePlain, LineStyleNothing, LineStyleThick}, true
	case "┠":
		return borderSymbol{LineStylePlain, LineStyleThick, LineStyleNothing, LineStyleThick}, true
	case "┡":
		return borderSymbol{LineStyleThick, LineStyleThick, LineStyleNothing, LineStylePlain}, true
	case "┢":
		return borderSymbol{LineStyleThick, LineStylePlain, LineStyleNothing, LineStyleThick}, true
	case "┣":
		return borderSymbol{LineStyleThick, LineStyleThick, LineStyleNothing, LineStyleThick}, true
	case "┤":
		return borderSymbol{LineStyleNothing, LineStylePlain, LineStylePlain, LineStylePlain}, true
	case "┥":
		return borderSymbol{LineStyleNothing, LineStylePlain, LineStyleThick, LineStylePlain}, true
	case "┦":
		return borderSymbol{LineStyleNothing, LineStyleThick, LineStylePlain, LineStylePlain}, true
	case "┧":
		return borderSymbol{LineStyleNothing, LineStylePlain, LineStylePlain, LineStyleThick}, true
	case "┨":
		return borderSymbol{LineStyleNothing, LineStyleThick, LineStylePlain, LineStyleThick}, true
	case "┩":
		return borderSymbol{LineStyleNothing, LineStyleThick, LineStyleThick, LineStylePlain}, true
	case "┪":
		return borderSymbol{LineStyleNothing, LineStylePlain, LineStyleThick, LineStyleThick}, true
	case "┫":
		return borderSymbol{LineStyleNothing, LineStyleThick, LineStyleThick, LineStyleThick}, true
	case "┬":
		return borderSymbol{LineStylePlain, LineStyleNothing, LineStylePlain, LineStylePlain}, true
	case "┭":
		return borderSymbol{LineStylePlain, LineStyleNothing, LineStyleThick, LineStylePlain}, true
	case "┮":
		return borderSymbol{LineStyleThick, LineStyleNothing, LineStylePlain, LineStylePlain}, true
	case "┯":
		return borderSymbol{LineStyleThick, LineStyleNothing, LineStyleThick, LineStylePlain}, true
	case "┰":
		return borderSymbol{LineStylePlain, LineStyleNothing, LineStylePlain, LineStyleThick}, true
	case "┱":
		return borderSymbol{LineStylePlain, LineStyleNothing, LineStyleThick, LineStyleThick}, true
	case "┲":
		return borderSymbol{LineStyleThick, LineStyleNothing, LineStylePlain, LineStyleThick}, true
	case "┳":
		return borderSymbol{LineStyleThick, LineStyleNothing, LineStyleThick, LineStyleThick}, true
	case "┴":
		return borderSymbol{LineStylePlain, LineStylePlain, LineStylePlain, LineStyleNothing}, true
	case "┵":
		return borderSymbol{LineStylePlain, LineStylePlain, LineStyleThick, LineStyleNothing}, true
	case "┶":
		return borderSymbol{LineStyleThick, LineStylePlain, LineStylePlain, LineStyleNothing}, true
	case "┷":
		return borderSymbol{LineStyleThick, LineStylePlain, LineStyleThick, LineStyleNothing}, true
	case "┸":
		return borderSymbol{LineStylePlain, LineStyleThick, LineStylePlain, LineStyleNothing}, true
	case "┹":
		return borderSymbol{LineStylePlain, LineStyleThick, LineStyleThick, LineStyleNothing}, true
	case "┺":
		return borderSymbol{LineStyleThick, LineStyleThick, LineStylePlain, LineStyleNothing}, true
	case "┻":
		return borderSymbol{LineStyleThick, LineStyleThick, LineStyleThick, LineStyleNothing}, true
	case "┼":
		return borderSymbol{LineStylePlain, LineStylePlain, LineStylePlain, LineStylePlain}, true
	case "┽":
		return borderSymbol{LineStylePlain, LineStylePlain, LineStyleThick, LineStylePlain}, true
	case "┾":
		return borderSymbol{LineStyleThick, LineStylePlain, LineStylePlain, LineStylePlain}, true
	case "┿":
		return borderSymbol{LineStyleThick, LineStylePlain, LineStyleThick, LineStylePlain}, true
	case "╀":
		return borderSymbol{LineStylePlain, LineStyleThick, LineStylePlain, LineStylePlain}, true
	case "╁":
		return borderSymbol{LineStylePlain, LineStylePlain, LineStylePlain, LineStyleThick}, true
	case "╂":
		return borderSymbol{LineStylePlain, LineStyleThick, LineStylePlain, LineStyleThick}, true
	case "╃":
		return borderSymbol{LineStylePlain, LineStyleThick, LineStyleThick, LineStylePlain}, true
	case "╄":
		return borderSymbol{LineStyleThick, LineStyleThick, LineStylePlain, LineStylePlain}, true
	case "╅":
		return borderSymbol{LineStylePlain, LineStylePlain, LineStyleThick, LineStyleThick}, true
	case "╆":
		return borderSymbol{LineStyleThick, LineStylePlain, LineStylePlain, LineStyleThick}, true
	case "╇":
		return borderSymbol{LineStyleThick, LineStyleThick, LineStyleThick, LineStylePlain}, true
	case "╈":
		return borderSymbol{LineStyleThick, LineStylePlain, LineStyleThick, LineStyleThick}, true
	case "╉":
		return borderSymbol{LineStylePlain, LineStyleThick, LineStyleThick, LineStyleThick}, true
	case "╊":
		return borderSymbol{LineStyleThick, LineStyleThick, LineStylePlain, LineStyleThick}, true
	case "╋":
		return borderSymbol{LineStyleThick, LineStyleThick, LineStyleThick, LineStyleThick}, true
	case "╌":
		return borderSymbol{LineStyleDoubleDash, LineStyleNothing, LineStyleDoubleDash, LineStyleNothing}, true
	case "╍":
		return borderSymbol{LineStyleDoubleDashThick, LineStyleNothing, LineStyleDoubleDashThick, LineStyleNothing}, true
	case "╎":
		return borderSymbol{LineStyleNothing, LineStyleDoubleDash, LineStyleNothing, LineStyleDoubleDash}, true
	case "╏":
		return borderSymbol{LineStyleNothing, LineStyleDoubleDashThick, LineStyleNothing, LineStyleDoubleDashThick}, true
	case "═":
		return borderSymbol{LineStyleDouble, LineStyleNothing, LineStyleDouble, LineStyleNothing}, true
	case "║":
		return borderSymbol{LineStyleNothing, LineStyleDouble, LineStyleNothing, LineStyleDouble}, true
	case "╒":
		return borderSymbol{LineStyleDouble, LineStyleNothing, LineStyleNothing, LineStylePlain}, true
	case "╓":
		return borderSymbol{LineStylePlain, LineStyleNothing, LineStyleNothing, LineStyleDouble}, true
	case "╔":
		return borderSymbol{LineStyleDouble, LineStyleNothing, LineStyleNothing, LineStyleDouble}, true
	case "╕":
		return borderSymbol{LineStyleNothing, LineStyleNothing, LineStyleDouble, LineStylePlain}, true
	case "╖":
		return borderSymbol{LineStyleNothing, LineStyleNothing, LineStylePlain, LineStyleDouble}, true
	case "╗":
		return borderSymbol{LineStyleNothing, LineStyleNothing, LineStyleDouble, LineStyleDouble}, true
	case "╘":
		return borderSymbol{LineStyleDouble, LineStylePlain, LineStyleNothing, LineStyleNothing}, true
	case "╙":
		return borderSymbol{LineStylePlain, LineStyleDouble, LineStyleNothing, LineStyleNothing}, true
	case "╚":
		return borderSymbol{LineStyleDouble, LineStyleDouble, LineStyleNothing, LineStyleNothing}, true
	case "╛":
		return borderSymbol{LineStyleNothing, LineStylePlain, LineStyleDouble, LineStyleNothing}, true
	case "╜":
		return borderSymbol{LineStyleNothing, LineStyleDouble, LineStylePlain, LineStyleNothing}, true
	case "╝":
		return borderSymbol{LineStyleNothing, LineStyleDouble, LineStyleDouble, LineStyleNothing}, true
	case "╞":
		return borderSymbol{LineStyleDouble, LineStylePlain, LineStyleNothing, LineStylePlain}, true
	case "╟":
		return borderSymbol{LineStylePlain, LineStyleDouble, LineStyleNothing, LineStyleDouble}, true
	case "╠":
		return borderSymbol{LineStyleDouble, LineStyleDouble, LineStyleNothing, LineStyleDouble}, true
	case "╡":
		return borderSymbol{LineStyleNothing, LineStylePlain, LineStyleDouble, LineStylePlain}, true
	case "╢":
		return borderSymbol{LineStyleNothing, LineStyleDouble, LineStylePlain, LineStyleDouble}, true
	case "╣":
		return borderSymbol{LineStyleNothing, LineStyleDouble, LineStyleDouble, LineStyleDouble}, true
	case "╤":
		return borderSymbol{LineStyleDouble, LineStyleNothing, LineStyleDouble, LineStylePlain}, true
	case "╥":
		return borderSymbol{LineStylePlain, LineStyleNothing, LineStylePlain, LineStyleDouble}, true
	case "╦":
		return borderSymbol{LineStyleDouble, LineStyleNothing, LineStyleDouble, LineStyleDouble}, true
	case "╧":
		return borderSymbol{LineStyleDouble, LineStylePlain, LineStyleDouble, LineStyleNothing}, true
	case "╨":
		return borderSymbol{LineStylePlain, LineStyleDouble, LineStylePlain, LineStyleNothing}, true
	case "╩":
		return borderSymbol{LineStyleDouble, LineStyleDouble, LineStyleDouble, LineStyleNothing}, true
	case "╪":
		return borderSymbol{LineStyleDouble, LineStylePlain, LineStyleDouble, LineStylePlain}, true
	case "╫":
		return borderSymbol{LineStylePlain, LineStyleDouble, LineStylePlain, LineStyleDouble}, true
	case "╬":
		return borderSymbol{LineStyleDouble, LineStyleDouble, LineStyleDouble, LineStyleDouble}, true
	case "╭":
		return borderSymbol{LineStyleRounded, LineStyleNothing, LineStyleNothing, LineStyleRounded}, true
	case "╮":
		return borderSymbol{LineStyleNothing, LineStyleNothing, LineStyleRounded, LineStyleRounded}, true
	case "╯":
		return borderSymbol{LineStyleNothing, LineStyleRounded, LineStyleRounded, LineStyleNothing}, true
	case "╰":
		return borderSymbol{LineStyleRounded, LineStyleRounded, LineStyleNothing, LineStyleNothing}, true
	case "╴":
		return borderSymbol{LineStyleNothing, LineStyleNothing, LineStylePlain, LineStyleNothing}, true
	case "╵":
		return borderSymbol{LineStyleNothing, LineStylePlain, LineStyleNothing, LineStyleNothing}, true
	case "╶":
		return borderSymbol{LineStylePlain, LineStyleNothing, LineStyleNothing, LineStyleNothing}, true
	case "╷":
		return borderSymbol{LineStyleNothing, LineStyleNothing, LineStyleNothing, LineStylePlain}, true
	case "╸":
		return borderSymbol{LineStyleNothing, LineStyleNothing, LineStyleThick, LineStyleNothing}, true
	case "╹":
		return borderSymbol{LineStyleNothing, LineStyleThick, LineStyleNothing, LineStyleNothing}, true
	case "╺":
		return borderSymbol{LineStyleThick, LineStyleNothing, LineStyleNothing, LineStyleNothing}, true
	case "╻":
		return borderSymbol{LineStyleNothing, LineStyleNothing, LineStyleNothing, LineStyleThick}, true
	case "╼":
		return borderSymbol{LineStyleThick, LineStyleNothing, LineStylePlain, LineStyleNothing}, true
	case "╽":
		return borderSymbol{LineStyleNothing, LineStylePlain, LineStyleNothing, LineStyleThick}, true
	case "╾":
		return borderSymbol{LineStylePlain, LineStyleNothing, LineStyleThick, LineStyleNothing}, true
	case "╿":
		return borderSymbol{LineStyleNothing, LineStyleThick, LineStyleNothing, LineStylePlain}, true
	default:
		return borderSymbol{}, false
	}
}

func formatBorderSymbol(b borderSymbol) (string, bool) {
	switch b {
	case borderSymbol{LineStylePlain, LineStyleNothing, LineStylePlain, LineStyleNothing}:
		return "─", true
	case borderSymbol{LineStyleThick, LineStyleNothing, LineStyleThick, LineStyleNothing}:
		return "━", true
	case borderSymbol{LineStyleNothing, LineStylePlain, LineStyleNothing, LineStylePlain}:
		return "│", true
	case borderSymbol{LineStyleNothing, LineStyleThick, LineStyleNothing, LineStyleThick}:
		return "┃", true
	case borderSymbol{LineStyleTripleDash, LineStyleNothing, LineStyleTripleDash, LineStyleNothing}:
		return "┄", true
	case borderSymbol{LineStyleTripleDashThick, LineStyleNothing, LineStyleTripleDashThick, LineStyleNothing}:
		return "┅", true
	case borderSymbol{LineStyleNothing, LineStyleTripleDash, LineStyleNothing, LineStyleTripleDash}:
		return "┆", true
	case borderSymbol{LineStyleNothing, LineStyleTripleDashThick, LineStyleNothing, LineStyleTripleDashThick}:
		return "┇", true
	case borderSymbol{LineStyleQuadrupleDash, LineStyleNothing, LineStyleQuadrupleDash, LineStyleNothing}:
		return "┈", true
	case borderSymbol{LineStyleQuadrupleDashThick, LineStyleNothing, LineStyleQuadrupleDashThick, LineStyleNothing}:
		return "┉", true
	case borderSymbol{LineStyleNothing, LineStyleQuadrupleDash, LineStyleNothing, LineStyleQuadrupleDash}:
		return "┊", true
	case borderSymbol{LineStyleNothing, LineStyleQuadrupleDashThick, LineStyleNothing, LineStyleQuadrupleDashThick}:
		return "┋", true
	case borderSymbol{LineStylePlain, LineStyleNothing, LineStyleNothing, LineStylePlain}:
		return "┌", true
	case borderSymbol{LineStyleThick, LineStyleNothing, LineStyleNothing, LineStylePlain}:
		return "┍", true
	case borderSymbol{LineStylePlain, LineStyleNothing, LineStyleNothing, LineStyleThick}:
		return "┎", true
	case borderSymbol{LineStyleThick, LineStyleNothing, LineStyleNothing, LineStyleThick}:
		return "┏", true
	case borderSymbol{LineStyleNothing, LineStyleNothing, LineStylePlain, LineStylePlain}:
		return "┐", true
	case borderSymbol{LineStyleNothing, LineStyleNothing, LineStyleThick, LineStylePlain}:
		return "┑", true
	case borderSymbol{LineStyleNothing, LineStyleNothing, LineStylePlain, LineStyleThick}:
		return "┒", true
	case borderSymbol{LineStyleNothing, LineStyleNothing, LineStyleThick, LineStyleThick}:
		return "┓", true
	case borderSymbol{LineStylePlain, LineStylePlain, LineStyleNothing, LineStyleNothing}:
		return "└", true
	case borderSymbol{LineStyleThick, LineStylePlain, LineStyleNothing, LineStyleNothing}:
		return "┕", true
	case borderSymbol{LineStylePlain, LineStyleThick, LineStyleNothing, LineStyleNothing}:
		return "┖", true
	case borderSymbol{LineStyleThick, LineStyleThick, LineStyleNothing, LineStyleNothing}:
		return "┗", true
	case borderSymbol{LineStyleNothing, LineStylePlain, LineStylePlain, LineStyleNothing}:
		return "┘", true
	case borderSymbol{LineStyleNothing, LineStylePlain, LineStyleThick, LineStyleNothing}:
		return "┙", true
	case borderSymbol{LineStyleNothing, LineStyleThick, LineStylePlain, LineStyleNothing}:
		return "┚", true
	case borderSymbol{LineStyleNothing, LineStyleThick, LineStyleThick, LineStyleNothing}:
		return "┛", true
	case borderSymbol{LineStylePlain, LineStylePlain, LineStyleNothing, LineStylePlain}:
		return "├", true
	case borderSymbol{LineStyleThick, LineStylePlain, LineStyleNothing, LineStylePlain}:
		return "┝", true
	case borderSymbol{LineStylePlain, LineStyleThick, LineStyleNothing, LineStylePlain}:
		return "┞", true
	case borderSymbol{LineStylePlain, LineStylePlain, LineStyleNothing, LineStyleThick}:
		return "┟", true
	case borderSymbol{LineStylePlain, LineStyleThick, LineStyleNothing, LineStyleThick}:
		return "┠", true
	case borderSymbol{LineStyleThick, LineStyleThick, LineStyleNothing, LineStylePlain}:
		return "┡", true
	case borderSymbol{LineStyleThick, LineStylePlain, LineStyleNothing, LineStyleThick}:
		return "┢", true
	case borderSymbol{LineStyleThick, LineStyleThick, LineStyleNothing, LineStyleThick}:
		return "┣", true
	case borderSymbol{LineStyleNothing, LineStylePlain, LineStylePlain, LineStylePlain}:
		return "┤", true
	case borderSymbol{LineStyleNothing, LineStylePlain, LineStyleThick, LineStylePlain}:
		return "┥", true
	case borderSymbol{LineStyleNothing, LineStyleThick, LineStylePlain, LineStylePlain}:
		return "┦", true
	case borderSymbol{LineStyleNothing, LineStylePlain, LineStylePlain, LineStyleThick}:
		return "┧", true
	case borderSymbol{LineStyleNothing, LineStyleThick, LineStylePlain, LineStyleThick}:
		return "┨", true
	case borderSymbol{LineStyleNothing, LineStyleThick, LineStyleThick, LineStylePlain}:
		return "┩", true
	case borderSymbol{LineStyleNothing, LineStylePlain, LineStyleThick, LineStyleThick}:
		return "┪", true
	case borderSymbol{LineStyleNothing, LineStyleThick, LineStyleThick, LineStyleThick}:
		return "┫", true
	case borderSymbol{LineStylePlain, LineStyleNothing, LineStylePlain, LineStylePlain}:
		return "┬", true
	case borderSymbol{LineStylePlain, LineStyleNothing, LineStyleThick, LineStylePlain}:
		return "┭", true
	case borderSymbol{LineStyleThick, LineStyleNothing, LineStylePlain, LineStylePlain}:
		return "┮", true
	case borderSymbol{LineStyleThick, LineStyleNothing, LineStyleThick, LineStylePlain}:
		return "┯", true
	case borderSymbol{LineStylePlain, LineStyleNothing, LineStylePlain, LineStyleThick}:
		return "┰", true
	case borderSymbol{LineStylePlain, LineStyleNothing, LineStyleThick, LineStyleThick}:
		return "┱", true
	case borderSymbol{LineStyleThick, LineStyleNothing, LineStylePlain, LineStyleThick}:
		return "┲", true
	case borderSymbol{LineStyleThick, LineStyleNothing, LineStyleThick, LineStyleThick}:
		return "┳", true
	case borderSymbol{LineStylePlain, LineStylePlain, LineStylePlain, LineStyleNothing}:
		return "┴", true
	case borderSymbol{LineStylePlain, LineStylePlain, LineStyleThick, LineStyleNothing}:
		return "┵", true
	case borderSymbol{LineStyleThick, LineStylePlain, LineStylePlain, LineStyleNothing}:
		return "┶", true
	case borderSymbol{LineStyleThick, LineStylePlain, LineStyleThick, LineStyleNothing}:
		return "┷", true
	case borderSymbol{LineStylePlain, LineStyleThick, LineStylePlain, LineStyleNothing}:
		return "┸", true
	case borderSymbol{LineStylePlain, LineStyleThick, LineStyleThick, LineStyleNothing}:
		return "┹", true
	case borderSymbol{LineStyleThick, LineStyleThick, LineStylePlain, LineStyleNothing}:
		return "┺", true
	case borderSymbol{LineStyleThick, LineStyleThick, LineStyleThick, LineStyleNothing}:
		return "┻", true
	case borderSymbol{LineStylePlain, LineStylePlain, LineStylePlain, LineStylePlain}:
		return "┼", true
	case borderSymbol{LineStylePlain, LineStylePlain, LineStyleThick, LineStylePlain}:
		return "┽", true
	case borderSymbol{LineStyleThick, LineStylePlain, LineStylePlain, LineStylePlain}:
		return "┾", true
	case borderSymbol{LineStyleThick, LineStylePlain, LineStyleThick, LineStylePlain}:
		return "┿", true
	case borderSymbol{LineStylePlain, LineStyleThick, LineStylePlain, LineStylePlain}:
		return "╀", true
	case borderSymbol{LineStylePlain, LineStylePlain, LineStylePlain, LineStyleThick}:
		return "╁", true
	case borderSymbol{LineStylePlain, LineStyleThick, LineStylePlain, LineStyleThick}:
		return "╂", true
	case borderSymbol{LineStylePlain, LineStyleThick, LineStyleThick, LineStylePlain}:
		return "╃", true
	case borderSymbol{LineStyleThick, LineStyleThick, LineStylePlain, LineStylePlain}:
		return "╄", true
	case borderSymbol{LineStylePlain, LineStylePlain, LineStyleThick, LineStyleThick}:
		return "╅", true
	case borderSymbol{LineStyleThick, LineStylePlain, LineStylePlain, LineStyleThick}:
		return "╆", true
	case borderSymbol{LineStyleThick, LineStyleThick, LineStyleThick, LineStylePlain}:
		return "╇", true
	case borderSymbol{LineStyleThick, LineStylePlain, LineStyleThick, LineStyleThick}:
		return "╈", true
	case borderSymbol{LineStylePlain, LineStyleThick, LineStyleThick, LineStyleThick}:
		return "╉", true
	case borderSymbol{LineStyleThick, LineStyleThick, LineStylePlain, LineStyleThick}:
		return "╊", true
	case borderSymbol{LineStyleThick, LineStyleThick, LineStyleThick, LineStyleThick}:
		return "╋", true
	case borderSymbol{LineStyleDoubleDash, LineStyleNothing, LineStyleDoubleDash, LineStyleNothing}:
		return "╌", true
	case borderSymbol{LineStyleDoubleDashThick, LineStyleNothing, LineStyleDoubleDashThick, LineStyleNothing}:
		return "╍", true
	case borderSymbol{LineStyleNothing, LineStyleDoubleDash, LineStyleNothing, LineStyleDoubleDash}:
		return "╎", true
	case borderSymbol{LineStyleNothing, LineStyleDoubleDashThick, LineStyleNothing, LineStyleDoubleDashThick}:
		return "╏", true
	case borderSymbol{LineStyleDouble, LineStyleNothing, LineStyleDouble, LineStyleNothing}:
		return "═", true
	case borderSymbol{LineStyleNothing, LineStyleDouble, LineStyleNothing, LineStyleDouble}:
		return "║", true
	case borderSymbol{LineStyleDouble, LineStyleNothing, LineStyleNothing, LineStylePlain}:
		return "╒", true
	case borderSymbol{LineStylePlain, LineStyleNothing, LineStyleNothing, LineStyleDouble}:
		return "╓", true
	case borderSymbol{LineStyleDouble, LineStyleNothing, LineStyleNothing, LineStyleDouble}:
		return "╔", true
	case borderSymbol{LineStyleNothing, LineStyleNothing, LineStyleDouble, LineStylePlain}:
		return "╕", true
	case borderSymbol{LineStyleNothing, LineStyleNothing, LineStylePlain, LineStyleDouble}:
		return "╖", true
	case borderSymbol{LineStyleNothing, LineStyleNothing, LineStyleDouble, LineStyleDouble}:
		return "╗", true
	case borderSymbol{LineStyleDouble, LineStylePlain, LineStyleNothing, LineStyleNothing}:
		return "╘", true
	case borderSymbol{LineStylePlain, LineStyleDouble, LineStyleNothing, LineStyleNothing}:
		return "╙", true
	case borderSymbol{LineStyleDouble, LineStyleDouble, LineStyleNothing, LineStyleNothing}:
		return "╚", true
	case borderSymbol{LineStyleNothing, LineStylePlain, LineStyleDouble, LineStyleNothing}:
		return "╛", true
	case borderSymbol{LineStyleNothing, LineStyleDouble, LineStylePlain, LineStyleNothing}:
		return "╜", true
	case borderSymbol{LineStyleNothing, LineStyleDouble, LineStyleDouble, LineStyleNothing}:
		return "╝", true
	case borderSymbol{LineStyleDouble, LineStylePlain, LineStyleNothing, LineStylePlain}:
		return "╞", true
	case borderSymbol{LineStylePlain, LineStyleDouble, LineStyleNothing, LineStyleDouble}:
		return "╟", true
	case borderSymbol{LineStyleDouble, LineStyleDouble, LineStyleNothing, LineStyleDouble}:
		return "╠", true
	case borderSymbol{LineStyleNothing, LineStylePlain, LineStyleDouble, LineStylePlain}:
		return "╡", true
	case borderSymbol{LineStyleNothing, LineStyleDouble, LineStylePlain, LineStyleDouble}:
		return "╢", true
	case borderSymbol{LineStyleNothing, LineStyleDouble, LineStyleDouble, LineStyleDouble}:
		return "╣", true
	case borderSymbol{LineStyleDouble, LineStyleNothing, LineStyleDouble, LineStylePlain}:
		return "╤", true
	case borderSymbol{LineStylePlain, LineStyleNothing, LineStylePlain, LineStyleDouble}:
		return "╥", true
	case borderSymbol{LineStyleDouble, LineStyleNothing, LineStyleDouble, LineStyleDouble}:
		return "╦", true
	case borderSymbol{LineStyleDouble, LineStylePlain, LineStyleDouble, LineStyleNothing}:
		return "╧", true
	case borderSymbol{LineStylePlain, LineStyleDouble, LineStylePlain, LineStyleNothing}:
		return "╨", true
	case borderSymbol{LineStyleDouble, LineStyleDouble, LineStyleDouble, LineStyleNothing}:
		return "╩", true
	case borderSymbol{LineStyleDouble, LineStylePlain, LineStyleDouble, LineStylePlain}:
		return "╪", true
	case borderSymbol{LineStylePlain, LineStyleDouble, LineStylePlain, LineStyleDouble}:
		return "╫", true
	case borderSymbol{LineStyleDouble, LineStyleDouble, LineStyleDouble, LineStyleDouble}:
		return "╬", true
	case borderSymbol{LineStyleRounded, LineStyleNothing, LineStyleNothing, LineStyleRounded}:
		return "╭", true
	case borderSymbol{LineStyleNothing, LineStyleNothing, LineStyleRounded, LineStyleRounded}:
		return "╮", true
	case borderSymbol{LineStyleNothing, LineStyleRounded, LineStyleRounded, LineStyleNothing}:
		return "╯", true
	case borderSymbol{LineStyleRounded, LineStyleRounded, LineStyleNothing, LineStyleNothing}:
		return "╰", true
	case borderSymbol{LineStyleNothing, LineStyleNothing, LineStylePlain, LineStyleNothing}:
		return "╴", true
	case borderSymbol{LineStyleNothing, LineStylePlain, LineStyleNothing, LineStyleNothing}:
		return "╵", true
	case borderSymbol{LineStylePlain, LineStyleNothing, LineStyleNothing, LineStyleNothing}:
		return "╶", true
	case borderSymbol{LineStyleNothing, LineStyleNothing, LineStyleNothing, LineStylePlain}:
		return "╷", true
	case borderSymbol{LineStyleNothing, LineStyleNothing, LineStyleThick, LineStyleNothing}:
		return "╸", true
	case borderSymbol{LineStyleNothing, LineStyleThick, LineStyleNothing, LineStyleNothing}:
		return "╹", true
	case borderSymbol{LineStyleThick, LineStyleNothing, LineStyleNothing, LineStyleNothing}:
		return "╺", true
	case borderSymbol{LineStyleNothing, LineStyleNothing, LineStyleNothing, LineStyleThick}:
		return "╻", true
	case borderSymbol{LineStyleThick, LineStyleNothing, LineStylePlain, LineStyleNothing}:
		return "╼", true
	case borderSymbol{LineStyleNothing, LineStylePlain, LineStyleNothing, LineStyleThick}:
		return "╽", true
	case borderSymbol{LineStylePlain, LineStyleNothing, LineStyleThick, LineStyleNothing}:
		return "╾", true
	case borderSymbol{LineStyleNothing, LineStyleThick, LineStyleNothing, LineStylePlain}:
		return "╿", true
	default:
		return "", false
	}
}

func (b borderSymbol) isStraight() bool {
	return (b.up == b.down && b.left == b.right) && (b.up == LineStyleNothing || b.left == LineStyleNothing)
}

func (b borderSymbol) isCorner() bool {
	if b.up == b.right && b.down == LineStyleNothing && b.left == LineStyleNothing {
		return true
	}
	if b.up == LineStyleNothing && b.right == b.down && b.left == LineStyleNothing {
		return true
	}
	if b.up == LineStyleNothing && b.right == LineStyleNothing && b.down == b.left {
		return true
	}
	if b.up == b.left && b.right == LineStyleNothing && b.down == LineStyleNothing {
		return true
	}
	return false
}

func (b borderSymbol) contains(style LineStyle) bool {
	return b.up == style || b.right == style || b.down == style || b.left == style
}

func (b borderSymbol) replace(from, to LineStyle) borderSymbol {
	if b.up == from {
		b.up = to
	}
	if b.right == from {
		b.right = to
	}
	if b.down == from {
		b.down = to
	}
	if b.left == from {
		b.left = to
	}
	return b
}

func (b borderSymbol) fuzzy(other borderSymbol) borderSymbol {
	if !b.isStraight() {
		b = b.replace(LineStyleDoubleDash, LineStylePlain).
			replace(LineStyleTripleDash, LineStylePlain).
			replace(LineStyleQuadrupleDash, LineStylePlain).
			replace(LineStyleDoubleDashThick, LineStyleThick).
			replace(LineStyleTripleDashThick, LineStyleThick).
			replace(LineStyleQuadrupleDashThick, LineStyleThick)
	}

	if !b.isCorner() {
		b = b.replace(LineStyleRounded, LineStylePlain)
	}

	if b.contains(LineStyleDouble) && b.contains(LineStyleThick) {
		if other.contains(LineStyleDouble) {
			b = b.replace(LineStyleThick, LineStyleDouble)
		} else {
			b = b.replace(LineStyleDouble, LineStyleThick)
		}
	}

	if _, ok := formatBorderSymbol(b); !ok {
		if other.contains(LineStyleDouble) {
			b = b.replace(LineStylePlain, LineStyleDouble)
		} else {
			b = b.replace(LineStyleDouble, LineStylePlain)
		}
	}
	return b
}

func (b borderSymbol) merge(other borderSymbol, strategy MergeStrategy) borderSymbol {
	exactResult := borderSymbol{
		right: b.right.merge(other.right),
		up:    b.up.merge(other.up),
		left:  b.left.merge(other.left),
		down:  b.down.merge(other.down),
	}
	switch strategy {
	case MergeStrategyReplace:
		return other
	case MergeStrategyExact:
		return exactResult
	case MergeStrategyFuzzy:
		return exactResult.fuzzy(other)
	default:
		return other
	}
}

func (s MergeStrategy) Merge(prev, next string) string {
	if s == MergeStrategyReplace {
		return next
	}

	prevSym, okPrev := parseBorderSymbol(prev)
	nextSym, okNext := parseBorderSymbol(next)

	if okPrev && okNext {
		merged := prevSym.merge(nextSym, s)
		if res, ok := formatBorderSymbol(merged); ok {
			return res
		}
		return next
	}

	if !okPrev && okNext {
		return prev
	}

	return next
}

func MergeSymbol(prev, next string, strategy MergeStrategy) string {
	return strategy.Merge(prev, next)
}
