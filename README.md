# xray-master

Subscription master server for a cluster of [xray-node](https://github.com/thethoughtcriminal/xray-node) VPN nodes.

- Register nodes and manage them via Admin API / CLI
- Add a user once → provisioned on all nodes (same email + UUID)
- Public subscription URL with content negotiation:
  - default: base64 `vless://` / `hysteria2://` links
  - `User-Agent: Happ/*`: JSON array of full Xray configs
- Profile groups in config: `smart_multi` (balancer + observatory) or `single` (one outbound per config)

**Technical spec:** [docs/TECHNICAL.md](docs/TECHNICAL.md)  
**HTTP API:** [docs/API.md](docs/API.md) · [OpenAPI](docs/openapi.yaml)

## Install (production)

On a dedicated VPS (Ubuntu/Debian):

```bash
curl -fsSL https://raw.githubusercontent.com/thethoughtcriminal/xray-master/main/scripts/install.sh | sudo bash
```

With HTTPS reverse proxy (Caddy) — DNS for the domain must already point to this server:

```bash
curl -fsSL https://raw.githubusercontent.com/thethoughtcriminal/xray-master/main/scripts/install.sh | \
  sudo XRAY_MASTER_PUBLIC_URL=https://sub.example.com XRAY_MASTER_INSTALL_CADDY=1 bash
```

The installer builds the binary, writes `/etc/xray-master/config.yaml` (generates `admin_key`), creates a systemd service, and listens on `127.0.0.1:9480` by default.

Uninstall:

```bash
curl -fsSL https://raw.githubusercontent.com/thethoughtcriminal/xray-master/main/scripts/uninstall.sh | sudo bash -s -- --yes
```

## Quick start (development)

```bash
go build -o bin/xray-master ./cmd/xray-master
cp configs/config.example.yaml /etc/xray-master/config.yaml
# edit admin_key, public_url, profiles

xray-master serve --config /etc/xray-master/config.yaml
```

## Register a node

Master SSHs into the VPS and installs xray-node automatically. One-time: add the master's SSH public key to the node:

```bash
cat /etc/xray-master/id_ed25519.pub   # on master
# → paste into root@NODE:~/.ssh/authorized_keys
```

Then on master:

```bash
xray-master node add --name nl-1 --ip 203.0.113.10
# optional: --public-host nl.example.com  (default: IP)
```

Add the node to `subscription.profiles` in config when it should appear in user subscriptions, then restart:

```bash
nano /etc/xray-master/config.yaml
systemctl restart xray-master
```

New users are provisioned on all registered nodes automatically. To backfill existing users on a new node:

```bash
xray-master sync users
```

Manual registration (xray-node already installed) is still supported:

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
