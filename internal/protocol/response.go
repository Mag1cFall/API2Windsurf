package protocol

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"
)

const (
	respFieldBotID     = 1
	respFieldTimestamp = 2
	respFieldText      = 3
	respFieldSeq       = 4
	respFieldEOT       = 5
	respFieldToolCall  = 6
	respFieldThinking  = 9
	respFieldOutputID  = 15
	respFieldThinkID   = 16
	respFieldRequestID = 17

	tsFieldSeconds = 1
	tsFieldNanos   = 2

	toolFieldID   = 1
	toolFieldName = 2
	toolFieldArgs = 3

	eotText      = 4
	eotToolCalls = 10
)

func timestampSubmessage() []byte {
	now := time.Now()
	var ts []byte
	ts = appendVarintField(ts, tsFieldSeconds, uint64(now.Unix()))
	ts = appendVarintField(ts, tsFieldNanos, uint64(now.Nanosecond()))
	return ts
}

func EncodeTextFrame(botID, text string, seq uint64) []byte {
	var body []byte
	body = appendStringField(body, respFieldBotID, botID)
	body = appendBytesField(body, respFieldTimestamp, timestampSubmessage())
	if text != "" {
		body = appendStringField(body, respFieldText, text)
	}
	body = appendVarintField(body, respFieldSeq, seq)
	return frame(body, 0)
}

func EncodeThinkingFrame(botID, thinking, thinkingID string, seq uint64) []byte {
	var body []byte
	body = appendStringField(body, respFieldBotID, botID)
	body = appendBytesField(body, respFieldTimestamp, timestampSubmessage())
	if thinking != "" {
		body = appendStringField(body, respFieldThinking, thinking)
	}
	body = appendVarintField(body, respFieldSeq, seq)
	if thinkingID != "" {
		body = appendStringField(body, respFieldThinkID, thinkingID)
	}
	return frame(body, 0)
}

func EncodeToolCallFrame(botID, id, name, argsJSON string, seq uint64) []byte {
	var sub []byte
	if id != "" {
		sub = appendStringField(sub, toolFieldID, id)
	}
	if name != "" {
		sub = appendStringField(sub, toolFieldName, name)
	}
	if argsJSON != "" {
		sub = appendStringField(sub, toolFieldArgs, argsJSON)
	}
	var body []byte
	body = appendStringField(body, respFieldBotID, botID)
	body = appendBytesField(body, respFieldTimestamp, timestampSubmessage())
	body = appendBytesField(body, respFieldToolCall, sub)
	body = appendVarintField(body, respFieldSeq, seq)
	return frame(body, 0)
}

func EncodeEndOfTurnFrame(botID string, seq uint64, withToolCalls bool) []byte {
	eot := uint64(eotText)
	if withToolCalls {
		eot = eotToolCalls
	}
	var body []byte
	body = appendStringField(body, respFieldBotID, botID)
	body = appendBytesField(body, respFieldTimestamp, timestampSubmessage())
	body = appendVarintField(body, respFieldSeq, seq)
	body = appendVarintField(body, respFieldEOT, eot)
	return frame(body, 0)
}

func EncodeEndOfStreamSuccess() []byte {
	return frame([]byte("{}"), flagEndStream)
}

type endOfStreamError struct {
	Error endOfStreamErrorBody `json:"error"`
}

type endOfStreamErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func EncodeEndOfStreamError(code, message string) []byte {
	body, _ := json.Marshal(endOfStreamError{Error: endOfStreamErrorBody{Code: code, Message: message}})
	return frame(body, flagEndStream)
}

func AttachStreamIDs(frameBytes []byte, outputID, requestID string) []byte {
	if len(frameBytes) <= envelopeHeaderLen {
		return frameBytes
	}
	flag := frameBytes[0]
	if flag&flagEndStream != 0 {
		return frameBytes
	}
	payload := append([]byte{}, frameBytes[envelopeHeaderLen:]...)
	payload = appendStringField(payload, respFieldOutputID, outputID)
	payload = appendStringField(payload, respFieldRequestID, requestID)
	return frame(payload, flag)
}

func NewBotID() string {
	return "bot-" + randomHex(6)
}

func NewID() string {
	return randomHex(16)
}

func randomHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
