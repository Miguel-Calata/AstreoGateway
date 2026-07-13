package store

import (
	"testing"

	"astreoGateway/internal/model"
)

func TestAPIKeyCRUD(t *testing.T) {
	db := testDB(t)
	if _, err := db.Exec(`INSERT INTO providers (id, name, slug, protocol, base_url, enabled) VALUES ('p1', 'p1', 'p1', 'openai', 'https://api.example.com/v1', 1)`); err != nil {
		t.Fatalf("provider: %v", err)
	}

	k := &model.APIKey{
		ProviderID: "p1",
		Label:      "main",
		Value:      "sk-secret",
		Priority:   1,
		Enabled:    true,
	}
	if err := CreateAPIKey(db, k); err != nil {
		t.Fatalf("create: %v", err)
	}
	if k.ID == "" {
		t.Fatal("expected ID")
	}

	got, err := GetAPIKeyByID(db, k.ID)
	if err != nil || got == nil {
		t.Fatalf("get: %v nil=%v", err, got == nil)
	}
	if got.Value != "sk-secret" || got.Label != "main" || !got.Enabled {
		t.Fatalf("got %+v", got)
	}

	list, err := ListAPIKeysByProvider(db, "p1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}

	k.Label = "updated"
	k.Value = "sk-new"
	k.Enabled = false
	if err := UpdateAPIKey(db, k); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = GetAPIKeyByID(db, k.ID)
	if got.Label != "updated" || got.Value != "sk-new" || got.Enabled {
		t.Fatalf("after update: %+v", got)
	}

	if err := DeleteAPIKey(db, k.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	gone, err := GetAPIKeyByID(db, k.ID)
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if gone != nil {
		t.Fatal("expected nil after delete")
	}
}
