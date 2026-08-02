package protocol

// RPC command type strings — exhaustive list from rpc-types.ts RpcCommand.
// Kept as constants so Go callers avoid magic strings.
const (
	CmdPrompt          = "prompt"
	CmdSteer           = "steer"
	CmdFollowUp        = "follow_up"
	CmdDequeueMessages = "dequeue_messages"

	CmdAbort                   = "abort"
	CmdAbortAndPrompt          = "abort_and_prompt"
	CmdNewSession              = "new_session"
	CmdGetState                = "get_state"
	CmdGetAvailableCommands    = "get_available_commands"
	CmdSetTodos                = "set_todos"
	CmdSetHostTools            = "set_host_tools"
	CmdSetHostURISchemes       = "set_host_uri_schemes"
	CmdSetSubagentSubscription = "set_subagent_subscription"
	CmdGetSubagents            = "get_subagents"
	CmdGetSubagentMessages     = "get_subagent_messages"
	CmdSetModel                = "set_model"
	CmdCycleModel              = "cycle_model"
	CmdGetAvailableModels      = "get_available_models"
	CmdSetThinkingLevel        = "set_thinking_level"
	CmdCycleThinkingLevel      = "cycle_thinking_level"
	CmdSetSteeringMode         = "set_steering_mode"
	CmdSetFollowUpMode         = "set_follow_up_mode"
	CmdSetInterruptMode        = "set_interrupt_mode"
	CmdCompact                 = "compact"
	CmdSetAutoCompaction       = "set_auto_compaction"
	CmdSetAutoRetry            = "set_auto_retry"
	CmdAbortRetry              = "abort_retry"
	CmdBash                    = "bash"
	CmdAbortBash               = "abort_bash"
	CmdGetSessionStats         = "get_session_stats"
	CmdExportHTML              = "export_html"
	CmdSwitchSession           = "switch_session"
	CmdBranch                  = "branch"
	CmdGetBranchMessages       = "get_branch_messages"
	CmdGetLastAssistantText    = "get_last_assistant_text"
	CmdSetSessionName          = "set_session_name"
	CmdHandoff                 = "handoff"
	CmdGetMessages             = "get_messages"
	CmdGetLoginProviders       = "get_login_providers"
	CmdLogin                   = "login"
)

// AllRPCCommands lists every known RpcCommand type string.
// Unknown command types MUST still be forwarded losslessly via [RPCCommand.Raw].
func AllRPCCommands() []string {
	return []string{
		CmdPrompt,
		CmdSteer,
		CmdFollowUp,
		CmdDequeueMessages,

		CmdAbort,
		CmdAbortAndPrompt,
		CmdNewSession,
		CmdGetState,
		CmdGetAvailableCommands,
		CmdSetTodos,
		CmdSetHostTools,
		CmdSetHostURISchemes,
		CmdSetSubagentSubscription,
		CmdGetSubagents,
		CmdGetSubagentMessages,
		CmdSetModel,
		CmdCycleModel,
		CmdGetAvailableModels,
		CmdSetThinkingLevel,
		CmdCycleThinkingLevel,
		CmdSetSteeringMode,
		CmdSetFollowUpMode,
		CmdSetInterruptMode,
		CmdCompact,
		CmdSetAutoCompaction,
		CmdSetAutoRetry,
		CmdAbortRetry,
		CmdBash,
		CmdAbortBash,
		CmdGetSessionStats,
		CmdExportHTML,
		CmdSwitchSession,
		CmdBranch,
		CmdGetBranchMessages,
		CmdGetLastAssistantText,
		CmdSetSessionName,
		CmdHandoff,
		CmdGetMessages,
		CmdGetLoginProviders,
		CmdLogin,
	}
}

// IsKnownRPCCommand reports whether typ is in the RpcCommand catalog.
func IsKnownRPCCommand(typ string) bool {
	switch typ {
	case CmdPrompt, CmdSteer, CmdFollowUp, CmdDequeueMessages, CmdAbort, CmdAbortAndPrompt, CmdNewSession,
		CmdGetState, CmdGetAvailableCommands, CmdSetTodos, CmdSetHostTools, CmdSetHostURISchemes,

		CmdSetSubagentSubscription, CmdGetSubagents, CmdGetSubagentMessages,
		CmdSetModel, CmdCycleModel, CmdGetAvailableModels,
		CmdSetThinkingLevel, CmdCycleThinkingLevel,
		CmdSetSteeringMode, CmdSetFollowUpMode, CmdSetInterruptMode,
		CmdCompact, CmdSetAutoCompaction, CmdSetAutoRetry, CmdAbortRetry,
		CmdBash, CmdAbortBash,
		CmdGetSessionStats, CmdExportHTML, CmdSwitchSession, CmdBranch,
		CmdGetBranchMessages, CmdGetLastAssistantText, CmdSetSessionName, CmdHandoff,
		CmdGetMessages, CmdGetLoginProviders, CmdLogin:
		return true
	default:
		return false
	}
}

