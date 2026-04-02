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

Then replace the required placeholders in `.env`.

For the current bot skeleton, the minimum required values are:

- `TELEGRAM_BOT_TOKEN`
- `BACKEND_API_BASE_URL`

For full device provisioning flows later, you will also need to set the VPN/proxy values.

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
