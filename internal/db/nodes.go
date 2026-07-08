package db

import (
	"database/sql"
	"fmt"
	"time"
)

const (
	NodeStatusReady        = "ready"
	NodeStatusProvisioning = "provisioning"
	NodeStatusError        = "error"
)

type Node struct {
	ID         string
	Name       string
	IP         string
	APIURL     string
	APIKey     string
	PublicHost string
	Enabled    bool
	Status     string
	LastError  string
	CreatedAt  time.Time
}

func CreateNode(conn *sql.DB, n Node) error {
	if n.ID == "" || n.Name == "" || n.PublicHost == "" {
		return fmt.Errorf("node id, name, and public_host are required")
	}
	if n.Status == "" {
		n.Status = NodeStatusReady
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	_, err := conn.Exec(
		`INSERT INTO nodes (id, name, ip, api_url, api_key, public_host, enabled, status, last_error, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.Name, n.IP, n.APIURL, n.APIKey, n.PublicHost, boolToInt(n.Enabled), n.Status, n.LastError, n.CreatedAt.Format(time.RFC3339),
	)
	return err
}

func UpdateNode(conn *sql.DB, n Node) error {
	res, err := conn.Exec(
		`UPDATE nodes SET ip = ?, api_url = ?, api_key = ?, public_host = ?, enabled = ?, status = ?, last_error = ?
		 WHERE id = ?`,
		n.IP, n.APIURL, n.APIKey, n.PublicHost, boolToInt(n.Enabled), n.Status, n.LastError, n.ID,
	)
	if err != nil {
		return err
	}
	count, _ := res.RowsAffected()
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func ListNodes(conn *sql.DB) ([]Node, error) {
	rows, err := conn.Query(`SELECT id, name, ip, api_url, api_key, public_host, enabled, status, last_error, created_at FROM nodes ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Node
	for rows.Next() {
		n, err := scanNodeRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func GetNodeByName(conn *sql.DB, name string) (*Node, error) {
	row := conn.QueryRow(`SELECT id, name, ip, api_url, api_key, public_host, enabled, status, last_error, created_at FROM nodes WHERE name = ?`, name)
	return scanNode(row)
}

func GetNodeByID(conn *sql.DB, id string) (*Node, error) {
	row := conn.QueryRow(`SELECT id, name, ip, api_url, api_key, public_host, enabled, status, last_error, created_at FROM nodes WHERE id = ?`, id)
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
	if err := row.Scan(&n.ID, &n.Name, &n.IP, &n.APIURL, &n.APIKey, &n.PublicHost, &enabled, &n.Status, &n.LastError, &created); err != nil {
		return nil, err
	}
	n.Enabled = enabled == 1
	n.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return &n, nil
}

func scanNodeRow(rows *sql.Rows) (Node, error) {
	var n Node
	var enabled int
	var created string
	if err := rows.Scan(&n.ID, &n.Name, &n.IP, &n.APIURL, &n.APIKey, &n.PublicHost, &enabled, &n.Status, &n.LastError, &created); err != nil {
		return Node{}, err
	}
	n.Enabled = enabled == 1
	n.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return n, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
