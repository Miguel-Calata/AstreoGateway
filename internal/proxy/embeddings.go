package proxy

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"astreoGateway/internal/model"
	"astreoGateway/internal/protocol"
	"astreoGateway/internal/protocol/openai"
	"astreoGateway/internal/routing"
)

func (p *Proxy) Embeddings(ctx context.Context, w http.ResponseWriter, sel *routing.Selector, directive string, body []byte) {
	resolved, err := sel.Resolve(directive)
	if err != nil {
		writeProxyError(w, err)
		return
	}
	if resolved.Provider.Protocol == "" || !protocol.Get(resolved.Provider.Protocol).SupportsEmbeddings() {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "protocol does not support embeddings")
		return
	}

	fwdBody, rerr := rewriteModel(body, resolved.ModelName)
	if rerr != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", rerr.Error())
		return
	}

	if resolved.AliasRouting != "" {
		p.forwardEmbeddingsWithRetry(ctx, w, resolved, sel, directive, fwdBody)
		return
	}

	result := p.forwardOpenAIEmbeddings(ctx, resolved.Provider, resolved.APIKey, fwdBody)
	if result.err != nil {
		writeJSONError(w, http.StatusBadGateway, "upstream_error", result.err.Error())
		return
	}
	p.writeJSONResponse(w, result)
}

func (p *Proxy) forwardOpenAIEmbeddings(ctx context.Context, prov model.Provider, apiKey model.APIKey, body []byte) forwardResult {
	upstreamURL := openai.BuildEmbeddingsURL(prov.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return forwardResult{err: fmt.Errorf("new request: %w", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey.Value)
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
	return p.bufferJSON(resp)
}

func (p *Proxy) forwardEmbeddingsWithRetry(ctx context.Context, w http.ResponseWriter, resolved *routing.Resolved, sel *routing.Selector, directive string, body []byte) {
	alias, err := sel.LookupAlias(directive)
	if err != nil || alias == nil {
		result := p.forwardOpenAIEmbeddings(ctx, resolved.Provider, resolved.APIKey, body)
		if result.err != nil {
			writeJSONError(w, http.StatusBadGateway, "upstream_error", result.err.Error())
			return
		}
		p.writeJSONResponse(w, result)
		return
	}

	tried := make(map[string]bool)
	tried[resolved.Provider.ID+":"+resolved.ModelName] = true

	result := p.forwardOpenAIEmbeddings(ctx, resolved.Provider, resolved.APIKey, body)
	fc := classifyResult(result)
	if !shouldFailover(fc) {
		writeResult(w, result)
		return
	}
	if shouldSoftStale(fc) {
		p.markSoftStale(resolved.Provider.ID, resolved.ModelName)
	}

	for {
		nextTarget, nextProv, ferr := sel.NextFailoverTarget(*alias, tried)
		if ferr != nil {
			writeResult(w, result)
			return
		}
		key := nextTarget.ProviderID + ":" + nextTarget.ModelName
		tried[key] = true
		if !protocol.Get(nextProv.Protocol).SupportsEmbeddings() {
			continue
		}

		apiKey, ok := p.pool.Get(nextTarget.ProviderID)
		if !ok {
			continue
		}
		apiKeyFull := model.APIKey{ID: apiKey.ID, Value: apiKey.Value}

		fwdBody, rerr := rewriteModel(body, nextTarget.ModelName)
		if rerr != nil {
			continue
		}
		nextResult := p.forwardOpenAIEmbeddings(ctx, *nextProv, apiKeyFull, fwdBody)
		nextFc := classifyResult(nextResult)
		if !shouldFailover(nextFc) {
			writeResult(w, nextResult)
			return
		}
		if shouldSoftStale(nextFc) {
			p.markSoftStale(nextProv.ID, nextTarget.ModelName)
		}
		result = nextResult
	}
}

func writeResult(w http.ResponseWriter, r forwardResult) {
	if r.err != nil {
		writeJSONError(w, http.StatusBadGateway, "upstream_error", r.err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.status)
	w.Write(r.body)
}
