package app

import (
	"github.com/lyc-aon/ratatui-go/ompui/client"
	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/event"
	"github.com/lyc-aon/ratatui-go/ompui/interact"
	"github.com/lyc-aon/ratatui-go/ompui/protocol"
)

// cmd is one unit of work drained by the single app loop.
// Only the loop mutates App state.
type cmdKind uint8

const (
	cmdNone cmdKind = iota
	cmdTermEvent
	cmdRPCEvent
	cmdTick
	cmdResize
	cmdQuit
	cmdForceRender
	cmdRPCDone // background Call finished
	cmdOpenURLDone
	cmdCoreDied
	cmdSignal
)

type command struct {
	kind cmdKind

	term event.Event
	rpc  client.Event

	// rpcDone carries a completed background RPC (prompt/abort/etc).
	rpcDone rpcDone

	// openURL result for status display
	openURLErr error
	openURL    string

	// signal is os signal name when kind==cmdSignal
	signal string

	// err is a fatal local error posted onto the loop
	err error
}

type rpcDone struct {
	op      string // "prompt" | "abort" | "steer" | "follow_up" | "call"
	resp    client.Response
	err     error
	restore string // editor text to restore on failure
}

// extDialog holds one interactive extension UI or remote-component overlay.
type extDialog struct {
	id     string
	method string
	req    protocol.ExtensionUIRequest
	handle interact.OverlayHandle
	comp   component.Component
}
