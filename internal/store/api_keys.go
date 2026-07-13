package store

import (
	"database/sql"
	"fmt"

	"astreoGateway/internal/model"
)

func CreateAPIKey(db *sql.DB, k *model.APIKey) error {
	if k.ID == "" {
		k.ID = newID()
	}
	_, err := db.Exec(`INSERT INTO api_keys (id, provider_id, label, key_value, priority, enabled) VALUES (?, ?, ?, ?, ?, ?)`,
		k.ID, k.ProviderID, k.Label, k.Value, k.Priority, boolToInt(k.Enabled))
	if err != nil {
		return fmt.Errorf("insert api_key: %w", err)
	}
	return nil
}

func GetAPIKeyByID(db *sql.DB, id string) (*model.APIKey, error) {
	var k model.APIKey
	var enabled int
	err := db.QueryRow(`SELECT id, provider_id, label, key_value, priority, enabled FROM api_keys WHERE id = ?`, id).
		Scan(&k.ID, &k.ProviderID, &k.Label, &k.Value, &k.Priority, &enabled)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get api_key: %w", err)
	}
	k.Enabled = intToBool(enabled)
	return &k, nil
}

func ListAPIKeysByProvider(db *sql.DB, providerID string) ([]model.APIKey, error) {
	rows, err := db.Query(`SELECT id, provider_id, label, key_value, priority, enabled FROM api_keys WHERE provider_id = ? ORDER BY priority, label`, providerID)
	if err != nil {
		return nil, fmt.Errorf("list api_keys: %w", err)
	}
	defer rows.Close()

	out := make([]model.APIKey, 0)
	for rows.Next() {
		var k model.APIKey
		var enabled int
		if err := rows.Scan(&k.ID, &k.ProviderID, &k.Label, &k.Value, &k.Priority, &enabled); err != nil {
			return nil, fmt.Errorf("scan api_key: %w", err)
		}
		k.Enabled = intToBool(enabled)
		out = append(out, k)
	}
	return out, rows.Err()
}

func UpdateAPIKey(db *sql.DB, k *model.APIKey) error {
	res, err := db.Exec(`UPDATE api_keys SET label=?, key_value=?, priority=?, enabled=? WHERE id=?`,
		k.Label, k.Value, k.Priority, boolToInt(k.Enabled), k.ID)
	if err != nil {
		return fmt.Errorf("update api_key: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("api_key not found")
	}
	return nil
}

func DeleteAPIKey(db *sql.DB, id string) error {
	res, err := db.Exec(`DELETE FROM api_keys WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete api_key: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("api_key not found")
	}
	return nil
}
