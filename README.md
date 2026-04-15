# VPN Backend MVP

Go backend for a personal/family VPN MVP.

## Stack

- Go
- chi
- pgx / Postgres
- Goose
- Docker Compose
- Telegram bot in the same repository

## Local Development

### 1. Create local env file

```bash
cp .env.example .env
```

For a machine-local override file, you can use:

```bash
cp .env.local.example .env.local
```

Both `make ...` and plain `go run ...` now read env files automatically in this order:

- `.env.local`
- `.env`

The example files are templates only. They are not loaded automatically.

For local work, keep real values in ignored files:

- `.env` for shared local runtime defaults
- `.env.local` for machine-specific values and secrets

That keeps git clean while still allowing `make ...` and `go run ...` to work without manual `export` commands.

Fill these only when you want `CreateDevice` to touch a real proxy/VPN setup:

- `DEVICE_PRIVATE_KEY_CIPHER_KEY`
- `VPN_SERVER_PUBLIC_KEY`
- `VPN_SERVER_ENDPOINT`
- `VPN_SERVER_TUNNEL_ADDRESS`
- `VPN_ALLOWED_IPS`
- `PROXY_SSH_HOST`
- `PROXY_SSH_USER`
- `PROXY_SSH_PRIVATE_KEY_PATH`
- `PROXY_SSH_KNOWN_HOSTS_PATH` or `PROXY_SSH_INSECURE_SKIP_HOST_KEY_CHECK=true`
- `PROXY_ADD_PEER_COMMAND`
- `PROXY_REMOVE_PEER_COMMAND`

If `VPN_ALLOWED_IPS` is omitted, generated client configs default to `0.0.0.0/0`.
Set `VPN_ALLOWED_IPS` only when you need a different client-route policy.

Generate the private-key cipher key locally when needed:

```bash
openssl rand -base64 32
```

For the bot, the minimum additional values are:

- `TELEGRAM_BOT_TOKEN`
- `BACKEND_API_BASE_URL`

### 2. Start local Postgres

```bash
make db-up
```

The local database uses Docker Compose.

Detailed guide:
- [docs/dev-db.md](/Users/antongavrilov/Desktop/workspace/vpn-backend/docs/dev-db.md)

### 3. Run database migrations

```bash
make migrate-up
```

The project uses Goose as a pinned Go tool. A separate global `goose` binary is not required.

Preferred commands:

```bash
make migrate-status
make migrate-up
make migrate-down
```

Direct invocation also works on Go versions that support tool dependencies:

```bash
go tool goose -dir migrations postgres "$DB_URL" status
```

Migration workflow:
- [docs/migrations.md](/Users/antongavrilov/Desktop/workspace/vpn-backend/docs/migrations.md)

### 4. Start backend API

```bash
make api-run
```

The API exposes:

- `GET /live`
- `GET /health`
- `GET /ready`
- minimal device lifecycle HTTP routes

If VPN/proxy env vars are still empty, the API still starts, but real provisioning flows such as `CreateDevice` remain unavailable.

### 5. Start Telegram bot

In a separate terminal:

```bash
make bot-run
```

Bot setup and behavior:
- [docs/telegram-bot.md](/Users/antongavrilov/Desktop/workspace/vpn-backend/docs/telegram-bot.md)

## Useful Commands

```bash
make db-up
make db-down
make db-logs
make migrate-status
make migrate-up
make api-run
make bot-run
```

## Notes

- The bot is a thin client over backend API.
- The bot does not access the database directly.
- Device provisioning depends on real VPN/proxy settings and is not available until those env values are configured.
- For the smoothest local workflow, prefer `make ...` targets or keep settings in `.env` / `.env.local`.
