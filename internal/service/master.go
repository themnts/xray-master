package service

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/thethoughtcriminal/xray-master/internal/config"
	"github.com/thethoughtcriminal/xray-master/internal/db"
	"github.com/thethoughtcriminal/xray-master/internal/nodeclient"
	"github.com/thethoughtcriminal/xray-master/internal/subscription"
)

const MasterClientComment = "managed by xray-master"

type Master struct {
	cfg  *config.Config
	conn *sql.DB
}

func New(cfg *config.Config, conn *sql.DB) *Master {
	return &Master{cfg: cfg, conn: conn}
}

type AddNodeInput struct {
	Name       string
	IP         string
	APIURL     string
	APIKey     string
	PublicHost string
}

func (m *Master) AddNode(in AddNodeInput) (*db.Node, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if strings.TrimSpace(in.IP) != "" {
		return nil, fmt.Errorf("%w: use 'node token create' and run xray-node join on the VPS (self-registration)", ErrValidation)
	}
	if _, err := db.GetNodeByName(m.conn, in.Name); err == nil {
		return nil, fmt.Errorf("%w: node %q already exists", ErrConflict, in.Name)
	} else if err != sql.ErrNoRows {
		return nil, err
	}
	return m.addNodeManual(in)
}

func (m *Master) addNodeManual(in AddNodeInput) (*db.Node, error) {
	if in.APIURL == "" || in.APIKey == "" || in.PublicHost == "" {
		return nil, fmt.Errorf("%w: api_url, api_key, and public_host are required (or use node token create + xray-node join)", ErrValidation)
	}
	node := db.Node{
		ID:         uuid.NewString(),
		Name:       in.Name,
		IP:         in.IP,
		APIURL:     strings.TrimRight(in.APIURL, "/"),
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

func (m *Master) ListNodes() ([]db.Node, error) {
	return db.ListNodes(m.conn)
}

func (m *Master) DeleteNode(id string) error {
	if err := db.DeleteNode(m.conn, id); err == sql.ErrNoRows {
		return fmt.Errorf("%w: node not found", ErrNotFound)
	} else if err != nil {
		return err
	}
	return nil
}

type AddUserInput struct {
	Email      string
	UUID       string
	ExpiryTime int64
	TotalBytes int64
	Note       string
}

type AddUserResult struct {
	User       db.User           `json:"user"`
	NodeErrors map[string]string `json:"node_errors,omitempty"`
}

func (m *Master) AddUser(in AddUserInput) (*AddUserResult, error) {
	if in.Email == "" {
		return nil, fmt.Errorf("%w: email is required", ErrValidation)
	}
	if _, err := db.GetUserByEmail(m.conn, in.Email); err == nil {
		return nil, fmt.Errorf("%w: user %q already exists", ErrConflict, in.Email)
	} else if err != sql.ErrNoRows {
		return nil, err
	}
	userUUID := in.UUID
	if userUUID == "" {
		userUUID = uuid.NewString()
	}
	user := db.User{
		ID:         uuid.NewString(),
		Email:      in.Email,
		UUID:       userUUID,
		Enabled:    true,
		ExpiryTime: in.ExpiryTime,
		TotalBytes: in.TotalBytes,
		Note:       in.Note,
	}
	if err := db.CreateUser(m.conn, user); err != nil {
		return nil, err
	}
	nodeErrors := m.syncUserToNodes(&user, true)
	return &AddUserResult{User: user, NodeErrors: nodeErrors}, nil
}

func (m *Master) ListUsers() ([]db.User, error) {
	return db.ListUsers(m.conn)
}

type SyncUsersResult struct {
	UsersSynced int               `json:"users_synced"`
	NodeErrors  map[string]string `json:"node_errors,omitempty"`
}

func (m *Master) SyncAllUsers() (*SyncUsersResult, error) {
	users, err := db.ListUsers(m.conn)
	if err != nil {
		return nil, err
	}
	result := &SyncUsersResult{NodeErrors: map[string]string{}}
	for _, user := range users {
		if !user.Enabled {
			continue
		}
		errs := m.syncUserToNodes(&user, true)
		if len(errs) == 0 {
			result.UsersSynced++
			continue
		}
		for node, msg := range errs {
			result.NodeErrors[user.Email+"/"+node] = msg
		}
	}
	if len(result.NodeErrors) == 0 {
		result.NodeErrors = nil
	}
	return result, nil
}

func (m *Master) SetUserEnabled(id string, enabled bool) error {
	user, err := db.GetUserByID(m.conn, id)
	if err == sql.ErrNoRows {
		return fmt.Errorf("%w: user not found", ErrNotFound)
	} else if err != nil {
		return err
	}
	if err := db.SetUserEnabled(m.conn, id, enabled); err != nil {
		return err
	}
	user.Enabled = enabled
	m.syncUserToNodes(user, enabled)
	return nil
}

func (m *Master) DeleteUser(id string) error {
	user, err := db.GetUserByID(m.conn, id)
	if err == sql.ErrNoRows {
		return fmt.Errorf("%w: user not found", ErrNotFound)
	} else if err != nil {
		return err
	}
	if err := db.DeleteUser(m.conn, id); err != nil {
		return err
	}
	m.syncUserToNodes(user, false)
	return nil
}

type UserStats struct {
	Email      string                 `json:"email"`
	Up         int64                  `json:"up"`
	Down       int64                  `json:"down"`
	ByNode     map[string]NodeTraffic `json:"by_node"`
	NodeErrors map[string]string      `json:"node_errors,omitempty"`
}

type NodeTraffic struct {
	Inbound string `json:"inbound"`
	Up      int64  `json:"up"`
	Down    int64  `json:"down"`
}

func (m *Master) UserStats(email string) (*UserStats, error) {
	user, err := db.GetUserByEmail(m.conn, email)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: user not found", ErrNotFound)
	} else if err != nil {
		return nil, err
	}
	stats := &UserStats{
		Email:  user.Email,
		ByNode: map[string]NodeTraffic{},
	}
	nodeErrors := map[string]string{}
	entries, err := m.profileEntries()
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		client := nodeclient.New(entry.Node.APIURL, entry.Node.APIKey)
		traffic, err := client.ClientStats(entry.Inbound, user.Email)
		if err != nil {
			nodeErrors[entry.Node.Name] = err.Error()
			continue
		}
		prev := stats.ByNode[entry.Node.Name]
		prev.Inbound = entry.Inbound
		prev.Up += traffic.Up
		prev.Down += traffic.Down
		stats.ByNode[entry.Node.Name] = prev
		stats.Up += traffic.Up
		stats.Down += traffic.Down
	}
	if len(nodeErrors) > 0 {
		stats.NodeErrors = nodeErrors
	}
	return stats, nil
}

