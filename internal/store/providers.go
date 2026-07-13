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
	if p.Slug == "" {
		p.Slug = p.Name
	}
	slug, err := uniqueProviderSlug(db, p.Slug, "")
	if err != nil {
		return fmt.Errorf("slug: %w", err)
	}
	if !ValidSlug(slug) {
		return fmt.Errorf("invalid slug %q", p.Slug)
	}
	p.Slug = slug

	headersJSON, err := json.Marshal(p.Headers)
	if err != nil {
		return fmt.Errorf("marshal headers: %w", err)
	}
	_, err = db.Exec(`INSERT INTO providers (id, name, slug, protocol, base_url, enabled, headers) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Slug, p.Protocol, p.BaseURL, boolToInt(p.Enabled), string(headersJSON))
	if err != nil {
		return fmt.Errorf("insert provider: %w", err)
	}
	return nil
}

func scanProvider(id, name, slug, protocol, baseURL, headersJSON string, enabled int) (*model.Provider, error) {
	p := &model.Provider{
		ID:       id,
		Name:     name,
		Slug:     slug,
		Protocol: protocol,
		BaseURL:  baseURL,
		Enabled:  intToBool(enabled),
	}
	if err := json.Unmarshal([]byte(headersJSON), &p.Headers); err != nil {
		p.Headers = map[string]string{}
	}
	return p, nil
}

func GetProviderByID(db *sql.DB, id string) (*model.Provider, error) {
	var pID, name, slug, protocol, baseURL, headersJSON string
	var enabled int
	err := db.QueryRow(`SELECT id, name, slug, protocol, base_url, enabled, headers FROM providers WHERE id = ?`, id).
		Scan(&pID, &name, &slug, &protocol, &baseURL, &enabled, &headersJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get provider: %w", err)
	}
	return scanProvider(pID, name, slug, protocol, baseURL, headersJSON, enabled)
}

func GetProviderBySlug(db *sql.DB, slug string) (*model.Provider, error) {
	var pID, name, pSlug, protocol, baseURL, headersJSON string
	var enabled int
	err := db.QueryRow(`SELECT id, name, slug, protocol, base_url, enabled, headers FROM providers WHERE slug = ?`, slug).
		Scan(&pID, &name, &pSlug, &protocol, &baseURL, &enabled, &headersJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get provider by slug: %w", err)
	}
	return scanProvider(pID, name, pSlug, protocol, baseURL, headersJSON, enabled)
}

func GetProviderByName(db *sql.DB, name string) (*model.Provider, error) {
	var pID, pName, slug, protocol, baseURL, headersJSON string
	var enabled int
	err := db.QueryRow(`SELECT id, name, slug, protocol, base_url, enabled, headers FROM providers WHERE name = ?`, name).
		Scan(&pID, &pName, &slug, &protocol, &baseURL, &enabled, &headersJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get provider by name: %w", err)
	}
	return scanProvider(pID, pName, slug, protocol, baseURL, headersJSON, enabled)
}

func ListProviders(db *sql.DB) ([]model.Provider, error) {
	rows, err := db.Query(`SELECT id, name, slug, protocol, base_url, enabled, headers FROM providers ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer rows.Close()

	out := make([]model.Provider, 0)
	for rows.Next() {
		var p model.Provider
		var headersJSON string
		var enabled int
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.Protocol, &p.BaseURL, &enabled, &headersJSON); err != nil {
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
	if p.Slug == "" {
		p.Slug = p.Name
	}
	slug, err := uniqueProviderSlug(db, p.Slug, p.ID)
	if err != nil {
		return fmt.Errorf("slug: %w", err)
	}
	if !ValidSlug(slug) {
		return fmt.Errorf("invalid slug %q", p.Slug)
	}
	p.Slug = slug

	headersJSON, err := json.Marshal(p.Headers)
	if err != nil {
		return fmt.Errorf("marshal headers: %w", err)
	}
	res, err := db.Exec(`UPDATE providers SET name=?, slug=?, protocol=?, base_url=?, enabled=?, headers=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		p.Name, p.Slug, p.Protocol, p.BaseURL, boolToInt(p.Enabled), string(headersJSON), p.ID)
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
