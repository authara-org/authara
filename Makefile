ifneq (,$(wildcard .env))
	include .env
	export
endif

POSTGRESQL_SCHEMA ?= authgate

.PHONY: dev dev-tailwind connect-db migrate-up db-clean db-drop-table db-reset admin-by-email

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
	psql -U $(POSTGRESQL_USERNAME) -d $(POSTGRESQL_DATABASE) \
	-c "\
	DO $$ \
	DECLARE r RECORD; \
	BEGIN \
	  FOR r IN ( \
	    SELECT tablename \
	    FROM pg_tables \
	    WHERE schemaname = '$(POSTGRESQL_SCHEMA)' \
	  ) LOOP \
	    EXECUTE 'TRUNCATE TABLE $(POSTGRESQL_SCHEMA).' || quote_ident(r.tablename) || ' RESTART IDENTITY CASCADE'; \
	  END LOOP; \
	END $$; \
	"

db-truncate-table:
ifndef TABLE
	$(error TABLE is required. Usage: make db-drop-table TABLE=table_name)
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
		SELECT id FROM $(POSTGRESQL_SCHEMA).roles WHERE name = 'admin' \
	) \
	INSERT INTO $(POSTGRESQL_SCHEMA).user_roles (user_id, role_id) \
	SELECT u.id, r.id FROM u, r \
	ON CONFLICT DO NOTHING; \
	"
