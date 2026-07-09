# xray-master

Subscription master server for a cluster of [xray-node](https://github.com/themnts/xray-node) VPN nodes.

- Register nodes and manage them via Admin API / CLI
- Add a user once → provisioned on all nodes (same email + UUID)
- Public subscription URL with content negotiation:
  - default: base64 `vless://` / `hysteria2://` links
  - `User-Agent: Happ/*`: JSON array of full Xray configs
- Profile groups in config: `smart_multi` (balancer + observatory) or `single` (one outbound per config)

**Technical spec:** [docs/TECHNICAL.md](docs/TECHNICAL.md)  
**Configuration:** [docs/CONFIG.md](docs/CONFIG.md)  
**HTTP API:** [docs/API.md](docs/API.md) · [OpenAPI](docs/openapi.yaml)

## Install (production)

On a dedicated VPS (Ubuntu/Debian):

```bash
curl -fsSL https://raw.githubusercontent.com/themnts/xray-master/main/scripts/install.sh | sudo bash
```

With HTTPS reverse proxy (Caddy) — DNS for the domain must already point to this server:

```bash
curl -fsSL https://raw.githubusercontent.com/themnts/xray-master/main/scripts/install.sh | \
  sudo XRAY_MASTER_PUBLIC_URL=https://sub.example.com XRAY_MASTER_INSTALL_CADDY=1 bash
```

The installer builds the binary, writes `/etc/xray-master/config.yaml` and `/etc/xray-master/subscription.yaml` (generates `admin_key`), creates a systemd service, and listens on `127.0.0.1:9480` by default.

Uninstall:

```bash
curl -fsSL https://raw.githubusercontent.com/themnts/xray-master/main/scripts/uninstall.sh | sudo bash -s -- --yes
```

## Quick start (development)

```bash
go build -o bin/xray-master ./cmd/xray-master
cp configs/config.example.yaml /etc/xray-master/config.yaml
# edit admin_key, public_url, profiles

xray-master serve --config /etc/xray-master/config.yaml
```

## Register a node (self-enrollment)

**Step 1 — on master:** create a one-time enroll token:

```bash
xray-master node token create --name nl-1
# prints token + join command
```

**Step 2 — on the VPS:** install xray-node standalone (no master required):

```bash
curl -fsSL https://raw.githubusercontent.com/themnts/xray-node/main/scripts/install.sh | sudo bash
```

**Step 3 — on the VPS:** join the master (now or later):

```bash
xray-node join --master-url https://sub.example.com --token TOKEN --name nl-1
# or: curl -fsSL .../join.sh | sudo MASTER_URL=... ENROLL_TOKEN=... NODE_NAME=nl-1 bash
```

**Step 4 — on master:** sync users and edit subscription profiles when needed:

```bash
xray-master sync users
nano /etc/xray-master/subscription.yaml
systemctl restart xray-master
```

Manual registration (without enroll token) is still supported:

```bash
xray-master node add --name nl-1 \
  --api-url http://203.0.113.10:9472 \
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

Full contract with request/response schemas: **[docs/API.md](docs/API.md)**  
OpenAPI 3 spec for Swagger/Postman: **[docs/openapi.yaml](docs/openapi.yaml)**

## Development

```bash
go test ./...
go build -o bin/xray-master ./cmd/xray-master
```

## License

MIT
