package client

import (
	"context"
	"encoding/json"

	"github.com/michaelkelly/ratatui-go/ompui/protocol"
)

// Prompt sends a prompt command and waits for the RPC response
// ({agentInvoked?: bool} on success). Session events stream separately via
// Subscribe; a later prompt_result frame may also arrive for local-only prompts.
func (c *Client) Prompt(ctx context.Context, message string, opts ...PromptOption) (Response, error) {
	cfg := promptConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	fields := map[string]any{"message": message}
	if len(cfg.Images) > 0 {
		fields["images"] = cfg.Images
	}
	if cfg.StreamingBehavior != "" {
		fields["streamingBehavior"] = cfg.StreamingBehavior
	}
	cmd := protocol.BuildRPCCommand(protocol.CmdPrompt, cfg.ID, fields)
	return c.Call(ctx, cmd)
}

// PromptOption configures [Client.Prompt].
type PromptOption func(*promptConfig)

type promptConfig struct {
	ID                string
	Images            []any
	StreamingBehavior string // "steer" | "followUp"
}

// WithPromptID sets the correlation id.
func WithPromptID(id string) PromptOption {
	return func(c *promptConfig) { c.ID = id }
}

// WithPromptImages attaches image content values (protocol-shaped objects).
func WithPromptImages(images ...any) PromptOption {
	return func(c *promptConfig) { c.Images = append(c.Images, images...) }
}

// WithStreamingBehavior sets streamingBehavior to "steer" or "followUp".
func WithStreamingBehavior(behavior string) PromptOption {
	return func(c *promptConfig) { c.StreamingBehavior = behavior }
}

// Abort sends an abort command and waits for the response.
func (c *Client) Abort(ctx context.Context) (Response, error) {
	return c.Call(ctx, protocol.BuildRPCCommand(protocol.CmdAbort, "", nil))
}

// GetState sends get_state and returns the correlated response.
// Decode data with json.Unmarshal into [protocol.SessionState] or keep Raw.
func (c *Client) GetState(ctx context.Context) (Response, error) {
	return c.Call(ctx, protocol.BuildRPCCommand(protocol.CmdGetState, "", nil))
}

// GetStateDecoded is GetState plus decode into [protocol.SessionState].
func (c *Client) GetStateDecoded(ctx context.Context) (protocol.SessionState, Response, error) {
	resp, err := c.GetState(ctx)
	if err != nil {
		return protocol.SessionState{}, resp, err
	}
	var st protocol.SessionState
	if len(resp.Data) > 0 {
		if uerr := json.Unmarshal(resp.Data, &st); uerr != nil {
			return protocol.SessionState{}, resp, uerr
		}
	}
	return st, resp, nil
}

// GetAvailableCommands sends get_available_commands and waits for the response.
func (c *Client) GetAvailableCommands(ctx context.Context) (Response, error) {
	return c.Call(ctx, protocol.BuildRPCCommand(protocol.CmdGetAvailableCommands, "", nil))
}

// ExtensionUIResponseValue replies to an extension UI request with a string value
// (select / input / editor methods).
func (c *Client) ExtensionUIResponseValue(id, value string) error {
	v := value
	return c.ReplyExtensionUI(protocol.ExtensionUIResponse{
		Type:  protocol.MsgExtensionUIResponse,
		ID:    id,
		Value: &v,
	})
}

// ExtensionUIResponseConfirmed replies to a confirm extension UI request.
func (c *Client) ExtensionUIResponseConfirmed(id string, confirmed bool) error {
	v := confirmed
	return c.ReplyExtensionUI(protocol.ExtensionUIResponse{
		Type:      protocol.MsgExtensionUIResponse,
		ID:        id,
		Confirmed: &v,
	})
}

// ExtensionUIResponseCancelled replies with cancelled:true (optional timedOut).
func (c *Client) ExtensionUIResponseCancelled(id string, timedOut bool) error {
	cancelled := true
	resp := protocol.ExtensionUIResponse{
		Type:      protocol.MsgExtensionUIResponse,
		ID:        id,
		Cancelled: &cancelled,
	}
	if timedOut {
		t := true
		resp.TimedOut = &t
	}
	return c.ReplyExtensionUI(resp)
}

// Steer sends a steer command.
func (c *Client) Steer(ctx context.Context, message string) (Response, error) {
	return c.Call(ctx, protocol.BuildRPCCommand(protocol.CmdSteer, "", map[string]any{
		"message": message,
	}))
}

// FollowUp sends a follow_up command.
func (c *Client) FollowUp(ctx context.Context, message string) (Response, error) {
	return c.Call(ctx, protocol.BuildRPCCommand(protocol.CmdFollowUp, "", map[string]any{
		"message": message,
	}))
}

// AbortAndPrompt sends abort_and_prompt.
func (c *Client) AbortAndPrompt(ctx context.Context, message string) (Response, error) {
	return c.Call(ctx, protocol.BuildRPCCommand(protocol.CmdAbortAndPrompt, "", map[string]any{
		"message": message,
	}))
}

// CallCommand is a convenience for a bare type+fields command.
func (c *Client) CallCommand(ctx context.Context, typ string, fields map[string]any) (Response, error) {
	return c.Call(ctx, protocol.BuildRPCCommand(typ, "", fields))
}

// SendCommand is fire-and-forget for a bare type+fields command.
func (c *Client) SendCommand(typ string, fields map[string]any) error {
	return c.Send(protocol.BuildRPCCommand(typ, "", fields))
}
