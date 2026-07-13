package store

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteByte('-')
		}
	}
	s = nonSlug.ReplaceAllString(b.String(), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "provider"
	}
	if len(s) > 64 {
		s = strings.Trim(s[:64], "-")
	}
	if s == "" {
		return "provider"
	}
	return s
}

func ValidSlug(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	if strings.Contains(s, ":") {
		return false
	}
	for i, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.'
		if !ok {
			return false
		}
		if i == 0 && (r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	last := s[len(s)-1]
	if last == '-' || last == '_' || last == '.' {
		return false
	}
	return true
}

func uniqueProviderSlug(db *sql.DB, base, excludeID string) (string, error) {
	base = Slugify(base)
	if base == "" {
		base = "provider"
	}
	candidate := base
	for n := 2; n < 1000; n++ {
		var id string
		err := db.QueryRow(`SELECT id FROM providers WHERE slug = ?`, candidate).Scan(&id)
		if err == sql.ErrNoRows {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
		if excludeID != "" && id == excludeID {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, n)
	}
	return "", fmt.Errorf("could not allocate unique slug for %q", base)
}

func ensureProviderSlugs(db *sql.DB) error {
	rows, err := db.Query(`SELECT id, name, slug FROM providers WHERE slug = '' OR slug IS NULL`)
	if err != nil {
		return fmt.Errorf("list providers needing slug: %w", err)
	}
	defer rows.Close()

	type row struct {
		id, name, slug string
	}
	var need []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.name, &r.slug); err != nil {
			return err
		}
		need = append(need, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, r := range need {
		slug, err := uniqueProviderSlug(db, r.name, r.id)
		if err != nil {
			return err
		}
		if _, err := db.Exec(`UPDATE providers SET slug = ? WHERE id = ?`, slug, r.id); err != nil {
			return fmt.Errorf("backfill slug for %s: %w", r.id, err)
		}
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_providers_slug ON providers(slug)`); err != nil {
		return fmt.Errorf("unique slug index: %w", err)
	}
	return nil
}
