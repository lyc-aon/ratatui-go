// Package runtime is the Go frontend's sole TTY owner.
//
// One Terminal instance owns the injected input/output files: raw mode,
// capability probes, decoded input events, serialized writes, title/progress
// notifications, mouse modes, and restore. Rendering stays outside this package.
//
// Lifecycle matches oh-my-pi packages/tui ProcessTerminal:
//
//   - Start enables raw mode, bracketed paste, one input coordinator, and one
//     resize watcher; probes Kitty keyboard (with modifyOtherKeys fallback),
//     OSC 11 appearance, OSC 99, and DECRQM modes 2026/2048/2031/1010/1011.
//   - DA1 sentinels are FIFO-owned so unsupported probes resolve without leaking
//     reply bytes into user input.
//   - CPR and other probe replies share the same input loop; user keys adjacent
//     to fragmented probe replies survive.
//   - Stop/Restore and EmergencyRestore are idempotent; alternate-screen leave is
//     never written blindly.
//
// Callers construct Terminal with injected *os.File handles and explicit env/
// options. Keep rendering and component logic outside this package.
package runtime
