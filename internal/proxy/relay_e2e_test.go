package proxy

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"api2windsurf/internal/protocol"
)

func endpointLive(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func buildCascadeBody(t *testing.T, system, userText string) []byte {
	t.Helper()
	var msg []byte
	msg = appendVarintFieldTest(msg, 2, 1)
	msg = appendStringFieldTest(msg, 3, userText)

	var body []byte
	if system != "" {
		body = appendStringFieldTest(body, 2, system)
	}
	body = appendBytesFieldTest(body, 3, msg)
	body = appendStringFieldTest(body, 21, "cascade")
	return body
}

func appendVarintFieldTest(dst []byte, field, value uint64) []byte {
	dst = appendUvarint(dst, field<<3)
	return appendUvarint(dst, value)
}

func appendStringFieldTest(dst []byte, field uint64, s string) []byte {
	return appendBytesFieldTest(dst, field, []byte(s))
}

func appendBytesFieldTest(dst []byte, field uint64, data []byte) []byte {
	dst = appendUvarint(dst, field<<3|2)
	dst = appendUvarint(dst, uint64(len(data)))
	return append(dst, data...)
}

func appendUvarint(dst []byte, v uint64) []byte {
	for v >= 0x80 {
		dst = append(dst, byte(v)|0x80)
		v >>= 7
	}
	return append(dst, byte(v))
}

func decodeText(raw []byte) string {
	var sb strings.Builder
	pos := 0
	for pos+5 <= len(raw) {
		flag := raw[pos]
		ln := int(uint32(raw[pos+1])<<24 | uint32(raw[pos+2])<<16 | uint32(raw[pos+3])<<8 | uint32(raw[pos+4]))
		pos += 5
		if pos+ln > len(raw) {
			break
		}
		payload := raw[pos : pos+ln]
		pos += ln
		if flag&0x02 != 0 {
			continue
		}
		for _, f := range protocol.ScanForTest(payload) {
			if f.Num == 3 && f.Wire == 2 {
				sb.Write(f.Data)
			}
		}
	}
	return sb.String()
}

func TestRelayEndToEndLocalEndpoint(t *testing.T) {
	const addr = "127.0.0.1:8317"
	if !endpointLive(addr) {
		t.Skipf("local endpoint %s not reachable, skipping live test", addr)
	}

	body := buildCascadeBody(t, "You are a test bot. Reply with exactly: PONG", "Say PONG")
	rec := httptest.NewRecorder()

	up := Upstream{Provider: "openai", BaseURL: "http://127.0.0.1:8317", APIKey: "123", Model: "claude-opus-4.7"}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result := Relay(ctx, rec, http.DefaultClient, up, body, RelayOptions{ShowReasoning: true})
	if !result.Served {
		t.Fatalf("relay did not serve: %v", result.Err)
	}
	if result.Err != nil {
		t.Fatalf("relay error: %v", result.Err)
	}
	text := decodeText(rec.Body.Bytes())
	if strings.TrimSpace(text) == "" {
		t.Fatalf("no text decoded from cascade frames")
	}
	t.Logf("decoded cascade reply: %q (prompt=%d completion=%d)", text, result.Prompt, result.Completion)
	if !strings.Contains(strings.ToUpper(text), "PONG") {
		t.Fatalf("expected reply to contain PONG, got %q", text)
	}
}
