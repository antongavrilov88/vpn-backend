# MVP VPN Backend Transport Runbook

This document is a template for the current manual WireGuard-based transport setup used by the MVP VPN backend. Fill every placeholder before using it in production or staging.

## Purpose

Use this runbook when you need to:

- bootstrap or re-check the WireGuard transport on the VPN server;
- add a new peer for a backend-managed client;
- remove a peer that is no longer allowed to connect;
- inspect runtime state during incident response or manual support.

Assumptions:

- WireGuard is already installed on the host;
- the operator has shell access to `<SERVER_HOSTNAME>` as a privileged user;
- the backend currently depends on this transport being managed manually.

## Server Info

| Field | Value |
| --- | --- |
| Environment | `<ENVIRONMENT>` |
| Hostname | `<SERVER_HOSTNAME>` |
| Public IP | `<SERVER_IP>` |
| SSH user | `<SSH_USER>` |
| SSH port | `<SSH_PORT>` |
| OS | `<SERVER_OS>` |
| WireGuard config path | `<WG_CONFIG_PATH>` |
| WireGuard interface name | `<WG_INTERFACE>` |
| Service manager | `<SERVICE_MANAGER>` |
| Persistent keepalive default | `<PERSISTENT_KEEPALIVE_SECONDS>` |

Optional notes:

- `<SERVER_NOTES>`

## WireGuard Interface

### Interface parameters

| Parameter | Placeholder |
| --- | --- |
| Interface name | `<WG_INTERFACE>` |
| Interface address | `<WG_SERVER_ADDRESS>/<VPN_CIDR>` |
| Listen port | `<WG_PORT>` |
| Private key path | `<WG_PRIVATE_KEY_PATH>` |
| Public key | `<WG_SERVER_PUBLIC_KEY>` |
| Config file | `<WG_CONFIG_PATH>` |
| PostUp commands | `<WG_POST_UP_COMMANDS>` |
| PostDown commands | `<WG_POST_DOWN_COMMANDS>` |

### Expected config shape

Use this as a reference structure for `<WG_CONFIG_PATH>`:

```ini
[Interface]
Address = <WG_SERVER_ADDRESS>/<VPN_CIDR>
ListenPort = <WG_PORT>
PrivateKey = <WG_SERVER_PRIVATE_KEY>
SaveConfig = <WG_SAVE_CONFIG>
PostUp = <WG_POST_UP_COMMANDS>
PostDown = <WG_POST_DOWN_COMMANDS>

# Peer blocks are appended manually or by an operator script.
[Peer]
PublicKey = <PEER_PUBLIC_KEY>
PresharedKey = <PEER_PRESHARED_KEY>
AllowedIPs = <PEER_VPN_IP>/32
PersistentKeepalive = <PERSISTENT_KEEPALIVE_SECONDS>
```

### Bootstrap checklist

1. Confirm the config file exists at `<WG_CONFIG_PATH>`.
2. Confirm the interface name is `<WG_INTERFACE>`.
3. Confirm the server address belongs to `<VPN_SUBNET>`.
4. Confirm the UDP port `<WG_PORT>` is reachable through the host firewall and cloud firewall.
5. Start or restart the interface through `<SERVICE_MANAGER>` only after validating syntax.

Example service commands:

```bash
sudo <SERVICE_MANAGER> status <WG_SERVICE_NAME>
sudo <SERVICE_MANAGER> restart <WG_SERVICE_NAME>
sudo <SERVICE_MANAGER> enable <WG_SERVICE_NAME>
```

If the setup uses `wg-quick`, replace `<WG_SERVICE_NAME>` with `wg-quick@<WG_INTERFACE>`.

## Subnet and Peer Addressing

### VPN subnet

| Item | Placeholder |
| --- | --- |
| VPN subnet | `<VPN_SUBNET>` |
| Server tunnel IP | `<WG_SERVER_ADDRESS>` |
| Peer CIDR size | `<PEER_ADDRESS_CIDR>` |
| Reserved range | `<RESERVED_VPN_RANGE>` |
| Dynamic/manual allocation rule | `<PEER_ALLOCATION_RULE>` |

### Address allocation rules

- Each peer must receive a unique tunnel IP inside `<VPN_SUBNET>`.
- Do not reuse an IP until the old peer entry has been removed from config and runtime state.
- Use `/32` in `AllowedIPs` for a single peer address unless your topology explicitly requires a larger route.
- Keep a separate inventory of mappings:

| Peer label | External identity | Public key | VPN IP | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `<PEER_NAME>` | `<PEER_OWNER_OR_CLIENT_ID>` | `<PEER_PUBLIC_KEY>` | `<PEER_VPN_IP>` | `<ACTIVE_OR_REVOKED>` | `<PEER_NOTES>` |

### Peer naming convention

Recommended fields to track in comments or inventory:

- `<PEER_NAME>`: stable human-readable label;
- `<PEER_OWNER_OR_CLIENT_ID>`: backend-side user, device, or tenant identifier;
- `<PEER_CREATED_AT>`: creation timestamp;
- `<PEER_ROTATED_AT>`: last key rotation timestamp.

## Commands For Add/Remove Peer

These commands are templates. Replace placeholders before execution.

### Add peer

1. Generate peer material on the client side or on a secure provisioning host.
2. Assign a free IP `<PEER_VPN_IP>` from `<VPN_SUBNET>`.
3. Add the peer to runtime state.
4. Persist the peer in `<WG_CONFIG_PATH>`.
5. Verify handshake and transfer counters.

