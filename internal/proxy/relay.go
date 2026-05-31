package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"api2windsurf/internal/protocol"
)

type RelayOptions struct {
	ShowReasoning     bool
	MaxTokensOverride int
}

type RelayResult struct {
	Served     bool
	Provider   string
	Model      string
	Prompt     int
	Completion int
	Err        error
}

func Relay(ctx context.Context, w http.ResponseWriter, client *http.Client, up Upstream, cascadeBody []byte, opts RelayOptions) RelayResult {
	req, err := protocol.DecodeChatRequest(cascadeBody)
	if err != nil {
		return RelayResult{Served: false, Err: err}
	}
	if opts.MaxTokensOverride > 0 {
		req.MaxTokens = uint64(opts.MaxTokensOverride)
	}
	if strings.TrimSpace(up.Model) == "" {
		up.Model = req.Model
	}
	if strings.TrimSpace(up.Model) == "" {
		return RelayResult{Served: false, Err: fmt.Errorf("no model configured and IDE sent none")}
	}
	if client == nil {
		client = http.DefaultClient
	}

	httpReq, err := BuildUpstreamRequest(up, req)
	if err != nil {
		return RelayResult{Served: false, Err: fmt.Errorf("build upstream request: %w", err)}
	}
	httpReq = httpReq.WithContext(ctx)

	resp, err := client.Do(httpReq)
	if err != nil {
		return writeErrorStream(w, up, "unavailable", fmt.Sprintf("upstream unreachable: %v", err))
	}
	if resp.StatusCode >= 400 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		resp.Body.Close()
		msg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
		return writeErrorStream(w, up, "upstream_error", msg)
	}
	defer resp.Body.Close()

	return streamToCascade(w, resp.Body, up, opts)
}

func streamToCascade(w http.ResponseWriter, body io.Reader, up Upstream, opts RelayOptions) RelayResult {
	w.Header().Set("Content-Type", "application/connect+proto")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)

	botID := protocol.NewBotID()
	outputID := protocol.NewID()
	requestID := protocol.NewID()
	thinkingID := "thinking-" + protocol.NewID()
	var seq uint64
	first := true

	writeFrame := func(b []byte) {
		if first {
			b = protocol.AttachStreamIDs(b, outputID, requestID)
			first = false
		}
		_, _ = w.Write(b)
		if flusher != nil {
			flusher.Flush()
		}
	}

	acc := newToolAccumulator()
	u := &usage{}
	h := streamHandlers{
		onText: func(text string) {
			if text == "" {
				return
			}
			writeFrame(protocol.EncodeTextFrame(botID, text, seq))
			seq++
		},
		onReasoning: func(text string) {
			if text == "" || !opts.ShowReasoning {
				return
			}
			writeFrame(protocol.EncodeThinkingFrame(botID, text, thinkingID, seq))
			seq++
		},
		onToolCall: acc.add,
	}

	hasToolCalls, streamErr := streamUpstream(body, up.Provider, h, u)
	if streamErr != nil {
		writeFrame(protocol.EncodeEndOfStreamError("internal", "stream parse failed: "+streamErr.Error()))
		return RelayResult{Served: true, Provider: up.Provider, Model: up.Model, Prompt: u.prompt, Completion: u.completion, Err: streamErr}
	}

	if hasToolCalls {
		acc.flush(botID, &seq, writeFrame)
		writeFrame(protocol.EncodeEndOfTurnFrame(botID, seq, true))
		seq++
		writeFrame(protocol.EncodeEndOfStreamSuccess())
	} else {
		writeFrame(protocol.EncodeEndOfTurnFrame(botID, seq, false))
		seq++
		writeFrame(protocol.EncodeEndOfStreamSuccess())
	}
	return RelayResult{Served: true, Provider: up.Provider, Model: up.Model, Prompt: u.prompt, Completion: u.completion}
}

func writeErrorStream(w http.ResponseWriter, up Upstream, code, message string) RelayResult {
	w.Header().Set("Content-Type", "application/connect+proto")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(protocol.EncodeEndOfStreamError(code, message))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return RelayResult{Served: true, Provider: up.Provider, Model: up.Model, Err: fmt.Errorf("%s", message)}
}

type toolAccumulator struct {
	order []int
	calls map[int]*accumulatedCall
}

type accumulatedCall struct {
	id   string
	name string
	args strings.Builder
}

func newToolAccumulator() *toolAccumulator {
	return &toolAccumulator{calls: map[int]*accumulatedCall{}}
}

func (a *toolAccumulator) add(tc toolDelta) {
	c, ok := a.calls[tc.Index]
	if !ok {
		c = &accumulatedCall{}
		a.calls[tc.Index] = c
		a.order = append(a.order, tc.Index)
	}
	if tc.ID != "" {
		c.id = tc.ID
	}
	if tc.Name != "" {
		c.name = tc.Name
	}
	if tc.Args != "" {
		c.args.WriteString(tc.Args)
	}
}

func (a *toolAccumulator) flush(botID string, seq *uint64, writeFrame func([]byte)) {
	for _, idx := range a.order {
		c := a.calls[idx]
		args := c.args.String()
		if args == "" {
			args = "{}"
		}
		writeFrame(protocol.EncodeToolCallFrame(botID, c.id, c.name, args, *seq))
		*seq++
	}
}

const RelayTimeout = 3 * time.Minute
