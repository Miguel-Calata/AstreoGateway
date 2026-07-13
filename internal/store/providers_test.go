package store

import (
	"testing"

	"astreoGateway/internal/model"
)

func TestProviderCRUD(t *testing.T) {
	db := testDB(t)

	p := &model.Provider{
		Name:     "openai",
		Protocol: "openai",
		BaseURL:  "https://api.openai.com/v1",
		Enabled:  true,
		Headers:  map[string]string{"X-Custom": "val"},
	}
	if err := CreateProvider(db, p); err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected ID to be set")
	}

	got, err := GetProviderByID(db, p.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("expected provider, got nil")
	}
	if got.Name != "openai" || got.Protocol != "openai" {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.Headers["X-Custom"] != "val" {
		t.Fatalf("headers not preserved: %+v", got.Headers)
	}

	providers, err := ListProviders(db)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}

	got.BaseURL = "https://new.openai.com/v1"
	if err := UpdateProvider(db, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	got2, _ := GetProviderByID(db, p.ID)
	if got2.BaseURL != "https://new.openai.com/v1" {
		t.Fatalf("update not persisted: %s", got2.BaseURL)
	}

	if err := DeleteProvider(db, p.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	gone, _ := GetProviderByID(db, p.ID)
	if gone != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestProviderDuplicateName(t *testing.T) {
	db := testDB(t)
	p1 := &model.Provider{Name: "dup", Protocol: "openai", BaseURL: "https://a", Headers: map[string]string{}}
	p2 := &model.Provider{Name: "dup", Protocol: "openai", BaseURL: "https://b", Headers: map[string]string{}}
	if err := CreateProvider(db, p1); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if err := CreateProvider(db, p2); err == nil {
		t.Fatal("expected error on duplicate name")
	}
}
