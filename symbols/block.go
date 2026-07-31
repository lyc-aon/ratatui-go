package symbols

const (
	BlockFull          = "█"
	BlockSevenEighths  = "▉"
	BlockThreeQuarters = "▊"
	BlockFiveEighths   = "▋"
	BlockHalf          = "▌"
	BlockThreeEighths  = "▍"
	BlockOneQuarter    = "▎"
	BlockOneEighth     = "▏"
)

type BlockSet struct {
	Full          string
	SevenEighths  string
	ThreeQuarters string
	FiveEighths   string
	Half          string
	ThreeEighths  string
	OneQuarter    string
	OneEighth     string
	Empty         string
}

var BlockThreeLevels = BlockSet{
	Full:          BlockFull,
	SevenEighths:  BlockFull,
	ThreeQuarters: BlockHalf,
	FiveEighths:   BlockHalf,
	Half:          BlockHalf,
	ThreeEighths:  BlockHalf,
	OneQuarter:    BlockHalf,
	OneEighth:     " ",
	Empty:         " ",
}

var BlockNineLevels = BlockSet{
	Full:          BlockFull,
	SevenEighths:  BlockSevenEighths,
	ThreeQuarters: BlockThreeQuarters,
	FiveEighths:   BlockFiveEighths,
	Half:          BlockHalf,
	ThreeEighths:  BlockThreeEighths,
	OneQuarter:    BlockOneQuarter,
	OneEighth:     BlockOneEighth,
	Empty:         " ",
}
