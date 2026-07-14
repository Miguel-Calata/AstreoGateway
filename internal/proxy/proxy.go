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

	"astreoGateway/internal/discovery"
	"astreoGateway/internal/keypool"
	"astreoGateway/internal/model"
	"astreoGateway/internal/protocol"
	"astreoGateway/internal/routing"
)

var ErrUpstreamTimeout = fmt.Errorf("upstream timeout")

type failClass int

const (
	failNone          failClass = iota
	failDown                    // 5xx, timeout, network error
	failRate                    // 429
	failModelMissing            // 404 model/function not found
	failAuth                    // 401, 403
	failClient                  // 400, 422 — no failover
)

func shouldFailover(c failClass) bool { return c != failNone && c != failClient }
func shouldCooldown(c failClass) bool { return c == failRate || c == failDown }
func shouldSoftStale(c failClass) bool { return c == failModelMissing }

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
	cache       *discovery.Cache
}

func New(pool *keypool.Pool, timeout, keyCooldown time.Duration, logger *slog.Logger) *Proxy {
	return &Proxy{
		pool:        pool,
		httpC:       &http.Client{Timeout: timeout},
		logger:      logger,
		keyCooldown: keyCooldown,
	}
}

func (p *Proxy) SetDiscoveryCache(c *discovery.Cache) {
	p.cache = c
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

	if resolved.AliasRouting != "" {
		p.forwardWithRetry(ctx, w, resolved, sel, directive, fwdBody, isStream)
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

	if shouldCooldown(classifyUpstream(resp.StatusCode, nil)) {
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

func (p *Proxy) forwardWithRetry(ctx context.Context, w http.ResponseWriter, resolved *routing.Resolved, sel *routing.Selector, directive string, body []byte, isStream bool) {
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
	// TODO(mid-stream-failover): if the stream already started writing (result.wrote)
	// and the upstream cuts mid-SSE (e.g. max output tokens, reset, 5xx), we cannot
	// retry without corrupting the client's SSE stream. Future work: buffer first N
	// events, only retry if no content has been flushed yet, or implement upstream
	// request range replay. Motivated by providers that limit output tokens and abort
	// mid-response.
	if result.wrote {
		return
	}

	fc := classifyResult(result)
	if !shouldFailover(fc) {
		p.writeJSONResponse(w, result)
		return
	}

	if shouldSoftStale(fc) {
		p.markSoftStale(resolved.Provider.ID, resolved.ModelName)
	}

	p.logger.Warn("proxy: retrying after upstream failure",
		"alias", alias.Name,
		"routing", resolved.AliasRouting,
		"tried", resolved.Provider.ID+":"+resolved.ModelName,
		"status", result.status,
		"class", fc,
	)

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

		nextBody, rerr := rewriteModel(body, nextTarget.ModelName)
		if rerr != nil {
			continue
		}

		nextResult := p.forwardOnce(ctx, *nextProv, apiKeyFull, nextBody, isStream, w)
		if nextResult.wrote {
			return
		}
		nextFc := classifyResult(nextResult)
		if !shouldFailover(nextFc) {
			p.writeJSONResponse(w, nextResult)
			return
		}
		if shouldSoftStale(nextFc) {
			p.markSoftStale(nextProv.ID, nextTarget.ModelName)
		}
		p.logger.Warn("proxy: retrying after upstream failure",
			"alias", alias.Name,
			"routing", resolved.AliasRouting,
			"tried", key,
			"status", nextResult.status,
			"class", nextFc,
		)
		result = nextResult
	}
}

func (p *Proxy) markSoftStale(providerID, modelName string) {
	if p.cache != nil {
		p.cache.MarkRuntimeStale(providerID, modelName)
	}
}

func classifyUpstream(status int, body []byte) failClass {
	switch {
	case status == 429:
		return failRate
	case status >= 500:
		return failDown
	case status == 404:
		return failModelMissing
	case status == 401 || status == 403:
		return failAuth
	case status == 400 || status == 422:
		return failClient
	default:
		return failNone
	}
}

func classifyResult(r forwardResult) failClass {
	if r.err != nil {
		return failDown
	}
	return classifyUpstream(r.status, r.body)
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
