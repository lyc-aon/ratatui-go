// Package input implements an incremental byte-stream terminal input decoder.
//
// It frames CSI/OSC/DCS/APC/SS3/meta sequences and Unicode scalars, assembles
// bracketed paste, and decodes keys (legacy, CSI-u, modifyOtherKeys, numpad),
// SGR/X10 mouse, focus, and raw sequences into event.Event values.
//
// This package never opens files, owns a TTY, or runs capability probes.
// Callers feed raw bytes via Decoder.Write / Decoder.Push and drain events.
//
// Behavior matches oh-my-pi packages/tui stdin-buffer, keys, mouse,
// bracketed-paste and crates/pi-natives keys.rs.
package input
