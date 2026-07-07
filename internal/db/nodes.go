package db

import (
	"database/sql"
	"fmt"
	"time"
)

type Node struct {
	ID         string
	Name       string
	APIURL     string
	APIKey     string
	PublicHost string
	Enabled    bool
	CreatedAt  time.Time
}

func CreateNode(conn *sql.DB, n Node) error {
	if n.ID == "" || n.Name == "" || n.APIURL == "" || n.APIKey == "" || n.PublicHost == "" {
		return fmt.Errorf("node id, name, api_url, api_key, and public_host are required")
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	_, err := conn.Exec(
		`INSERT INTO nodes (id, name, api_url, api_key, public_host, enabled, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.Name, n.APIURL, n.APIKey, n.PublicHost, boolToInt(n.Enabled), n.CreatedAt.Format(time.RFC3339),
	)
	return err
}

func ListNodes(conn *sql.DB) ([]Node, error) {
	rows, err := conn.Query(`SELECT id, name, api_url, api_key, public_host, enabled, created_at FROM nodes ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Node
	for rows.Next() {
		var n Node
		var enabled int
		var created string
		if err := rows.Scan(&n.ID, &n.Name, &n.APIURL, &n.APIKey, &n.PublicHost, &enabled, &created); err != nil {
			return nil, err
		}
		n.Enabled = enabled == 1
		n.CreatedAt, _ = time.Parse(time.RFC3339, created)
		out = append(out, n)
	}
	return out, rows.Err()
}

func GetNodeByName(conn *sql.DB, name string) (*Node, error) {
	row := conn.QueryRow(`SELECT id, name, api_url, api_key, public_host, enabled, created_at FROM nodes WHERE name = ?`, name)
	return scanNode(row)
}

func GetNodeByID(conn *sql.DB, id string) (*Node, error) {
	row := conn.QueryRow(`SELECT id, name, api_url, api_key, public_host, enabled, created_at FROM nodes WHERE id = ?`, id)
	return scanNode(row)
}

func DeleteNode(conn *sql.DB, id string) error {
	res, err := conn.Exec(`DELETE FROM nodes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func SetNodeEnabled(conn *sql.DB, id string, enabled bool) error {
	res, err := conn.Exec(`UPDATE nodes SET enabled = ? WHERE id = ?`, boolToInt(enabled), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func scanNode(row *sql.Row) (*Node, error) {
	var n Node
	var enabled int
	var created string
	if err := row.Scan(&n.ID, &n.Name, &n.APIURL, &n.APIKey, &n.PublicHost, &enabled, &created); err != nil {
		return nil, err
	}
	n.Enabled = enabled == 1
	n.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return &n, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
