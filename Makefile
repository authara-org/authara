ifneq (,$(wildcard .env))
	include .env
	export
endif

POSTGRESQL_SCHEMA ?= authara
export EMAIL

DOCKER_COMPOSE_FILE = docker-compose.dev.yaml
DOCKER_COMPOSE_DEV  = docker compose -f $(DOCKER_COMPOSE_FILE)
DOCKER_COMPOSE_TEST = docker compose -f $(DOCKER_COMPOSE_FILE)
DOCKER_COMPOSE_ENV  = POSTGRESQL_DATABASE='$(POSTGRESQL_DATABASE)' POSTGRESQL_USERNAME='$(POSTGRESQL_USERNAME)' POSTGRESQL_PASSWORD='$(POSTGRESQL_PASSWORD)' AUTHARA_INTERNAL_API_TOKEN='$(AUTHARA_INTERNAL_API_TOKEN)' AUTHARA_JWT_ISSUER='$(AUTHARA_JWT_ISSUER)' AUTHARA_JWT_KEYS='$(AUTHARA_JWT_KEYS)' AUTHARA_WEBHOOK_SECRET='$(AUTHARA_WEBHOOK_SECRET)'

POSTGRES_SERVICE   = postgres
AUTHARA_SERVICE   = authara
MIGRATIONS_SERVICE = backend-migrations
MAILPIT_SERVICE    = mailpit

TEST_DB_NAME       ?= authara_test
TEST_DB_HOST       ?= postgres
TEST_DB_PORT       ?= 5432
TEST_DB_SCHEMA     ?= authara
TEST_DB_TIMEZONE   ?= UTC
TEST_DB_LOG_SQL    ?= false

.PHONY: dev dev-tailwind mailpit-up mailpit-down connect-db migrate-up db-clean db-truncate-table db-reset admin-by-email operator-by-email \
	test test-up test-db-create test-migrate test-run test-down test-reset \
	test-coverage test-coverage-profile test-coverage-html generate openapi-generate check-generated

generate: openapi-generate

openapi-generate:
	go generate ./internal/http/openapi

check-generated: generate
	git diff --exit-code -- internal/http/openapi/generated.go

dev:
	@if command -v tmux >/dev/null 2>&1; then \
		echo "Starting dev environment with tmux..."; \
		tmux new-session -d -s authara -c '$(CURDIR)' \
			'$(DOCKER_COMPOSE_ENV) $(DOCKER_COMPOSE_DEV) up' \; \
			split-window -h -c '$(CURDIR)/frontend' \
			'npm run dev:tailwind' \; \
		attach; \
	else \
		echo ""; \
		echo "tmux not found."; \
		echo ""; \
		echo "Please run the following in two terminals:"; \
		echo "  1) $(DOCKER_COMPOSE_DEV) up"; \
		echo "  2) cd frontend && npm run dev:tailwind"; \
		echo ""; \
	fi

dev-tailwind:
	cd frontend && npm run dev:tailwind

mailpit-up:
	$(DOCKER_COMPOSE_DEV) up -d $(MAILPIT_SERVICE)
	@echo "Mailpit SMTP: localhost:1025"
	@echo "Mailpit UI:   http://localhost:8025"

mailpit-down:
	$(DOCKER_COMPOSE_DEV) stop $(MAILPIT_SERVICE)

connect-db:
	$(DOCKER_COMPOSE_DEV) exec -it $(POSTGRES_SERVICE) \
	psql -U $(POSTGRESQL_USERNAME) -d $(POSTGRESQL_DATABASE)

migrate-up:
	$(DOCKER_COMPOSE_DEV) build $(MIGRATIONS_SERVICE)
	$(DOCKER_COMPOSE_DEV) run --rm $(MIGRATIONS_SERVICE)

db-clean:
	$(DOCKER_COMPOSE_DEV) exec -T $(POSTGRES_SERVICE) \
	psql -U $(POSTGRESQL_USERNAME) -d $(POSTGRESQL_DATABASE) \
	-Atc "SELECT 'TRUNCATE TABLE $(POSTGRESQL_SCHEMA).' || string_agg(quote_ident(tablename), ', ') || ' RESTART IDENTITY CASCADE;' FROM pg_tables WHERE schemaname = '$(POSTGRESQL_SCHEMA)'" \
	| $(DOCKER_COMPOSE_DEV) exec -T $(POSTGRES_SERVICE) \
	psql -U $(POSTGRESQL_USERNAME) -d $(POSTGRESQL_DATABASE)

db-truncate-table:
ifndef TABLE
	$(error TABLE is required. Usage: make db-truncate-table TABLE=table_name)
endif
	$(DOCKER_COMPOSE_DEV) exec -T $(POSTGRES_SERVICE) \
	psql -U $(POSTGRESQL_USERNAME) -d $(POSTGRESQL_DATABASE) \
	-c "TRUNCATE TABLE $(POSTGRESQL_SCHEMA).$(TABLE) RESTART IDENTITY CASCADE;"

db-reset:
	$(DOCKER_COMPOSE_DEV) exec -T $(POSTGRES_SERVICE) \
	psql -U $(POSTGRESQL_USERNAME) -d $(POSTGRESQL_DATABASE) \
	-c "DROP SCHEMA IF EXISTS $(POSTGRESQL_SCHEMA) CASCADE; \
	    DROP SCHEMA IF EXISTS public CASCADE; \
	    CREATE SCHEMA public;"
	$(MAKE) migrate-up

admin-by-email:
ifndef EMAIL
	$(error EMAIL is required. Usage: make admin-by-email EMAIL=user@example.com)
endif
	$(DOCKER_COMPOSE_DEV) exec -T $(AUTHARA_SERVICE) \
	go run ./cmd/authara admin grant --email "$$EMAIL"

