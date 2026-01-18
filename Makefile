ifneq (,$(wildcard .env))
	include .env
	export
endif

.PHONY: dev dev-tailwind connect-db migrate-up db-clean db-drop-table db-reset

DOCKER_COMPOSE_DEV = docker compose -f docker-compose.dev.yaml
POSTGRES_SERVICE   = postgres

dev:
	@if command -v tmux >/dev/null 2>&1; then \
		echo "Starting dev environment with tmux..."; \
		tmux new-session -d -s authgate \
			'docker compose -f docker-compose.dev.yaml up' \; \
			split-window -h \
			'cd frontend && npm run dev:tailwind' \; \
			attach; \
	else \
		echo ""; \
		echo "tmux not found."; \
		echo ""; \
		echo "Please run the following in two terminals:"; \
		echo "  1) docker compose -f docker-compose.dev.yaml up"; \
		echo "  2) cd frontend && npm run dev:tailwind"; \
		echo ""; \
	fi

dev-tailwind:
	cd frontend && npm run dev:tailwind

connect-db:
	$(DOCKER_COMPOSE_DEV) exec -it $(POSTGRES_SERVICE) \
	psql -U $(POSTGRESQL_USERNAME) -d $(POSTGRESQL_DATABASE)

migrate-up:
	$(DOCKER_COMPOSE_DEV) run --rm backend-migrations


db-clean:
	$(DOCKER_COMPOSE_DEV) exec -T $(POSTGRES_SERVICE) \
	psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) \
	-c "DROP SCHEMA IF EXISTS $(POSTGRES_SCHEMA) CASCADE; CREATE SCHEMA $(POSTGRES_SCHEMA);"

db-drop-table:
ifndef TABLE
	$(error TABLE is required. Usage: make db-drop-table TABLE=table_name)
endif
	$(DOCKER_COMPOSE_DEV) exec -T $(POSTGRES_SERVICE) \
	psql -U $(POSTGRESQL_USERNAME) -d $(POSTGRESQL_DATABASE) \
	-c "DROP TABLE IF EXISTS $(POSTGRESQL_SCHEMA).$(TABLE) CASCADE;"

db-reset:
	$(DOCKER_COMPOSE_DEV) exec -T $(POSTGRES_SERVICE) \
	psql -U $(POSTGRESQL_USERNAME) -d $(POSTGRESQL_DATABASE) \
	-c "DROP SCHEMA IF EXISTS $(POSTGRESQL_SCHEMA) CASCADE; \
	    DROP SCHEMA IF EXISTS public CASCADE; \
	    CREATE SCHEMA public;"
	$(MAKE) migrate-up

