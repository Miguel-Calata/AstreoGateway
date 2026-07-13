package store

import (
	"testing"
)

func TestSettings(t *testing.T) {
	db := testDB(t)

	val, err := GetSetting(db, "nonexistent")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if val != "" {
		t.Fatalf("expected empty, got '%s'", val)
	}

	if err := SetSetting(db, "test_key", "test_value"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, _ := GetSetting(db, "test_key")
	if got != "test_value" {
		t.Fatalf("expected 'test_value', got '%s'", got)
	}

	if err := SetSetting(db, "test_key", "updated"); err != nil {
		t.Fatalf("update: %v", err)
	}
	got2, _ := GetSetting(db, "test_key")
	if got2 != "updated" {
		t.Fatalf("expected 'updated', got '%s'", got2)
	}
}
