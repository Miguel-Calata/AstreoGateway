package anthropic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"astreoGateway/internal/protocol/core"
)

const ProtocolName = "anthropic"

// Protocol implements protocol.Protocol for Anthropic's Messages API.
type Protocol struct{}

func New() *Protocol { return &Protocol{} }

func (p *Protocol) Name() string { return ProtocolName }

// --- Models discovery ---

func (p *Protocol) ModelsURL(base string) string {
	return BuildMessagesURL(base)
}

func (p *Protocol) ModelsAuth(req *http.Request, apiKey string) {
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", VersionHeader)
}

func (p *Protocol) ParseModels(body []byte) ([]core.ModelEntry, error) {
	resp, err := core.ParseModelsResponse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// --- Chat ---

func (p *Protocol) ChatURL(base, model string, stream bool) (string, bool) {
	_ = model
	_ = stream
	return BuildMessagesURL(base), false
}

func (p *Protocol) AuthHeaders(req *http.Request, apiKey string) {
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", VersionHeader)
}

func (p *Protocol) SupportsEmbeddings() bool { return false }

// --- Translation ---

func (p *Protocol) TranslateRequest(body []byte) ([]byte, *core.ChatRequest, error) {
	var oaiReq core.ChatRequest
	if err := json.Unmarshal(body, &oaiReq); err != nil {
		return nil, nil, fmt.Errorf("parse openai request: %w", err)
	}
	antReq, err := OpenAIToAnthropic(&oaiReq, oaiReq.Model)
	if err != nil {
		return nil, nil, err
	}
	payload, err := json.Marshal(antReq)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal anthropic request: %w", err)
	}
	return payload, &oaiReq, nil
}

func (p *Protocol) TranslateResponse(upstream []byte, model string) ([]byte, error) {
	var antResp MessagesResponse
	if err := json.Unmarshal(upstream, &antResp); err != nil {
		return nil, fmt.Errorf("decode anthropic response: %w", err)
	}
	oaiResp, err := AnthropicToOpenAI(&antResp, model)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(oaiResp)
	if err != nil {
		return nil, fmt.Errorf("marshal openai response: %w", err)
	}
	return out, nil
}

func (p *Protocol) TranslateStream(r io.Reader, w http.ResponseWriter, model string, includeUsage bool, logger *slog.Logger) error {
	return TranslateStream(r, w, model, includeUsage, logger)
}
