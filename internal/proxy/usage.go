package proxy

import (
	"sync"
	"time"
)

type UsageRecord struct {
	At           string `json:"at"`
	Model        string `json:"model"`
	Provider     string `json:"provider"`
	PromptTokens int    `json:"prompt_tokens"`
	OutputTokens int    `json:"output_tokens"`
	TotalTokens  int    `json:"total_tokens"`
	DurationMs   int64  `json:"duration_ms"`
	Status       string `json:"status"`
	ErrorDetail  string `json:"error_detail,omitempty"`
}

type UsageSummary struct {
	TotalRequests int `json:"total_requests"`
	TotalTokens   int `json:"total_tokens"`
	ErrorCount    int `json:"error_count"`
}

type UsageTracker struct {
	mu      sync.Mutex
	records []UsageRecord
	summary UsageSummary
}

func NewUsageTracker() *UsageTracker { return &UsageTracker{} }

const maxRetainedRecords = 200

func (t *UsageTracker) Record(r UsageRecord) {
	if r.At == "" {
		r.At = time.Now().Format(time.RFC3339)
	}
	r.TotalTokens = r.PromptTokens + r.OutputTokens
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records = append(t.records, r)
	if len(t.records) > maxRetainedRecords {
		t.records = t.records[len(t.records)-maxRetainedRecords:]
	}
	t.summary.TotalRequests++
	t.summary.TotalTokens += r.TotalTokens
	if r.Status != "ok" {
		t.summary.ErrorCount++
	}
}

func (t *UsageTracker) Summary() UsageSummary {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.summary
}

func (t *UsageTracker) Recent(n int) []UsageRecord {
	t.mu.Lock()
	defer t.mu.Unlock()
	if n <= 0 || n > len(t.records) {
		n = len(t.records)
	}
	out := make([]UsageRecord, n)
	for i := range n {
		out[i] = t.records[len(t.records)-1-i]
	}
	return out
}
