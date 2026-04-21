# Environment Setup And Testing

This guide explains:

- which env files exist in this repository and on the hosts
- what every env field means
- where to get each value
- how to update env safely
- how to test the contour without guessing

Use this together with:

- [README.md](/Users/antongavrilov/Desktop/workspace/vpn-backend/README.md)
- [docs/runbooks/golden-contour.md](/Users/antongavrilov/Desktop/workspace/vpn-backend/docs/runbooks/golden-contour.md)
- [docs/secrets-inventory.md](/Users/antongavrilov/Desktop/workspace/vpn-backend/docs/secrets-inventory.md)

## Config File Map

| File | Where it lives | Purpose |
| --- | --- | --- |
| `.env.example` | repo | shared template for local `.env` |
| `.env.local.example` | repo | template for machine-specific `.env.local` overrides |
| `.env.freeze` | repo | frozen operational facts + smoke-test parameters |
| `/opt/vpn/env/api.env` | DO app host | runtime env for `app-api-1` |
| `/opt/vpn/env/bot.env` | DO app host | runtime env for `app-bot-1` |

## Important Load Rules

### Local code path

The application loads env files in this order:

1. `.env`
2. `.env.local` overrides `.env`

That behavior comes from [internal/config/dotenv.go](/Users/antongavrilov/Desktop/workspace/vpn-backend/internal/config/dotenv.go).

### App-host container path

The deployed containers do not read repo `.env` files directly.

They get runtime env from:

- `/opt/vpn/env/api.env`
- `/opt/vpn/env/bot.env`

through [deploy/docker-compose.apphost.yml](/Users/antongavrilov/Desktop/workspace/vpn-backend/deploy/docker-compose.apphost.yml).

Important operational rule:

- editing `/opt/vpn/env/api.env` or `/opt/vpn/env/bot.env` is not enough by itself
- `docker restart` does not reload `env_file`
- after env changes, recreate the container:

```bash
cd /opt/vpn
docker compose up -d --force-recreate api
docker compose up -d --force-recreate bot
```

## API Env Reference

These fields live in local `.env` or on the app host in `/opt/vpn/env/api.env`.

### App / HTTP

| Variable | Meaning | Where to get it / what to put |
| --- | --- | --- |
| `APP_ENV` | app mode | `development` locally, `production` on app host |
| `HTTP_ADDR` | API listen address | usually `:8080` |
| `HTTP_READ_TIMEOUT` | request read timeout | usually keep code default `5s` |
| `HTTP_WRITE_TIMEOUT` | response write timeout | usually keep `10s` |
| `HTTP_IDLE_TIMEOUT` | HTTP keep-alive timeout | usually keep `30s` |
| `HTTP_REQUEST_TIMEOUT` | request deadline | usually keep `30s` |
| `HTTP_READINESS_TIMEOUT` | readiness deadline | usually keep `2s` |
| `HTTP_SHUTDOWN_TIMEOUT` | graceful shutdown budget | usually keep `10s` |

### Database

| Variable | Meaning | Where to get it / what to put |
| --- | --- | --- |
| `DB_URL` | optional full Postgres DSN | use this only when you prefer one explicit DSN |
| `POSTGRES_HOST` | Postgres host | local dev: `localhost`; app host: managed DB or compose DB host |
| `POSTGRES_PORT` | Postgres port | local dev: `5433`; otherwise your real DB port |
| `POSTGRES_DB` | DB name | local dev default: `vpn_mvp` |
| `POSTGRES_USER` | DB user | from your Postgres setup |
| `POSTGRES_PASSWORD` | DB password | from your Postgres setup |
| `POSTGRES_SSL_MODE` | SSL mode | local dev: `disable`; managed DBs often `require` |
| `DB_MAX_CONNS` | optional pool ceiling | set when you want explicit staging/prod pool limits |
| `DB_MIN_CONNS` | optional pool floor | set when you want warm idle connections |
| `DB_MAX_CONN_LIFETIME` | optional pool lifetime | for long-running staging/prod pools |
| `DB_MAX_CONN_IDLE_TIME` | optional pool idle timeout | for long-running staging/prod pools |
| `DB_HEALTH_CHECK_PERIOD` | optional pool health cadence | usually `1m` if you want it explicit |

