// Package client is a production Go host for a Bun OMP core running
// `--mode rpc-ui` (or plain `--mode rpc`).
//
// The Go frontend owns the TTY elsewhere. This package only owns the child
// process and the JSONL (or length-prefixed) RPC pipes:
//
//   - Spawns an argv vector with no shell interpolation ([Command]).
//   - Speaks the existing Bun rpc-ui JSONL frames via [protocol], preserving
//     every unknown frame as a raw envelope.
//   - In compatibility mode (historical Bun, no hello), writes bare
//     `{id,type,...}` command lines — never v1 `{v,type:"rpc_command",payload}`
//     wrappers — until the peer advertises v1 via hello.
//   - Correlates concurrent [Client.Call] requests by id, with real context
//     cancellation.
//   - Delivers ordered events through a single reader → bounded queue →
//     dispatcher path that never silently drops session, tool, or extension-UI
//     frames.
//   - Accepts historical Bun startup (`{"type":"ready"}`, no hello) and
//     validates a v1 [protocol.MsgHello] when the peer sends one.
//
// It does not render, open a TTY, or own terminal state.
package client
