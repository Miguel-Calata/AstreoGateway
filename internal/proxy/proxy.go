package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"astreoGateway/internal/keypool"
	"astreoGateway/internal/model"
	"astreoGateway/internal/protocol"
	"astreoGateway/internal/routing"
)

var ErrUpstreamTimeout = fmt.Errorf("upstream timeout")

type forwardResult struct {
	status int
	body   []byte
	header http.Header
	err    error
	wrote  bool
}

type Proxy struct {
	pool        *keypool.Pool
	httpC       *http.Client
	logger      *slog.Logger
	keyCooldown time.Duration
}

func New(pool *keypool.Pool, timeout, keyCooldown time.Duration, logger *slog.Logger) *Proxy {
	return &Proxy{
		pool:        pool,
		httpC:       &http.Client{Timeout: timeout},
		logger:      logger,
		keyCooldown: keyCooldown,
	}
}

func (p *Proxy) ChatCompletions(ctx context.Context, w http.ResponseWriter, sel *routing.Selector, directive string, body []byte, isStream bool) {
	resolved, err := sel.Resolve(directive)
	if err != nil {
		writeProxyError(w, err)
		return
	}

	fwdBody, rerr := rewriteModel(body, resolved.ModelName)
	if rerr != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", rerr.Error())
		return
	}

	if resolved.AliasRouting == "failover" {
		p.forwardWithFailover(ctx, w, resolved, sel, directive, fwdBody, isStream)
		return
	}

	result := p.forwardOnce(ctx, resolved.Provider, resolved.APIKey, fwdBody, isStream, w)
	if result.err != nil {
		writeJSONError(w, http.StatusBadGateway, "upstream_error", result.err.Error())
		return
	}
	if result.wrote {
		return
	}
	p.writeJSONResponse(w, result)
}

func (p *Proxy) forwardOnce(ctx context.Context, prov model.Provider, apiKey model.APIKey, body []byte, isStream bool, w http.ResponseWriter) forwardResult {
	proto := protocol.Get(prov.Protocol)
	if proto == nil {
		return forwardResult{status: http.StatusBadRequest, err: fmt.Errorf("unsupported protocol: %s", prov.Protocol)}
	}
	return p.forward(ctx, proto, prov, apiKey, body, isStream, w)
}

func (p *Proxy) forward(ctx context.Context, proto protocol.Protocol, prov model.Provider, apiKey model.APIKey, body []byte, isStream bool, w http.ResponseWriter) forwardResult {
	upstreamBody, parsedReq, err := proto.TranslateRequest(body)
	if err != nil {
		b, _ := json.Marshal(map[string]any{
			"error": map[string]string{"message": err.Error(), "type": "invalid_request"},
		})
		return forwardResult{status: http.StatusBadRequest, body: b}
	}

	chatURL, _ := proto.ChatURL(prov.BaseURL, parsedReq.Model, isStream)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(upstreamBody))
	if err != nil {
		return forwardResult{err: fmt.Errorf("new request: %w", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	proto.AuthHeaders(req, apiKey.Value)
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
		return forwardResult{status: resp.StatusCode, body: wrapUpstreamError(b, resp.StatusCode), header: resp.Header}
	}

	if isStream {
		includeUsage := parsedReq.StreamOpts != nil && parsedReq.StreamOpts.IncludeUsage
		if err := proto.TranslateStream(resp.Body, w, parsedReq.Model, includeUsage, p.logger); err != nil {
			return forwardResult{status: 200, err: err, wrote: true}
		}
		return forwardResult{status: 200, wrote: true}
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return forwardResult{status: resp.StatusCode, err: fmt.Errorf("read body: %w", err)}
	}
	translated, err := proto.TranslateResponse(respBody, parsedReq.Model)
	if err != nil {
		return forwardResult{err: err}
	}
	h := resp.Header.Clone()
	h.Set("Content-Type", "application/json")
	return forwardResult{status: 200, body: translated, header: h}
}

func (p *Proxy) writeJSONResponse(w http.ResponseWriter, result forwardResult) {
	if result.header != nil {
		for _, h := range []string{"Content-Type", "X-Request-Id"} {
			if v := result.header.Get(h); v != "" {
				w.Header().Set(h, v)
			}
		}
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(result.status)
	w.Write(result.body)
}

func (p *Proxy) bufferJSON(resp *http.Response) forwardResult {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return forwardResult{status: resp.StatusCode, err: fmt.Errorf("read body: %w", err)}
	}
	return forwardResult{status: resp.StatusCode, body: body, header: resp.Header}
}

func (p *Proxy) forwardWithFailover(ctx context.Context, w http.ResponseWriter, resolved *routing.Resolved, sel *routing.Selector, directive string, body []byte, isStream bool) {
	alias, err := sel.LookupAlias(directive)
	if err != nil || alias == nil {
		result := p.forwardOnce(ctx, resolved.Provider, resolved.APIKey, body, isStream, w)
		if result.err != nil {
			writeJSONError(w, http.StatusBadGateway, "upstream_error", result.err.Error())
			return
		}
		if !result.wrote {
			p.writeJSONResponse(w, result)
		}
		return
	}

	tried := make(map[string]bool)
	tried[resolved.Provider.ID+":"+resolved.ModelName] = true

	result := p.forwardOnce(ctx, resolved.Provider, resolved.APIKey, body, isStream, w)
	if result.wrote {
		return
	}
	if !isRetryable(result.status) {
		p.writeJSONResponse(w, result)
		return
	}

	for {
		nextTarget, nextProv, ferr := sel.NextFailoverTarget(*alias, tried)
		if ferr != nil {
			p.writeJSONResponse(w, result)
			return
		}
		key := nextTarget.ProviderID + ":" + nextTarget.ModelName
		tried[key] = true

		apiKey, ok := p.pool.Get(nextTarget.ProviderID)
		if !ok {
			continue
		}
		apiKeyFull := model.APIKey{ID: apiKey.ID, Value: apiKey.Value}

		nextResult := p.forwardOnce(ctx, *nextProv, apiKeyFull, body, isStream, w)
		if nextResult.wrote {
			return
		}
		if !isRetryable(nextResult.status) {
			p.writeJSONResponse(w, nextResult)
			return
		}
		result = nextResult
	}
}

func wrapUpstreamError(body []byte, status int) []byte {
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

func rewriteModel(body []byte, modelName string) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("unmarshal body: %w", err)
	}
	modelJSON, err := json.Marshal(modelName)
	if err != nil {
		return nil, fmt.Errorf("marshal model: %w", err)
	}
	m["model"] = modelJSON
	return json.Marshal(m)
}

func isRetryable(status int) bool {
	return status == 429 || status >= 500
}

func writeProxyError(w http.ResponseWriter, err error) {
	switch err {
	case routing.ErrProviderNotFound, routing.ErrUnknownModel:
		writeJSONError(w, http.StatusNotFound, "not_found", err.Error())
	case routing.ErrAliasNoTargets:
		writeJSONError(w, http.StatusServiceUnavailable, "service_unavailable", err.Error())
	case routing.ErrProtocolMismatch:
		writeJSONError(w, http.StatusBadRequest, "invalid_request", err.Error())
	case routing.ErrNoAPIKey:
		writeJSONError(w, http.StatusServiceUnavailable, "service_unavailable", err.Error())
	default:
		writeJSONError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}

func writeJSONError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"message": message,
			"type":    errType,
		},
	})
}
