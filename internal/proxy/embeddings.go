package proxy

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"astreoGateway/internal/model"
	"astreoGateway/internal/protocol"
	"astreoGateway/internal/protocol/openai"
	"astreoGateway/internal/routing"
)

func (p *Proxy) Embeddings(ctx context.Context, w http.ResponseWriter, sel *routing.Selector, directive string, body []byte) *Outcome {
	out := &Outcome{}
	resolved, err := sel.Resolve(directive)
	if err != nil {
		writeProxyError(w, err)
		out.Status = proxyErrorStatus(err)
		out.ErrorClass = "resolve"
		return out
	}
	out.ProviderSlug = resolved.Provider.Slug
	out.ModelName = resolved.ModelName
	if resolved.Provider.Protocol == "" || !protocol.Get(resolved.Provider.Protocol).SupportsEmbeddings() {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "protocol does not support embeddings")
		out.Status = http.StatusBadRequest
		out.ErrorClass = "client"
		return out
	}

	fwdBody, rerr := rewriteModel(body, resolved.ModelName)
	if rerr != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", rerr.Error())
		out.Status = http.StatusBadRequest
		out.ErrorClass = "client"
		return out
	}

	if resolved.AliasRouting != "" {
		return p.forwardEmbeddingsWithRetry(ctx, w, resolved, sel, directive, fwdBody, out)
	}

	started := time.Now()
	result := p.forwardOpenAIEmbeddings(ctx, resolved.Provider, resolved.APIKey, fwdBody)
	appendAttempt(out, resolved.Provider.Slug, resolved.ModelName, resolved.APIKey.ID, result, started)
	if result.err != nil {
		writeJSONError(w, http.StatusBadGateway, "upstream_error", result.err.Error())
		out.Status = http.StatusBadGateway
		out.ErrorClass = "down"
		return out
	}
	p.writeJSONResponse(w, result)
	return out
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

func (p *Proxy) forwardEmbeddingsWithRetry(ctx context.Context, w http.ResponseWriter, resolved *routing.Resolved, sel *routing.Selector, directive string, body []byte, out *Outcome) *Outcome {
	alias, err := sel.LookupAlias(directive)
	if err != nil || alias == nil {
		started := time.Now()
		result := p.forwardOpenAIEmbeddings(ctx, resolved.Provider, resolved.APIKey, body)
		appendAttempt(out, resolved.Provider.Slug, resolved.ModelName, resolved.APIKey.ID, result, started)
		if result.err != nil {
			writeJSONError(w, http.StatusBadGateway, "upstream_error", result.err.Error())
			out.Status = http.StatusBadGateway
			out.ErrorClass = "down"
			return out
		}
		p.writeJSONResponse(w, result)
		return out
	}
	out.AliasName = alias.Name

	tried := make(map[string]bool)
	tried[resolved.Provider.ID+":"+resolved.ModelName] = true

	started := time.Now()
	result := p.forwardOpenAIEmbeddings(ctx, resolved.Provider, resolved.APIKey, body)
	appendAttempt(out, resolved.Provider.Slug, resolved.ModelName, resolved.APIKey.ID, result, started)
	fc := classifyResult(result)
	if !shouldFailover(fc) {
		writeResult(w, result)
		return out
	}
	if shouldSoftStale(fc) {
		p.markSoftStale(resolved.Provider.ID, resolved.ModelName)
	}

	for {
		nextTarget, nextProv, ferr := sel.NextFailoverTarget(*alias, tried)
		if ferr != nil {
			writeResult(w, result)
			return out
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
		attemptStart := time.Now()
		nextResult := p.forwardOpenAIEmbeddings(ctx, *nextProv, apiKeyFull, fwdBody)
		appendAttempt(out, nextProv.Slug, nextTarget.ModelName, apiKeyFull.ID, nextResult, attemptStart)
		nextFc := classifyResult(nextResult)
		if !shouldFailover(nextFc) {
			writeResult(w, nextResult)
			return out
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
