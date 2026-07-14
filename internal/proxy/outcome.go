package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"astreoGateway/internal/routing"
)

type Attempt struct {
	ProviderSlug string `json:"provider_slug"`
	ModelName    string `json:"model_name"`
	KeyID        string `json:"key_id"`
	Status       int    `json:"status"`
	FailClass    string `json:"fail_class"`
	DurationMs   int64  `json:"duration_ms"`
}

type Outcome struct {
	ProviderSlug     string
	ModelName        string
	AliasName        string
	Status           int
	Attempts         []Attempt
	TokensPrompt     int
	TokensCompletion int
	ErrorClass       string
	Stream           bool
}

func (c failClass) String() string {
	switch c {
	case failDown:
		return "down"
	case failRate:
		return "rate_limited"
	case failModelMissing:
		return "model_missing"
	case failAuth:
		return "auth"
	case failClient:
		return "client"
	default:
		return ""
	}
}

func parseUsage(body []byte) (prompt, completion int) {
	var m struct {
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &m); err != nil || m.Usage == nil {
		return 0, 0
	}
	return m.Usage.PromptTokens, m.Usage.CompletionTokens
}

type usageCaptureWriter struct {
	http.ResponseWriter
	buf        []byte
	prompt     int
	completion int
}

func (u *usageCaptureWriter) Write(p []byte) (int, error) {
	n, err := u.ResponseWriter.Write(p)
	u.buf = append(u.buf, p...)
	for {
		i := bytes.IndexByte(u.buf, '\n')
		if i < 0 {
			break
		}
		line := string(bytes.TrimSpace(u.buf[:i]))
		u.buf = u.buf[i+1:]
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var chunk struct {
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal([]byte(data), &chunk) == nil && chunk.Usage != nil {
			u.prompt = chunk.Usage.PromptTokens
			u.completion = chunk.Usage.CompletionTokens
		}
	}
	return n, err
}

func (u *usageCaptureWriter) Flush() {
	if f, ok := u.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (u *usageCaptureWriter) Unwrap() http.ResponseWriter {
	return u.ResponseWriter
}

func proxyErrorStatus(err error) int {
	switch err {
	case routing.ErrProviderNotFound, routing.ErrUnknownModel:
		return http.StatusNotFound
	case routing.ErrAliasNoTargets, routing.ErrNoAPIKey:
		return http.StatusServiceUnavailable
	case routing.ErrProtocolMismatch:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func elapsedMs(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

// ensureStreamIncludeUsage injects stream_options.include_usage for OpenAI-compatible
// upstreams so the final SSE chunk carries usage for the access log. Only applied
// to protocol=openai request bodies (Anthropic/Gemini translators force usage emission
// themselves without mutating client-facing semantics of the upstream protocol).
func ensureStreamIncludeUsage(body []byte) []byte {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return body
	}
	opts := map[string]any{"include_usage": true}
	if raw, ok := m["stream_options"]; ok {
		var existing map[string]any
		if json.Unmarshal(raw, &existing) == nil {
			for k, v := range existing {
				opts[k] = v
			}
			opts["include_usage"] = true
		}
	}
	b, err := json.Marshal(opts)
	if err != nil {
		return body
	}
	m["stream_options"] = b
	out, err := json.Marshal(m)
	if err != nil {
		return body
	}
	return out
}

func resultStatus(r forwardResult) int {
	if r.err != nil && r.wrote {
		if r.status >= 400 {
			return r.status
		}
		return http.StatusBadGateway
	}
	if r.status != 0 {
		return r.status
	}
	if r.err != nil {
		return http.StatusBadGateway
	}
	return 0
}

func appendAttempt(out *Outcome, slug, model, keyID string, r forwardResult, started time.Time) {
	st := resultStatus(r)
	fc := classifyResult(r)
	if r.err != nil && r.wrote && fc == failNone {
		fc = failDown
	}
	out.Attempts = append(out.Attempts, Attempt{
		ProviderSlug: slug,
		ModelName:    model,
		KeyID:        keyID,
		Status:       st,
		FailClass:    fc.String(),
		DurationMs:   elapsedMs(started),
	})
	out.Status = st
	if fc != failNone {
		out.ErrorClass = fc.String()
	} else {
		out.ErrorClass = ""
	}
	out.ProviderSlug = slug
	out.ModelName = model
	if r.status == 200 && r.err == nil && len(r.body) > 0 {
		p, c := parseUsage(r.body)
		out.TokensPrompt = p
		out.TokensCompletion = c
	}
}

// finalizeStreamOutcome normalizes Outcome after a stream that already wrote headers.
// The client may have seen HTTP 200 SSE; the log records the logical failure status.
func finalizeStreamOutcome(out *Outcome, r forwardResult) {
	if out == nil || r.err == nil {
		return
	}
	st := resultStatus(r)
	out.Status = st
	if out.ErrorClass == "" {
		out.ErrorClass = "down"
	}
	if n := len(out.Attempts); n > 0 {
		out.Attempts[n-1].Status = st
		if out.Attempts[n-1].FailClass == "" {
			out.Attempts[n-1].FailClass = "down"
		}
	}
}
