# Local Postgres For Development

This project uses Docker Compose to run a local Postgres instance for development and upcoming migrations.

Before using the local setup, create a real `.env` file from the example:

```bash
cp .env.example .env
```

Docker Compose reads `.env` automatically. It does not read `.env.example`.

## Connection Parameters

The local database uses the values from `.env`:

| Variable | Value |
| --- | --- |
| `POSTGRES_HOST` | `localhost` |
| `POSTGRES_PORT` | `5433` |
| `POSTGRES_DB` | `vpn_mvp` |
| `POSTGRES_USER` | `postgres` |
| `POSTGRES_PASSWORD` | `postgres` |
| `POSTGRES_SSL_MODE` | `disable` |

The application can build `DB_URL` from these variables automatically. The resulting connection string is:

```text
postgres://postgres:postgres@localhost:5433/vpn_mvp?sslmode=disable
```

If you prefer, you can set `DB_URL` directly and skip the derived Postgres variables.

## Start The Database

Using Docker Compose directly:

```bash
docker compose up -d postgres
```

Using Make:

```bash
make db-up
```

## Stop The Database

Using Docker Compose directly:

```bash
docker compose down
```

Using Make:

```bash
make db-down
```

## View Logs

Using Docker Compose directly:

```bash
docker compose logs -f postgres
```

Using Make:

```bash
make db-logs
```

## Check Container Health

```bash
docker compose ps
```

Wait until the `postgres` service reports a `healthy` status.

## Connect To The Database

With `psql` on the host:

```bash
psql "postgres://postgres:postgres@localhost:5433/vpn_mvp?sslmode=disable"
```

Inside the container:

```bash
docker compose exec postgres psql -U postgres -d vpn_mvp
```

## Notes

- Data is stored in the named Docker volume `postgres_data`.
- No backend container is used yet. The database runs as the only local development service.
- The default host port is `5433` to avoid conflicts with an already installed local Postgres on `5432`.
- If `5433` is also busy on your machine, change `POSTGRES_PORT` before running `docker compose up -d`.
