package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"api2windsurf/internal/protocol"
)

type Upstream struct {
	Provider string
	BaseURL  string
	APIKey   string
	Model    string
}

func BuildUpstreamRequest(up Upstream, req *protocol.ChatRequest) (*http.Request, error) {
	switch strings.ToLower(strings.TrimSpace(up.Provider)) {
	case "anthropic":
		return buildAnthropic(up, req)
	case "google":
		return buildGemini(up, req)
	default:
		return buildOpenAI(up, req)
	}
}

type openAIPayload struct {
	Model     string            `json:"model"`
	Messages  []json.RawMessage `json:"messages"`
	Stream    bool              `json:"stream"`
	Tools     []openAITool      `json:"tools,omitempty"`
	MaxTokens *int              `json:"max_tokens,omitempty"`
	Stop      []string          `json:"stop,omitempty"`
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

func buildOpenAI(up Upstream, req *protocol.ChatRequest) (*http.Request, error) {
	payload := openAIPayload{
		Model:    up.Model,
		Messages: openAIMessages(req),
		Stream:   true,
	}
	if req.MaxTokens > 0 {
		mt := int(req.MaxTokens)
		payload.MaxTokens = &mt
	}
	if len(req.Stop) > 0 {
		payload.Stop = req.Stop
	}
	for _, t := range req.Tools {
		var params json.RawMessage
		if t.Schema != "" {
			params = json.RawMessage(t.Schema)
		}
		payload.Tools = append(payload.Tools, openAITool{
			Type:     "function",
			Function: openAIFunction{Name: t.Name, Description: t.Description, Parameters: params},
		})
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, chatCompletionsURL(up.BaseURL), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+up.APIKey)
	httpReq.Header.Set("Accept", "text/event-stream")
	return httpReq, nil
}

func openAIMessages(req *protocol.ChatRequest) []json.RawMessage {
	var msgs []json.RawMessage
	if strings.TrimSpace(req.System) != "" {
		m, _ := json.Marshal(map[string]string{"role": "system", "content": req.System})
		msgs = append(msgs, m)
	}
	for _, cm := range req.Messages {
		switch cm.Role {
		case "assistant":
			msg := map[string]any{"role": "assistant", "content": cm.Content}
			if len(cm.ToolCalls) > 0 {
				var calls []map[string]any
				for _, tc := range cm.ToolCalls {
					calls = append(calls, map[string]any{
						"id":   tc.ID,
						"type": "function",
						"function": map[string]string{
							"name":      tc.Name,
							"arguments": tc.Input,
						},
					})
				}
				msg["tool_calls"] = calls
			}
			m, _ := json.Marshal(msg)
			msgs = append(msgs, m)
		case "tool":
			content := cm.Content
			if content == "" {
				content = " "
			}
			m, _ := json.Marshal(map[string]string{
				"role":         "tool",
				"content":      content,
				"tool_call_id": cm.ToolCallID,
			})
			msgs = append(msgs, m)
		case "system":
			m, _ := json.Marshal(map[string]string{"role": "system", "content": cm.Content})
			msgs = append(msgs, m)
		default:
			m, _ := json.Marshal(map[string]string{"role": "user", "content": cm.Content})
			msgs = append(msgs, m)
		}
	}
	return msgs
}

type anthropicPayload struct {
	Model     string          `json:"model"`
	System    string          `json:"system,omitempty"`
	Messages  json.RawMessage `json:"messages"`
	Stream    bool            `json:"stream"`
	MaxTokens int             `json:"max_tokens"`
	Stop      []string        `json:"stop_sequences,omitempty"`
	Tools     []anthropicTool `json:"tools,omitempty"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

const anthropicDefaultMaxTokens = 64000

func buildAnthropic(up Upstream, req *protocol.ChatRequest) (*http.Request, error) {
	msgs := anthropicMessages(req)
	if len(msgs) == 0 {
		return nil, fmt.Errorf("anthropic: no messages after decode")
	}
	msgsJSON, err := json.Marshal(msgs)
	if err != nil {
		return nil, err
	}
	maxTok := anthropicDefaultMaxTokens
	if req.MaxTokens > 0 {
		maxTok = int(req.MaxTokens)
	}
	payload := anthropicPayload{
		Model:     up.Model,
		System:    mergedSystem(req),
		Messages:  msgsJSON,
		Stream:    true,
		MaxTokens: maxTok,
	}
	if len(req.Stop) > 0 {
		payload.Stop = req.Stop
	}
	for _, t := range req.Tools {
		schema := json.RawMessage(`{"type":"object","properties":{}}`)
		if t.Schema != "" {
			schema = json.RawMessage(t.Schema)
		}
		payload.Tools = append(payload.Tools, anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
		})
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, messagesURL(up.BaseURL), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", up.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Accept", "text/event-stream")
	return httpReq, nil
}

func mergedSystem(req *protocol.ChatRequest) string {
	parts := make([]string, 0, 2)
	if s := strings.TrimSpace(req.System); s != "" {
		parts = append(parts, s)
	}
	for _, cm := range req.Messages {
		if cm.Role == "system" && strings.TrimSpace(cm.Content) != "" {
			parts = append(parts, cm.Content)
		}
	}
	return strings.Join(parts, "\n\n")
}

func anthropicMessages(req *protocol.ChatRequest) []map[string]any {
	type pending struct {
		role    string
		content []map[string]any
	}
	var merged []pending
	push := func(role string, blocks ...map[string]any) {
		if n := len(merged); n > 0 && merged[n-1].role == role {
			merged[n-1].content = append(merged[n-1].content, blocks...)
			return
		}
		merged = append(merged, pending{role: role, content: append([]map[string]any{}, blocks...)})
	}
	for _, cm := range req.Messages {
		switch cm.Role {
		case "system":
			continue
		case "assistant":
			var blocks []map[string]any
			if cm.Content != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": cm.Content})
			}
			for _, tc := range cm.ToolCalls {
				var input any
				if err := json.Unmarshal([]byte(tc.Input), &input); err != nil {
					input = map[string]any{}
				}
				blocks = append(blocks, map[string]any{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Name,
					"input": input,
				})
			}
			if len(blocks) == 0 {
				blocks = append(blocks, map[string]any{"type": "text", "text": ""})
			}
			push("assistant", blocks...)
		case "tool":
			push("user", map[string]any{
				"type":        "tool_result",
				"tool_use_id": cm.ToolCallID,
				"content":     cm.Content,
			})
		default:
			push("user", map[string]any{"type": "text", "text": cm.Content})
		}
	}
	if len(merged) > 0 && merged[0].role != "user" {
		merged = append([]pending{{role: "user", content: []map[string]any{{"type": "text", "text": ""}}}}, merged...)
	}
	out := make([]map[string]any, 0, len(merged))
	for _, p := range merged {
		out = append(out, map[string]any{"role": p.role, "content": p.content})
	}
	return out
}

type geminiPayload struct {
	Contents          []geminiContent     `json:"contents"`
	SystemInstruction *geminiInstruction  `json:"systemInstruction,omitempty"`
	Tools             []geminiToolWrapper `json:"tools,omitempty"`
}

type geminiToolWrapper struct {
	FunctionDeclarations []geminiFunctionDecl `json:"functionDeclarations"`
}

type geminiFunctionDecl struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                  `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

type geminiFunctionResponse struct {
	Name     string `json:"name"`
	Response any    `json:"response"`
}

type geminiInstruction struct {
	Parts []geminiPart `json:"parts"`
}

func buildGemini(up Upstream, req *protocol.ChatRequest) (*http.Request, error) {
	toolNames := map[string]string{}
	for _, m := range req.Messages {
		for _, tc := range m.ToolCalls {
			if tc.ID != "" {
				toolNames[tc.ID] = tc.Name
			}
		}
	}
	var contents []geminiContent
	push := func(role string, part geminiPart) {
		if n := len(contents); n > 0 && contents[n-1].Role == role {
			contents[n-1].Parts = append(contents[n-1].Parts, part)
			return
		}
		contents = append(contents, geminiContent{Role: role, Parts: []geminiPart{part}})
	}
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			continue
		case "assistant":
			if m.Content != "" {
				push("model", geminiPart{Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				var args json.RawMessage
				if strings.TrimSpace(tc.Input) != "" {
					args = json.RawMessage(tc.Input)
				}
				push("model", geminiPart{FunctionCall: &geminiFunctionCall{Name: tc.Name, Args: args}})
			}
		case "tool":
			var resp any
			if err := json.Unmarshal([]byte(m.Content), &resp); err != nil {
				resp = map[string]any{"result": m.Content}
			}
			push("user", geminiPart{FunctionResponse: &geminiFunctionResponse{
				Name:     toolNames[m.ToolCallID],
				Response: resp,
			}})
		default:
			push("user", geminiPart{Text: m.Content})
		}
	}
	if len(contents) == 0 {
		return nil, fmt.Errorf("gemini: no contents after decode")
	}
	if contents[0].Role != "user" {
		contents = append([]geminiContent{{Role: "user", Parts: []geminiPart{{Text: ""}}}}, contents...)
	}
	payload := geminiPayload{Contents: contents}
	if sys := mergedSystem(req); sys != "" {
		payload.SystemInstruction = &geminiInstruction{Parts: []geminiPart{{Text: sys}}}
	}
	if len(req.Tools) > 0 {
		decls := make([]geminiFunctionDecl, 0, len(req.Tools))
		for _, t := range req.Tools {
			var params json.RawMessage
			if strings.TrimSpace(t.Schema) != "" {
				params = json.RawMessage(t.Schema)
			}
			decls = append(decls, geminiFunctionDecl{Name: t.Name, Description: t.Description, Parameters: params})
		}
		payload.Tools = []geminiToolWrapper{{FunctionDeclarations: decls}}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?key=%s&alt=sse",
		strings.TrimRight(up.BaseURL, "/"),
		url.PathEscape(up.Model),
		url.QueryEscape(up.APIKey),
	)
	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	return httpReq, nil
}