Runtime add:

```bash
sudo wg set <WG_INTERFACE> \
  peer <PEER_PUBLIC_KEY> \
  preshared-key <PEER_PRESHARED_KEY_PATH> \
  allowed-ips <PEER_VPN_IP>/32 \
  persistent-keepalive <PERSISTENT_KEEPALIVE_SECONDS>
```

Config block to append:

```ini
# <PEER_NAME> / <PEER_OWNER_OR_CLIENT_ID>
[Peer]
PublicKey = <PEER_PUBLIC_KEY>
PresharedKey = <PEER_PRESHARED_KEY>
AllowedIPs = <PEER_VPN_IP>/32
PersistentKeepalive = <PERSISTENT_KEEPALIVE_SECONDS>
```

If the system relies on `wg-quick` config persistence, save runtime state carefully:

```bash
sudo wg showconf <WG_INTERFACE> | sudo tee <WG_CONFIG_PATH> >/dev/null
```

If config is managed manually, edit `<WG_CONFIG_PATH>` first and then reload:

```bash
sudo wg syncconf <WG_INTERFACE> <(sudo wg-quick strip <WG_CONFIG_PATH>)
```

### Remove peer

1. Identify the exact peer by public key and allocated IP.
2. Remove the peer from runtime state.
3. Remove the matching `[Peer]` block from `<WG_CONFIG_PATH>`.
4. Reload or resync the interface.
5. Confirm the peer no longer appears in `wg show`.

Runtime remove:

```bash
sudo wg set <WG_INTERFACE> peer <PEER_PUBLIC_KEY> remove
```

Config reload after cleanup:

```bash
sudo wg syncconf <WG_INTERFACE> <(sudo wg-quick strip <WG_CONFIG_PATH>)
```

### Minimal operator workflow

```bash
ssh -p <SSH_PORT> <SSH_USER>@<SERVER_IP>
sudo wg show <WG_INTERFACE>
sudo editor <WG_CONFIG_PATH>
sudo wg syncconf <WG_INTERFACE> <(sudo wg-quick strip <WG_CONFIG_PATH>)
sudo wg show <WG_INTERFACE>
```

## How To Inspect Runtime Status

### Primary commands

```bash
sudo wg show <WG_INTERFACE>
sudo wg show all
ip address show dev <WG_INTERFACE>
ip route show table all | grep <VPN_SUBNET>
sudo <SERVICE_MANAGER> status <WG_SERVICE_NAME>
```

### What to check

- Interface exists and is `UP`.
- Server address matches `<WG_SERVER_ADDRESS>/<VPN_CIDR>`.
- Expected peers are present.
- `latest handshake` is recent for active peers.
- `transfer` counters increase for peers that should be passing traffic.
- `allowed ips` match the intended `<PEER_VPN_IP>/32`.
- Firewall/NAT rules still exist if `<WG_POST_UP_COMMANDS>` depends on them.

### Deeper inspection

```bash
sudo wg show <WG_INTERFACE> dump
sudo journalctl -u <WG_SERVICE_NAME> --since "<LOG_LOOKBACK_WINDOW>"
sudo ss -lunp | grep <WG_PORT>
sudo tcpdump -ni any udp port <WG_PORT>
```

### Incident questions

- Is the problem limited to one peer or all peers?
- Did the peer ever complete a handshake?
- Was the same tunnel IP reused accidentally?
- Was `<WG_CONFIG_PATH>` changed without a matching runtime reload?
- Did a restart remove ephemeral changes because config was not persisted?

## Known Pitfalls

- `wg set` changes runtime state only. They can disappear after restart if `<WG_CONFIG_PATH>` was not updated.
- `SaveConfig = true` can overwrite manual formatting or comments in `<WG_CONFIG_PATH>`. Decide explicitly whether `<WG_SAVE_CONFIG>` should be enabled.
- Duplicate `AllowedIPs` across peers can cause routing conflicts and hard-to-debug traffic loss.
- Reusing `<PEER_VPN_IP>` too early can create intermittent misrouting if an old peer is still present somewhere.
- Missing firewall or NAT rules in `<WG_POST_UP_COMMANDS>` can make handshakes succeed while application traffic still fails.
- Key rotation must update both runtime state and persisted config.
- Editing the file and restarting without syntax validation can drop the whole interface.
- If the server sits behind cloud networking, host-level `ufw` or `iptables` checks are not enough; verify the external security group for `<WG_PORT>/udp`.
- `PersistentKeepalive` is usually required for peers behind NAT. Do not leave `<PERSISTENT_KEEPALIVE_SECONDS>` undefined for that case.

## Suggested Runbook Completion Checklist

Before this document is considered usable, fill in:

- all placeholders in the server info table;
- the exact contents or source of `<WG_POST_UP_COMMANDS>` and `<WG_POST_DOWN_COMMANDS>`;
- the real allocation rule for `<VPN_SUBNET>`;
- the authoritative command sequence for persistence and reload;
- the logging command that matches `<SERVICE_MANAGER>` on `<SERVER_OS>`;
- the location of the peer inventory used by the backend team.

## Revision Log

| Date | Author | Change |
| --- | --- | --- |
| `<YYYY-MM-DD>` | `<AUTHOR>` | Initial MVP transport runbook template |
