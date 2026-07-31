package style

// Foreground named-color builders.

func (s Style) Black() Style        { return s.WithFG(Black) }
func (s Style) Red() Style          { return s.WithFG(Red) }
func (s Style) Green() Style        { return s.WithFG(Green) }
func (s Style) Yellow() Style       { return s.WithFG(Yellow) }
func (s Style) Blue() Style         { return s.WithFG(Blue) }
func (s Style) Magenta() Style      { return s.WithFG(Magenta) }
func (s Style) Cyan() Style         { return s.WithFG(Cyan) }
func (s Style) Gray() Style         { return s.WithFG(Gray) }
func (s Style) DarkGray() Style     { return s.WithFG(DarkGray) }
func (s Style) LightRed() Style     { return s.WithFG(LightRed) }
func (s Style) LightGreen() Style   { return s.WithFG(LightGreen) }
func (s Style) LightYellow() Style  { return s.WithFG(LightYellow) }
func (s Style) LightBlue() Style    { return s.WithFG(LightBlue) }
func (s Style) LightMagenta() Style { return s.WithFG(LightMagenta) }
func (s Style) LightCyan() Style    { return s.WithFG(LightCyan) }
func (s Style) White() Style        { return s.WithFG(White) }

// Background named-color builders.

func (s Style) OnBlack() Style        { return s.WithBG(Black) }
func (s Style) OnRed() Style          { return s.WithBG(Red) }
func (s Style) OnGreen() Style        { return s.WithBG(Green) }
func (s Style) OnYellow() Style       { return s.WithBG(Yellow) }
func (s Style) OnBlue() Style         { return s.WithBG(Blue) }
func (s Style) OnMagenta() Style      { return s.WithBG(Magenta) }
func (s Style) OnCyan() Style         { return s.WithBG(Cyan) }
func (s Style) OnGray() Style         { return s.WithBG(Gray) }
func (s Style) OnDarkGray() Style     { return s.WithBG(DarkGray) }
func (s Style) OnLightRed() Style     { return s.WithBG(LightRed) }
func (s Style) OnLightGreen() Style   { return s.WithBG(LightGreen) }
func (s Style) OnLightYellow() Style  { return s.WithBG(LightYellow) }
func (s Style) OnLightBlue() Style    { return s.WithBG(LightBlue) }
func (s Style) OnLightMagenta() Style { return s.WithBG(LightMagenta) }
func (s Style) OnLightCyan() Style    { return s.WithBG(LightCyan) }
func (s Style) OnWhite() Style        { return s.WithBG(White) }

// Modifier builders.

func (s Style) Bold() Style       { return s.WithAddModifier(ModBold) }
func (s Style) Dim() Style        { return s.WithAddModifier(ModDim) }
func (s Style) Italic() Style     { return s.WithAddModifier(ModItalic) }
func (s Style) Underlined() Style { return s.WithAddModifier(ModUnderlined) }
func (s Style) SlowBlink() Style  { return s.WithAddModifier(ModSlowBlink) }
func (s Style) RapidBlink() Style { return s.WithAddModifier(ModRapidBlink) }
func (s Style) Reversed() Style   { return s.WithAddModifier(ModReversed) }
func (s Style) Hidden() Style     { return s.WithAddModifier(ModHidden) }
func (s Style) CrossedOut() Style { return s.WithAddModifier(ModCrossedOut) }

func (s Style) NotBold() Style       { return s.WithRemoveModifier(ModBold) }
func (s Style) NotDim() Style        { return s.WithRemoveModifier(ModDim) }
func (s Style) NotItalic() Style     { return s.WithRemoveModifier(ModItalic) }
func (s Style) NotUnderlined() Style { return s.WithRemoveModifier(ModUnderlined) }
func (s Style) NotSlowBlink() Style  { return s.WithRemoveModifier(ModSlowBlink) }
func (s Style) NotRapidBlink() Style { return s.WithRemoveModifier(ModRapidBlink) }
func (s Style) NotReversed() Style   { return s.WithRemoveModifier(ModReversed) }
func (s Style) NotHidden() Style     { return s.WithRemoveModifier(ModHidden) }
func (s Style) NotCrossedOut() Style { return s.WithRemoveModifier(ModCrossedOut) }
