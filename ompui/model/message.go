package model

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// DecodeMessage parses an AgentMessage while preserving all original bytes.
func DecodeMessage(raw json.RawMessage) (Message, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return Message{}, fmt.Errorf("model: empty message")
	}
	var head struct {
		Role       string          `json:"role"`
		Content    json.RawMessage `json:"content"`
		Timestamp  int64           `json:"timestamp"`
		Synthetic  bool            `json:"synthetic"`
		ToolCallID string          `json:"toolCallId"`
		ToolName   string          `json:"toolName"`
		IsError    bool            `json:"isError"`
		Provider   string          `json:"provider"`
		Model      string          `json:"model"`
		StopReason string          `json:"stopReason"`
		Error      string          `json:"errorMessage"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return Message{}, fmt.Errorf("model: decode message: %w", err)
	}
	if head.Role == "" {
		return Message{}, fmt.Errorf("model: message missing role")
	}
	blocks, err := decodeContent(head.Content)
	if err != nil {
		return Message{}, err
	}
	return Message{
		Role:       head.Role,
		Content:    blocks,
		Timestamp:  head.Timestamp,
		Synthetic:  head.Synthetic,
		ToolCallID: head.ToolCallID,
		ToolName:   head.ToolName,
		IsError:    head.IsError,
		Provider:   head.Provider,
		Model:      head.Model,
		StopReason: head.StopReason,
		Error:      head.Error,
		Raw:        cloneRaw(raw),
	}, nil
}

func decodeContent(raw json.RawMessage) ([]ContentBlock, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return nil, fmt.Errorf("model: decode text content: %w", err)
		}
		return []ContentBlock{{Kind: ContentText, Text: text, Raw: cloneRaw(raw)}}, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("model: decode content array: %w", err)
	}
	out := make([]ContentBlock, 0, len(items))
	for _, item := range items {
		block, err := decodeBlock(item)
		if err != nil {
			return nil, err
		}
		out = append(out, block)
	}
	return out, nil
}

func decodeBlock(raw json.RawMessage) (ContentBlock, error) {
	var wire struct {
		Type      string          `json:"type"`
		Text      string          `json:"text"`
		Thinking  string          `json:"thinking"`
		Data      string          `json:"data"`
		MIMEType  string          `json:"mimeType"`
		ID        string          `json:"id"`
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
		Intent    string          `json:"intent"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return ContentBlock{}, fmt.Errorf("model: decode content block: %w", err)
	}
	block := ContentBlock{Kind: ContentUnknown, Raw: cloneRaw(raw)}
	switch wire.Type {
	case "text":
		block.Kind = ContentText
		block.Text = wire.Text
	case "thinking":
		block.Kind = ContentThinking
		block.Text = wire.Thinking
	case "redactedThinking":
		block.Kind = ContentRedactedThinking
		block.Data = wire.Data
	case "image":
		block.Kind = ContentImage
		block.Data = wire.Data
		block.MIMEType = wire.MIMEType
	case "toolCall":
		block.Kind = ContentToolCall
		block.ToolCall = &ToolCall{
			ID:        wire.ID,
			Name:      wire.Name,
			Arguments: cloneRaw(wire.Arguments),
			Intent:    wire.Intent,
			Raw:       cloneRaw(raw),
		}
	}
	return block, nil
}

func decodeMessageList(raw json.RawMessage) ([]Message, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("model: decode message list: %w", err)
	}
	out := make([]Message, 0, len(items))
	for _, item := range items {
		message, err := DecodeMessage(item)
		if err != nil {
			return nil, err
		}
		out = append(out, message)
	}
	return out, nil
}

func cloneRaw(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}
