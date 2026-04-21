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

failures=0
warnings=0
canonical_ran=0
canonical_passed=0

note() {
  printf '[info] %s\n' "$*"
}

pass() {
  printf '[pass] %s\n' "$*"
}

warn() {
  printf '[warn] %s\n' "$*" >&2
  warnings=$((warnings + 1))
}

fail() {
  printf '[fail] %s\n' "$*" >&2
  failures=$((failures + 1))
}

require_value() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "required value is empty: $name" >&2
    exit 1
  fi
}

run_remote() {
  local user="$1"
  local host="$2"
  shift 2
  ssh -o BatchMode=yes -o ConnectTimeout=10 "$user@$host" "$@"
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local label="$3"

  if grep -Fq "$needle" <<<"$haystack"; then
    pass "$label"
  else
    fail "$label (missing: $needle)"
  fi
}

require_value DO_APP_HOST
require_value DO_APP_USER
require_value YC_HOST
require_value YC_USER
require_value APP_API_CONTAINER
require_value APP_API_BASE_URL
require_value APP_SSH_CONFIG
require_value APP_SSH_ALIAS
require_value YC_WG_CLIENTS_IF
require_value YC_WG_UPSTREAM_IF
require_value YC_WG_CLIENTS_PORT
require_value YC_WG_CLIENTS_PUBLIC_KEY
require_value CANONICAL_CLIENT_ENDPOINT
require_value CANONICAL_ALLOWED_IPS
require_value CANONICAL_PERSISTENT_KEEPALIVE
require_value CLIENT_SUBNET
require_value SAMPLE_CLIENT_IP

note "checking API health on DO app host"
if api_health="$(run_remote "$DO_APP_USER" "$DO_APP_HOST" "curl -fsS $APP_API_BASE_URL/health")"; then
  assert_contains "$api_health" '"status":"ok"' "API health returned ok"
else
  fail "API health request failed on DO app host"
fi

note "checking YC listener invariants"
if yc_public_key="$(run_remote "$YC_USER" "$YC_HOST" "sudo wg show $YC_WG_CLIENTS_IF public-key")"; then
  if [[ "$yc_public_key" == "$YC_WG_CLIENTS_PUBLIC_KEY" ]]; then
    pass "YC $YC_WG_CLIENTS_IF public key matches frozen value"
  else
    fail "YC $YC_WG_CLIENTS_IF public key mismatch (got $yc_public_key)"
  fi
else
  fail "could not read YC $YC_WG_CLIENTS_IF public key"
fi

if yc_listen_port="$(run_remote "$YC_USER" "$YC_HOST" "sudo wg show $YC_WG_CLIENTS_IF listen-port")"; then
  if [[ "$yc_listen_port" == "$YC_WG_CLIENTS_PORT" ]]; then
    pass "YC $YC_WG_CLIENTS_IF listen port matches frozen value"
  else
    fail "YC $YC_WG_CLIENTS_IF listen port mismatch (got $yc_listen_port)"
  fi
else
  fail "could not read YC $YC_WG_CLIENTS_IF listen port"
fi

note "checking YC transport routing"
if yc_wg_show="$(run_remote "$YC_USER" "$YC_HOST" "sudo wg show")"; then
  assert_contains "$yc_wg_show" "interface: $YC_WG_CLIENTS_IF" "YC exposes $YC_WG_CLIENTS_IF"
  assert_contains "$yc_wg_show" "interface: $YC_WG_UPSTREAM_IF" "YC exposes $YC_WG_UPSTREAM_IF"
else
  fail "could not read YC wg show"
fi

if yc_route_get="$(run_remote "$YC_USER" "$YC_HOST" "ip route get 1.1.1.1 from $SAMPLE_CLIENT_IP iif $YC_WG_CLIENTS_IF")"; then
  assert_contains "$yc_route_get" "dev $YC_WG_UPSTREAM_IF" "YC routes client-originated traffic via $YC_WG_UPSTREAM_IF"
else
  fail "could not evaluate YC route-get for sample client"
fi

note "checking DO egress forwarding and NAT"
if [[ -n "${DO_EGRESS_HOST:-}" && -n "${DO_EGRESS_USER:-}" ]]; then
  require_value DO_EGRESS_WG_IF

  if do_wg_show="$(run_remote "$DO_EGRESS_USER" "$DO_EGRESS_HOST" "sudo wg show")"; then
    assert_contains "$do_wg_show" "interface: $DO_EGRESS_WG_IF" "DO egress exposes $DO_EGRESS_WG_IF"
  else
    fail "could not read DO egress wg show"
  fi

  if do_forward="$(run_remote "$DO_EGRESS_USER" "$DO_EGRESS_HOST" "sudo iptables -S FORWARD")"; then
    assert_contains "$do_forward" "$CLIENT_SUBNET" "DO egress FORWARD rules mention client subnet"
  else
    fail "could not read DO egress FORWARD rules"
  fi

  if do_nat="$(run_remote "$DO_EGRESS_USER" "$DO_EGRESS_HOST" "sudo iptables -t nat -S POSTROUTING")"; then
    assert_contains "$do_nat" "$CLIENT_SUBNET" "DO egress POSTROUTING mentions client subnet"
    assert_contains "$do_nat" "MASQUERADE" "DO egress POSTROUTING contains MASQUERADE"
  else
    fail "could not read DO egress POSTROUTING rules"
  fi

  if do_ip_forward="$(run_remote "$DO_EGRESS_USER" "$DO_EGRESS_HOST" "sudo sysctl -n net.ipv4.ip_forward")"; then
    if [[ "$do_ip_forward" == "1" ]]; then
      pass "DO egress ip_forward is enabled"
    else
      fail "DO egress ip_forward is not enabled (got $do_ip_forward)"
    fi
  else
    fail "could not read DO egress ip_forward"
  fi