operator-by-email:
ifndef EMAIL
	$(error EMAIL is required. Usage: make operator-by-email EMAIL=user@example.com)
endif
	$(DOCKER_COMPOSE_DEV) exec -T $(AUTHARA_SERVICE) \
	go run ./cmd/authara operator grant --email "$$EMAIL"

# Full local test flow
test: test-up test-db-create test-migrate test-run

test-up:
	$(DOCKER_COMPOSE_TEST) up -d $(POSTGRES_SERVICE) $(MAILPIT_SERVICE)
	until $(DOCKER_COMPOSE_TEST) exec -T $(POSTGRES_SERVICE) \
		pg_isready -U $(POSTGRESQL_USERNAME) -d postgres >/dev/null 2>&1; do \
		echo "waiting for postgres..."; \
		sleep 2; \
	done

test-db-create:
	@if ! $(DOCKER_COMPOSE_TEST) exec -T $(POSTGRES_SERVICE) \
		psql -U $(POSTGRESQL_USERNAME) -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='$(TEST_DB_NAME)'" | grep -q 1; then \
		echo "creating test database $(TEST_DB_NAME)..."; \
		$(DOCKER_COMPOSE_TEST) exec -T $(POSTGRES_SERVICE) \
			psql -U $(POSTGRESQL_USERNAME) -d postgres -v ON_ERROR_STOP=1 \
			-c "CREATE DATABASE $(TEST_DB_NAME)"; \
	else \
		echo "test database $(TEST_DB_NAME) already exists"; \
	fi

test-migrate:
	$(DOCKER_COMPOSE_TEST) run --rm \
		-e POSTGRESQL_HOST=$(TEST_DB_HOST) \
		-e POSTGRESQL_PORT=$(TEST_DB_PORT) \
		-e POSTGRESQL_DATABASE=$(TEST_DB_NAME) \
		-e POSTGRESQL_USERNAME=$(POSTGRESQL_USERNAME) \
		-e POSTGRESQL_PASSWORD=$(POSTGRESQL_PASSWORD) \
		$(MIGRATIONS_SERVICE) \
		up -env=default -config=./dbconfig.yaml

test-run:
	$(DOCKER_COMPOSE_TEST) run --rm \
		-e POSTGRESQL_HOST=$(TEST_DB_HOST) \
		-e POSTGRESQL_PORT=$(TEST_DB_PORT) \
		-e POSTGRESQL_DATABASE=$(TEST_DB_NAME) \
		-e POSTGRESQL_USERNAME=$(POSTGRESQL_USERNAME) \
		-e POSTGRESQL_PASSWORD=$(POSTGRESQL_PASSWORD) \
		-e POSTGRESQL_SCHEMA=$(TEST_DB_SCHEMA) \
		-e POSTGRESQL_TIMEZONE=$(TEST_DB_TIMEZONE) \
		-e POSTGRESQL_LOG_SQL=$(TEST_DB_LOG_SQL) \
		-e AUTHARA_TEST_MAILPIT_HTTP_URL=http://mailpit:8025 \
		-e AUTHARA_TEST_MAILPIT_SMTP_HOST=mailpit \
		-e AUTHARA_TEST_MAILPIT_SMTP_PORT=1025 \
		$(AUTHARA_SERVICE) \
		go test ./... -count=1

test-reset:
	$(DOCKER_COMPOSE_TEST) exec -T $(POSTGRES_SERVICE) \
		psql -U $(POSTGRESQL_USERNAME) -d postgres -v ON_ERROR_STOP=1 \
		-c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '$(TEST_DB_NAME)' AND pid <> pg_backend_pid();" \
		-c "DROP DATABASE IF EXISTS $(TEST_DB_NAME);"

test-down:
	$(DOCKER_COMPOSE_TEST) down

test-coverage: test-up test-db-create test-migrate
	$(DOCKER_COMPOSE_TEST) run --rm \
		-e POSTGRESQL_HOST=$(TEST_DB_HOST) \
		-e POSTGRESQL_PORT=$(TEST_DB_PORT) \
		-e POSTGRESQL_DATABASE=$(TEST_DB_NAME) \
		-e POSTGRESQL_USERNAME=$(POSTGRESQL_USERNAME) \
		-e POSTGRESQL_PASSWORD=$(POSTGRESQL_PASSWORD) \
		-e POSTGRESQL_SCHEMA=$(TEST_DB_SCHEMA) \
		-e POSTGRESQL_TIMEZONE=$(TEST_DB_TIMEZONE) \
		-e POSTGRESQL_LOG_SQL=$(TEST_DB_LOG_SQL) \
		$(AUTHARA_SERVICE) \
		go test ./... -count=1 -cover

test-coverage-profile: test-up test-db-create test-migrate
	$(DOCKER_COMPOSE_TEST) run --rm \
		-v $(PWD):/app \
		-e POSTGRESQL_HOST=$(TEST_DB_HOST) \
		-e POSTGRESQL_PORT=$(TEST_DB_PORT) \
		-e POSTGRESQL_DATABASE=$(TEST_DB_NAME) \
		-e POSTGRESQL_USERNAME=$(POSTGRESQL_USERNAME) \
		-e POSTGRESQL_PASSWORD=$(POSTGRESQL_PASSWORD) \
		-e POSTGRESQL_SCHEMA=$(TEST_DB_SCHEMA) \
		-e POSTGRESQL_TIMEZONE=$(TEST_DB_TIMEZONE) \
		-e POSTGRESQL_LOG_SQL=$(TEST_DB_LOG_SQL) \
		$(AUTHARA_SERVICE) \
		sh -c 'go test ./... -count=1 -coverprofile=coverage.out && go tool cover -func=coverage.out'

test-coverage-html: test-coverage-profile
	go tool cover -html=coverage.out
