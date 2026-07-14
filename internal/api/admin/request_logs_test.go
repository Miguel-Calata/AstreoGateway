package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"astreoGateway/internal/discovery"
	"astreoGateway/internal/keypool"
	"astreoGateway/internal/metrics"
	"astreoGateway/internal/store"

	"github.com/go-chi/chi/v5"
)

func TestRequestLogsAdminAPI(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	secret, err := EnsureSecret(db)
	if err != nil {
		t.Fatal(err)
	}
	pool := keypool.New()
	cache := discovery.New(db, pool, time.Minute, time.Second, nopLogger)
	logs := metrics.NewLogStore(50)
	now := time.Now().UnixMilli()
	logs.Append(metrics.RequestLog{
		Ts: now, Status: 200, ResolvedProvider: "oa", Directive: "oa:gpt",
		TokensPrompt: 10, TokensCompletion: 5, GatewayKeyID: "gk1", DurationMs: 40,
	})
	logs.Append(metrics.RequestLog{
		Ts: now, Status: 500, ResolvedProvider: "oa", Directive: "oa:gpt",
		GatewayKeyID: "gk1", DurationMs: 80, ErrorClass: "down",
	})

	adminHandler, err := NewRouter(db, secret, cache, pool, false, logs)
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	r.Mount("/", adminHandler)
	ts := httptest.NewServer(r)
	defer ts.Close()

	// bootstrap admin
	resp, err := http.Post(ts.URL+"/bootstrap", "application/json",
		jsonBody(map[string]string{"username": "admin", "password": "password123"}))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("bootstrap=%d", resp.StatusCode)
	}
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		cookie = c
	}
	if cookie == nil {
		// login if bootstrap didn't set cookie
		resp, err = http.Post(ts.URL+"/login", "application/json",
			jsonBody(map[string]string{"username": "admin", "password": "password123"}))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		for _, c := range resp.Cookies() {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no session cookie")
	}

	client := &http.Client{}
	do := func(method, path string) *http.Response {
		t.Helper()
		req, _ := http.NewRequest(method, ts.URL+path, nil)
		req.AddCookie(cookie)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	resp = do(http.MethodGet, "/request-logs?limit=10")
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("list=%d", resp.StatusCode)
	}
	var list metrics.ListResult
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if list.Total != 2 {
		t.Fatalf("total=%d", list.Total)
	}

	resp = do(http.MethodGet, "/request-logs?status_class=ok")
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		resp.Body.Close()
		t.Fatal(err)
	}
	resp.Body.Close()
	if list.Total != 1 {
		t.Fatalf("ok filter total=%d", list.Total)
	}

	resp = do(http.MethodGet, "/request-logs/stats?window=1h")
	if resp.StatusCode != 200 {
		resp.Body.Close()
		t.Fatalf("stats=%d", resp.StatusCode)
	}
	var st metrics.StatsResult
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		resp.Body.Close()
		t.Fatal(err)
	}
	resp.Body.Close()
	if st.TotalRequests != 2 {
		t.Fatalf("stats requests=%d", st.TotalRequests)
	}
	if st.TotalTokens != 15 {
		t.Fatalf("stats tokens=%d", st.TotalTokens)
	}

	resp = do(http.MethodDelete, "/request-logs")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete=%d", resp.StatusCode)
	}
	resp = do(http.MethodGet, "/request-logs")
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		resp.Body.Close()
		t.Fatal(err)
	}
	resp.Body.Close()
	if list.Total != 0 {
		t.Fatalf("after clear total=%d", list.Total)
	}
}
