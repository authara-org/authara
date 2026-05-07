ifneq (,$(wildcard .env))
	include .env
	export
endif

POSTGRESQL_SCHEMA ?= authara

DOCKER_COMPOSE_FILE = docker-compose.dev.yaml
DOCKER_COMPOSE_DEV  = docker compose -f $(DOCKER_COMPOSE_FILE)
DOCKER_COMPOSE_TEST = docker compose -f $(DOCKER_COMPOSE_FILE)

POSTGRES_SERVICE   = postgres
AUTHARA_SERVICE   = authara
MIGRATIONS_SERVICE = backend-migrations
MAILHOG_CONTAINER = mailhog
MAILHOG_IMAGE     = mailhog/mailhog

TEST_DB_NAME       ?= authara_test
TEST_DB_HOST       ?= postgres
TEST_DB_PORT       ?= 5432
TEST_DB_SCHEMA     ?= authara
TEST_DB_TIMEZONE   ?= UTC
TEST_DB_LOG_SQL    ?= false

.PHONY: dev dev-tailwind mailhog-up mailhog-down connect-db migrate-up db-clean db-truncate-table db-reset admin-by-email \
	test test-up test-db-create test-migrate test-run test-down test-reset \
	test-coverage test-coverage-profile test-coverage-html

dev:
	@if command -v tmux >/dev/null 2>&1; then \
		echo "Starting dev environment with tmux..."; \
		tmux new-session -d -s authara \
			'$(DOCKER_COMPOSE_DEV) up' \; \
			split-window -h \
			'cd frontend && npm run dev:tailwind' \; \
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

mailhog-up:
	@if docker ps -a --format '{{.Names}}' | grep -qx '$(MAILHOG_CONTAINER)'; then \
		docker start $(MAILHOG_CONTAINER); \
	else \
		docker run -d \
			--name $(MAILHOG_CONTAINER) \
			-p 1025:1025 \
			-p 8025:8025 \
			$(MAILHOG_IMAGE); \
	fi
	@echo "MailHog SMTP: localhost:1025"
	@echo "MailHog UI:   http://localhost:8025"

mailhog-down:
	-docker stop $(MAILHOG_CONTAINER)

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
	$(DOCKER_COMPOSE_DEV) exec -T $(POSTGRES_SERVICE) \
	psql -U $(POSTGRESQL_USERNAME) -d $(POSTGRESQL_DATABASE) \
	-c "\
	WITH u AS ( \
		SELECT id FROM $(POSTGRESQL_SCHEMA).users WHERE email = '$(EMAIL)' \
	), r AS ( \
		SELECT id FROM $(POSTGRESQL_SCHEMA).platform_roles WHERE name = 'admin' \
	) \
	INSERT INTO $(POSTGRESQL_SCHEMA).user_platform_roles (user_id, role_id) \
	SELECT u.id, r.id FROM u, r \
	ON CONFLICT DO NOTHING; \
	"

# Full local test flow
test: test-up test-db-create test-migrate test-run

test-up:
	$(DOCKER_COMPOSE_TEST) up -d $(POSTGRES_SERVICE)
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
