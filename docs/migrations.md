# Database Migrations With Goose

This project uses Goose for SQL migrations.

## Migration Directory

Migration files live in:

```text
migrations/
```

The directory is intentionally empty at this stage. Actual schema migrations will be added later.

## How Goose Is Run

The project pins Goose in `go.mod` as a Go tool and runs it through `go tool goose`, so a separate global installation is not required.

All commands are exposed through `make`.

Before using migrations, create a local `.env` file:

```bash
cp .env.example .env
```

The Makefile reads `.env` automatically. If `.env` is missing, it falls back to the local development defaults.

## Available Commands

Create a new migration:

```bash
make migrate-create name=<migration_name>
```

Apply all pending migrations:

```bash
make migrate-up
```

Roll back the most recent migration:

```bash
make migrate-down
```

Check migration status:

```bash
make migrate-status
```

When the `migrations/` directory is still empty, the status, up, and down targets print a clear message and exit successfully.

## Database Connection

Migration commands use the local database connection settings from environment variables:

- `POSTGRES_HOST`
- `POSTGRES_PORT`
- `POSTGRES_DB`
- `POSTGRES_USER`
- `POSTGRES_PASSWORD`

The Makefile builds the Postgres connection string from those values.

Default local values:

```text
POSTGRES_HOST=localhost
POSTGRES_PORT=5433
POSTGRES_DB=vpn_mvp
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
```

## Typical Workflow

1. Start the local database:

```bash
make db-up
```

2. Create a migration:

```bash
make migrate-create name=create_users_table
```

3. Edit the generated SQL files in `migrations/`.

4. Apply migrations:

```bash
make migrate-up
```

5. Check status:

```bash
make migrate-status
```

6. Roll back the latest migration if needed:

```bash
make migrate-down
```

## Notes

- This setup is for local development only.
- No schema migrations are added in this task.
- No backend container is required for running migrations.
