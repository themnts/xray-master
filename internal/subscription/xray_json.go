package subscription

import (
	"fmt"

	"github.com/thethoughtcriminal/xray-master/internal/config"
	"github.com/thethoughtcriminal/xray-master/internal/nodeclient"
)

func buildSmartMultiConfig(profile config.ProfileConfig, ctx *BuildContext) (map[string]any, error) {
	var outbounds []map[string]any
	var balancers []map[string]any
	var rules []map[string]any
	balancerTag := "lb_smart"

	for i, entry := range profile.Entries {
		node, ib, err := resolveEntry(ctx, entry)
		if err != nil {
			return nil, err
		}
		tag := fmt.Sprintf("proxy-%d", i)
		ob, err := outboundFromInbound(ctx.User.UUID, node.PublicHost, ib, tag)
		if err != nil {
			return nil, err
		}
		outbounds = append(outbounds, ob)
	}

	outbounds = append(outbounds,
		map[string]any{"protocol": "freedom", "tag": "direct"},
		map[string]any{"protocol": "blackhole", "tag": "block"},
	)

	balancers = append(balancers, map[string]any{
		"tag":      balancerTag,
		"selector": outboundTags(len(profile.Entries)),
		"strategy": map[string]any{
			"type": "leastPing",
		},
	})

	rules = append(rules,
		map[string]any{"type": "field", "network": "tcp,udp", "balancerTag": balancerTag},
	)

	return map[string]any{
		"remarks": profile.Name,
		"dns": map[string]any{
			"servers":       []string{"8.8.8.8", "1.1.1.1"},
			"queryStrategy": "UseIP",
		},
		"inbounds": defaultInbounds(),
		"outbounds": outbounds,
		"routing": map[string]any{
			"domainStrategy": "IPIfNonMatch",
			"balancers":      balancers,
			"rules":          rules,
		},
		"observatory": map[string]any{
			"enable":               true,
			"subjectSelector":      outboundTags(len(profile.Entries)),
			"probeUrl":             "https://www.google.com/generate_204",
			"probeInterval":        "10s",
			"enableConcurrency":    true,
		},
	}, nil
}

func buildSingleConfig(profile config.ProfileConfig, entry config.ProfileEntry, ctx *BuildContext) (map[string]any, error) {
	node, ib, err := resolveEntry(ctx, entry)
	if err != nil {
		return nil, err
	}
	remarks := entry.Label
	if remarks == "" {
		remarks = profile.Name
	}
	proxy, err := outboundFromInbound(ctx.User.UUID, node.PublicHost, ib, "proxy")
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"remarks": remarks,
		"dns": map[string]any{
			"servers":       []string{"8.8.8.8", "1.1.1.1"},
			"queryStrategy": "UseIP",
		},
		"inbounds": defaultInbounds(),
		"outbounds": []map[string]any{
			proxy,
			{"protocol": "freedom", "tag": "direct"},
			{"protocol": "blackhole", "tag": "block"},
		},
		"routing": map[string]any{
			"domainStrategy": "IPIfNonMatch",
			"rules": []map[string]any{
				{"type": "field", "network": "tcp,udp", "outboundTag": "proxy"},
			},
		},
	}, nil
}

func defaultInbounds() []map[string]any {
	return []map[string]any{
		{
			"listen":   "127.0.0.1",
			"port":     10808,
			"protocol": "socks",
			"settings": map[string]any{
				"auth": "noauth",
				"udp":  true,
			},
			"sniffing": map[string]any{
				"enabled":      true,
				"destOverride": []string{"http", "tls", "quic"},
			},
			"tag": "socks",
		},
		{
			"listen":   "127.0.0.1",
			"port":     10809,
			"protocol": "http",
			"settings": map[string]any{},
			"tag":      "http",
		},
	}
}

func outboundFromInbound(userUUID, host string, ib nodeclient.Inbound, tag string) (map[string]any, error) {
	switch ib.Protocol {
	case "vless":
		stream, err := ib.StreamSettings.Map()
		if err != nil {
			return nil, err
		}
		rs, _ := stream["realitySettings"].(map[string]any)
		settings, _ := rs["settings"].(map[string]any)
		return map[string]any{
			"protocol": "vless",
			"tag":      tag,
			"settings": map[string]any{
				"vnext": []map[string]any{
					{
						"address": host,
						"port":    ib.Port,
						"users": []map[string]any{
							{
								"id":         userUUID,
								"encryption": "none",
								"flow":       "xtls-rprx-vision",
							},
						},
					},
				},
			},
			"streamSettings": map[string]any{
				"network":  "tcp",
				"security": "reality",
				"realitySettings": map[string]any{
					"serverName":  firstString(rs["serverNames"]),
					"publicKey":   settings["publicKey"],
					"fingerprint": settings["fingerprint"],
					"shortId":     firstShortID(rs["shortIds"]),
				},
			},
		}, nil
	case "hysteria":
		stream, err := ib.StreamSettings.Map()
		if err != nil {
			return nil, err
		}
		tls, _ := stream["tlsSettings"].(map[string]any)
		return map[string]any{
			"protocol": "hysteria",
			"tag":      tag,
			"settings": map[string]any{
				"servers": []map[string]any{
					{
						"address": host,
						"port":    ib.Port,
						"users": []map[string]any{
							{
								"auth": userUUID,
							},
						},
					},
				},
			},
			"streamSettings": map[string]any{
				"network":  "hysteria",
				"security": "tls",
				"tlsSettings": map[string]any{
					"serverName": tls["serverName"],
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported protocol %q", ib.Protocol)
	}
}

func outboundTags(n int) []string {
	tags := make([]string, n)
	for i := 0; i < n; i++ {
		tags[i] = fmt.Sprintf("proxy-%d", i)
	}
	return tags
}
