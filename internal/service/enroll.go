package service

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/thethoughtcriminal/xray-master/internal/db"
	"github.com/thethoughtcriminal/xray-master/internal/nodeclient"
)

type CreateEnrollTokenInput struct {
	Name     string
	TTLHours int
}

type CreateEnrollTokenResult struct {
	Token     string    `json:"token"`
	Name      string    `json:"name"`
	ExpiresAt time.Time `json:"expires_at"`
	MasterURL string    `json:"master_url"`
	JoinCmd   string    `json:"join_command"`
}

func (m *Master) CreateEnrollToken(in CreateEnrollTokenInput) (*CreateEnrollTokenResult, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if _, err := db.GetNodeByName(m.conn, in.Name); err == nil {
		return nil, fmt.Errorf("%w: node %q already registered", ErrConflict, in.Name)
	} else if err != sql.ErrNoRows {
		return nil, err
	}
	ttl := time.Duration(in.TTLHours) * time.Hour
	if ttl <= 0 {
		ttl = time.Duration(m.cfg.Enroll.EnrollTTLHours) * time.Hour
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	masterIP := strings.TrimSpace(m.cfg.Enroll.MasterIP)
	plain, rec, err := db.CreateEnrollToken(m.conn, in.Name, masterIP, ttl)
	if err != nil {
		return nil, err
	}
	masterURL := strings.TrimRight(m.cfg.Server.PublicURL, "/")
	joinCmd := fmt.Sprintf(
		"xray-node join --master-url %s --token %s --name %s",
		masterURL, plain, in.Name,
	)
	if masterIP != "" {
		joinCmd += fmt.Sprintf(" --master-ip %s", masterIP)
	}
	return &CreateEnrollTokenResult{
		Token:     plain,
		Name:      rec.NodeName,
		ExpiresAt: rec.ExpiresAt,
		MasterURL: masterURL,
		JoinCmd:   joinCmd,
	}, nil
}

type EnrollNodeInput struct {
	Token      string
	Name       string
	APIURL     string
	APIKey     string
	PublicHost string
	IP         string
}

func (m *Master) EnrollNode(in EnrollNodeInput) (*db.Node, error) {
	if in.Token == "" {
		return nil, fmt.Errorf("%w: token is required", ErrValidation)
	}
	if in.APIURL == "" || in.APIKey == "" || in.PublicHost == "" {
		return nil, fmt.Errorf("%w: api_url, api_key, and public_host are required", ErrValidation)
	}

	tokenRec, err := db.ConsumeEnrollToken(m.conn, in.Token)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: invalid or expired enroll token", ErrValidation)
	} else if err != nil {
		return nil, err
	}

	name := tokenRec.NodeName
	if in.Name != "" && in.Name != name {
		return nil, fmt.Errorf("%w: token is for node %q, not %q", ErrValidation, name, in.Name)
	}
	if _, err := db.GetNodeByName(m.conn, name); err == nil {
		return nil, fmt.Errorf("%w: node %q already registered", ErrConflict, name)
	} else if err != sql.ErrNoRows {
		return nil, err
	}

	apiURL := strings.TrimRight(in.APIURL, "/")
	client := nodeclient.New(apiURL, in.APIKey)
	if _, err := client.ListInbounds(); err != nil {
		return nil, fmt.Errorf("%w: cannot reach node API at %s: %v", ErrValidation, apiURL, err)
	}

	ip := strings.TrimSpace(in.IP)
	if ip == "" {
		ip = hostFromURL(apiURL)
	}

	node := db.Node{
		ID:         uuid.NewString(),
		Name:       name,
		IP:         ip,
		APIURL:     apiURL,
		APIKey:     in.APIKey,
		PublicHost: in.PublicHost,
		Enabled:    true,
		Status:     db.NodeStatusReady,
	}
	if err := db.CreateNode(m.conn, node); err != nil {
		return nil, err
	}
	return &node, nil
}

func hostFromURL(apiURL string) string {
	apiURL = strings.TrimPrefix(apiURL, "http://")
	apiURL = strings.TrimPrefix(apiURL, "https://")
	if i := strings.Index(apiURL, "/"); i >= 0 {
		apiURL = apiURL[:i]
	}
	if i := strings.Index(apiURL, ":"); i >= 0 {
		return apiURL[:i]
	}
	return apiURL
}
