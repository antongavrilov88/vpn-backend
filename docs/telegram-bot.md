# Telegram Bot

## Minimal setup

1. Create a local env file:

```bash
cp .env.example .env
```

2. Replace the required placeholders in `.env`:

- `TELEGRAM_BOT_TOKEN`
- `BACKEND_API_BASE_URL`

For local development, `BACKEND_API_BASE_URL=http://localhost:18080` is the expected value if the API runs on the current local port.

## Required env values

```dotenv
APP_ENV=development
TELEGRAM_BOT_TOKEN=<TELEGRAM_BOT_TOKEN>
TELEGRAM_POLL_TIMEOUT=30s
BACKEND_API_BASE_URL=http://localhost:18080
BACKEND_API_TIMEOUT=5s
```

## Start backend API

```bash
make api-run
```

Or:

```bash
go run ./cmd/api
```

## Start Telegram bot

In a separate terminal:

```bash
make bot-run
```

Or:

```bash
go run ./cmd/bot
```

## Current bot behavior

The bot currently supports:

- `/start`
- `/help`

The bot is a thin client over backend API:

- it does not access the database directly
- it does not manage WireGuard directly
- it checks backend API availability on startup
- `/start` also reflects current backend availability

## Notes

- The bot uses Telegram long polling.
- The machine running the bot must be able to reach:
  - `https://api.telegram.org`
  - your backend API URL from `BACKEND_API_BASE_URL`
