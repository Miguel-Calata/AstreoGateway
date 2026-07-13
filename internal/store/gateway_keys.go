package store

import (
	"crypto/sha256"
	"database/sql"
	"fmt"

	"astreoGateway/internal/model"
)

func CreateGatewayKey(db *sql.DB, token string, label string) (*model.GatewayKey, error) {
	hash := sha256Hex(token)
	prefix := token
	if len(token) > 8 {
		prefix = token[:8]
	}
	k := model.GatewayKey{
		ID:      newID(),
		Label:   label,
		Hash:    hash,
		Prefix:  prefix,
		Enabled: true,
	}
	_, err := db.Exec(`INSERT INTO gateway_keys (id, label, key_hash, prefix, enabled) VALUES (?, ?, ?, ?, ?)`,
		k.ID, k.Label, k.Hash, k.Prefix, boolToInt(k.Enabled))
	if err != nil {
		return nil, fmt.Errorf("insert gateway_key: %w", err)
	}
	return &k, nil
}

func GetGatewayKeyByID(db *sql.DB, id string) (*model.GatewayKey, error) {
	var k model.GatewayKey
	var enabled int
	err := db.QueryRow(`SELECT id, label, key_hash, prefix, enabled FROM gateway_keys WHERE id = ?`, id).
		Scan(&k.ID, &k.Label, &k.Hash, &k.Prefix, &enabled)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get gateway_key: %w", err)
	}
	k.Enabled = intToBool(enabled)
	return &k, nil
}

func ListGatewayKeys(db *sql.DB) ([]model.GatewayKey, error) {
	rows, err := db.Query(`SELECT id, label, key_hash, prefix, enabled FROM gateway_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list gateway_keys: %w", err)
	}
	defer rows.Close()

	var out []model.GatewayKey
	for rows.Next() {
		var k model.GatewayKey
		var enabled int
		if err := rows.Scan(&k.ID, &k.Label, &k.Hash, &k.Prefix, &enabled); err != nil {
			return nil, fmt.Errorf("scan gateway_key: %w", err)
		}
		k.Enabled = intToBool(enabled)
		out = append(out, k)
	}
	return out, rows.Err()
}

func VerifyGatewayKey(db *sql.DB, token string) (*model.GatewayKey, error) {
	hash := sha256Hex(token)
	var k model.GatewayKey
	var enabled int
	err := db.QueryRow(`SELECT id, label, key_hash, prefix, enabled FROM gateway_keys WHERE key_hash = ? AND enabled = 1`, hash).
		Scan(&k.ID, &k.Label, &k.Hash, &k.Prefix, &enabled)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("verify gateway_key: %w", err)
	}
	k.Enabled = intToBool(enabled)
	return &k, nil
}

func DeleteGatewayKey(db *sql.DB, id string) error {
	res, err := db.Exec(`DELETE FROM gateway_keys WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete gateway_key: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("gateway_key not found")
	}
	return nil
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}
