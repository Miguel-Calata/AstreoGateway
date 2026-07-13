package proxy

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"astreoGateway/internal/model"
	"astreoGateway/internal/protocol/openai"
	"astreoGateway/internal/routing"
)

func (p *Proxy) Embeddings(ctx context.Context, w http.ResponseWriter, sel *routing.Selector, directive string, body []byte) {
	resolved, err := sel.Resolve(directive)
	if err != nil {
		writeProxyError(w, err)
		return
	}
	if resolved.Provider.Protocol == "anthropic" {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "protocol does not support embeddings")
		return
	}

	fwdBody, rerr := rewriteModel(body, resolved.ModelName)
	if rerr != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", rerr.Error())
		return
	}

	if resolved.AliasRouting == "failover" {
		p.forwardEmbeddingsWithFailover(ctx, w, resolved, sel, directive, fwdBody)
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

	if isRetryable(resp.StatusCode) {
		p.pool.MarkCooldown(prov.ID, apiKey.ID, p.keyCooldown)
	}
	return p.bufferJSON(resp)
}

func (p *Proxy) forwardEmbeddingsWithFailover(ctx context.Context, w http.ResponseWriter, resolved *routing.Resolved, sel *routing.Selector, directive string, body []byte) {
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
	if result.err != nil {
		// network errors: try next if possible
	} else if !isRetryable(result.status) {
		p.writeJSONResponse(w, result)
		return
	}

	for {
		nextTarget, nextProv, ferr := sel.NextFailoverTarget(*alias, tried)
		if ferr != nil {
			if result.err != nil {
				writeJSONError(w, http.StatusBadGateway, "upstream_error", result.err.Error())
				return
			}
			p.writeJSONResponse(w, result)
			return
		}
		key := nextTarget.ProviderID + ":" + nextTarget.ModelName
		tried[key] = true
		if nextProv.Protocol == "anthropic" {
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
		if nextResult.err != nil {
			result = nextResult
			continue
		}
		if !isRetryable(nextResult.status) {
			p.writeJSONResponse(w, nextResult)
			return
		}
		result = nextResult
	}
}
