package store

import (
	"testing"
)

func TestGatewayKeyCRUD(t *testing.T) {
	db := testDB(t)

	k, err := CreateGatewayKey(db, "aigw_abc123def456", "test key")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if k.ID == "" {
		t.Fatal("expected ID")
	}
	if k.Prefix != "aigw_abc" {
		t.Fatalf("prefix wrong: %s", k.Prefix)
	}

	keys, err := ListGatewayKeys(db)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1, got %d", len(keys))
	}

	verified, err := VerifyGatewayKey(db, "aigw_abc123def456")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if verified == nil || verified.ID != k.ID {
		t.Fatal("verify failed")
	}

	bad, _ := VerifyGatewayKey(db, "wrong_token")
	if bad != nil {
		t.Fatal("expected nil for wrong token")
	}

	if err := DeleteGatewayKey(db, k.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	gone, _ := VerifyGatewayKey(db, "aigw_abc123def456")
	if gone != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestGatewayKeyShortToken(t *testing.T) {
	db := testDB(t)
	k, err := CreateGatewayKey(db, "short", "short key")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if k.Prefix != "short" {
		t.Fatalf("expected prefix 'short', got '%s'", k.Prefix)
	}
}
