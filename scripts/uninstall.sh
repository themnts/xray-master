#!/usr/bin/env bash
set -euo pipefail

# Remove xray-master installed by scripts/install.sh.
# Usage: curl -fsSL .../uninstall.sh | sudo bash -s --
#    or: curl -fsSL .../uninstall.sh | sudo bash -s -- --yes

INSTALL_DIR="${XRAY_MASTER_INSTALL_DIR:-/opt/xray-master}"
CONFIG_DIR="/etc/xray-master"
DATA_DIR="/var/lib/xray-master"
BIN_PATH="/usr/local/bin/xray-master"
SERVICE_PATH="/etc/systemd/system/xray-master.service"
CADDY_MARKER="# xray-master"

KEEP_DATA=0
ASSUME_YES=0

usage() {
  cat <<EOF
Usage: $0 [options]

Options:
  -y, --yes        Skip confirmation
  --keep-data      Keep ${DATA_DIR} (SQLite database)
  -h, --help       Show this help
EOF
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -y | --yes)
        ASSUME_YES=1
        shift
        ;;
      --keep-data)
        KEEP_DATA=1
        shift
        ;;
      -h | --help)
        usage
        exit 0
        ;;
      *)
        echo "Unknown option: $1" >&2
        usage >&2
        exit 1
        ;;
    esac
  done
}

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    echo "Run as root: sudo $0" >&2
    exit 1
  fi
}

confirm() {
  if [[ "${ASSUME_YES}" -eq 1 ]]; then
    return 0
  fi
  echo "This will remove:"
  echo "  - xray-master service, binary, config, and ${INSTALL_DIR}"
  if [[ "${KEEP_DATA}" -eq 0 ]]; then
    echo "  - ${DATA_DIR} (users and nodes database)"
  else
    echo "  - database will be kept (--keep-data)"
  fi

  local reply=""
  if [[ -t 0 ]]; then
    read -r -p "Continue? [y/N] " reply || true
  elif [[ -r /dev/tty ]]; then
    read -r -p "Continue? [y/N] " reply </dev/tty || true
  else
    echo "Non-interactive shell: pass --yes to confirm uninstall." >&2
    exit 1
  fi

  case "${reply}" in
    y | Y | yes | YES) ;;
    *)
      echo "Aborted."
      exit 0
      ;;
  esac
}

remove_caddy_block() {
  local caddyfile="/etc/caddy/Caddyfile"
  if [[ ! -f "${caddyfile}" ]] || ! grep -q "${CADDY_MARKER}" "${caddyfile}"; then
    return 0
  fi
  echo "Removing xray-master block from Caddyfile..."
  local tmp
  tmp="$(mktemp)"
  awk -v marker="${CADDY_MARKER}" '
    $0 == marker { skip=1; next }
    skip && /^[[:space:]]*}/ { skip=0; next }
    skip { next }
    { print }
  ' "${caddyfile}" >"${tmp}"
  mv "${tmp}" "${caddyfile}"
  systemctl reload caddy 2>/dev/null || true
}

remove_xray_master() {
  echo "Stopping xray-master..."
  systemctl stop xray-master 2>/dev/null || true
  systemctl disable xray-master 2>/dev/null || true

  if [[ -f "${SERVICE_PATH}" ]]; then
    rm -f "${SERVICE_PATH}"
  fi
  systemctl daemon-reload
  systemctl reset-failed 2>/dev/null || true

  if [[ -f "${BIN_PATH}" ]]; then
    rm -f "${BIN_PATH}"
  fi
  if [[ -d "${CONFIG_DIR}" ]]; then
    rm -rf "${CONFIG_DIR}"
  fi
  if [[ -d "${INSTALL_DIR}" ]]; then
    rm -rf "${INSTALL_DIR}"
  fi
  if [[ "${KEEP_DATA}" -eq 0 && -d "${DATA_DIR}" ]]; then
    rm -rf "${DATA_DIR}"
  fi
}

print_done() {
  cat <<EOF

Uninstall complete.

Re-install:
  curl -fsSL https://raw.githubusercontent.com/thethoughtcriminal/xray-master/main/scripts/install.sh | sudo bash

EOF
}

main() {
  parse_args "$@"
  require_root
  confirm
  remove_caddy_block
  remove_xray_master
  print_done
}

main "$@"
