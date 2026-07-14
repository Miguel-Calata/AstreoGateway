package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAccessLogMiddleware(t *testing.T) {
	store := NewLogStore(100)
	handler := AccessLog(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entry := EntryFromContext(r.Context())
		if entry == nil {
			t.Fatal("missing entry in context")
		}
		entry.Directive = "oa:gpt-5"
		entry.ResolvedProvider = "oa"
		entry.ResolvedModel = "gpt-5"
		entry.TokensPrompt = 3
		entry.TokensCompletion = 7
		entry.Attempts = 1
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))

	req := httptest.NewRequest(http.MethodPost, "/chat/completions", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	res := store.List(ListQuery{Limit: 10})
	if res.Total != 1 {
		t.Fatalf("total=%d", res.Total)
	}
	e := res.Items[0]
	if e.Status != 200 {
		t.Fatalf("status=%d", e.Status)
	}
	if e.Directive != "oa:gpt-5" || e.ResolvedProvider != "oa" {
		t.Fatalf("entry=%+v", e)
	}
	if e.TokensPrompt != 3 || e.TokensCompletion != 7 {
		t.Fatalf("tokens=%d/%d", e.TokensPrompt, e.TokensCompletion)
	}
	if e.ClientIP != "10.0.0.1" {
		t.Fatalf("ip=%s", e.ClientIP)
	}
	if e.DurationMs < 0 {
		t.Fatalf("duration=%d", e.DurationMs)
	}
	if e.ID == "" || e.ID != e.RequestID {
		t.Fatalf("id=%q rid=%q", e.ID, e.RequestID)
	}
}

func TestAccessLogKeepsLogicalErrorStatus(t *testing.T) {
	store := NewLogStore(10)
	handler := AccessLog(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entry := EntryFromContext(r.Context())
		// Stream already wrote 200 to client, but Outcome reports failure.
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {}\n\n"))
		entry.Status = 502
		entry.ErrorClass = "down"
		entry.Directive = "oa:gpt"
	}))
	req := httptest.NewRequest(http.MethodPost, "/chat/completions", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	e := store.List(ListQuery{Limit: 1}).Items[0]
	if e.Status != 502 {
		t.Fatalf("status=%d want 502 (logical)", e.Status)
	}
	if e.ErrorClass != "down" {
		t.Fatalf("class=%s", e.ErrorClass)
	}
}
