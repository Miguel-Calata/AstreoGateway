package gemini

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"astreoGateway/internal/protocol/core"
)

const ProtocolName = "gemini"

type Protocol struct{}

func New() *Protocol { return &Protocol{} }

func (p *Protocol) Name() string { return ProtocolName }

// --- Models discovery ---

func (p *Protocol) ModelsURL(base string) string {
	return BuildModelsURL(base)
}

func (p *Protocol) ModelsAuth(req *http.Request, apiKey string) {
	req.Header.Set("x-goog-api-key", apiKey)
}

func (p *Protocol) ParseModels(body []byte) ([]core.ModelEntry, error) {
	var resp ListModelsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode gemini models: %w", err)
	}
	out := make([]core.ModelEntry, 0, len(resp.Models))
	for _, m := range resp.Models {
		if !containsGenerateContent(m.SupportedGenerationMethods) {
			continue
		}
		id := strings.TrimPrefix(m.Name, "models/")
		if id == "" {
			continue
		}
		out = append(out, core.ModelEntry{
			ID:      id,
			Object:  "model",
			OwnedBy: m.DisplayName,
		})
	}
	return out, nil
}

func containsGenerateContent(methods []string) bool {
	for _, m := range methods {
		if m == "generateContent" {
			return true
		}
	}
	return false
}

// --- Chat ---

func (p *Protocol) ChatURL(base, model string, stream bool) (string, bool) {
	if stream {
		return BuildStreamGenerateContentURL(base, model), true
	}
	return BuildGenerateContentURL(base, model), true
}

func (p *Protocol) AuthHeaders(req *http.Request, apiKey string) {
	req.Header.Set("x-goog-api-key", apiKey)
}

func (p *Protocol) SupportsEmbeddings() bool { return false }

// --- Translation ---

func (p *Protocol) TranslateRequest(body []byte) ([]byte, *core.ChatRequest, error) {
	var oaiReq core.ChatRequest
	if err := json.Unmarshal(body, &oaiReq); err != nil {
		return nil, nil, fmt.Errorf("parse openai request: %w", err)
	}
	gemReq, err := OpenAIToGemini(&oaiReq, oaiReq.Model)
	if err != nil {
		return nil, nil, err
	}
	payload, err := json.Marshal(gemReq)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal gemini request: %w", err)
	}
	return payload, &oaiReq, nil
}

func (p *Protocol) TranslateResponse(upstream []byte, model string) ([]byte, error) {
	var gemResp GenerateContentResponse
	if err := json.Unmarshal(upstream, &gemResp); err != nil {
		return nil, fmt.Errorf("decode gemini response: %w", err)
	}
	oaiResp, err := GeminiToOpenAI(&gemResp, model)
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