type SubscriptionResult struct {
	Format  string `json:"format"`
	Body    []byte `json:"-"`
	Headers map[string]string
}

func (m *Master) BuildSubscription(subToken string, userAgent string) (*SubscriptionResult, error) {
	user, err := db.GetUserBySubToken(m.conn, subToken)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: subscription not found", ErrNotFound)
	} else if err != nil {
		return nil, err
	}
	if !user.Enabled {
		return nil, fmt.Errorf("%w: subscription disabled", ErrValidation)
	}
	if user.ExpiryTime > 0 && time.Now().UnixMilli() > user.ExpiryTime {
		return nil, fmt.Errorf("%w: subscription expired", ErrValidation)
	}

	ctx, err := m.loadBuildContext(user)
	if err != nil {
		return nil, err
	}
	builder := subscription.NewBuilder(m.cfg.Subscription)

	if subscription.IsHappUA(userAgent) {
		body, err := builder.BuildHappJSON(ctx)
		if err != nil {
			return nil, err
		}
		return &SubscriptionResult{
			Format:  "happ_json",
			Body:    body,
			Headers: subscription.Headers(user, m.cfg.Subscription.UpdateIntervalHours, statsFromContext(ctx)),
		}, nil
	}

	body, err := builder.BuildBase64Links(ctx)
	if err != nil {
		return nil, err
	}
	return &SubscriptionResult{
		Format:  "base64_links",
		Body:    body,
		Headers: subscription.Headers(user, m.cfg.Subscription.UpdateIntervalHours, statsFromContext(ctx)),
	}, nil
}

