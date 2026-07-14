package admin

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"astreoGateway/internal/discovery"
	"astreoGateway/internal/keypool"
	_ "astreoGateway/internal/protocol/registry"
	"astreoGateway/internal/store"

	"github.com/go-chi/chi/v5"
)

var nopLogger = slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError + 1}))

func testSetup(t *testing.T) (*chi.Mux, string, *keypool.Pool) {
	t.Helper()
	return testSetupSecure(t, false)
}

func testSetupSecure(t *testing.T, cookieSecure bool) (*chi.Mux, string, *keypool.Pool) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	secret, err := EnsureSecret(db)
	if err != nil {
		t.Fatalf("ensure secret: %v", err)
	}
	pool := keypool.New()
	cache := discovery.New(db, pool, 5*time.Minute, 5*time.Second, nopLogger)
	adminHandler, err := NewRouter(db, secret, cache, pool, cookieSecure, nil)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	r := chi.NewRouter()
	r.Route("/admin/api", func(r chi.Router) {
		r.Mount("/", adminHandler)
	})
	return r, secret, pool
}

func jsonBody(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

func TestBootstrapNeeded(t *testing.T) {
	r, _, _ := testSetup(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/admin/api/bootstrap")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["needed"] != true {
		t.Fatalf("expected needed=true, got %v", body["needed"])
	}
}

func TestBootstrapCreate(t *testing.T) {
	r, _, _ := testSetup(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/admin/api/bootstrap", "application/json",
		jsonBody(map[string]string{"username": "admin", "password": "password123"}))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["username"] != "admin" {
		t.Fatalf("expected admin, got %v", body["username"])
	}
	cookie := resp.Header.Get("Set-Cookie")
	if cookie == "" {
		t.Fatal("expected session cookie")
	}
	if strings.Contains(strings.ToLower(cookie), "secure") {
		t.Fatalf("Secure should be off by default, got %s", cookie)
	}
}

func TestBootstrapCookieSecure(t *testing.T) {
	r, _, _ := testSetupSecure(t, true)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/admin/api/bootstrap", "application/json",
		jsonBody(map[string]string{"username": "admin", "password": "password123"}))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	cookie := resp.Header.Get("Set-Cookie")
	if !strings.Contains(strings.ToLower(cookie), "secure") {
		t.Fatalf("expected Secure cookie, got %s", cookie)
	}
}

