# Secrets Inventory

This inventory lists the files, environment variables, and host-side material required to recover and operate the current frozen VPN contour.

It is intentionally operational and scoped to the current working setup.

## Important Rules

- Do not commit real secrets to git.
- `.env.example`, `.env.local.example`, and `.env.freeze` are templates or frozen facts only.
- Raw recovery material belongs in local backup bundles, not in the repository.
- The current contour mixes two kinds of values:
  - true secrets
  - non-secret but operationally critical invariants such as the correct WireGuard listener public key

## DO App Host

### Runtime env files

These files are host-side sources of truth for the running containers:

| Path | Consumer | Type | Notes |
| --- | --- | --- | --- |
| `/opt/vpn/env/api.env` | `app-api-1` | mixed | includes both secrets and operational config |
| `/opt/vpn/env/bot.env` | `app-bot-1` | mixed | includes Telegram bot token and backend API base URL |

Key values in `api.env` for the current contour:

- `DEVICE_PRIVATE_KEY_CIPHER_KEY`
- `VPN_SERVER_PUBLIC_KEY`
- `VPN_SERVER_ENDPOINT`
- `VPN_SERVER_TUNNEL_ADDRESS`
- `VPN_ALLOWED_IPS`
- `VPN_DNS`
- `VPN_PERSISTENT_KEEPALIVE`
- `PROXY_SSH_HOST`
- `PROXY_SSH_CONFIG_PATH`
- `PROXY_SSH_TIMEOUT`
- `PROXY_ADD_PEER_COMMAND`
- `PROXY_REMOVE_PEER_COMMAND`

Operationally critical invariant:

- `VPN_SERVER_PUBLIC_KEY` must match the live public key of YC `wg-clients`
- for the frozen contour that value is:
  `LWLQRaO8fEtIFmUkjg8BJoXPXra2BX81dKh/tqCT+h0=`

Important runtime note:

- editing `/opt/vpn/env/api.env` is not enough by itself
- `docker restart app-api-1` does not reread `env_file`
- after env changes, recreate the container:

```bash
docker compose up -d --force-recreate api
```

### Mounted secret files

The app-host compose file mounts these into the API container in
[deploy/docker-compose.apphost.yml](/Users/antongavrilov/Desktop/workspace/vpn-backend/deploy/docker-compose.apphost.yml:22):

| Host path | Container path | Type | Purpose |
| --- | --- | --- | --- |
| `/opt/vpn/secrets/proxy_ed25519` | `/run/secrets/proxy_ssh_key` | secret | SSH private key for provisioning path |
| `/opt/vpn/secrets/proxy_known_hosts` | `/run/secrets/proxy_known_hosts` | sensitive config | host key verification for YC |
| `/opt/vpn/secrets/ssh_config.app` | `/run/secrets/ssh_config.app` | sensitive config | dedicated OpenSSH client config for runtime namespace / control plane |

Additional app-host local files worth treating as recovery material:

- the deployed compose file uploaded by the workflow to `/opt/vpn/docker-compose.yml`
- any still-active working copy under `/opt/vpn/app/` if the host has not yet been normalized to the workflow layout
- deployed image tag / compose project state

### Bot runtime values

Important keys in `bot.env`:

- `TELEGRAM_BOT_TOKEN`
- `BACKEND_API_BASE_URL`

These are not used to generate WireGuard client configs, but they are required to operate the bot flow end-to-end.

## YC Entry Host

Files and state to preserve:

| Path / item | Type | Purpose |
| --- | --- | --- |
| `/etc/wireguard/wg-clients.conf` | secret + config | client-facing WireGuard interface |
| `/etc/wireguard/wg-upstream.conf` | secret + config | upstream tunnel toward DO egress |
| `/usr/local/bin/vpn-peer-add` | operational script | control-plane add-peer contract |
| `/usr/local/bin/vpn-peer-remove` | operational script | control-plane remove-peer contract |
| `~vpnmgr/.ssh/authorized_keys` | secret-ish access control | app-host provisioning access |

Runtime invariants on YC:

- `wg-clients` public key:
  `LWLQRaO8fEtIFmUkjg8BJoXPXra2BX81dKh/tqCT+h0=`
- `wg-clients` listen port:
  `51821`

These values are not secret, but they are operationally critical and must be backed up / documented exactly.

## DO Egress Host

Files and state to preserve:

| Path / item | Type | Purpose |
| --- | --- | --- |
| `/etc/wireguard/wg-upstream.conf` | secret + config | upstream WireGuard interface |
| `iptables-save` output | operational state | forwarding + NAT rules |
| `net.ipv4.ip_forward` setting | operational state | required for egress forwarding |

Frozen fact:

- the proven public WireGuard endpoint seen from YC is `206.189.53.27:51822`

Still operator-specific:

- the shell-access host/user for DO egress are not frozen in this repo yet
- put them into `.env.freeze` only when you want host-side collection or verification scripts to SSH into the DO egress box

## Repo-Tracked Non-Secrets

These files are safe to track and should reflect the current frozen contour:

| File | Purpose |
| --- | --- |
| `.env.freeze` | frozen, non-secret operational facts |
| `docs/runbooks/golden-contour.md` | canonical contour and smoke tests |
| `docs/secrets-inventory.md` | this inventory |

## Recovery Priority

If only one host survives, recover in this order:

1. DO app host env and SSH provisioning material
2. YC WireGuard configs and peer-management scripts
3. DO egress WireGuard config and NAT/FORWARD state

Why:

- the app host controls provisioning and config generation
- YC holds the client listener truth
- DO egress holds the live NAT exit path

## Backup Guidance

The backup bundle script should capture raw copies of:

- `/opt/vpn/env/api.env`
- `/opt/vpn/env/bot.env`
- `/opt/vpn/secrets/proxy_ed25519`
- `/opt/vpn/secrets/proxy_known_hosts`
- `/opt/vpn/secrets/ssh_config.app`
- YC WireGuard configs
- DO egress WireGuard config
- peer-management scripts
- iptables-save outputs

Those artifacts belong under `backup/` locally and are intentionally git-ignored.
