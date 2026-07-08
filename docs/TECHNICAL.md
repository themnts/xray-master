# xray-master — техническое описание

Мастер-сервер подписок для кластера VPN-нод на базе [xray-node](https://github.com/thethoughtcriminal/xray-node).

**Модуль Go:** `github.com/thethoughtcriminal/xray-master`  
**Версия Go:** 1.22+

---

## 1. Назначение

| Компонент | Репозиторий | Роль |
|-----------|-------------|------|
| VPN-нода | `xray-node` | Управление одним VPS (3x-ui API) |
| Мастер | `xray-master` | Кластер нод, пользователи, подписки |

```
Администратор
      │ X-Admin-Key
      ▼
xray-master (:9480)
      │ X-API-Key (на каждую ноду)
      ▼
xray-node (:9472) × N
      ▼
3x-ui + Xray

Пользователь (Happ)
      │ GET /sub/{token}
      ▼
xray-master → base64 links или JSON configs
```

---

## 2. Архитектура кода

```
cmd/xray-master
internal/
  config/         # config.yaml, profiles
  db/             # SQLite: nodes, users
  nodeclient/     # HTTP клиент xray-node API
  service/        # Master: оркестрация
  subscription/   # сборка links + Happ JSON
  api/            # публичная подписка + admin API
  cli/            # cobra
```

---

## 3. Модель данных (SQLite)

### nodes

| Поле | Описание |
|------|----------|
| `name` | Уникальное имя (ссылка из `subscription.profiles`) |
| `api_url` | URL xray-node API |
| `api_key` | `X-API-Key` ноды |
| `public_host` | Hostname в клиентских ссылках |

### users

| Поле | Описание |
|------|----------|
| `email` | Одинаковый на всех нодах |
| `uuid` | Один UUID на всех нодах |
| `sub_token` | Токен в URL подписки |
| `expiry_time` | Unix ms, 0 = без срока |
| `total_bytes` | Лимит трафика, 0 = без лимита |

---

## 4. Профили подписки (config.yaml)

```yaml
subscription:
  profiles:
  - name: "🚀 SMART Auto"
    mode: smart_multi      # один JSON: N outbound + balancer + observatory
    entries:
    - node: nl-1
      inbound: vless-reality
      label: "🇳🇱 Netherlands"

  - name: "Dedicated Hy2"
    mode: single           # отдельный JSON / отдельные ссылки
    entries:
    - node: de-1
      inbound: hysteria2
      label: "🇩🇪 Hysteria2"
```

| mode | base64 подписка | Happ JSON |
|------|-----------------|-----------|
| `smart_multi` | N ссылок с общим префиксом имени | 1 конфиг с balancer `lb_smart` |
| `single` | 1 ссылка на entry | 1 конфиг на entry |

---

## 5. Content negotiation

| User-Agent | Content-Type | Тело |
|------------|--------------|------|
| обычный (curl) | `text/plain` | base64(`vless://...\nhysteria2://...`) |
| содержит `Happ` | `application/json` | `[{...xray config...}, ...]` |

Заголовки Happ: `subscription-userinfo`, `profile-update-interval`, `routing-enable`.

---

## 6. Добавление пользователя

1. Создать запись в SQLite
2. Для каждой **зарегистрированной ready-ноды** — на **каждый enabled inbound**:
   - `POST /clients` на xray-node с `email`, `uuid`, `comment: managed by xray-master`
3. Вернуть URL: `{public_url}/sub/{sub_token}`

Профили подписки (`subscription.profiles`) влияют только на содержимое `/sub/{token}`, не на провижинг.

Один email + один UUID на всех нодах (учёт трафика Xray по email).

---

## 7. HTTP API

### Публичный

`GET /sub/{token}` — без авторизации

### Admin

Auth: `X-Admin-Key`

См. [README.md](../README.md).

---

## 8. Безопасность

- `admin_key` и `api_key` нод — секреты
- Мастер подключается к нодам по SSH (bootstrap) и HTTP `:9472` (только с IP мастера, через ufw)
- SSH-ключ мастера: `/etc/xray-master/id_ed25519` — добавить на каждую новую ноду в `authorized_keys`
- `public_url` — HTTPS с валидным сертификатом для продакшена

---

## 9. Provisioning нод (v2)

```
xray-master node add --name nl-1 --ip 203.0.113.10
```

1. SSH `root@IP` с ключом из `provision.ssh_key_path`
2. `curl install.sh | bash` → 3x-ui + xray-node
3. `api.listen: 0.0.0.0:9472` + `ufw allow from MASTER_IP`
4. Сохранить `api_url`, `api_key`, `public_host` в SQLite

После добавления ноды в `subscription.profiles`:

```bash
systemctl restart xray-master
xray-master sync users
```

---

## 10. Ограничения v1

- Профили задаются в config.yaml (не в БД)
- Агрегация трафика — сумма по нодам из profile entries
- Нет веб-UI, только CLI + Admin API
- Нет авто-синхронизации inbound metadata (читается с нод при каждой выдаче подписки)

---

## 11. Связь с xray-node

Мастер использует xray-node HTTP API:

| Операция | xray-node endpoint |
|----------|-------------------|
| Список inbounds | `GET /inbounds` |
| Добавить клиента | `POST /clients` |
| Вкл/выкл | `POST /clients/{email}/enable|disable` |
| Трафик | `GET /clients/{email}/stats` |

Inbound `remark` в профиле должен совпадать с `remark` на ноде (`vless-reality`, `hysteria2`).