### Device private-key encryption

| Variable | Meaning | Where to get it / what to put |
| --- | --- | --- |
| `DEVICE_PRIVATE_KEY_CIPHER_KEY` | symmetric key used to encrypt stored client private keys | generate once per environment with `openssl rand -base64 32` and keep it stable |

### Client-config generation

| Variable | Meaning | Where to get it / what to put |
| --- | --- | --- |
| `VPN_SERVER_PUBLIC_KEY` | WireGuard public key written into generated client configs as `[Peer].PublicKey` | `sudo wg show wg-clients public-key` on the YC entry host |
| `VPN_SERVER_ENDPOINT` | client-visible UDP endpoint | public IP/domain and port of `wg-clients`, for example `111.88.157.89:51821` |
| `VPN_SERVER_TUNNEL_ADDRESS` | server-side address/subnet of the client-facing interface | from `/etc/wireguard/wg-clients.conf`, `Address = ...` |
| `VPN_ALLOWED_IPS` | routes pushed to clients | `0.0.0.0/0` for full tunnel; leave empty if you want backend default |
| `VPN_DNS` | DNS servers pushed to clients | your chosen resolver list, e.g. `1.1.1.1` |
| `VPN_PERSISTENT_KEEPALIVE` | client keepalive seconds | `25` is the current contour value |

### Provisioning SSH / control plane

| Variable | Meaning | Where to get it / what to put |
| --- | --- | --- |
| `PROXY_SSH_HOST` | provisioning SSH target | SSH alias like `yc-vpnmgr` when using a dedicated config, or raw host/IP otherwise |
| `PROXY_SSH_PORT` | provisioning SSH port | usually `22` |
| `PROXY_SSH_USER` | provisioning SSH user | usually `vpnmgr` |
| `PROXY_SSH_CONFIG_PATH` | dedicated OpenSSH config file | app host container path is usually `/run/secrets/ssh_config.app` |
| `PROXY_SSH_PRIVATE_KEY_PATH` | private key path for native Go SSH fallback mode | app host container path is usually `/run/secrets/proxy_ssh_key` |
| `PROXY_SSH_KNOWN_HOSTS_PATH` | host-key verification file for native Go SSH fallback mode | app host container path is usually `/run/secrets/proxy_known_hosts` |
| `PROXY_SSH_INSECURE_SKIP_HOST_KEY_CHECK` | bypass host-key verification | keep `false` unless you are doing an intentional bootstrap/debug escape hatch |
| `PROXY_ADD_PEER_COMMAND` | remote add-peer command | current contour: `sudo -n /usr/local/bin/vpn-peer-add` |
| `PROXY_REMOVE_PEER_COMMAND` | remote remove-peer command | current contour: `sudo -n /usr/local/bin/vpn-peer-remove` |
| `PROXY_SSH_TIMEOUT` | timeout for SSH provisioning step | current contour commonly uses `5s` |

### App-host files that back the SSH path

These are mounted into the API container:

- `/opt/vpn/secrets/proxy_ed25519`
- `/opt/vpn/secrets/proxy_known_hosts`
- `/opt/vpn/secrets/ssh_config.app`

## Bot Env Reference

These fields live in local `.env` when you run the bot locally, or in `/opt/vpn/env/bot.env` on the app host.

