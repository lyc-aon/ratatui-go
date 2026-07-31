package runtime

import "os"

// blindEmergencyWrite emits the crash-path mode teardown to os.Stdout.
// alt controls whether DECRST 1049 is included (never blind on Windows when false).
func blindEmergencyWrite(alt bool) {
	payload := seqSyncOutputDisable +
		seqAutowrapEnable +
		seqBracketedPasteDisable +
		seqMode2031Disable +
		seqInBandResizeDisable +
		seqEnhancedPasteDisable +
		seqKittyPop +
		seqModifyOtherKeysDisable +
		seqMouseDisableAll
	if alt {
		payload += seqLeaveAltScreen
	}
	payload += seqShowCursor
	_, _ = os.Stdout.WriteString(payload)
}
