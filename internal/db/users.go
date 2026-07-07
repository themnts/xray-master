package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

type User struct {
	ID         string
	Email      string
	UUID       string
	SubToken   string
	Enabled    bool
	ExpiryTime int64
	TotalBytes int64
	Note       string
	CreatedAt  time.Time
}

func CreateUser(conn *sql.DB, u User) error {
	if u.ID == "" || u.Email == "" || u.UUID == "" {
		return fmt.Errorf("user id, email, and uuid are required")
	}
	if u.SubToken == "" {
		token, err := randomToken(16)
		if err != nil {
			return err
		}
		u.SubToken = token
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now().UTC()
	}
	_, err := conn.Exec(
		`INSERT INTO users (id, email, uuid, sub_token, enabled, expiry_time, total_bytes, note, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Email, u.UUID, u.SubToken, boolToInt(u.Enabled), u.ExpiryTime, u.TotalBytes, u.Note, u.CreatedAt.Format(time.RFC3339),
	)
	return err
}

func ListUsers(conn *sql.DB) ([]User, error) {
	rows, err := conn.Query(`SELECT id, email, uuid, sub_token, enabled, expiry_time, total_bytes, note, created_at FROM users ORDER BY email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func GetUserByID(conn *sql.DB, id string) (*User, error) {
	row := conn.QueryRow(`SELECT id, email, uuid, sub_token, enabled, expiry_time, total_bytes, note, created_at FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func GetUserByEmail(conn *sql.DB, email string) (*User, error) {
	row := conn.QueryRow(`SELECT id, email, uuid, sub_token, enabled, expiry_time, total_bytes, note, created_at FROM users WHERE email = ?`, email)
	return scanUser(row)
}

func GetUserBySubToken(conn *sql.DB, token string) (*User, error) {
	row := conn.QueryRow(`SELECT id, email, uuid, sub_token, enabled, expiry_time, total_bytes, note, created_at FROM users WHERE sub_token = ?`, token)
	return scanUser(row)
}

func SetUserEnabled(conn *sql.DB, id string, enabled bool) error {
	res, err := conn.Exec(`UPDATE users SET enabled = ? WHERE id = ?`, boolToInt(enabled), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func DeleteUser(conn *sql.DB, id string) error {
	res, err := conn.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func scanUser(row *sql.Row) (*User, error) {
	var u User
	var enabled int
	var created string
	if err := row.Scan(&u.ID, &u.Email, &u.UUID, &u.SubToken, &enabled, &u.ExpiryTime, &u.TotalBytes, &u.Note, &created); err != nil {
		return nil, err
	}
	u.Enabled = enabled == 1
	u.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return &u, nil
}

func scanUserRow(rows *sql.Rows) (User, error) {
	var u User
	var enabled int
	var created string
	if err := rows.Scan(&u.ID, &u.Email, &u.UUID, &u.SubToken, &enabled, &u.ExpiryTime, &u.TotalBytes, &u.Note, &created); err != nil {
		return User{}, err
	}
	u.Enabled = enabled == 1
	u.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return u, nil
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