// Known session-level stdout frame types beyond AgentSessionEvent proper.
const (
	// FrameAvailableCommandsUpdate is emitted when slash commands change.
	FrameAvailableCommandsUpdate = MsgAvailableCommandsUpdate
	// FramePromptResult is emitted for local-only prompt completion.
	FramePromptResult = MsgPromptResult
	// FrameCommandOutput is user-visible text from a local slash command.
	FrameCommandOutput = MsgCommandOutput
	// FrameSubagentLifecycle / Progress / Event are subagent stream frames.
	FrameSubagentLifecycle = MsgSubagentLifecycle
	FrameSubagentProgress  = MsgSubagentProgress
	FrameSubagentEvent     = MsgSubagentEvent
)

// RPCCommandFieldDoc documents the known fields per command for maintainers.
// Values are human-readable field lists matching rpc-types.ts; not used at runtime.
var RPCCommandFieldDoc = map[string]string{
	CmdPrompt:          "message:string, images?:ImageContent[], streamingBehavior?:'steer'|'followUp'",
	CmdSteer:           "message:string, images?:ImageContent[]",
	CmdFollowUp:        "message:string, images?:ImageContent[]",
	CmdDequeueMessages: "(none)",

	CmdAbort:                   "(none)",
	CmdAbortAndPrompt:          "message:string, images?:ImageContent[]",
	CmdNewSession:              "parentSession?:string",
	CmdGetState:                "(none)",
	CmdGetAvailableCommands:    "(none)",
	CmdSetTodos:                "phases:TodoPhase[]",
	CmdSetHostTools:            "tools:RpcHostToolDefinition[]",
	CmdSetHostURISchemes:       "schemes:RpcHostUriSchemeDefinition[]",
	CmdSetSubagentSubscription: "level:'off'|'progress'|'events'",
	CmdGetSubagents:            "(none)",
	CmdGetSubagentMessages:     "subagentId?:string, sessionFile?:string, fromByte?:number",
	CmdSetModel:                "provider:string, modelId:string",
	CmdCycleModel:              "(none)",
	CmdGetAvailableModels:      "(none)",
	CmdSetThinkingLevel:        "level:ThinkingLevel",
	CmdCycleThinkingLevel:      "(none)",
	CmdSetSteeringMode:         "mode:'all'|'one-at-a-time'",
	CmdSetFollowUpMode:         "mode:'all'|'one-at-a-time'",
	CmdSetInterruptMode:        "mode:'immediate'|'wait'",
	CmdCompact:                 "customInstructions?:string",
	CmdSetAutoCompaction:       "enabled:boolean",
	CmdSetAutoRetry:            "enabled:boolean",
	CmdAbortRetry:              "(none)",
	CmdBash:                    "command:string",
	CmdAbortBash:               "(none)",
	CmdGetSessionStats:         "(none)",
	CmdExportHTML:              "outputPath?:string",
	CmdSwitchSession:           "sessionPath:string",
	CmdBranch:                  "entryId:string",
	CmdGetBranchMessages:       "(none)",
	CmdGetLastAssistantText:    "(none)",
	CmdSetSessionName:          "name:string",
	CmdHandoff:                 "customInstructions?:string",
	CmdGetMessages:             "(none)",
	CmdGetLoginProviders:       "(none)",
	CmdLogin:                   "providerId:string",
}

// BuildRPCCommand constructs an RPCCommand from type, id, and field map.
func BuildRPCCommand(typ, id string, fields map[string]any) RPCCommand {
	return RPCCommand{Type: typ, ID: id, Fields: fields}
}

// EnvelopeFromRPCCommand wraps a command as MsgRPCCommand.
func EnvelopeFromRPCCommand(cmd RPCCommand) (Envelope, error) {
	raw, err := cmd.MarshalJSON()
	if err != nil {
		return Envelope{}, err
	}
	return WrapRPCCommand(RawPayload(raw))
}

// EnvelopeFromHistorical wraps any historical stdout/stdin JSON object.
func EnvelopeFromHistorical(frame []byte) (Envelope, error) {
	return WrapHistorical(RawPayload(frame))
}

// MessageKinds returns every Msg* constant known to this package.
func MessageKinds() []string {
	return []string{
		MsgHello,
		MsgShutdown,
		MsgError,
		MsgRPCCommand,
		MsgRPCResponse,
		MsgSessionEvent,
		MsgAvailableCommandsUpdate,
		MsgPromptResult,
		MsgCommandOutput,
		MsgSubagentLifecycle,
		MsgSubagentProgress,
		MsgSubagentEvent,
		MsgExtensionUIRequest,
		MsgExtensionUIResponse,
		MsgHostToolCall,
		MsgHostToolCancel,
		MsgHostToolUpdate,
		MsgHostToolResult,
		MsgHostURIRequest,
		MsgHostURICancel,
		MsgHostURIResult,
		MsgEditorState,
		MsgEditorUpdate,
		MsgEditorQuery,
		MsgStatusSync,
		MsgWorkingMessage,
		MsgToolsExpanded,
		MsgThemeSync,
		MsgThemeQuery,
		MsgComponentOpen,
		MsgComponentRender,
		MsgComponentResult,
		MsgComponentInput,
		MsgComponentInputResult,
		MsgComponentInvalidate,
		MsgComponentDispose,
		MsgComponentFocus,
		MsgComponentFocusRequest,
		MsgTerminalInputSubscription,
		MsgTerminalInput,
		MsgTerminalInputResult,
		MsgOverlayMount,
		MsgOverlayUnmount,
		MsgOverlayUpdate,
	}
}
