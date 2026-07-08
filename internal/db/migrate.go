package db

import (
	"database/sql"
	"fmt"
)

func migrate(conn *sql.DB) error {
	columns := map[string]string{
		"ip":         "TEXT NOT NULL DEFAULT ''",
		"status":     "TEXT NOT NULL DEFAULT 'ready'",
		"last_error": "TEXT NOT NULL DEFAULT ''",
	}
	for name, ddl := range columns {
		ok, err := hasColumn(conn, "nodes", name)
		if err != nil {
			return err
		}
		if ok {
			continue
		}
		if _, err := conn.Exec(fmt.Sprintf("ALTER TABLE nodes ADD COLUMN %s %s", name, ddl)); err != nil {
			return fmt.Errorf("add column nodes.%s: %w", name, err)
		}
	}
	return nil
}

func hasColumn(conn *sql.DB, table, column string) (bool, error) {
	rows, err := conn.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}
