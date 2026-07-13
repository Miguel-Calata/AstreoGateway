package store

import (
	"testing"
)

func TestAdminUserCRUD(t *testing.T) {
	db := testDB(t)

	count, err := CountAdminUsers(db)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}

	u, err := CreateAdminUser(db, "admin", "password123")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.ID == "" || u.Username != "admin" {
		t.Fatalf("unexpected user: %+v", u)
	}

	count, _ = CountAdminUsers(db)
	if count != 1 {
		t.Fatalf("expected 1, got %d", count)
	}

	got, err := GetAdminUserByUsername(db, "admin")
	if err != nil {
		t.Fatalf("get by username: %v", err)
	}
	if got == nil {
		t.Fatal("expected user, got nil")
	}
	if !CheckPassword(got.PasswordHash, "password123") {
		t.Fatal("password mismatch")
	}
	if CheckPassword(got.PasswordHash, "wrong") {
		t.Fatal("wrong password should fail")
	}

	gotByID, _ := GetAdminUserByID(db, u.ID)
	if gotByID == nil || gotByID.Username != "admin" {
		t.Fatalf("get by id failed: %+v", gotByID)
	}

	noUser, _ := GetAdminUserByUsername(db, "nonexistent")
	if noUser != nil {
		t.Fatal("expected nil for nonexistent user")
	}
}
