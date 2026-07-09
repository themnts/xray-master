package db

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

type EnrollToken struct {
	ID        string
	NodeName  string
	TokenHash string
	MasterIP  string
	ExpiresAt time.Time
	UsedAt    time.Time
	CreatedAt time.Time
}

func HashEnrollToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func CreateEnrollToken(conn *sql.DB, nodeName, masterIP string, ttl time.Duration) (string, *EnrollToken, error) {
	if nodeName == "" {
		return "", nil, fmt.Errorf("node name is required")
	}
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	plain := hex.EncodeToString(b)
	now := time.Now().UTC()
	rec := EnrollToken{
		ID:        hex.EncodeToString(mustRand(16)),
		NodeName:  nodeName,
		TokenHash: HashEnrollToken(plain),
		MasterIP:  masterIP,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
	}
	_, err := conn.Exec(
		`INSERT INTO enroll_tokens (id, node_name, token_hash, master_ip, expires_at, used_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rec.ID, rec.NodeName, rec.TokenHash, rec.MasterIP, rec.ExpiresAt.Format(time.RFC3339), "",
		rec.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return "", nil, err
	}
	return plain, &rec, nil
}

func ConsumeEnrollToken(conn *sql.DB, plain string) (*EnrollToken, error) {
	hash := HashEnrollToken(plain)
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := conn.Exec(
		`UPDATE enroll_tokens SET used_at = ?
		 WHERE token_hash = ? AND used_at = '' AND expires_at > ?`,
		now, hash, now,
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, sql.ErrNoRows
	}
	row := conn.QueryRow(
		`SELECT id, node_name, token_hash, master_ip, expires_at, used_at, created_at
		 FROM enroll_tokens WHERE token_hash = ?`, hash,
	)
	return scanEnrollToken(row)
}

func scanEnrollToken(row *sql.Row) (*EnrollToken, error) {
	var t EnrollToken
	var expires, used, created string
	if err := row.Scan(&t.ID, &t.NodeName, &t.TokenHash, &t.MasterIP, &expires, &used, &created); err != nil {
		return nil, err
	}
	t.ExpiresAt, _ = time.Parse(time.RFC3339, expires)
	if used != "" {
		t.UsedAt, _ = time.Parse(time.RFC3339, used)
	}
	t.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return &t, nil
}

func mustRand(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}
