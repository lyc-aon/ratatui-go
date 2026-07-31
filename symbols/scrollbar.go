package symbols

type ScrollbarSet struct {
	Track string
	Thumb string
	Begin string
	End   string
}

var ScrollbarDoubleVertical = ScrollbarSet{
	Track: LineDoubleVertical,
	Thumb: BlockFull,
	Begin: "▲",
	End:   "▼",
}

var ScrollbarDoubleHorizontal = ScrollbarSet{
	Track: LineDoubleHorizontal,
	Thumb: BlockFull,
	Begin: "◄",
	End:   "►",
}

var ScrollbarVertical = ScrollbarSet{
	Track: LineVertical,
	Thumb: BlockFull,
	Begin: "↑",
	End:   "↓",
}

var ScrollbarHorizontal = ScrollbarSet{
	Track: LineHorizontal,
	Thumb: BlockFull,
	Begin: "←",
	End:   "→",
}
