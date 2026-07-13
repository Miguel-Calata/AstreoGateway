package store

import (
	"database/sql"
	"fmt"

	"astreoGateway/internal/model"

	"golang.org/x/crypto/bcrypt"
)

func CountAdminUsers(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM admin_users`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count admin_users: %w", err)
	}
	return n, nil
}

func CreateAdminUser(db *sql.DB, username, password string) (*model.AdminUser, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("bcrypt hash: %w", err)
	}
	u := model.AdminUser{
		ID:       newID(),
		Username: username,
	}
	_, err = db.Exec(`INSERT INTO admin_users (id, username, password_hash) VALUES (?, ?, ?)`,
		u.ID, u.Username, string(hash))
	if err != nil {
		return nil, fmt.Errorf("insert admin_user: %w", err)
	}
	return &u, nil
}

func GetAdminUserByUsername(db *sql.DB, username string) (*model.AdminUser, error) {
	var u model.AdminUser
	err := db.QueryRow(`SELECT id, username, password_hash FROM admin_users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get admin_user: %w", err)
	}
	return &u, nil
}

func GetAdminUserByID(db *sql.DB, id string) (*model.AdminUser, error) {
	var u model.AdminUser
	err := db.QueryRow(`SELECT id, username, password_hash FROM admin_users WHERE id = ?`, id).
		Scan(&u.ID, &u.Username, &u.PasswordHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get admin_user: %w", err)
	}
	return &u, nil
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
