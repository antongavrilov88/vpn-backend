# Proxy Peer Management Contract

This document defines the MVP contract between the backend and the VPN proxy host for peer lifecycle operations.

## Purpose

The backend does not call a proxy-side HTTP API.

The backend connects to the proxy host over SSH and executes fixed remote scripts for peer lifecycle operations.

The proxy-side scripts own:

- WireGuard runtime updates
- config persistence
- interface resync or reload
- any host-specific operational details

The backend only supplies validated peer identifiers and connection settings.

## Transport Direction

Backend to proxy transport is:

- SSH connection to the proxy host
- execution of fixed commands
- no client private key sent to the proxy

Backend inputs for the proxy are limited to:

- peer public key
- assigned client IP

## Remote Script Contract

### Add peer

Command shape:

```bash
sudo /usr/local/bin/vpn-peer-add <public_key> <assigned_ip>
```

Argument order:

1. `<public_key>`
2. `<assigned_ip>`

Expected meaning:

- `<public_key>`: WireGuard client public key in standard base64 form
- `<assigned_ip>`: peer route in CIDR form, for example `10.68.0.2/32`

Expected behavior:

- add the peer to WireGuard runtime state
- persist the peer in server-side WireGuard config if required by the host setup
- ensure the peer is active after the command finishes successfully

### Remove peer

Command shape:

```bash
sudo /usr/local/bin/vpn-peer-remove <public_key>
```

Argument order:

1. `<public_key>`

Expected behavior:

- remove the peer from WireGuard runtime state
- remove or disable the peer in persistent config if required by the host setup
- ensure the peer is no longer active after the command finishes successfully

## Success And Failure Rules

Success contract:

- exit code `0` means the operation succeeded
- stdout is optional and informational only
- backend should not depend on stdout format for success

Failure contract:

- non-zero exit code means the operation failed
- stderr should contain a human-readable failure reason when possible
- backend may capture stdout/stderr for logs and error wrapping, but should not parse them as structured output in MVP

Idempotency expectation:

- `vpn-peer-add` should fail if the exact peer cannot be created safely
- `vpn-peer-remove` may treat an already-missing peer as success or a clearly explained non-fatal failure

## Backend SSH Config Shape

The backend SSH transport config is defined by these settings:

| Env var | Purpose |
| --- | --- |
| `PROXY_SSH_HOST` | proxy host or IP |
| `PROXY_SSH_PORT` | SSH port |
| `PROXY_SSH_USER` | SSH user |
| `PROXY_SSH_PRIVATE_KEY_PATH` | path to backend SSH private key used for proxy access |
| `PROXY_SSH_KNOWN_HOSTS_PATH` | path to known hosts file for proxy host verification |
| `PROXY_SSH_INSECURE_SKIP_HOST_KEY_CHECK` | explicit insecure fallback for non-production bootstrap only |
| `PROXY_ADD_PEER_COMMAND` | fixed remote add-peer command path |
| `PROXY_REMOVE_PEER_COMMAND` | fixed remote remove-peer command path |
| `PROXY_SSH_TIMEOUT` | SSH connect/command timeout |

Recommended sample values:

```text
PROXY_ADD_PEER_COMMAND=sudo /usr/local/bin/vpn-peer-add
PROXY_REMOVE_PEER_COMMAND=sudo /usr/local/bin/vpn-peer-remove
```

## Security Notes

- backend must not send client private key to the proxy
- backend must validate public key and assigned IP before building the remote command
- backend should treat command paths as fixed config, not user input
- host key verification should use `known_hosts` by default
- insecure host key skipping must be explicit and only used when unavoidable

## Out Of Scope For This Contract

This contract does not define:

- peer status inspection over SSH
- reconciliation behavior
- stdout JSON payloads
- client config generation
- QR generation
- Telegram or external API integration
