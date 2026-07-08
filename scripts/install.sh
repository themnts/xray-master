#!/usr/bin/env bash
set -euo pipefail

# Quick install: xray-master subscription server.
# Usage: curl -fsSL .../install.sh | sudo bash
#    or: sudo ./scripts/install.sh
#
# Optional env:
#   XRAY_MASTER_PUBLIC_URL=https://sub.example.com
#   XRAY_MASTER_INSTALL_CADDY=1          # reverse proxy via Caddy (needs DNS → this host)
#   XRAY_MASTER_LISTEN=127.0.0.1:9480    # default: localhost only

REPO_URL="${XRAY_MASTER_REPO:-https://github.com/thethoughtcriminal/xray-master.git}"
REPO_BRANCH="${XRAY_MASTER_BRANCH:-main}"
INSTALL_DIR="${XRAY_MASTER_INSTALL_DIR:-/opt/xray-master}"
CONFIG_PATH="/etc/xray-master/config.yaml"
DATA_DIR="/var/lib/xray-master"
BIN_PATH="/usr/local/bin/xray-master"
SERVICE_PATH="/etc/systemd/system/xray-master.service"
LISTEN="${XRAY_MASTER_LISTEN:-127.0.0.1:9480}"
PUBLIC_URL="${XRAY_MASTER_PUBLIC_URL:-}"
INSTALL_CADDY="${XRAY_MASTER_INSTALL_CADDY:-0}"

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    echo "Run as root: sudo $0" >&2
    exit 1
  fi
}

install_deps() {
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y curl git ca-certificates golang-go openssl
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y curl git ca-certificates golang openssl
  else
    echo "Install curl, git, Go, and openssl manually, then re-run." >&2
    exit 1
  fi
}

clone_or_update_repo() {
  if [[ -d "${INSTALL_DIR}/.git" ]]; then
    echo "Updating xray-master source..."
    git -C "${INSTALL_DIR}" fetch origin "${REPO_BRANCH}"
    git -C "${INSTALL_DIR}" clean -fd
    git -C "${INSTALL_DIR}" reset --hard "origin/${REPO_BRANCH}"
  else
    git clone --branch "${REPO_BRANCH}" "${REPO_URL}" "${INSTALL_DIR}"
  fi
}

build_binary() {
  echo "Building xray-master..."
  (
    cd "${INSTALL_DIR}"
    go mod download
    go build -o "${BIN_PATH}" ./cmd/xray-master
  )
}

patch_config_value() {
  local key="$1"
  local value="$2"
  local file="$3"
  local tmp
  tmp="$(mktemp)"
  awk -v key="${key}" -v value="${value}" '
    $0 ~ "^[[:space:]]*" key ":" {
      sub(/:.*/, ": " value)
    }
    { print }
  ' "${file}" >"${tmp}"
  mv "${tmp}" "${file}"
}

write_ssh_key() {
  local key="/etc/xray-master/id_ed25519"
  if [[ -f "${key}" ]]; then
    echo "SSH key exists: ${key}"
    return
  fi
  ssh-keygen -t ed25519 -N "" -f "${key}"
  chmod 600 "${key}"
  echo "Generated SSH key for node provisioning: ${key}.pub"
  echo "Add this public key to root@<node>: ~/.ssh/authorized_keys"
  cat "${key}.pub"
}

write_config() {
  mkdir -p /etc/xray-master "${DATA_DIR}"
  if [[ ! -f "${CONFIG_PATH}" ]]; then
    cp "${INSTALL_DIR}/configs/config.example.yaml" "${CONFIG_PATH}"
    ADMIN_KEY="$(openssl rand -hex 24)"
    sed -i "s/CHANGE_ME_ADMIN_KEY/${ADMIN_KEY}/" "${CONFIG_PATH}"
    patch_config_value "listen" "${LISTEN}" "${CONFIG_PATH}"
    patch_config_value "db_path" "${DATA_DIR}/data.db" "${CONFIG_PATH}"
    if [[ -n "${PUBLIC_URL}" ]]; then
      patch_config_value "public_url" "${PUBLIC_URL}" "${CONFIG_PATH}"
    fi
    chmod 600 "${CONFIG_PATH}"
    echo "Generated admin key in ${CONFIG_PATH}"
  else
    echo "Config exists: ${CONFIG_PATH}"
  fi
}

write_systemd() {
  cat >"${SERVICE_PATH}" <<EOF
[Unit]
Description=xray-master subscription server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${BIN_PATH} serve --config ${CONFIG_PATH}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable xray-master
  systemctl restart xray-master
}

