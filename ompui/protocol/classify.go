package protocol

// Kind is a coarse routing class for an envelope.
type Kind int

const (
	// KindUnknown is an unrecognized type. Forward via raw payload.
	KindUnknown Kind = iota
	// KindHello is the handshake message.
	KindHello
	// KindShutdown is a graceful close request.
	KindShutdown
	// KindError is a protocol-level error.
	KindError
	// KindRPCCommand is a host→core RpcCommand.
	KindRPCCommand
	// KindRPCResponse is a core→host RpcResponse.
	KindRPCResponse
	// KindSessionEvent is a streamed session/agent/subagent event.
	KindSessionEvent
	// KindExtensionUIRequest is core→host extension UI.
	KindExtensionUIRequest
	// KindExtensionUIResponse is host→core extension UI reply.
	KindExtensionUIResponse
	// KindHostTool is any host_tool_* frame.
	KindHostTool
	// KindHostURI is any host_uri_* frame.
	KindHostURI
	// KindEditor is editor_state/update/query.
	KindEditor
	// KindStatus is status_sync / working_message / tools_expanded.
	KindStatus
	// KindTheme is theme_sync / theme_query.
	KindTheme
	// KindComponent is remote component_* messages.
	KindComponent
	// KindOverlay is overlay mount/unmount/update.
	KindOverlay
	// KindTerminalInput is terminal_input_subscription / terminal_input / terminal_input_result.
	KindTerminalInput
)

// String returns a stable name for Kind.
func (k Kind) String() string {
	switch k {
	case KindHello:
		return "hello"
	case KindShutdown:
		return "shutdown"
	case KindError:
		return "error"
	case KindRPCCommand:
		return "rpc_command"
	case KindRPCResponse:
		return "rpc_response"
	case KindSessionEvent:
		return "session_event"
	case KindExtensionUIRequest:
		return "extension_ui_request"
	case KindExtensionUIResponse:
		return "extension_ui_response"
	case KindHostTool:
		return "host_tool"
	case KindHostURI:
		return "host_uri"
	case KindEditor:
		return "editor"
	case KindStatus:
		return "status"
	case KindTheme:
		return "theme"
	case KindComponent:
		return "component"
	case KindOverlay:
		return "overlay"
	case KindTerminalInput:
		return "terminal_input"
	default:
		return "unknown"
	}
}

// Classify returns the routing kind for env.Type.
// Unknown types return KindUnknown so callers can forward losslessly.
func Classify(env Envelope) Kind {
	return ClassifyType(env.Type)
}

// ClassifyType classifies a bare type string.
func ClassifyType(typ string) Kind {
	switch typ {
	case MsgHello:
		return KindHello
	case MsgShutdown:
		return KindShutdown
	case MsgError:
		return KindError
	case MsgRPCCommand:
		return KindRPCCommand
	case MsgRPCResponse:
		return KindRPCResponse
	case MsgSessionEvent,
		MsgAvailableCommandsUpdate,
		MsgPromptResult,
		MsgCommandOutput,
		MsgSubagentLifecycle,
		MsgSubagentProgress,
		MsgSubagentEvent,
		// Bare AgentSessionEvent types also stream on the wire as their own type.
		EventAgentStart, EventAgentEnd, EventTurnStart, EventTurnEnd,
		EventMessageStart, EventMessageUpdate, EventMessageEnd,
		EventToolExecutionStart, EventToolExecutionUpdate, EventToolExecutionEnd,
		EventAutoCompactionStart, EventAutoCompactionEnd,
		EventAutoRetryStart, EventAutoRetryEnd,
		EventRetryFallbackApplied, EventRetryFallbackSucceeded,
		EventTtsrTriggered, EventTodoReminder, EventTodoAutoClear,
		EventIRCMessage, EventNotice, EventThinkingLevelChanged, EventGoalUpdated:
		return KindSessionEvent
	case MsgExtensionUIRequest:
		return KindExtensionUIRequest
	case MsgExtensionUIResponse:
		return KindExtensionUIResponse
	case MsgHostToolCall, MsgHostToolCancel, MsgHostToolUpdate, MsgHostToolResult:
		return KindHostTool
	case MsgHostURIRequest, MsgHostURICancel, MsgHostURIResult:
		return KindHostURI
	case MsgEditorState, MsgEditorUpdate, MsgEditorQuery:
		return KindEditor
	case MsgStatusSync, MsgWorkingMessage, MsgToolsExpanded:
		return KindStatus
	case MsgThemeSync, MsgThemeQuery:
		return KindTheme
	case MsgComponentOpen, MsgComponentRender, MsgComponentResult,
		MsgComponentInput, MsgComponentInputResult, MsgComponentInvalidate,
		MsgComponentDispose, MsgComponentFocus, MsgComponentFocusRequest:
		return KindComponent
	case MsgOverlayMount, MsgOverlayUnmount, MsgOverlayUpdate:
		return KindOverlay
	case MsgTerminalInputSubscription, MsgTerminalInput, MsgTerminalInputResult:
		return KindTerminalInput
	default:
		// Historical bare RpcCommand types arriving without rpc_command wrapper.
		if IsKnownRPCCommand(typ) {
			return KindRPCCommand
		}
		return KindUnknown
	}
}

// IsSessionEventType reports whether typ is a known streamed session/agent event.
func IsSessionEventType(typ string) bool {
	return ClassifyType(typ) == KindSessionEvent
}
