package proxy

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

type sseDelta struct {
	Text      string
	Reasoning string
	ToolCalls []toolDelta
}

type toolDelta struct {
	Index int
	ID    string
	Name  string
	Args  string
}

type usage struct {
	prompt     int
	completion int
}

type streamHandlers struct {
	onText      func(string)
	onReasoning func(string)
	onToolCall  func(toolDelta)
}

func streamUpstream(body io.Reader, provider string, h streamHandlers, u *usage) (hasToolCalls bool, err error) {
	provider = strings.ToLower(provider)
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 32*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(line[5:])
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			return hasToolCalls, nil
		}
		if u != nil {
			scanUsage(provider, data, u)
		}
		switch provider {
		case "anthropic":
			if anthropicIsStop(data) {
				return hasToolCalls, nil
			}
			d := parseAnthropic(data)
			emit(d, h, &hasToolCalls)
		case "google":
			d := parseGemini(data)
			emit(d, h, &hasToolCalls)
		default:
			d := parseOpenAI(data)
			emit(d, h, &hasToolCalls)
		}
	}
	if e := scanner.Err(); e != nil && e != io.EOF {
		return hasToolCalls, e
	}
	return hasToolCalls, nil
}

func emit(d sseDelta, h streamHandlers, hasToolCalls *bool) {
	if d.Reasoning != "" && h.onReasoning != nil {
		h.onReasoning(d.Reasoning)
	}
	if d.Text != "" && h.onText != nil {
		h.onText(d.Text)
	}
	for _, tc := range d.ToolCalls {
		*hasToolCalls = true
		if h.onToolCall != nil {
			h.onToolCall(tc)
		}
	}
}

func parseOpenAI(data string) sseDelta {
	var p struct {
		Choices []struct {
			Delta struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
				Reasoning        string `json:"reasoning"`
				ToolCalls        []struct {
					Index    int    `json:"index"`
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if json.Unmarshal([]byte(data), &p) != nil || len(p.Choices) == 0 {
		return sseDelta{}
	}
	delta := p.Choices[0].Delta
	reasoning := delta.ReasoningContent
	if reasoning == "" {
		reasoning = delta.Reasoning
	}
	out := sseDelta{Text: delta.Content, Reasoning: reasoning}
	for _, tc := range delta.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, toolDelta{
			Index: tc.Index,
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Args:  tc.Function.Arguments,
		})
	}
	return out
}

func parseAnthropic(data string) sseDelta {
	var p struct {
		Type         string `json:"type"`
		Index        int    `json:"index"`
		ContentBlock struct {
			Type string `json:"type"`
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"content_block"`
		Delta struct {
			Type        string `json:"type"`
			Text        string `json:"text"`
			Thinking    string `json:"thinking"`
			PartialJSON string `json:"partial_json"`
		} `json:"delta"`
	}
	if json.Unmarshal([]byte(data), &p) != nil {
		return sseDelta{}
	}
	switch p.Type {
	case "content_block_start":
		if p.ContentBlock.Type == "tool_use" {
			return sseDelta{ToolCalls: []toolDelta{{
				Index: p.Index,
				ID:    p.ContentBlock.ID,
				Name:  p.ContentBlock.Name,
			}}}
		}
	case "content_block_delta":
		switch p.Delta.Type {
		case "text_delta":
			return sseDelta{Text: p.Delta.Text}
		case "thinking_delta":
			return sseDelta{Reasoning: p.Delta.Thinking}
		case "input_json_delta":
			return sseDelta{ToolCalls: []toolDelta{{Index: p.Index, Args: p.Delta.PartialJSON}}}
		}
	}
	return sseDelta{}
}

func anthropicIsStop(data string) bool {
	var p struct {
		Type string `json:"type"`
	}
	if json.Unmarshal([]byte(data), &p) != nil {
		return false
	}
	return p.Type == "message_stop"
}

func parseGemini(data string) sseDelta {
	var p struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text         string `json:"text"`
					FunctionCall *struct {
						Name string          `json:"name"`
						Args json.RawMessage `json:"args"`
					} `json:"functionCall"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if json.Unmarshal([]byte(data), &p) != nil || len(p.Candidates) == 0 {
		return sseDelta{}
	}
	var out sseDelta
	var sb strings.Builder
	idx := 0
	for _, part := range p.Candidates[0].Content.Parts {
		if part.Text != "" {
			sb.WriteString(part.Text)
		}
		if part.FunctionCall != nil {
			args := ""
			if len(part.FunctionCall.Args) > 0 {
				args = string(part.FunctionCall.Args)
			}
			out.ToolCalls = append(out.ToolCalls, toolDelta{
				Index: idx,
				Name:  part.FunctionCall.Name,
				Args:  args,
			})
			idx++
		}
	}
	out.Text = sb.String()
	return out
}

func scanUsage(provider, data string, u *usage) {
	switch provider {
	case "anthropic":
		var p struct {
			Message struct {
				Usage struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal([]byte(data), &p) != nil {
			return
		}
		if p.Message.Usage.InputTokens > 0 {
			u.prompt = p.Message.Usage.InputTokens
		}
		if p.Usage.InputTokens > 0 {
			u.prompt = p.Usage.InputTokens
		}
		if p.Message.Usage.OutputTokens > u.completion {
			u.completion = p.Message.Usage.OutputTokens
		}
		if p.Usage.OutputTokens > u.completion {
			u.completion = p.Usage.OutputTokens
		}
	case "google":
		var p struct {
			UsageMetadata struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
			} `json:"usageMetadata"`
		}
		if json.Unmarshal([]byte(data), &p) != nil {
			return
		}
		if p.UsageMetadata.PromptTokenCount > 0 {
			u.prompt = p.UsageMetadata.PromptTokenCount
		}
		if p.UsageMetadata.CandidatesTokenCount > u.completion {
			u.completion = p.UsageMetadata.CandidatesTokenCount
		}
	default:
		var p struct {
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal([]byte(data), &p) != nil || p.Usage == nil {
			return
		}
		if p.Usage.PromptTokens > 0 {
			u.prompt = p.Usage.PromptTokens
		}
		if p.Usage.CompletionTokens > u.completion {
			u.completion = p.Usage.CompletionTokens
		}
	}
}
