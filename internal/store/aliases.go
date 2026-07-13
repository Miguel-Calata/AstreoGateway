package store

import (
	"database/sql"
	"fmt"

	"astreoGateway/internal/model"
)

func CreateAlias(db *sql.DB, a *model.Alias) error {
	if a.ID == "" {
		a.ID = newID()
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`INSERT INTO aliases (id, name, routing, enabled) VALUES (?, ?, ?, ?)`,
		a.ID, a.Name, a.Routing, boolToInt(a.Enabled))
	if err != nil {
		return fmt.Errorf("insert alias: %w", err)
	}
	for _, t := range a.Targets {
		_, err = tx.Exec(`INSERT INTO alias_targets (alias_id, provider_id, model_name, position) VALUES (?, ?, ?, ?)`,
			a.ID, t.ProviderID, t.ModelName, t.Position)
		if err != nil {
			return fmt.Errorf("insert alias_target: %w", err)
		}
	}
	return tx.Commit()
}

func GetAliasByID(db *sql.DB, id string) (*model.Alias, error) {
	var a model.Alias
	var enabled int
	err := db.QueryRow(`SELECT id, name, routing, enabled FROM aliases WHERE id = ?`, id).
		Scan(&a.ID, &a.Name, &a.Routing, &enabled)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get alias: %w", err)
	}
	a.Enabled = intToBool(enabled)

	targets, err := listAliasTargets(db, id)
	if err != nil {
		return nil, err
	}
	a.Targets = targets
	return &a, nil
}

func GetAliasByName(db *sql.DB, name string) (*model.Alias, error) {
	var a model.Alias
	var enabled int
	err := db.QueryRow(`SELECT id, name, routing, enabled FROM aliases WHERE name = ?`, name).
		Scan(&a.ID, &a.Name, &a.Routing, &enabled)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get alias by name: %w", err)
	}
	a.Enabled = intToBool(enabled)

	targets, err := listAliasTargets(db, a.ID)
	if err != nil {
		return nil, err
	}
	a.Targets = targets
	return &a, nil
}

func ListAliases(db *sql.DB) ([]model.Alias, error) {
	rows, err := db.Query(`SELECT id, name, routing, enabled FROM aliases ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list aliases: %w", err)
	}
	defer rows.Close()

	var out []model.Alias
	for rows.Next() {
		var a model.Alias
		var enabled int
		if err := rows.Scan(&a.ID, &a.Name, &a.Routing, &enabled); err != nil {
			return nil, fmt.Errorf("scan alias: %w", err)
		}
		a.Enabled = intToBool(enabled)
		targets, err := listAliasTargets(db, a.ID)
		if err != nil {
			return nil, err
		}
		a.Targets = targets
		out = append(out, a)
	}
	return out, rows.Err()
}

func UpdateAlias(db *sql.DB, a *model.Alias) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.Exec(`UPDATE aliases SET name=?, routing=?, enabled=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		a.Name, a.Routing, boolToInt(a.Enabled), a.ID)
	if err != nil {
		return fmt.Errorf("update alias: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("alias not found")
	}

	_, err = tx.Exec(`DELETE FROM alias_targets WHERE alias_id = ?`, a.ID)
	if err != nil {
		return fmt.Errorf("delete targets: %w", err)
	}
	for _, t := range a.Targets {
		_, err = tx.Exec(`INSERT INTO alias_targets (alias_id, provider_id, model_name, position) VALUES (?, ?, ?, ?)`,
			a.ID, t.ProviderID, t.ModelName, t.Position)
		if err != nil {
			return fmt.Errorf("insert alias_target: %w", err)
		}
	}
	return tx.Commit()
}

func DeleteAlias(db *sql.DB, id string) error {
	res, err := db.Exec(`DELETE FROM aliases WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete alias: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("alias not found")
	}
	return nil
}

func listAliasTargets(db *sql.DB, aliasID string) ([]model.AliasTarget, error) {
	rows, err := db.Query(`SELECT provider_id, model_name, position FROM alias_targets WHERE alias_id = ? ORDER BY position`, aliasID)
	if err != nil {
		return nil, fmt.Errorf("list targets: %w", err)
	}
	defer rows.Close()

	var out []model.AliasTarget
	for rows.Next() {
		var t model.AliasTarget
		if err := rows.Scan(&t.ProviderID, &t.ModelName, &t.Position); err != nil {
			return nil, fmt.Errorf("scan target: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
