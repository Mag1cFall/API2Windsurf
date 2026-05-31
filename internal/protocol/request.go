package protocol

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
)

type ChatRequest struct {
	Model     string
	System    string
	Messages  []Message
	Tools     []Tool
	MaxTokens uint64
	Stop      []string
}

type Message struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
}

type ToolCall struct {
	ID    string
	Name  string
	Input string
}

type Tool struct {
	Name        string
	Description string
	Schema      string
}

const (
	reqFieldSystem  = 2
	reqFieldMessage = 3
	reqFieldGenConf = 8
	reqFieldTool    = 10
	reqFieldModel   = 21
)

const (
	msgFieldSource    = 2
	msgFieldContent   = 3
	msgFieldToolCall  = 6
	msgFieldToolReply = 7

	sourceUser         = 1
	sourceAssistant    = 2
	sourceTool         = 4
	sourceSystemPrompt = 5
)

func DecodeChatRequest(body []byte) (*ChatRequest, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("empty chat body")
	}
	req := &ChatRequest{}
	for _, f := range scan(body) {
		switch {
		case f.Num == reqFieldSystem && f.Wire == wireBytes:
			req.System = string(f.Data)
		case f.Num == reqFieldMessage && f.Wire == wireBytes:
			if m, ok := decodeMessage(f.Data); ok {
				req.Messages = append(req.Messages, m)
			}
		case f.Num == reqFieldGenConf && f.Wire == wireBytes:
			decodeGenConfig(f.Data, req)
		case f.Num == reqFieldTool && f.Wire == wireBytes:
			if t, ok := decodeTool(f.Data); ok {
				req.Tools = append(req.Tools, t)
			}
		case f.Num == reqFieldModel && f.Wire == wireBytes:
			req.Model = strings.TrimSpace(string(f.Data))
		}
	}
	if len(req.Messages) == 0 && strings.TrimSpace(req.System) == "" {
		return nil, fmt.Errorf("chat body has no system prompt and no messages")
	}
	return req, nil
}

func decodeMessage(data []byte) (Message, bool) {
	var m Message
	seen := false
	for _, f := range scan(data) {
		switch {
		case f.Num == msgFieldSource && f.Wire == wireVarint:
			seen = true
			m.Role = roleForSource(f.Uint)
		case f.Num == msgFieldContent && f.Wire == wireBytes:
			m.Content = string(f.Data)
			seen = true
		case f.Num == msgFieldToolCall && f.Wire == wireBytes:
			if tc := decodeToolCall(f.Data); tc.ID != "" || tc.Name != "" {
				m.ToolCalls = append(m.ToolCalls, tc)
			}
			seen = true
		case f.Num == msgFieldToolReply && f.Wire == wireBytes:
			m.ToolCallID = string(f.Data)
			seen = true
		}
	}
	if !seen {
		return Message{}, false
	}
	return m, true
}

func roleForSource(source uint64) string {
	switch source {
	case sourceUser:
		return "user"
	case sourceAssistant:
		return "assistant"
	case sourceTool:
		return "tool"
	case sourceSystemPrompt:
		return "system"
	default:
		return "user"
	}
}

func decodeToolCall(data []byte) ToolCall {
	var tc ToolCall
	for _, f := range scan(data) {
		if f.Wire != wireBytes {
			continue
		}
		switch f.Num {
		case 1:
			tc.ID = string(f.Data)
		case 2:
			tc.Name = string(f.Data)
		case 3:
			tc.Input = string(f.Data)
		}
	}
	return tc
}

func decodeTool(data []byte) (Tool, bool) {
	var t Tool
	for _, f := range scan(data) {
		if f.Wire != wireBytes {
			continue
		}
		switch f.Num {
		case 1:
			t.Name = string(f.Data)
		case 2:
			t.Description = string(f.Data)
		case 3:
			t.Schema = string(f.Data)
		}
	}
	if t.Name == "" {
		return Tool{}, false
	}
	return t, true
}

const (
	genFieldMaxTokens = 2
	genFieldStop      = 9
)

func decodeGenConfig(data []byte, req *ChatRequest) {
	for _, f := range scan(data) {
		switch {
		case f.Num == genFieldMaxTokens && f.Wire == wireVarint:
			req.MaxTokens = f.Uint
		case f.Num == genFieldStop && f.Wire == wireBytes:
			req.Stop = append(req.Stop, string(f.Data))
		}
	}
}

func StripEnvelope(raw []byte) ([]byte, error) {
	if len(raw) < envelopeHeaderLen {
		return nil, fmt.Errorf("body shorter than envelope header")
	}
	flag := raw[0]
	payload := raw[envelopeHeaderLen:]
	if flag&flagCompressed == 0 {
		return payload, nil
	}
	zr, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer zr.Close()
	out, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("gzip decompress: %w", err)
	}
	return out, nil
}
