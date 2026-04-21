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

ARTIFACT_ROOT="${ARTIFACT_ROOT:-$ROOT_DIR/artifacts}"
STAMP="$(date +%Y%m%d-%H%M%S)"
OUT_DIR="$ARTIFACT_ROOT/freeze-$STAMP"

mkdir -p "$OUT_DIR"/app-host "$OUT_DIR"/yc "$OUT_DIR"/do-egress

require_value() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "required value is empty: $name" >&2
    exit 1
  fi
}

run_remote_capture() {
  local user="$1"
  local host="$2"
  local output_file="$3"
  shift 3

  ssh -o BatchMode=yes -o ConnectTimeout=10 "$user@$host" "$@" >"$output_file"
}

require_value DO_APP_HOST
require_value DO_APP_USER
require_value YC_HOST
require_value YC_USER
require_value APP_API_CONTAINER
require_value APP_SSH_CONFIG
require_value APP_SSH_ALIAS
require_value YC_WG_CLIENTS_IF
require_value YC_WG_UPSTREAM_IF
require_value SAMPLE_CLIENT_IP

cp "$FREEZE_ENV_FILE" "$OUT_DIR/.env.freeze"

run_remote_capture "$DO_APP_USER" "$DO_APP_HOST" "$OUT_DIR/app-host/docker-ps.txt" \
  "docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'"

run_remote_capture "$DO_APP_USER" "$DO_APP_HOST" "$OUT_DIR/app-host/api-env-redacted.txt" \
  "grep -E '^(VPN_SERVER_PUBLIC_KEY|VPN_SERVER_ENDPOINT|VPN_SERVER_TUNNEL_ADDRESS|VPN_ALLOWED_IPS|VPN_DNS|VPN_PERSISTENT_KEEPALIVE|PROXY_SSH_HOST|PROXY_SSH_CONFIG_PATH|PROXY_SSH_TIMEOUT|PROXY_ADD_PEER_COMMAND|PROXY_REMOVE_PEER_COMMAND)=' /opt/vpn/env/api.env || true"

run_remote_capture "$DO_APP_USER" "$DO_APP_HOST" "$OUT_DIR/app-host/api-runtime-env-redacted.txt" \
  "docker exec $APP_API_CONTAINER sh -lc 'printf \"VPN_SERVER_PUBLIC_KEY=%s\nVPN_SERVER_ENDPOINT=%s\nVPN_SERVER_TUNNEL_ADDRESS=%s\nVPN_ALLOWED_IPS=%s\nVPN_DNS=%s\nVPN_PERSISTENT_KEEPALIVE=%s\nPROXY_SSH_HOST=%s\nPROXY_SSH_CONFIG_PATH=%s\nPROXY_SSH_TIMEOUT=%s\n\" \"\$VPN_SERVER_PUBLIC_KEY\" \"\$VPN_SERVER_ENDPOINT\" \"\$VPN_SERVER_TUNNEL_ADDRESS\" \"\$VPN_ALLOWED_IPS\" \"\$VPN_DNS\" \"\$VPN_PERSISTENT_KEEPALIVE\" \"\$PROXY_SSH_HOST\" \"\$PROXY_SSH_CONFIG_PATH\" \"\$PROXY_SSH_TIMEOUT\"'"

run_remote_capture "$DO_APP_USER" "$DO_APP_HOST" "$OUT_DIR/app-host/runtime-ssh-smoke.txt" \
  "docker run --rm --network \"container:$APP_API_CONTAINER\" -v /opt/vpn/secrets:/opt/vpn/secrets:ro alpine:3.20 sh -lc 'apk add --no-cache openssh-client >/dev/null && ssh -F $APP_SSH_CONFIG $APP_SSH_ALIAS \"echo SSH_OK user=\\\$(whoami)\"'"

run_remote_capture "$DO_APP_USER" "$DO_APP_HOST" "$OUT_DIR/app-host/host-shell-ssh-debug.txt" \
  "ssh -F $APP_SSH_CONFIG $APP_SSH_ALIAS \"echo SSH_OK user=\\\$(whoami)\" 2>&1 || true"

run_remote_capture "$YC_USER" "$YC_HOST" "$OUT_DIR/yc/wg-show.txt" \
  "sudo wg show"

run_remote_capture "$YC_USER" "$YC_HOST" "$OUT_DIR/yc/routing.txt" \
  "printf '## ip rule\n'; ip rule; printf '\n## ip route show table 51820\n'; ip route show table 51820; printf '\n## ip route get 1.1.1.1 from $SAMPLE_CLIENT_IP iif $YC_WG_CLIENTS_IF\n'; ip route get 1.1.1.1 from $SAMPLE_CLIENT_IP iif $YC_WG_CLIENTS_IF"

run_remote_capture "$YC_USER" "$YC_HOST" "$OUT_DIR/yc/listener-invariant.txt" \
  "printf 'public-key='; sudo wg show $YC_WG_CLIENTS_IF public-key; printf 'listen-port='; sudo wg show $YC_WG_CLIENTS_IF listen-port"

if [[ -n "${DO_EGRESS_HOST:-}" && -n "${DO_EGRESS_USER:-}" ]]; then
  require_value DO_EGRESS_WG_IF

  run_remote_capture "$DO_EGRESS_USER" "$DO_EGRESS_HOST" "$OUT_DIR/do-egress/wg-show.txt" \
    "sudo wg show"

  run_remote_capture "$DO_EGRESS_USER" "$DO_EGRESS_HOST" "$OUT_DIR/do-egress/forward-and-nat.txt" \
    "printf '## FORWARD\n'; sudo iptables -S FORWARD; printf '\n## POSTROUTING\n'; sudo iptables -t nat -S POSTROUTING; printf '\n## ip_forward\n'; sudo sysctl -n net.ipv4.ip_forward"
else
  printf 'DO egress shell collection skipped because DO_EGRESS_HOST / DO_EGRESS_USER are not set in %s\n' "$FREEZE_ENV_FILE" >"$OUT_DIR/do-egress/README.txt"
fi

cat >"$OUT_DIR/README.txt" <<EOF
Freeze-state artifacts collected at: $STAMP

Contents:
- app-host/: redacted app runtime and provisioning-path state
- yc/: WireGuard listener and routing state
- do-egress/: upstream WireGuard + NAT / forwarding state

This bundle is redacted operational state, not raw secret backup material.
Use scripts/backup_bundle.sh for raw recovery artifacts.
EOF

echo "freeze-state artifacts written to $OUT_DIR"
