#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FREEZE_ENV_FILE="${FREEZE_ENV_FILE:-$ROOT_DIR/.env.freeze}"

if [[ ! -f "$FREEZE_ENV_FILE" ]]; then
  echo "freeze env file not found: $FREEZE_ENV_FILE" >&2
  exit 1
fi

set -a
source "$FREEZE_ENV_FILE"
set +a

require_value() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "required value is empty: $name" >&2
    exit 1
  fi
}

run_remote_to_file() {
  local user="$1"
  local host="$2"
  local output_file="$3"
  shift 3

  ssh -o BatchMode=yes -o ConnectTimeout=10 "$user@$host" "$@" >"$output_file"
}

copy_first_existing_remote_file() {
  local user="$1"
  local host="$2"
  local output_file="$3"
  shift 3

  local path
  for path in "$@"; do
    if ssh -o BatchMode=yes -o ConnectTimeout=10 "$user@$host" "test -f $path"; then
      ssh -o BatchMode=yes -o ConnectTimeout=10 "$user@$host" "cat $path" >"$output_file"
      printf '%s\n' "$path" >"${output_file}.source"
      return 0
    fi
  done

  echo "none of the candidate remote files exist for $output_file" >&2
  return 1
}

require_value DO_APP_HOST
require_value DO_APP_USER
require_value YC_HOST
require_value YC_USER
umask 077

BUNDLE_ROOT="${BUNDLE_ROOT:-$ROOT_DIR/backup}"
STAMP="$(date +%Y%m%d-%H%M%S)"
OUT_DIR="$BUNDLE_ROOT/bundle-$STAMP"

mkdir -p "$OUT_DIR"/app-host "$OUT_DIR"/yc "$OUT_DIR"/do-egress

cp "$FREEZE_ENV_FILE" "$OUT_DIR/.env.freeze"

copy_first_existing_remote_file "$DO_APP_USER" "$DO_APP_HOST" "$OUT_DIR/app-host/docker-compose.yml" \
  /opt/vpn/docker-compose.yml \
  /opt/vpn/app/docker-compose.yml

run_remote_to_file "$DO_APP_USER" "$DO_APP_HOST" "$OUT_DIR/app-host/api.env" \
  "cat /opt/vpn/env/api.env"
run_remote_to_file "$DO_APP_USER" "$DO_APP_HOST" "$OUT_DIR/app-host/bot.env" \
  "cat /opt/vpn/env/bot.env"
run_remote_to_file "$DO_APP_USER" "$DO_APP_HOST" "$OUT_DIR/app-host/proxy_ed25519" \
  "cat /opt/vpn/secrets/proxy_ed25519"
run_remote_to_file "$DO_APP_USER" "$DO_APP_HOST" "$OUT_DIR/app-host/proxy_known_hosts" \
  "cat /opt/vpn/secrets/proxy_known_hosts"
run_remote_to_file "$DO_APP_USER" "$DO_APP_HOST" "$OUT_DIR/app-host/ssh_config.app" \
  "cat /opt/vpn/secrets/ssh_config.app"
run_remote_to_file "$DO_APP_USER" "$DO_APP_HOST" "$OUT_DIR/app-host/docker-ps.txt" \
  "docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'"

run_remote_to_file "$YC_USER" "$YC_HOST" "$OUT_DIR/yc/wg-clients.conf" \
  "sudo cat /etc/wireguard/wg-clients.conf"
run_remote_to_file "$YC_USER" "$YC_HOST" "$OUT_DIR/yc/wg-upstream.conf" \
  "sudo cat /etc/wireguard/wg-upstream.conf"
run_remote_to_file "$YC_USER" "$YC_HOST" "$OUT_DIR/yc/vpn-peer-add" \
  "sudo cat /usr/local/bin/vpn-peer-add"
run_remote_to_file "$YC_USER" "$YC_HOST" "$OUT_DIR/yc/vpn-peer-remove" \
  "sudo cat /usr/local/bin/vpn-peer-remove"
run_remote_to_file "$YC_USER" "$YC_HOST" "$OUT_DIR/yc/authorized_keys" \
  "vpnmgr_home=\$(getent passwd vpnmgr | cut -d: -f6) && sudo cat \"\$vpnmgr_home/.ssh/authorized_keys\""
run_remote_to_file "$YC_USER" "$YC_HOST" "$OUT_DIR/yc/wg-show.txt" \
  "sudo wg show"

if [[ -n "${DO_EGRESS_HOST:-}" && -n "${DO_EGRESS_USER:-}" ]]; then
  run_remote_to_file "$DO_EGRESS_USER" "$DO_EGRESS_HOST" "$OUT_DIR/do-egress/wg-upstream.conf" \
    "sudo cat /etc/wireguard/wg-upstream.conf"
  run_remote_to_file "$DO_EGRESS_USER" "$DO_EGRESS_HOST" "$OUT_DIR/do-egress/iptables-save.txt" \
    "sudo iptables-save"
  run_remote_to_file "$DO_EGRESS_USER" "$DO_EGRESS_HOST" "$OUT_DIR/do-egress/sysctl-net.ipv4.ip_forward.txt" \
    "sudo sysctl -n net.ipv4.ip_forward"
  run_remote_to_file "$DO_EGRESS_USER" "$DO_EGRESS_HOST" "$OUT_DIR/do-egress/wg-show.txt" \
    "sudo wg show"
else
  printf 'DO egress shell backup skipped because DO_EGRESS_HOST / DO_EGRESS_USER are not set in %s\n' "$FREEZE_ENV_FILE" >"$OUT_DIR/do-egress/README.txt"
fi

cat >"$OUT_DIR/README.txt" <<EOF
Raw recovery bundle created at: $STAMP

WARNING:
- This bundle contains live secrets and recovery material.
- Keep it out of git and handle it as sensitive operational data.
- Use artifacts/ from freeze_collect.sh for redacted operational snapshots.

Contents:
- app-host/: live env files, SSH provisioning material, compose file, running container snapshot
- yc/: live WireGuard configs, peer-management scripts, vpnmgr authorized_keys, wg state
- do-egress/: live upstream WireGuard config, iptables-save output, sysctl state, wg state
EOF

echo "backup bundle written to $OUT_DIR"
