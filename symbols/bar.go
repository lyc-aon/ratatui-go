package symbols

const (
	BarFull          = "█"
	BarSevenEighths  = "▇"
	BarThreeQuarters = "▆"
	BarFiveEighths   = "▅"
	BarHalf          = "▄"
	BarThreeEighths  = "▃"
	BarOneQuarter    = "▂"
	BarOneEighth     = "▁"
)

type BarSet struct {
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

var BarThreeLevels = BarSet{
	Full:          BarFull,
	SevenEighths:  BarFull,
	ThreeQuarters: BarHalf,
	FiveEighths:   BarHalf,
	Half:          BarHalf,
	ThreeEighths:  BarHalf,
	OneQuarter:    BarHalf,
	OneEighth:     " ",
	Empty:         " ",
}

var BarNineLevels = BarSet{
	Full:          BarFull,
	SevenEighths:  BarSevenEighths,
	ThreeQuarters: BarThreeQuarters,
	FiveEighths:   BarFiveEighths,
	Half:          BarHalf,
	ThreeEighths:  BarThreeEighths,
	OneQuarter:    BarOneQuarter,
	OneEighth:     BarOneEighth,
	Empty:         " ",
}
