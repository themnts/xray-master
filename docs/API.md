# xray-master HTTP API

Machine-readable spec: [openapi.yaml](./openapi.yaml)

Interactive viewer (paste the raw OpenAPI URL into [Swagger Editor](https://editor.swagger.io/)):

```
https://raw.githubusercontent.com/thethoughtcriminal/xray-master/main/docs/openapi.yaml
```

Default base URL: `http://127.0.0.1:9480` (see `server.listen` in config).

---

## Authentication

| Scope | Header | Value |
|-------|--------|-------|
| Public endpoints | — | no auth |
| Admin endpoints | `X-Admin-Key` | `server.admin_key` from `/etc/xray-master/config.yaml` |

All JSON admin responses use `Content-Type: application/json`.

Errors are always:

```json
{"error": "message"}
```

| HTTP | Meaning |
|------|---------|
| 400 | validation (missing field, expired/disabled subscription, …) |
| 401 | invalid `X-Admin-Key` |
| 404 | not found |
| 409 | duplicate node name or user email |
| 502 | xray-node unreachable or returned error |
| 500 | other server error |

---

## Public endpoints

### `GET /healthz`

Health check.

**Response `200`:**

```json
{"status": "ok"}
```

**Example:**

```bash
curl http://127.0.0.1:9480/healthz
```

---

### `GET /sub/{token}`

User subscription. `{token}` is `SubToken` from `GET /users` (32 hex characters, generated on user creation).

**Response `200` — content negotiation:**

| `User-Agent` | `Content-Type` | Body |
|--------------|----------------|------|
| contains `Happ` (any case) | `application/json` | `[{...}, ...]` Xray config objects |
| anything else | `text/plain; charset=utf-8` | base64(`vless://...\nhysteria2://...`) |

**Response headers (Happ):**

| Header | Description |
|--------|-------------|
| `profile-update-interval` | Hours between refresh (`subscription.update_interval_hours`) |
| `routing-enable` | Always `true` |
| `subscription-userinfo` | `upload=N; download=N; total=N; expire=N` when limits set (`expire` in seconds) |

**Errors:**

| HTTP | `error` example |
|------|-----------------|
| 404 | `subscription not found` |
| 400 | `subscription disabled` |
| 400 | `subscription expired` |
| 502 | `node nl-1: ...` |

**Examples:**

```bash
# base64 share links
curl http://127.0.0.1:9480/sub/a1b2c3d4e5f6789012345678abcdef01

# Happ JSON configs
curl -H "User-Agent: Happ/1.0" http://127.0.0.1:9480/sub/a1b2c3d4e5f6789012345678abcdef01
```

Full subscription URL for clients: `{public_url}/sub/{SubToken}`

---

## Admin endpoints

All require:

```http
X-Admin-Key: <admin_key>
```

---

### `GET /nodes`

List registered VPN nodes.

**Response `200`:** array of `Node`

```json
[
  {
    "ID": "550e8400-e29b-41d4-a716-446655440000",
    "Name": "nl-1",
    "APIURL": "http://127.0.0.1:9472",
    "APIKey": "secret",
    "PublicHost": "nl.example.com",
    "Enabled": true,
    "CreatedAt": "2026-07-07T20:00:00Z"
  }
]
```

> Response fields use **PascalCase** (Go default JSON encoding).

---

### `POST /nodes`

Register a new node.

**Request body:**

```json
{
  "name": "nl-1",
  "api_url": "http://127.0.0.1:9472",
  "api_key": "NODE_API_KEY",
  "public_host": "nl.example.com"
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `name` | yes | Unique name; must match `subscription.profiles[].entries[].node` |
| `api_url` | yes | xray-node API base URL |
| `api_key` | yes | xray-node `X-API-Key` |
| `public_host` | yes | Hostname in client links |

**Response `201`:** created `Node` object.

**Response `409`:** node name already exists.

---

### `DELETE /nodes/{id}`

Remove node by UUID (`Node.ID`).

**Response `200`:**

```json
{"status": "deleted"}
```

---

### `GET /users`

List subscription users.

**Response `200`:** array of `User`

```json
[
  {
    "ID": "660e8400-e29b-41d4-a716-446655440001",
    "Email": "user@example.com",
    "UUID": "770e8400-e29b-41d4-a716-446655440002",
    "SubToken": "a1b2c3d4e5f6789012345678abcdef01",
    "Enabled": true,
    "ExpiryTime": 0,
    "TotalBytes": 0,
    "Note": "",
    "CreatedAt": "2026-07-07T20:00:00Z"
  }
]
```

| Field | Description |
|-------|-------------|
| `ExpiryTime` | Unix **milliseconds**; `0` = no expiry |
| `TotalBytes` | Traffic limit in bytes; `0` = unlimited |
| `SubToken` | Token for `GET /sub/{token}` |

---

### `POST /users`

Create user and provision on all unique nodes from `subscription.profiles`.

**Request body:**

```json
{
  "email": "user@example.com",
  "uuid": "",
  "expiry_time": 0,
  "total_bytes": 0,
  "note": ""
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `email` | yes | Same email on all nodes |
| `uuid` | no | Client UUID; auto-generated if empty |
| `expiry_time` | no | Unix ms expiry |
| `total_bytes` | no | Traffic cap |
| `note` | no | Free-form note |

**Response `201`:**

```json
{
  "user": { "...": "..." },
  "node_errors": {
    "nl-1": "node POST /clients: connection refused"
  }
}
```

`node_errors` is present only when some nodes failed; the user is still saved in the database.

**Response `409`:** email already exists.

---

### `POST /users/{id}/enable`

### `POST /users/{id}/disable`

Enable or disable user. `{id}` is **user UUID** (`User.ID`), not email.

No request body.

**Response `200`:**

```json
{"enabled": true}
```

```json
{"enabled": false}
```

Also syncs state to all nodes in subscription profiles.

---

### `DELETE /users/{id}`

Delete user from database and disable client on nodes. `{id}` is user UUID.

Does not fully remove the client record on xray-node.

**Response `200`:**

```json
{"status": "deleted"}
```

---

### `GET /users/{email}/stats`

Aggregate traffic for a user across profile nodes.

`{email}` must be URL-encoded (`user@example.com` → `user%40example.com`).

**Response `200`:**

```json
{
  "email": "user@example.com",
  "up": 1048576,
  "down": 5242880,
  "by_node": {
    "nl-1": {
      "inbound": "vless-reality",
      "up": 1048576,
      "down": 5242880
    }
  },
  "node_errors": {
    "se-1": "node GET /clients/...: timeout"
  }
}
```

---

## curl cheat sheet

```bash
KEY="$(awk '/admin_key:/ {print $2}' /etc/xray-master/config.yaml)"
BASE="http://127.0.0.1:9480"

curl "$BASE/healthz"

curl -H "X-Admin-Key: $KEY" "$BASE/nodes"

curl -X POST "$BASE/nodes" \
  -H "X-Admin-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"nl-1","api_url":"http://127.0.0.1:9472","api_key":"NODE_KEY","public_host":"nl.example.com"}'

curl -X POST "$BASE/users" \
  -H "X-Admin-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com"}'

curl -H "X-Admin-Key: $KEY" "$BASE/users/user%40example.com/stats"

curl "$BASE/sub/SUB_TOKEN"
curl -H "User-Agent: Happ/1.0" "$BASE/sub/SUB_TOKEN"
```

---

## Not implemented (v1)

- `PATCH` / update user or node
- `GET /users/{id}` (single user)
- Token rotation
- Pagination
- API versioning (`/v1/...`)
- Webhooks