| Variable | Meaning | Where to get it / what to put |
| --- | --- | --- |
| `APP_ENV` | bot mode | `development` locally, `production` on app host |
| `TELEGRAM_BOT_TOKEN` | BotFather token | get from BotFather |
| `TELEGRAM_POLL_TIMEOUT` | Telegram long-poll timeout | usually `30s` |
| `BACKEND_API_BASE_URL` | bot -> API URL | local: `http://127.0.0.1:8080`; compose/app-host: `http://api:8080` |
| `BACKEND_API_TIMEOUT` | bot -> API timeout | usually `5s` or `10s` |

## Freeze-State Env Reference

These fields live in `.env.freeze` and parameterize the freeze/verification scripts.

| Variable | Meaning | Where to get it / what to put |
| --- | --- | --- |
| `YC_HOST` / `YC_USER` | shell access target for the YC entry host | operator SSH target that can run `sudo wg show` |
| `DO_APP_HOST` / `DO_APP_USER` | shell access target for the DO app host | operator SSH target for the app host |
| `DO_EGRESS_HOST` / `DO_EGRESS_USER` | shell access target for the DO egress host | fill when you want egress shell verification and collection |
| `APP_PROJECT_NAME` | compose project name on the app host | current contour: `app` |
| `APP_API_CONTAINER` | API container name | current contour: `app-api-1` |
| `APP_BOT_CONTAINER` | bot container name | current contour: `app-bot-1` |
| `APP_API_BASE_URL` | app-host loopback URL for API checks | usually `http://127.0.0.1:8080` |
| `APP_SSH_CONFIG` | host-side SSH config file used by the proven secondary runtime smoke | current contour: `/opt/vpn/secrets/ssh_config.app` |
| `APP_SSH_ALIAS` | SSH alias inside that config | current contour: `yc-vpnmgr` |
| `YC_WG_CLIENTS_IF` | client-facing interface name | current contour: `wg-clients` |
| `YC_WG_UPSTREAM_IF` | YC -> DO upstream interface name | current contour: `wg-upstream` |
| `DO_EGRESS_WG_IF` | egress WireGuard interface name | current contour: `wg-upstream` |
| `YC_WG_CLIENTS_PORT` | client-facing UDP port | `51821` |
| `YC_WG_CLIENTS_PUBLIC_KEY` | public key of the real client-facing listener | `sudo wg show wg-clients public-key` |
| `CANONICAL_CLIENT_ENDPOINT` | endpoint that generated client configs must contain | current contour: `111.88.157.89:51821` |
| `DO_EGRESS_PUBLIC_ENDPOINT` | public WireGuard endpoint seen from YC | current contour: `206.189.53.27:51822` |
| `CLIENT_SUBNET` | client subnet | current contour: `10.69.0.0/24` |
| `SAMPLE_CLIENT_IP` | known-good sample client IP for route-get checks | current contour: `10.69.0.10` |
| `CANONICAL_ALLOWED_IPS` | expected AllowedIPs in generated configs | current contour: `0.0.0.0/0` |
| `CANONICAL_DNS` | expected DNS value in generated configs | current contour: `1.1.1.1` |
| `CANONICAL_PERSISTENT_KEEPALIVE` | expected keepalive in generated configs | current contour: `25` |
| `TRANSIT_SUBNET` | optional upstream transit subnet | fill only when you explicitly want it frozen |
| `SMOKE_TELEGRAM_ID` | optional Telegram identity for disposable canonical smoke | set exactly one of this or `SMOKE_USER_ID` |
| `SMOKE_USER_ID` | optional backend user id for disposable canonical smoke | set exactly one of this or `SMOKE_TELEGRAM_ID` |
| `SMOKE_DEVICE_NAME_PREFIX` | disposable device-name prefix used by `verify_contour.sh` | any safe prefix, current default: `freeze-smoke` |

## How To Fill The Files

### Local `.env`

1. Copy the template:
```bash
cp .env.example .env
```
2. If you only want local API + DB work:
   - keep Postgres values pointing to the local compose DB
   - leave VPN / proxy / Telegram fields empty
3. If you want real provisioning from local:
   - fill `DEVICE_PRIVATE_KEY_CIPHER_KEY`
   - fill all `VPN_*`
   - fill all required `PROXY_*`
