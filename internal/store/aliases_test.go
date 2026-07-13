package store

import (
	"testing"

	"astreoGateway/internal/model"
)

func TestAliasCRUD(t *testing.T) {
	db := testDB(t)

	p := &model.Provider{Name: "openai", Protocol: "openai", BaseURL: "https://api.openai.com/v1", Headers: map[string]string{}}
	CreateProvider(db, p)

	a := &model.Alias{
		Name:    "coding",
		Routing: "failover",
		Enabled: true,
		Targets: []model.AliasTarget{
			{ProviderID: p.ID, ModelName: "gpt-5", Position: 0},
			{ProviderID: p.ID, ModelName: "gpt-4o", Position: 1},
		},
	}
	if err := CreateAlias(db, a); err != nil {
		t.Fatalf("create: %v", err)
	}
	if a.ID == "" {
		t.Fatal("expected ID")
	}

	got, err := GetAliasByID(db, a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil || len(got.Targets) != 2 {
		t.Fatalf("expected alias with 2 targets, got %+v", got)
	}
	if got.Targets[0].ModelName != "gpt-5" {
		t.Fatalf("target order wrong: %+v", got.Targets)
	}

	byName, _ := GetAliasByName(db, "coding")
	if byName == nil || byName.ID != a.ID {
		t.Fatalf("get by name failed: %+v", byName)
	}

	a.Targets = []model.AliasTarget{
		{ProviderID: p.ID, ModelName: "gpt-5-turbo", Position: 0},
	}
	if err := UpdateAlias(db, a); err != nil {
		t.Fatalf("update: %v", err)
	}
	updated, _ := GetAliasByID(db, a.ID)
	if len(updated.Targets) != 1 || updated.Targets[0].ModelName != "gpt-5-turbo" {
		t.Fatalf("update targets failed: %+v", updated.Targets)
	}

	aliases, _ := ListAliases(db)
	if len(aliases) != 1 {
		t.Fatalf("expected 1 alias, got %d", len(aliases))
	}

	if err := DeleteAlias(db, a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	gone, _ := GetAliasByID(db, a.ID)
	if gone != nil {
		t.Fatal("expected nil after delete")
	}
}