else
  warn "DO_EGRESS_HOST / DO_EGRESS_USER are not set; DO egress SSH checks skipped"
fi

if [[ -n "${SMOKE_TELEGRAM_ID:-}" && -n "${SMOKE_USER_ID:-}" ]]; then
  fail "set only one of SMOKE_TELEGRAM_ID or SMOKE_USER_ID"
fi

if [[ -n "${SMOKE_TELEGRAM_ID:-}" || -n "${SMOKE_USER_ID:-}" ]]; then
  canonical_ran=1
  smoke_name="${SMOKE_DEVICE_NAME_PREFIX:-freeze-smoke}-$(date +%Y%m%d%H%M%S)"

  if [[ -n "${SMOKE_TELEGRAM_ID:-}" ]]; then
    header_name="X-Telegram-ID"
    header_value="$SMOKE_TELEGRAM_ID"
  else
    header_name="X-User-ID"
    header_value="$SMOKE_USER_ID"
  fi

  note "running canonical control-plane smoke via POST /devices/"
  if create_output="$(run_remote "$DO_APP_USER" "$DO_APP_HOST" "body_file=\$(mktemp) && status=\$(curl -sS -o \"\$body_file\" -w '%{http_code}' -H 'Content-Type: application/json' -H '$header_name: $header_value' -d '{\"name\":\"$smoke_name\"}' '$APP_API_BASE_URL/devices/' ) && printf 'STATUS=%s\n' \"\$status\" && cat \"\$body_file\" && rm -f \"\$body_file\"")"; then
    create_status="$(printf '%s\n' "$create_output" | sed -n 's/^STATUS=//p' | head -n1)"
    create_body="$(printf '%s\n' "$create_output" | sed '1{/^STATUS=/d;}')"

    if [[ "$create_status" == "201" ]]; then
      pass "canonical create-device request returned 201"
      canonical_passed=1
      assert_contains "$create_body" "PublicKey = $YC_WG_CLIENTS_PUBLIC_KEY" "generated config uses frozen YC listener public key"
      assert_contains "$create_body" "Endpoint = $CANONICAL_CLIENT_ENDPOINT" "generated config uses frozen client endpoint"
      assert_contains "$create_body" "AllowedIPs = $CANONICAL_ALLOWED_IPS" "generated config uses frozen allowed IPs"
      assert_contains "$create_body" "PersistentKeepalive = $CANONICAL_PERSISTENT_KEEPALIVE" "generated config uses frozen keepalive"

      created_device_id="$(printf '%s' "$create_body" | tr -d '\n' | sed -n 's/.*"id":\([0-9][0-9]*\).*/\1/p' | head -n1)"
      if [[ -n "$created_device_id" ]]; then
        if run_remote "$DO_APP_USER" "$DO_APP_HOST" "curl -fsS -X POST -H '$header_name: $header_value' '$APP_API_BASE_URL/devices/$created_device_id/revoke' >/dev/null"; then
          pass "disposable smoke device $created_device_id revoked"
        else
          warn "created disposable smoke device $created_device_id but revoke failed"
        fi
      else
        warn "canonical smoke succeeded but device id could not be parsed for revoke"
      fi
    else
      fail "canonical create-device request returned status $create_status"
      printf '%s\n' "$create_body" >&2
    fi
  else
    fail "canonical create-device request failed to execute on DO app host"
  fi
else
  warn "SMOKE_TELEGRAM_ID or SMOKE_USER_ID is not set; canonical POST /devices/ smoke skipped"
fi

note "running secondary runtime namespace SSH smoke"
runtime_secondary_ok=0
if runtime_smoke="$(run_remote "$DO_APP_USER" "$DO_APP_HOST" "docker run --rm --network \"container:$APP_API_CONTAINER\" -v /opt/vpn/secrets:/opt/vpn/secrets:ro alpine:3.20 sh -lc 'apk add --no-cache openssh-client >/dev/null && ssh -F $APP_SSH_CONFIG $APP_SSH_ALIAS \"echo SSH_OK user=\\\$(whoami)\"'")"; then
  runtime_secondary_ok=1
  assert_contains "$runtime_smoke" "SSH_OK" "runtime namespace SSH smoke returned SSH_OK"
else
  if (( canonical_passed )); then
    warn "runtime namespace SSH smoke failed even though canonical provisioning smoke passed"
  elif (( canonical_ran == 0 )); then
    fail "runtime namespace SSH smoke failed and no canonical provisioning smoke was configured"
  else
    fail "runtime namespace SSH smoke failed"
  fi
fi

if (( failures > 0 )); then
  printf '\nContour verification finished with %d failure(s) and %d warning(s).\n' "$failures" "$warnings" >&2
  exit 1
fi

printf '\nContour verification passed with %d warning(s).\n' "$warnings"