func TestBootstrapRejectsSecond(t *testing.T) {
	r, _, _ := testSetup(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	_, err := http.Post(ts.URL+"/admin/api/bootstrap", "application/json",
		jsonBody(map[string]string{"username": "admin", "password": "password123"}))
	if err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}

	resp, err := http.Post(ts.URL+"/admin/api/bootstrap", "application/json",
		jsonBody(map[string]string{"username": "admin2", "password": "password456"}))
	if err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestLoginLogout(t *testing.T) {
	r, _, _ := testSetup(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	_, err := http.Post(ts.URL+"/admin/api/bootstrap", "application/json",
		jsonBody(map[string]string{"username": "admin", "password": "password123"}))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	resp, err := http.Post(ts.URL+"/admin/api/login", "application/json",
		jsonBody(map[string]string{"username": "admin", "password": "password123"}))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	cookie := resp.Header.Get("Set-Cookie")
	if cookie == "" {
		t.Fatal("expected session cookie on login")
	}

	resp2, err := http.Post(ts.URL+"/admin/api/logout", "application/json", nil)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	r, _, _ := testSetup(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	_, err := http.Post(ts.URL+"/admin/api/bootstrap", "application/json",
		jsonBody(map[string]string{"username": "admin", "password": "password123"}))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	resp, err := http.Post(ts.URL+"/admin/api/login", "application/json",
		jsonBody(map[string]string{"username": "admin", "password": "wrong"}))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestSessionRequiresAuth(t *testing.T) {
	r, _, _ := testSetup(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/admin/api/session")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestSessionWithAuth(t *testing.T) {
	r, _, _ := testSetup(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	loginResp, err := http.Post(ts.URL+"/admin/api/bootstrap", "application/json",
		jsonBody(map[string]string{"username": "admin", "password": "password123"}))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	cookie := loginResp.Header.Get("Set-Cookie")

	req, _ := http.NewRequest("GET", ts.URL+"/admin/api/session", nil)
	req.Header.Set("Cookie", cookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["username"] != "admin" {
		t.Fatalf("expected admin, got %v", body)
	}
}

func TestProvidersCRUD(t *testing.T) {
	r, _, _ := testSetup(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	loginResp, _ := http.Post(ts.URL+"/admin/api/bootstrap", "application/json",
		jsonBody(map[string]string{"username": "admin", "password": "password123"}))
	cookie := loginResp.Header.Get("Set-Cookie")

	doReq := func(method, path string, body any) *http.Response {
		var req *http.Request
		if body != nil {
			b, _ := json.Marshal(body)
			req, _ = http.NewRequest(method, ts.URL+path, bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(method, ts.URL+path, nil)
		}
		req.Header.Set("Cookie", cookie)
		resp, _ := http.DefaultClient.Do(req)
		return resp
	}

	resp := doReq("POST", "/admin/api/providers", map[string]any{
		"name": "openai", "protocol": "openai", "base_url": "https://api.openai.com/v1", "enabled": true,
	})
	if resp.StatusCode != 201 {
		t.Fatalf("create: expected 201, got %d", resp.StatusCode)
	}
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	_ = resp.Body.Close()
	providerID := created["id"].(string)

	resp2 := doReq("GET", "/admin/api/providers", nil)
	if resp2.StatusCode != 200 {
		t.Fatalf("list: expected 200, got %d", resp2.StatusCode)
	}
	var list []any
	json.NewDecoder(resp2.Body).Decode(&list)
	_ = resp2.Body.Close()
	if len(list) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(list))
	}

	resp3 := doReq("GET", "/admin/api/providers/"+providerID, nil)
	if resp3.StatusCode != 200 {
		t.Fatalf("get: expected 200, got %d", resp3.StatusCode)
	}
	_ = resp3.Body.Close()

	resp4 := doReq("PUT", "/admin/api/providers/"+providerID, map[string]any{
		"name": "openai", "protocol": "openai", "base_url": "https://new.openai.com/v1", "enabled": true,
	})
	if resp4.StatusCode != 200 {
		t.Fatalf("update: expected 200, got %d", resp4.StatusCode)
	}
	_ = resp4.Body.Close()

	resp5 := doReq("DELETE", "/admin/api/providers/"+providerID, nil)
	if resp5.StatusCode != 204 {
		t.Fatalf("delete: expected 204, got %d", resp5.StatusCode)
	}
	_ = resp5.Body.Close()
}

func TestAliasesCRUD(t *testing.T) {
	r, _, _ := testSetup(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	loginResp, _ := http.Post(ts.URL+"/admin/api/bootstrap", "application/json",
		jsonBody(map[string]string{"username": "admin", "password": "password123"}))
	cookie := loginResp.Header.Get("Set-Cookie")

	doReq := func(method, path string, body any) *http.Response {
		var req *http.Request
		if body != nil {
			b, _ := json.Marshal(body)
			req, _ = http.NewRequest(method, ts.URL+path, bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(method, ts.URL+path, nil)
		}
		req.Header.Set("Cookie", cookie)
		resp, _ := http.DefaultClient.Do(req)
		return resp
	}

	provResp := doReq("POST", "/admin/api/providers", map[string]any{
		"name": "openai", "protocol": "openai", "base_url": "https://api.openai.com/v1", "enabled": true,
	})
	var prov map[string]any
	json.NewDecoder(provResp.Body).Decode(&prov)
	_ = provResp.Body.Close()
	providerID := prov["id"].(string)

	resp := doReq("POST", "/admin/api/aliases?allow_unknown_models=1", map[string]any{
		"name": "coding", "routing": "failover", "enabled": true,
		"targets": []map[string]any{
			{"provider_id": providerID, "model_name": "gpt-5", "position": 0},
		},
	})
	if resp.StatusCode != 201 {
		t.Fatalf("create alias: expected 201, got %d", resp.StatusCode)
	}
	var alias map[string]any
	json.NewDecoder(resp.Body).Decode(&alias)
	_ = resp.Body.Close()
	aliasID := alias["id"].(string)

	resp2 := doReq("GET", "/admin/api/aliases/"+aliasID, nil)
	if resp2.StatusCode != 200 {
		t.Fatalf("get alias: expected 200, got %d", resp2.StatusCode)
	}
	_ = resp2.Body.Close()

	resp3 := doReq("DELETE", "/admin/api/aliases/"+aliasID, nil)
	if resp3.StatusCode != 204 {
		t.Fatalf("delete alias: expected 204, got %d", resp3.StatusCode)
	}
	_ = resp3.Body.Close()
}

func TestAPIKeysCRUDAndKeypoolReload(t *testing.T) {
	r, _, pool := testSetup(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	loginResp, _ := http.Post(ts.URL+"/admin/api/bootstrap", "application/json",
		jsonBody(map[string]string{"username": "admin", "password": "password123"}))
	cookie := loginResp.Header.Get("Set-Cookie")

	doReq := func(method, path string, body any) *http.Response {
		var req *http.Request
		if body != nil {
			b, _ := json.Marshal(body)
			req, _ = http.NewRequest(method, ts.URL+path, bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(method, ts.URL+path, nil)
		}
		req.Header.Set("Cookie", cookie)
		resp, _ := http.DefaultClient.Do(req)
		return resp
	}

	provResp := doReq("POST", "/admin/api/providers", map[string]any{
		"name": "openai", "protocol": "openai", "base_url": "https://api.openai.com/v1", "enabled": true,
	})
	var prov map[string]any
	json.NewDecoder(provResp.Body).Decode(&prov)
	_ = provResp.Body.Close()
	providerID := prov["id"].(string)

	if _, ok := pool.Get(providerID); ok {
		t.Fatal("pool should be empty before creating key")
	}

	resp := doReq("POST", "/admin/api/providers/"+providerID+"/api-keys", map[string]any{
		"label": "main", "key_value": "sk-live-test", "priority": 0, "enabled": true,
	})
	if resp.StatusCode != 201 {
		t.Fatalf("create key: expected 201, got %d", resp.StatusCode)
	}
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	_ = resp.Body.Close()
	keyID := created["id"].(string)
	if created["key_value"] != "sk-live-test" {
		t.Fatalf("create should return key_value once, got %v", created["key_value"])
	}

	got, ok := pool.Get(providerID)
	if !ok || got.Value != "sk-live-test" {
		t.Fatalf("pool should have new key after create, got %+v ok=%v", got, ok)
	}

	resp2 := doReq("GET", "/admin/api/providers/"+providerID+"/api-keys", nil)
	if resp2.StatusCode != 200 {
		t.Fatalf("list: expected 200, got %d", resp2.StatusCode)
	}
	var listed []map[string]any
	json.NewDecoder(resp2.Body).Decode(&listed)
	_ = resp2.Body.Close()
	if len(listed) != 1 {
		t.Fatalf("list: expected 1 key, got %d", len(listed))
	}
	if _, has := listed[0]["key_value"]; has {
		t.Fatalf("list must not include key_value, got %v", listed[0])
	}

	respGet := doReq("GET", "/admin/api/api-keys/"+keyID, nil)
	if respGet.StatusCode != 200 {
		t.Fatalf("get: expected 200, got %d", respGet.StatusCode)
	}
	var one map[string]any
	json.NewDecoder(respGet.Body).Decode(&one)
	_ = respGet.Body.Close()
	if _, has := one["key_value"]; has {
		t.Fatalf("get must not include key_value, got %v", one)
	}

	resp3 := doReq("PUT", "/admin/api/api-keys/"+keyID, map[string]any{
		"label": "main", "key_value": "sk-rotated", "priority": 0, "enabled": true,
	})
	if resp3.StatusCode != 200 {
		t.Fatalf("update: expected 200, got %d", resp3.StatusCode)
	}
	var updated map[string]any
	json.NewDecoder(resp3.Body).Decode(&updated)
	_ = resp3.Body.Close()
	if _, has := updated["key_value"]; has {
		t.Fatalf("update must not include key_value, got %v", updated)
	}
	got, ok = pool.Get(providerID)
	if !ok || got.Value != "sk-rotated" {
		t.Fatalf("pool should have rotated key, got %+v ok=%v", got, ok)
	}

	respKeep := doReq("PUT", "/admin/api/api-keys/"+keyID, map[string]any{
		"label": "main-renamed", "priority": 1, "enabled": true,
	})
	if respKeep.StatusCode != 200 {
		t.Fatalf("update without key_value: expected 200, got %d", respKeep.StatusCode)
	}
	_ = respKeep.Body.Close()
	got, ok = pool.Get(providerID)
	if !ok || got.Value != "sk-rotated" {
		t.Fatalf("pool should keep secret when key_value omitted, got %+v ok=%v", got, ok)
	}

	resp4 := doReq("DELETE", "/admin/api/api-keys/"+keyID, nil)
	if resp4.StatusCode != 204 {
		t.Fatalf("delete: expected 204, got %d", resp4.StatusCode)
	}
	_ = resp4.Body.Close()
	if _, ok := pool.Get(providerID); ok {
		t.Fatal("pool should be empty after delete")
	}
}

func TestGatewayKeysCRUD(t *testing.T) {
	r, _, _ := testSetup(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	loginResp, _ := http.Post(ts.URL+"/admin/api/bootstrap", "application/json",
		jsonBody(map[string]string{"username": "admin", "password": "password123"}))
	cookie := loginResp.Header.Get("Set-Cookie")

	doReq := func(method, path string, body any) *http.Response {
		var req *http.Request
		if body != nil {
			b, _ := json.Marshal(body)
			req, _ = http.NewRequest(method, ts.URL+path, bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(method, ts.URL+path, nil)
		}
		req.Header.Set("Cookie", cookie)
		resp, _ := http.DefaultClient.Do(req)
		return resp
	}

	resp := doReq("POST", "/admin/api/gateway-keys", map[string]any{"label": "test"})
	if resp.StatusCode != 201 {
		t.Fatalf("create: expected 201, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	_ = resp.Body.Close()
	keyID := body["id"].(string)
	token, ok := body["token"].(string)
	if !ok || token == "" {
		t.Fatal("expected token in response")
	}
	if len(token) < 10 {
		t.Fatalf("token too short: %s", token)
	}

	resp2 := doReq("GET", "/admin/api/gateway-keys", nil)
	if resp2.StatusCode != 200 {
		t.Fatalf("list: expected 200, got %d", resp2.StatusCode)
	}
	var keys []any
	json.NewDecoder(resp2.Body).Decode(&keys)
	_ = resp2.Body.Close()
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}

	resp3 := doReq("DELETE", "/admin/api/gateway-keys/"+keyID, nil)
	if resp3.StatusCode != 204 {
		t.Fatalf("delete: expected 204, got %d", resp3.StatusCode)
	}
	_ = resp3.Body.Close()
}