public_url_domain() {
  local url="$1"
  url="${url#https://}"
  url="${url#http://}"
  url="${url%%/*}"
  url="${url%%:*}"
  echo "${url}"
}

should_install_caddy() {
  if [[ "${INSTALL_CADDY}" != "1" ]]; then
    return 1
  fi
  local url="${PUBLIC_URL}"
  if [[ -z "${url}" ]] && [[ -f "${CONFIG_PATH}" ]]; then
    url="$(awk '/^[[:space:]]*public_url:/ { print $2; exit }' "${CONFIG_PATH}")"
  fi
  local domain
  domain="$(public_url_domain "${url}")"
  [[ -n "${domain}" && "${domain}" != "sub.example.com" ]]
}

install_caddy_reverse_proxy() {
  local url="${PUBLIC_URL}"
  if [[ -z "${url}" ]] && [[ -f "${CONFIG_PATH}" ]]; then
    url="$(awk '/^[[:space:]]*public_url:/ { print $2; exit }' "${CONFIG_PATH}")"
  fi
  local domain
  domain="$(public_url_domain "${url}")"
  if [[ -z "${domain}" || "${domain}" == "sub.example.com" ]]; then
    echo "Skipping Caddy: set XRAY_MASTER_PUBLIC_URL to your subscription domain."
    return 0
  fi

  if ! command -v caddy >/dev/null 2>&1; then
    echo "Installing Caddy..."
    if command -v apt-get >/dev/null 2>&1; then
      apt-get install -y debian-keyring debian-archive-keyring apt-transport-https curl
      curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
      curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
      apt-get update -y
      apt-get install -y caddy
    elif command -v dnf >/dev/null 2>&1; then
      dnf install -y 'dnf-command(copr)'
      dnf copr enable -y @caddy/caddy
      dnf install -y caddy
    else
      echo "Install Caddy manually for HTTPS reverse proxy." >&2
      return 1
    fi
  fi

  local caddyfile="/etc/caddy/Caddyfile"
  local marker="# xray-master"
  if [[ -f "${caddyfile}" ]] && grep -q "${marker}" "${caddyfile}"; then
    echo "Caddy config for xray-master already present"
  else
    cat >>"${caddyfile}" <<EOF

${marker}
${domain} {
    reverse_proxy 127.0.0.1:9480
}
EOF
  fi

  systemctl enable caddy
  systemctl reload caddy || systemctl restart caddy
  echo "Caddy reverse proxy enabled for ${domain}"
}

print_next_steps() {
  local admin_key public_url
  admin_key="$(awk '/^[[:space:]]*admin_key:/ { print $2; exit }' "${CONFIG_PATH}")"
  public_url="$(awk '/^[[:space:]]*public_url:/ { print $2; exit }' "${CONFIG_PATH}")"

  cat <<EOF

xray-master installed.

Config:  ${CONFIG_PATH}
Data:    ${DATA_DIR}/data.db
Admin:   X-Admin-Key: ${admin_key}

1) Edit subscription profiles (node names must match registered nodes):
   nano ${CONFIG_PATH}

2) Verify local API:
   curl http://${LISTEN}/healthz

3) Add master's SSH public key to each new node (one-time bootstrap):
   cat /etc/xray-master/id_ed25519.pub
   # paste into root@NODE:~/.ssh/authorized_keys

4) Register VPN nodes (master SSHs in and installs xray-node):
   xray-master node add --name nl-1 --ip NODE_IP

5) Add node to subscription.profiles in ${CONFIG_PATH}, then:
   systemctl restart xray-master
   xray-master sync users

6) Add a user:
   xray-master user add --email user@example.com

7) Public subscription URL base:
   ${public_url}/sub/<token>

HTTPS: point DNS for your domain to this server, then either:
  XRAY_MASTER_PUBLIC_URL=https://sub.example.com XRAY_MASTER_INSTALL_CADDY=1 sudo -E bash ${INSTALL_DIR}/scripts/install.sh
  or configure nginx/Caddy manually to proxy to ${LISTEN}

Uninstall:
  curl -fsSL https://raw.githubusercontent.com/thethoughtcriminal/xray-master/main/scripts/uninstall.sh | sudo bash -s --

EOF
}

main() {
  require_root
  install_deps
  clone_or_update_repo
  build_binary
  write_config
  write_ssh_key
  write_systemd
  if should_install_caddy; then
    install_caddy_reverse_proxy
  fi
  print_next_steps
}

main "$@"
