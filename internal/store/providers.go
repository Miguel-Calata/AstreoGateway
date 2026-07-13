package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"astreoGateway/internal/model"
)

func CreateProvider(db *sql.DB, p *model.Provider) error {
	if p.ID == "" {
		p.ID = newID()
	}
	headersJSON, err := json.Marshal(p.Headers)
	if err != nil {
		return fmt.Errorf("marshal headers: %w", err)
	}
	_, err = db.Exec(`INSERT INTO providers (id, name, protocol, base_url, enabled, headers) VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Protocol, p.BaseURL, boolToInt(p.Enabled), string(headersJSON))
	if err != nil {
		return fmt.Errorf("insert provider: %w", err)
	}
	return nil
}

func GetProviderByID(db *sql.DB, id string) (*model.Provider, error) {
	var p model.Provider
	var headersJSON string
	var enabled int
	err := db.QueryRow(`SELECT id, name, protocol, base_url, enabled, headers FROM providers WHERE id = ?`, id).
		Scan(&p.ID, &p.Name, &p.Protocol, &p.BaseURL, &enabled, &headersJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get provider: %w", err)
	}
	p.Enabled = intToBool(enabled)
	if err := json.Unmarshal([]byte(headersJSON), &p.Headers); err != nil {
		p.Headers = map[string]string{}
	}
	return &p, nil
}

func ListProviders(db *sql.DB) ([]model.Provider, error) {
	rows, err := db.Query(`SELECT id, name, protocol, base_url, enabled, headers FROM providers ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer rows.Close()

	var out []model.Provider
	for rows.Next() {
		var p model.Provider
		var headersJSON string
		var enabled int
		if err := rows.Scan(&p.ID, &p.Name, &p.Protocol, &p.BaseURL, &enabled, &headersJSON); err != nil {
			return nil, fmt.Errorf("scan provider: %w", err)
		}
		p.Enabled = intToBool(enabled)
		if err := json.Unmarshal([]byte(headersJSON), &p.Headers); err != nil {
			p.Headers = map[string]string{}
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func UpdateProvider(db *sql.DB, p *model.Provider) error {
	headersJSON, err := json.Marshal(p.Headers)
	if err != nil {
		return fmt.Errorf("marshal headers: %w", err)
	}
	res, err := db.Exec(`UPDATE providers SET name=?, protocol=?, base_url=?, enabled=?, headers=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		p.Name, p.Protocol, p.BaseURL, boolToInt(p.Enabled), string(headersJSON), p.ID)
	if err != nil {
		return fmt.Errorf("update provider: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("provider not found")
	}
	return nil
}

func DeleteProvider(db *sql.DB, id string) error {
	res, err := db.Exec(`DELETE FROM providers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete provider: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("provider not found")
	}
	return nil
}
