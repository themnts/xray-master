# xray-master

Subscription master server for a cluster of [xray-node](https://github.com/thethoughtcriminal/xray-node) VPN nodes.

- Register nodes and manage them via Admin API / CLI
- Add a user once → provisioned on all nodes (same email + UUID)
- Public subscription URL with content negotiation:
  - default: base64 `vless://` / `hysteria2://` links
  - `User-Agent: Happ/*`: JSON array of full Xray configs
- Profile groups in config: `smart_multi` (balancer + observatory) or `single` (one outbound per config)

**Technical spec:** [docs/TECHNICAL.md](docs/TECHNICAL.md)

## Quick start

```bash
go build -o bin/xray-master ./cmd/xray-master
cp configs/config.example.yaml /etc/xray-master/config.yaml
# edit admin_key, public_url, profiles

xray-master serve --config /etc/xray-master/config.yaml
```

## Register a node

Each VPS runs `xray-node` (API on localhost, reachable via SSH tunnel or reverse proxy).

```bash
xray-master node add \
  --name nl-1 \
  --api-url http://127.0.0.1:9472 \
  --api-key NODE_API_KEY \
  --public-host nl.example.com
```

## Add user

```bash
xray-master user add --email user@example.com
# prints subscription URL: https://sub.example.com/sub/<token>
```

## Subscription endpoint

| Client | Request | Response |
|--------|---------|----------|
| Happ | `GET /sub/{token}` + `User-Agent: Happ/1.0` | JSON array of Xray configs |
| Others | `GET /sub/{token}` | base64-encoded share links |

## Admin API

Auth: `X-Admin-Key: <server.admin_key>`

| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Health check |
| GET | `/sub/{token}` | Public subscription (no auth) |
| GET | `/nodes` | List nodes |
| POST | `/nodes` | Register node |
| DELETE | `/nodes/{id}` | Remove node |
| GET | `/users` | List users |
| POST | `/users` | Add user on all nodes |
| GET | `/users/{email}/stats` | Aggregate traffic |
| POST | `/users/{id}/enable` | Enable user |
| POST | `/users/{id}/disable` | Disable user |
| DELETE | `/users/{id}` | Delete user |

## Development

```bash
go test ./...
go build -o bin/xray-master ./cmd/xray-master
```

## License

MIT