type profileEntry struct {
	Node    db.Node
	Inbound string
}

func (m *Master) profileEntries() ([]profileEntry, error) {
	nodes, err := db.ListNodes(m.conn)
	if err != nil {
		return nil, err
	}
	byName := map[string]db.Node{}
	for _, n := range nodes {
		byName[n.Name] = n
	}

	seen := map[string]struct{}{}
	var out []profileEntry
	for _, profile := range m.cfg.Subscription.Profiles {
		for _, entry := range profile.Entries {
			key := entry.Node + "/" + entry.Inbound
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			node, ok := byName[entry.Node]
			if !ok {
				continue
			}
			if !node.Enabled || node.Status != db.NodeStatusReady {
				continue
			}
			out = append(out, profileEntry{Node: node, Inbound: entry.Inbound})
		}
	}
	return out, nil
}

type syncTarget struct {
	Node    db.Node
	Inbound string
}

// syncTargets returns every enabled inbound on every registered ready node.
// Subscription profiles do not affect provisioning.
func (m *Master) syncTargets() ([]syncTarget, map[string]string) {
	nodes, err := db.ListNodes(m.conn)
	if err != nil {
		return nil, map[string]string{"_db": err.Error()}
	}
	nodeErrors := map[string]string{}
	var out []syncTarget
	for _, node := range nodes {
		if !node.Enabled || node.Status != db.NodeStatusReady {
			continue
		}
		client := nodeclient.New(node.APIURL, node.APIKey)
		inbounds, err := client.ListInbounds()
		if err != nil {
			nodeErrors[node.Name] = err.Error()
			continue
		}
		for _, ib := range inbounds {
			if !ib.Enable {
				continue
			}
			out = append(out, syncTarget{Node: node, Inbound: ib.Remark})
		}
	}
	return out, nodeErrors
}

func (m *Master) syncUserToNodes(user *db.User, enable bool) map[string]string {
	errs := map[string]string{}
	targets, nodeErrors := m.syncTargets()
	for k, v := range nodeErrors {
		errs[k+"/_connect"] = v
	}
	for _, target := range targets {
		client := nodeclient.New(target.Node.APIURL, target.Node.APIKey)
		key := target.Node.Name + "/" + target.Inbound
		if enable {
			if _, err := client.AddClient(nodeclient.AddClientRequest{
				InboundRemark: target.Inbound,
				Email:         user.Email,
				UUID:          user.UUID,
				Comment:       MasterClientComment,
			}); err != nil {
				errs[key] = err.Error()
			}
		} else {
			if err := client.SetClientEnabled(target.Inbound, user.Email, false); err != nil {
				errs[key] = err.Error()
			}
		}
	}
	return errs
}

func (m *Master) loadBuildContext(user *db.User) (*subscription.BuildContext, error) {
	ctx := &subscription.BuildContext{
		User: subscription.UserInfo{
			Email: user.Email,
			UUID:  user.UUID,
		},
		Nodes: map[string]subscription.NodeInfo{},
	}
	nodes, err := db.ListNodes(m.conn)
	if err != nil {
		return nil, err
	}
	for _, node := range nodes {
		if !node.Enabled || node.Status != db.NodeStatusReady {
			continue
		}
		client := nodeclient.New(node.APIURL, node.APIKey)
		inbounds, err := client.ListInbounds()
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", node.Name, err)
		}
		byRemark := map[string]nodeclient.Inbound{}
		for _, ib := range inbounds {
			byRemark[ib.Remark] = ib
		}
		ctx.Nodes[node.Name] = subscription.NodeInfo{
			Name:       node.Name,
			PublicHost: node.PublicHost,
			Inbounds:   byRemark,
		}
	}
	return ctx, nil
}

func statsFromContext(ctx *subscription.BuildContext) subscription.TrafficSummary {
	return subscription.TrafficSummary{}
}
