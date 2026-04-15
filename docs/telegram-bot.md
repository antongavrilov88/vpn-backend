# Telegram Bot

## Minimal setup

1. Create a local env file:

```bash
cp .env.example .env
```

2. Replace the required placeholders in `.env`:

- `TELEGRAM_BOT_TOKEN`
- `BACKEND_API_BASE_URL`

For local development, `BACKEND_API_BASE_URL=http://localhost:8080` is the expected value if the API runs with the default local HTTP address.

## Required env values

```dotenv
APP_ENV=development
TELEGRAM_BOT_TOKEN=<TELEGRAM_BOT_TOKEN>
TELEGRAM_POLL_TIMEOUT=30s
BACKEND_API_BASE_URL=http://localhost:8080
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
- `/devices`
- `/newdevice`
- `/config <device_id>`
- `/revoke <device_id>`

For normal device management, `/devices` is the primary flow:

- it sends readable per-device cards
- active devices include inline actions for:
  - `Get config`
  - `Show QR`
  - `Revoke`
- revoked devices are hidden from the main list

For the main create flow:

- tap `Create device` or send `/newdevice`
- the bot asks for the device name in the next message
- once you reply with the name, the bot provisions the device and returns the config

The bot also keeps a persistent reply keyboard with:

- `My devices`
- `Create device`
- `Help`

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
