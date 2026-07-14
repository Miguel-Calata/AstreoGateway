package metrics

import (
	"testing"
	"time"
)

func TestLogStoreRingEviction(t *testing.T) {
	s := NewLogStore(3)
	for i := 0; i < 5; i++ {
		s.Append(RequestLog{Directive: string(rune('a' + i)), Ts: int64(i + 1), Status: 200})
	}
	res := s.List(ListQuery{Limit: 10, Order: "ts_asc"})
	if res.Total != 3 {
		t.Fatalf("total=%d want 3", res.Total)
	}
	if res.Size != 3 || res.Capacity != 3 {
		t.Fatalf("size/cap = %d/%d", res.Size, res.Capacity)
	}
	// oldest three of a..e are c,d,e (indices 2,3,4)
	if res.Items[0].Directive != "c" || res.Items[2].Directive != "e" {
		t.Fatalf("items=%+v", res.Items)
	}
}

func TestLogStoreListFilters(t *testing.T) {
	s := NewLogStore(100)
	now := time.Now().UnixMilli()
	s.Append(RequestLog{Ts: now, Status: 200, ResolvedProvider: "oa", Directive: "oa:gpt", GatewayKeyID: "k1", TokensPrompt: 10, TokensCompletion: 5})
	s.Append(RequestLog{Ts: now, Status: 429, ResolvedProvider: "anth", Directive: "anth:claude", GatewayKeyID: "k2", ErrorClass: "rate_limited"})
	s.Append(RequestLog{Ts: now, Status: 500, ResolvedProvider: "oa", Directive: "oa:gpt", GatewayKeyID: "k1"})

	res := s.List(ListQuery{ProviderSlug: "oa", StatusClass: "ok", Limit: 10})
	if res.Total != 1 {
		t.Fatalf("filtered total=%d want 1", res.Total)
	}
	if res.Items[0].TokensPrompt != 10 {
		t.Fatalf("tokens not preserved")
	}
}

func TestLogStoreStats(t *testing.T) {
	s := NewLogStore(100)
	now := time.Now().UnixMilli()
	s.Append(RequestLog{Ts: now, Status: 200, ResolvedProvider: "oa", DurationMs: 100, TokensPrompt: 10, TokensCompletion: 20, GatewayKeyID: "k1"})
	s.Append(RequestLog{Ts: now, Status: 500, ResolvedProvider: "oa", DurationMs: 200, GatewayKeyID: "k1"})
	s.Append(RequestLog{Ts: now, Status: 200, ResolvedProvider: "gem", DurationMs: 50, TokensPrompt: 5, TokensCompletion: 5, GatewayKeyID: "k2"})

	st := s.Stats(StatsQuery{Window: "1h"})
	if st.TotalRequests != 3 {
		t.Fatalf("requests=%d", st.TotalRequests)
	}
	if st.TotalTokens != 40 {
		t.Fatalf("tokens=%d want 40", st.TotalTokens)
	}
	if st.ErrorRate < 0.3 || st.ErrorRate > 0.34 {
		t.Fatalf("error_rate=%v", st.ErrorRate)
	}
	if len(st.ByProvider) != 2 {
		t.Fatalf("providers=%+v", st.ByProvider)
	}
	if st.P95DurationMs < 100 {
		t.Fatalf("p95=%d", st.P95DurationMs)
	}
}

func TestLogStoreClear(t *testing.T) {
	s := NewLogStore(10)
	s.Append(RequestLog{Ts: 1, Status: 200})
	s.Clear()
	res := s.List(ListQuery{Limit: 10})
	if res.Total != 0 || res.Size != 0 {
		t.Fatalf("after clear: %+v", res)
	}
}
