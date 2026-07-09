# Configuration reference

xray-master uses two YAML files:

| File | Purpose |
|------|---------|
| `config.yaml` | Server, database, enroll tokens, paths |
| `subscription.yaml` | Subscription profiles for `GET /sub/{token}` |

Default paths (production install):

```
/etc/xray-master/config.yaml
/etc/xray-master/subscription.yaml
```

Examples in the repository: `configs/config.example.yaml`, `configs/subscription.example.yaml`.

---

## config.yaml

Main application config. Loaded via `--config` (default `/etc/xray-master/config.yaml`).

### `server`

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `listen` | string | no | `0.0.0.0:9480` | HTTP listen address (`host:port`). Use `127.0.0.1:9480` when a reverse proxy (Caddy/nginx) handles public traffic. |
| `admin_key` | string | **yes** | — | Secret for Admin API (`X-Admin-Key` header). Generated on first install. |
| `public_url` | string | **yes** | — | Public base URL for subscription links and enroll join commands (e.g. `https://sub.example.com` or `http://203.0.113.1:9480`). No trailing slash. |
| `db_path` | string | no | `/var/lib/xray-master/data.db` | SQLite database path (users, nodes, enroll tokens). |
| `subscription_path` | string | no | `/etc/xray-master/subscription.yaml` (or `subscription.yaml` next to `--config`) | Path to subscription profiles file. |

### `enroll`

Settings for node self-registration (`POST /nodes/enroll`, `xray-node join`).

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `master_ip` | string | no | `""` | Master public IP. Appended to `xray-node join --master-ip` so the node can run `ufw allow from <ip> to any port 9472`. |
| `enroll_ttl_hours` | int | no | `24` | Lifetime of one-time enroll tokens created by `xray-master node token create`. |

---

## subscription.yaml

Defines what users receive from `GET /sub/{token}`. **Does not affect user provisioning** on nodes — only subscription output format and content.

Reload: `systemctl restart xray-master` after edits.

### Root fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `update_interval_hours` | int | no | `12` | Sent to Happ clients as `profile-update-interval` (hours between refresh). |
| `profiles` | array | **yes** | — | List of subscription profiles (see below). |

### `profiles[]`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | **yes** | Profile display name (prefix in client app). |
| `mode` | string | **yes** | `smart_multi` — one JSON config with balancer across entries; `single` — one link/config per entry. |
| `entries` | array | **yes** | At least one node/inbound pair (see below). |

### `profiles[].entries[]`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `node` | string | **yes** | Registered node name (`xray-master node list`). Must match a node in the database. |
| `inbound` | string | **yes** | Inbound remark on that node (e.g. `vless-reality`, `hysteria2`). |
| `label` | string | no | Human label in share links / JSON (e.g. `🇳🇱 Netherlands`). |

### Profile modes

| `mode` | Base64 subscription (`text/plain`) | Happ JSON (`User-Agent: Happ/...`) |
|--------|-------------------------------------|-------------------------------------|
| `smart_multi` | N links sharing the profile name prefix | One Xray JSON with `lb_smart` balancer |
| `single` | One link per entry | One Xray JSON per entry |

---

## Example layout

**config.yaml**

```yaml
server:
  listen: 127.0.0.1:9480
  admin_key: YOUR_SECRET
  public_url: https://sub.example.com
  db_path: /var/lib/xray-master/data.db
  subscription_path: /etc/xray-master/subscription.yaml

enroll:
  master_ip: "203.0.113.1"
  enroll_ttl_hours: 24
```

**subscription.yaml**

```yaml
update_interval_hours: 12
profiles:
- name: "🚀 SMART Auto"
  mode: smart_multi
  entries:
  - node: nl-1
    inbound: vless-reality
    label: "🇳🇱 Netherlands"
```

---

## Validation errors

On `xray-master serve`, config is validated:

- Missing `server.admin_key` or `server.public_url`
- Profile without `name`, invalid `mode`, empty `entries`
- Entry without `node` or `inbound`
- Unreadable or missing subscription file

---

## Related docs

- [API.md](./API.md) — HTTP endpoints
- [TECHNICAL.md](./TECHNICAL.md) — architecture (Russian)
- [openapi.yaml](./openapi.yaml) — OpenAPI spec
