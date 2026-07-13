package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"astreoGateway/internal/model"
	"astreoGateway/internal/protocol/anthropic"
	"astreoGateway/internal/protocol/openai"
)

func (p *Proxy) forwardAnthropic(ctx context.Context, prov model.Provider, apiKey model.APIKey, body []byte, isStream bool, w http.ResponseWriter) forwardResult {
	var oaiReq openai.ChatRequest
	if err := json.Unmarshal(body, &oaiReq); err != nil {
		b, _ := json.Marshal(map[string]any{
			"error": map[string]string{"message": "invalid request body", "type": "invalid_request"},
		})
		return forwardResult{status: http.StatusBadRequest, body: b}
	}

	antReq, err := anthropic.OpenAIToAnthropic(&oaiReq, oaiReq.Model)
	if err != nil {
		b, _ := json.Marshal(map[string]any{
			"error": map[string]string{"message": err.Error(), "type": "invalid_request"},
		})
		return forwardResult{status: http.StatusBadRequest, body: b}
	}

	payload, err := json.Marshal(antReq)
	if err != nil {
		return forwardResult{err: fmt.Errorf("marshal anthropic request: %w", err)}
	}

	upstreamURL := anthropic.BuildMessagesURL(prov.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(payload))
	if err != nil {
		return forwardResult{err: fmt.Errorf("new request: %w", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey.Value)
	req.Header.Set("anthropic-version", anthropic.VersionHeader)
	for k, v := range prov.Headers {
		req.Header.Set(k, v)
	}

	resp, err := p.httpC.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return forwardResult{status: 504, err: ErrUpstreamTimeout}
		}
		return forwardResult{err: fmt.Errorf("upstream request: %w", err)}
	}
	defer resp.Body.Close()

	if isRetryable(resp.StatusCode) {
		p.pool.MarkCooldown(prov.ID, apiKey.ID, p.keyCooldown)
	}

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return forwardResult{status: resp.StatusCode, body: wrapAnthropicError(b, resp.StatusCode), header: resp.Header}
	}

	if isStream {
		includeUsage := oaiReq.StreamOpts != nil && oaiReq.StreamOpts.IncludeUsage
		if err := anthropic.TranslateStream(resp.Body, w, oaiReq.Model, includeUsage, p.logger); err != nil {
			return forwardResult{status: 200, err: err, wrote: true}
		}
		return forwardResult{status: 200, wrote: true}
	}

	var antResp anthropic.MessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&antResp); err != nil {
		return forwardResult{err: fmt.Errorf("decode anthropic response: %w", err)}
	}
	oaiResp, err := anthropic.AnthropicToOpenAI(&antResp, oaiReq.Model)
	if err != nil {
		return forwardResult{err: err}
	}
	out, err := json.Marshal(oaiResp)
	if err != nil {
		return forwardResult{err: err}
	}
	h := make(http.Header)
	h.Set("Content-Type", "application/json")
	return forwardResult{status: 200, body: out, header: h}
}

func wrapAnthropicError(body []byte, status int) []byte {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err == nil {
		if errObj, ok := raw["error"].(map[string]any); ok {
			msg, _ := errObj["message"].(string)
			if msg == "" {
				msg = string(body)
			}
			b, _ := json.Marshal(map[string]any{
				"error": map[string]string{"message": msg, "type": "upstream_error"},
			})
			return b
		}
	}
	msg := string(body)
	if msg == "" {
		msg = fmt.Sprintf("upstream status %d", status)
	}
	b, _ := json.Marshal(map[string]any{
		"error": map[string]string{"message": msg, "type": "upstream_error"},
	})
	return b
}
