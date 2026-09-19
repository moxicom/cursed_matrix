# cursed_matrix — task runner for the compose stack.
# `make` on its own lists the targets.

COMPOSE     := docker compose
DEV_COMPOSE := docker compose -f docker-compose.yml -f docker-compose.dev.yml

# Load .env and hand every value to the recipe shells. Compose reads .env on
# its own, but make does not, so without this the probes below fall back to the
# built-in defaults and report a healthy stack as down whenever a port was
# changed. The leading - keeps `make init` working before .env exists.
-include .env
export

POSTGRES_USER ?= cursed
POSTGRES_DB   ?= cursed_matrix

.DEFAULT_GOAL := help
.PHONY: help init up up-backend dev down stop restart build rebuild ps logs \
        logs-front health psql redis-cli pg-dump metrics grafana dev-reset \
        front-shell clean nuke back-build back-test back-test-integration \
        back-lint back-tidy back-migrate-up back-migrate-down back-migrate-status \
        back-migrate-create back-shell back-image-debug back-fmt

help: ## Show this help
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

init: ## Create .env from the template (does not overwrite an existing one)
	@test -f .env && echo ".env already exists — leaving it alone" || { \
	  cp .env.example .env; \
	  echo ".env created. Fill in the four empty secrets:"; \
	  echo "  openssl rand -hex 32"; \
	  echo "(hex, not base64: these go into postgres:// and redis:// URLs,"; \
	  echo " where a / or + from base64 breaks the URL parse)"; }

up: ## Start frontend, datastores and monitoring
	$(COMPOSE) up -d --build

up-backend: ## Start everything including back/ (requires back/Dockerfile)
	$(COMPOSE) --profile backend up -d --build

dev: ## Start with the Vite dev server (HMR) instead of nginx
	$(DEV_COMPOSE) up --build

dev-reset: ## Same as dev, but discard the container's node_modules volume first
	$(DEV_COMPOSE) up --build --renew-anon-volumes

down: ## Stop and remove containers, keep the data volumes
	$(COMPOSE) down --remove-orphans

stop: ## Stop containers without removing them
	$(COMPOSE) stop

restart: ## Restart every service
	$(COMPOSE) restart

build: ## Build images without starting anything
	$(COMPOSE) build

rebuild: ## Rebuild ignoring the layer cache
	$(COMPOSE) build --no-cache

ps: ## Show service status and health
	$(COMPOSE) ps

logs: ## Tail logs from every service
	$(COMPOSE) logs -f --tail=100

logs-front: ## Tail the frontend log only
	$(COMPOSE) logs -f --tail=100 front

health: ## Probe every endpoint that should answer
	@printf 'front      '; curl -fsS -o /dev/null -w '%{http_code}\n' http://localhost:$${FRONT_PORT:-8080}/healthz  || echo down
	@printf 'metrics    '; curl -fsS -o /dev/null -w '%{http_code}\n' http://localhost:$${VM_PORT:-8428}/health || echo down
	@printf 'grafana    '; curl -fsS -o /dev/null -w '%{http_code}\n' http://localhost:$${GRAFANA_PORT:-3000}/api/health || echo down
	@printf 'postgres   '; $(COMPOSE) exec -T postgres pg_isready -U $(POSTGRES_USER) -d $(POSTGRES_DB) || true
	@printf 'redis      '; $(COMPOSE) exec -T redis sh -c 'redis-cli -a "$$REDIS_PASSWORD" ping' 2>/dev/null || echo down

psql: ## Open a psql shell on the application database
	$(COMPOSE) exec postgres psql -U $(POSTGRES_USER) -d $(POSTGRES_DB)

redis-cli: ## Open a redis-cli shell (authenticated)
	$(COMPOSE) exec redis sh -c 'redis-cli -a "$$REDIS_PASSWORD"'

pg-dump: ## Dump the database to ./backups/<date>.sql
	@mkdir -p backups
	@out=backups/$$(date +%Y%m%d-%H%M%S).sql; \
	  if $(COMPOSE) exec -T postgres pg_dump -U $(POSTGRES_USER) -d $(POSTGRES_DB) > "$$out.part"; then \
	    mv "$$out.part" "$$out"; echo "written: $$out"; \
	  else \
	    rm -f "$$out.part"; \
	    echo "pg_dump failed — no file written" >&2; exit 1; \
	  fi

