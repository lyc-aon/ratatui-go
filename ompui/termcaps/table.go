package termcaps

// knownTerminals is the static capability table. Values match OMP
// terminal-capabilities.ts KNOWN_TERMINALS.
var knownTerminals = map[TerminalID]TerminalInfo{
	TerminalBase: {
		ID:             TerminalBase,
		ImageProtocol:  ImageProtocolNone,
		TrueColor:      false,
		Hyperlinks:     false,
		NotifyProtocol: NotifyProtocolBell,
	},
	TerminalTrueColor: {
		ID:             TerminalTrueColor,
		ImageProtocol:  ImageProtocolNone,
		TrueColor:      true,
		Hyperlinks:     false,
		NotifyProtocol: NotifyProtocolBell,
	},
	TerminalKitty: {
		ID:                         TerminalKitty,
		ImageProtocol:              ImageProtocolKitty,
		TrueColor:                  true,
		Hyperlinks:                 true,
		NotifyProtocol:             NotifyProtocolOSC99,
		DECCARA:                    true,
		SupportsScreenToScrollback: true,
		TextSizing:                 true,
	},
	TerminalGhostty: {
		ID:             TerminalGhostty,
		ImageProtocol:  ImageProtocolKitty,
		TrueColor:      true,
		Hyperlinks:     true,
		NotifyProtocol: NotifyProtocolOSC9,
	},
	TerminalWezTerm: {
		ID:             TerminalWezTerm,
		ImageProtocol:  ImageProtocolKitty,
		TrueColor:      true,
		Hyperlinks:     true,
		NotifyProtocol: NotifyProtocolOSC9,
	},
	TerminalITerm2: {
		ID:             TerminalITerm2,
		ImageProtocol:  ImageProtocolITerm2,
		TrueColor:      true,
		Hyperlinks:     true,
		NotifyProtocol: NotifyProtocolOSC9,
	},
	TerminalVSCode: {
		ID:             TerminalVSCode,
		ImageProtocol:  ImageProtocolNone,
		TrueColor:      true,
		Hyperlinks:     true,
		NotifyProtocol: NotifyProtocolBell,
	},
	TerminalAlacritty: {
		ID:             TerminalAlacritty,
		ImageProtocol:  ImageProtocolNone,
		TrueColor:      true,
		Hyperlinks:     true,
		NotifyProtocol: NotifyProtocolBell,
	},
}

// LookupTerminalInfo returns the static profile for id.
// Unknown ids fall back to the base profile.
func LookupTerminalInfo(id TerminalID) TerminalInfo {
	if info, ok := knownTerminals[id]; ok {
		return info.Clone()
	}
	return knownTerminals[TerminalBase].Clone()
}

// KnownTerminalIDs returns every id present in the static table.
func KnownTerminalIDs() []TerminalID {
	return []TerminalID{
		TerminalBase,
		TerminalTrueColor,
		TerminalKitty,
		TerminalGhostty,
		TerminalWezTerm,
		TerminalITerm2,
		TerminalVSCode,
		TerminalAlacritty,
	}
}