4. If you want to run the bot locally too:
   - fill `TELEGRAM_BOT_TOKEN`
   - fill `BACKEND_API_BASE_URL=http://127.0.0.1:8080`

### Machine-specific `.env.local`

1. Copy the template only if you need overrides:
```bash
cp .env.local.example .env.local
```
2. Uncomment only the fields you actually want to override.
3. Do not leave blank active assignments in `.env.local`; they override `.env`.

### App-host `/opt/vpn/env/api.env`

1. Put production values there for the API container.
2. Confirm the critical client-config fields:
   - `VPN_SERVER_PUBLIC_KEY`
   - `VPN_SERVER_ENDPOINT`
   - `VPN_ALLOWED_IPS`
   - `VPN_DNS`
   - `VPN_PERSISTENT_KEEPALIVE`
3. Confirm the provisioning SSH fields:
   - `PROXY_SSH_HOST`
   - `PROXY_SSH_CONFIG_PATH`
   - `PROXY_ADD_PEER_COMMAND`
   - `PROXY_REMOVE_PEER_COMMAND`
4. After changing the file:
```bash
cd /opt/vpn
docker compose up -d --force-recreate api
```

### App-host `/opt/vpn/env/bot.env`

1. Put:
   - `TELEGRAM_BOT_TOKEN`
   - `BACKEND_API_BASE_URL=http://api:8080`
   - timeout values if you want explicit overrides
2. After changing the file:
```bash
cd /opt/vpn
docker compose up -d --force-recreate bot
```

### Freeze-state `.env.freeze`

1. Fill the three SSH targets you want the scripts to use:
   - YC entry
   - DO app host
   - optional DO egress shell target
2. Freeze the proven WireGuard facts:
   - listener key
   - listener port
   - endpoint
   - subnet
3. Leave `SMOKE_TELEGRAM_ID` / `SMOKE_USER_ID` empty for read-only verification.
4. Fill exactly one of them when you want a disposable create/revoke smoke.

## How To Test Safely

### 1. Read-only contour verification

This does not change the contour, except for a temporary `docker run --rm` sidecar on the app host:

```bash
bash scripts/verify_contour.sh
```

It checks:

- API health on the app host
- `wg-clients` key and port on YC
- `wg-clients` -> `wg-upstream` routing on YC
- DO egress shell checks when `DO_EGRESS_HOST` / `DO_EGRESS_USER` are configured
- secondary runtime SSH smoke through the `app-api-1` namespace

### 2. Canonical provisioning smoke

Set exactly one of these in `.env.freeze`:

- `SMOKE_TELEGRAM_ID`
- `SMOKE_USER_ID`

Then run:

```bash
bash scripts/verify_contour.sh
```

This adds:

- a real disposable `POST /devices/` call against the running API
- invariant checks on the generated config
- an automatic revoke attempt for the disposable device

### 3. Redacted operational snapshot

```bash
bash scripts/freeze_collect.sh
```

Optional parameters:

- `FREEZE_ENV_FILE=/path/to/custom.freeze`
- `ARTIFACT_ROOT=/path/to/output-dir`

The result goes under `artifacts/`.

### 4. Raw recovery bundle

```bash
bash scripts/backup_bundle.sh
```

Optional parameters:

- `FREEZE_ENV_FILE=/path/to/custom.freeze`
- `BUNDLE_ROOT=/path/to/output-dir`

The result goes under `backup/`.

This script copies live recovery material and secrets. Keep the output out of git.

## Minimal Practical Test Flow

For everyday ops, the smallest useful sequence is:

1. `bash scripts/verify_contour.sh`
2. if you changed env on the app host, recreate the affected container
3. if you want a fresh frozen snapshot, run `bash scripts/freeze_collect.sh`
4. if you want a full recoverability bundle, run `bash scripts/backup_bundle.sh`