metrics: ## Open the VictoriaMetrics UI in a browser
	@open http://localhost:$${VM_PORT:-8428}/vmui 2>/dev/null || \
	 xdg-open http://localhost:$${VM_PORT:-8428}/vmui

grafana: ## Open Grafana in a browser
	@open http://localhost:$${GRAFANA_PORT:-3000} 2>/dev/null || \
	 xdg-open http://localhost:$${GRAFANA_PORT:-3000}

front-shell: ## Shell inside the running frontend container
	$(COMPOSE) exec front sh

# ---------------------------------------------------------------- backend
#
# The backend's own toolchain. Migrations and tests run from the host against
# the containerised database, which is why they build the DSN from .env rather
# than reading the one compose hands to the container (that one says "postgres",
# a hostname that only resolves inside the compose network).

BACK_DSN := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@127.0.0.1:$(or $(POSTGRES_PORT),5432)/$(POSTGRES_DB)?sslmode=disable
TEST_DB  := $(POSTGRES_DB)_test
BACK_TEST_REDIS := redis://:$(REDIS_PASSWORD)@127.0.0.1:$(or $(REDIS_PORT),6379)/9
BACK_TEST_DSN := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@127.0.0.1:$(or $(POSTGRES_PORT),5432)/$(TEST_DB)?sslmode=disable

back-build: ## Build the backend binaries
	cd back && go build ./...

back-test: ## Run the backend unit tests (no database needed)
	cd back && go test ./...

back-test-integration: ## Run the backend tests that need PostgreSQL (creates a test database)
	@$(COMPOSE) exec -T postgres psql -U $(POSTGRES_USER) -d postgres -tAc \
	  "SELECT 1 FROM pg_database WHERE datname = '$(TEST_DB)'" | grep -q 1 || \
	  $(COMPOSE) exec -T postgres createdb -U $(POSTGRES_USER) $(TEST_DB)
	@$(COMPOSE) exec -T postgres psql -U $(POSTGRES_USER) -d $(TEST_DB) -q \
	  -f /docker-entrypoint-initdb.d/01-extensions.sql
	@cd back && DATABASE_URL="$(BACK_TEST_DSN)" go run ./cmd/migrate up
	@cd back && TEST_DATABASE_URL="$(BACK_TEST_DSN)" TEST_REDIS_URL="$(BACK_TEST_REDIS)" \
	  go test -tags integration -count=1 ./...

back-fmt: ## Apply go fix and gofumpt to the backend
	cd back && go fix ./...
	cd back && golangci-lint fmt

back-lint: ## Lint the backend, formatting included (needs golangci-lint)
	cd back && golangci-lint run

back-tidy: ## Tidy the backend module
	cd back && go mod tidy

back-migrate-up: ## Apply pending migrations to the application database
	@cd back && DATABASE_URL="$(BACK_DSN)" go run ./cmd/migrate up

back-migrate-down: ## Roll back the last migration
	@cd back && DATABASE_URL="$(BACK_DSN)" go run ./cmd/migrate down

back-migrate-status: ## Show which migrations are applied
	@cd back && DATABASE_URL="$(BACK_DSN)" go run ./cmd/migrate status

back-migrate-create: ## Create a migration: make back-migrate-create NAME=add_something
	@test -n "$(NAME)" || { echo "NAME is required: make back-migrate-create NAME=add_something" >&2; exit 1; }
	@ts=$$(date +%Y%m%d%H%M%S); \
	  f=back/internal/migrations/$${ts}_$(NAME).sql; \
	  printf -- '-- +goose Up\n\n-- +goose Down\n' > $$f; \
	  echo "created: $$f"

back-shell: ## Shell in a debug build of the backend image (the runtime image is scratch and has none)
	@docker build --target debug -t cursed-matrix/back:debug ./back >/dev/null
	docker run --rm -it --entrypoint sh cursed-matrix/back:debug

clean: ## Remove containers and the images built here, keep data volumes
	$(COMPOSE) down --remove-orphans --rmi local

nuke: ## Remove containers AND all data volumes — destroys the database
	@printf 'This deletes the Postgres, Redis, VictoriaMetrics and Grafana volumes. Type yes: '; \
	  read ans; \
	  if [ "$$ans" = yes ]; then \
	    $(COMPOSE) down -v --remove-orphans; \
	  else \
	    echo "cancelled"; \
	  fi
