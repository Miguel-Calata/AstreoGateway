package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"astreoGateway/internal/protocol/core"
)

const ProtocolName = "openai"

// Protocol implements protocol.Protocol for OpenAI-compatible upstreams.
// All translation methods are passthrough: the wire format is identical.
type Protocol struct{}

func New() *Protocol { return &Protocol{} }

func (p *Protocol) Name() string { return ProtocolName }

// --- Models discovery ---

func (p *Protocol) ModelsURL(base string) string {
	return BuildModelsURL(base)
}

func (p *Protocol) ModelsAuth(req *http.Request, apiKey string) {
	req.Header.Set("Authorization", "Bearer "+apiKey)
}

func (p *Protocol) ParseModels(body []byte) ([]core.ModelEntry, error) {
	resp, err := ParseModelsResponse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// --- Chat ---

func (p *Protocol) ChatURL(base, model string, stream bool) (string, bool) {
	_ = model
	_ = stream
	return BuildChatCompletionsURL(base), false
}

func (p *Protocol) AuthHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Authorization", "Bearer "+apiKey)
}

func (p *Protocol) SupportsEmbeddings() bool { return true }

// --- Translation (passthrough) ---

func (p *Protocol) TranslateRequest(body []byte) ([]byte, *core.ChatRequest, error) {
	var req core.ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, nil, fmt.Errorf("parse openai request: %w", err)
	}
	return body, &req, nil
}

func (p *Protocol) TranslateResponse(upstream []byte, model string) ([]byte, error) {
	return upstream, nil
}

func (p *Protocol) TranslateStream(r io.Reader, w http.ResponseWriter, model string, includeUsage bool, logger *slog.Logger) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, canFlush := w.(http.Flusher)
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return werr
			}
			if canFlush {
				flusher.Flush()
			}
		}
		if err != nil {
			if err != io.EOF {
				return err
			}
			return nil
		}
	}
}
