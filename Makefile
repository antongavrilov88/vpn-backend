ifneq (,$(wildcard .env))
include .env
export
endif

POSTGRES_HOST ?= localhost
POSTGRES_PORT ?= 5433
POSTGRES_DB ?= vpn_mvp
POSTGRES_USER ?= postgres
POSTGRES_PASSWORD ?= postgres
POSTGRES_SSL_MODE ?= disable

DB_URL ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_HOST):$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=$(POSTGRES_SSL_MODE)
MIGRATIONS_DIR := migrations
GO := env GOCACHE=$(CURDIR)/.cache/go-build go
GOOSE := $(GO) tool goose

.PHONY: db-up db-down db-logs migrate-create migrate-up migrate-down migrate-status api-run bot-run

db-up:
	docker compose up -d postgres

db-down:
	docker compose down

db-logs:
	docker compose logs -f postgres

migrate-create:
ifndef name
	$(error usage: make migrate-create name=<migration_name>)
endif
	$(GOOSE) -dir $(MIGRATIONS_DIR) create $(name) sql

migrate-up:
	@if find $(MIGRATIONS_DIR) -maxdepth 1 -name '*.sql' | grep -q .; then \
		$(GOOSE) -dir $(MIGRATIONS_DIR) postgres "$(DB_URL)" up; \
	else \
		echo "no migration files found in $(MIGRATIONS_DIR)"; \
	fi

migrate-down:
	@if find $(MIGRATIONS_DIR) -maxdepth 1 -name '*.sql' | grep -q .; then \
		$(GOOSE) -dir $(MIGRATIONS_DIR) postgres "$(DB_URL)" down; \
	else \
		echo "no migration files found in $(MIGRATIONS_DIR)"; \
	fi

migrate-status:
	@if find $(MIGRATIONS_DIR) -maxdepth 1 -name '*.sql' | grep -q .; then \
		$(GOOSE) -dir $(MIGRATIONS_DIR) postgres "$(DB_URL)" status; \
	else \
		echo "no migration files found in $(MIGRATIONS_DIR)"; \
	fi

api-run:
	$(GO) run ./cmd/api

bot-run:
	$(GO) run ./cmd/bot
