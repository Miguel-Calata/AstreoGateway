package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOpen_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	nestedPath := filepath.Join(dir, "new", "nested", "dirs", "aigw.db")
	db, err := Open(nestedPath)
	if err != nil {
		t.Fatalf("open with nested path: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatal("ping after open with created dirs:", err)
	}
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}
