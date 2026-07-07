package subscription

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/thethoughtcriminal/xray-master/internal/config"
	"github.com/thethoughtcriminal/xray-master/internal/db"
	"github.com/thethoughtcriminal/xray-master/internal/nodeclient"
)

type UserInfo struct {
	Email string
	UUID  string
}

type NodeInfo struct {
	Name       string
	PublicHost string
	Inbounds   map[string]nodeclient.Inbound
}

type BuildContext struct {
	User  UserInfo
	Nodes map[string]NodeInfo
}

type TrafficSummary struct {
	Up    int64
	Down  int64
	Total int64
	Expire int64
}

type Builder struct {
	cfg config.SubscriptionConfig
}

func NewBuilder(cfg config.SubscriptionConfig) *Builder {
	return &Builder{cfg: cfg}
}

func IsHappUA(ua string) bool {
	return strings.Contains(strings.ToLower(ua), "happ")
}

func Headers(user *db.User, updateHours int, stats TrafficSummary) map[string]string {
	h := map[string]string{
		"profile-update-interval": fmt.Sprintf("%d", updateHours),
		"routing-enable":          "true",
	}
	if stats.Total > 0 || stats.Expire > 0 {
		h["subscription-userinfo"] = fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d",
			stats.Up, stats.Down, stats.Total, stats.Expire/1000)
	} else if user.TotalBytes > 0 || user.ExpiryTime > 0 {
		h["subscription-userinfo"] = fmt.Sprintf("upload=0; download=0; total=%d; expire=%d",
			user.TotalBytes, user.ExpiryTime/1000)
	}
	return h
}

func (b *Builder) BuildBase64Links(ctx *BuildContext) ([]byte, error) {
	var lines []string
	for _, profile := range b.cfg.Profiles {
		switch profile.Mode {
		case "smart_multi":
			for _, entry := range profile.Entries {
				link, err := b.buildLink(ctx, profile, entry)
				if err != nil {
					return nil, err
				}
				lines = append(lines, link)
			}
		case "single":
			for _, entry := range profile.Entries {
				link, err := b.buildLink(ctx, profile, entry)
				if err != nil {
					return nil, err
				}
				lines = append(lines, link)
			}
		default:
			return nil, fmt.Errorf("unknown profile mode %q", profile.Mode)
		}
	}
	payload := strings.Join(lines, "\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(payload))
	return []byte(encoded), nil
}

func (b *Builder) BuildHappJSON(ctx *BuildContext) ([]byte, error) {
	var configs []map[string]any
	for _, profile := range b.cfg.Profiles {
		switch profile.Mode {
		case "smart_multi":
			cfg, err := buildSmartMultiConfig(profile, ctx)
			if err != nil {
				return nil, err
			}
			configs = append(configs, cfg)
		case "single":
			for _, entry := range profile.Entries {
				cfg, err := buildSingleConfig(profile, entry, ctx)
				if err != nil {
					return nil, err
				}
				configs = append(configs, cfg)
			}
		default:
			return nil, fmt.Errorf("unknown profile mode %q", profile.Mode)
		}
	}
	return json.Marshal(configs)
}

func (b *Builder) buildLink(ctx *BuildContext, profile config.ProfileConfig, entry config.ProfileEntry) (string, error) {
	node, ib, err := resolveEntry(ctx, entry)
	if err != nil {
		return "", err
	}
	label := entry.Label
	if label == "" {
		label = profile.Name
	}
	if profile.Mode == "smart_multi" {
		label = profile.Name + " · " + label
	}
	switch ib.Protocol {
	case "vless":
		return buildVLESSLink(ctx.User.UUID, node.PublicHost, ib, label)
	case "hysteria":
		return buildHysteria2Link(ctx.User.UUID, node.PublicHost, ib, label)
	default:
		return "", fmt.Errorf("unsupported protocol %q on %s/%s", ib.Protocol, entry.Node, entry.Inbound)
	}
}

func resolveEntry(ctx *BuildContext, entry config.ProfileEntry) (NodeInfo, nodeclient.Inbound, error) {
	node, ok := ctx.Nodes[entry.Node]
	if !ok {
		return NodeInfo{}, nodeclient.Inbound{}, fmt.Errorf("node %q not found or disabled", entry.Node)
	}
	ib, ok := node.Inbounds[entry.Inbound]
	if !ok {
		return NodeInfo{}, nodeclient.Inbound{}, fmt.Errorf("inbound %q not found on node %q", entry.Inbound, entry.Node)
	}
	return node, ib, nil
}

func buildVLESSLink(userUUID, host string, ib nodeclient.Inbound, label string) (string, error) {
	stream, err := ib.StreamSettings.Map()
	if err != nil {
		return "", err
	}
	rs, _ := stream["realitySettings"].(map[string]any)
	if rs == nil {
		return "", fmt.Errorf("inbound %q has no realitySettings", ib.Remark)
	}
	settings, _ := rs["settings"].(map[string]any)
	pubKey, _ := settings["publicKey"].(string)
	fp, _ := settings["fingerprint"].(string)
	sni := firstString(rs["serverNames"])
	shortID := firstShortID(rs["shortIds"])
	q := url.Values{}
	q.Set("encryption", "none")
	q.Set("flow", "xtls-rprx-vision")
	q.Set("security", "reality")
	q.Set("sni", sni)
	q.Set("fp", fp)
	q.Set("pbk", pubKey)
	q.Set("sid", shortID)
	q.Set("type", "tcp")
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", userUUID, host, ib.Port, q.Encode(), url.PathEscape(label)), nil
}

func buildHysteria2Link(userUUID, host string, ib nodeclient.Inbound, label string) (string, error) {
	stream, err := ib.StreamSettings.Map()
	if err != nil {
		return "", err
	}
	tls, _ := stream["tlsSettings"].(map[string]any)
	sni, _ := tls["serverName"].(string)
	q := url.Values{}
	q.Set("sni", sni)
	q.Set("insecure", "0")
	return fmt.Sprintf("hysteria2://%s@%s:%d?%s#%s", userUUID, host, ib.Port, q.Encode(), url.PathEscape(label)), nil
}

func firstString(v any) string {
	switch t := v.(type) {
	case []any:
		if len(t) > 0 {
			if s, ok := t[0].(string); ok {
				return s
			}
		}
	case []string:
		if len(t) > 0 {
			return t[0]
		}
	}
	return ""
}

func firstShortID(v any) string {
	switch t := v.(type) {
	case []any:
		for _, item := range t {
			if s, ok := item.(string); ok && s != "" {
				return s
			}
		}
	case []string:
		for _, s := range t {
			if s != "" {
				return s
			}
		}
	}
	return ""
}
