# cursed_matrix — task runner. `make` lists the targets.

COMPOSE     := docker compose
DEV_COMPOSE := docker compose -f docker-compose.yml -f docker-compose.dev.yml

# make does not read .env on its own; the leading - keeps `make init` working before it exists.
-include .env
export

POSTGRES_USER ?= cursed
POSTGRES_DB   ?= cursed_matrix

.DEFAULT_GOAL := help
.PHONY: help init up migrate dev down stop restart build rebuild pull deploy ps logs \
        logs-front health psql redis-cli pg-dump metrics grafana dev-reset \
        front-shell clean nuke back-build back-test back-test-integration \
        secrets back-lint back-tidy back-migrate-up back-migrate-down back-migrate-status \
        back-migrate-create back-shell back-image-debug back-fmt back-generate

help: ## Show this help
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

init: ## Create .env from the template (does not overwrite an existing one)
	@test -f .env && echo ".env already exists — leaving it alone" || { \
	  cp .env.example .env; \
	  echo ".env created. Fill in the secrets: make secrets"; }

secrets: ## Generate any secret in .env that is still empty
	@test -f .env || { echo "no .env — run 'make init' first" >&2; exit 1; }
	@for name in POSTGRES_PASSWORD REDIS_PASSWORD JWT_SECRET GRAFANA_ADMIN_PASSWORD; do \
	  if grep -qE "^$$name=.+" .env; then \
	    echo "  $$name already set"; \
	  else \
	    value=$$(openssl rand -hex 32); \
	    sed -i.bak "s|^$$name=.*|$$name=$$value|" .env && rm -f .env.bak; \
	    echo "  $$name generated"; \
	  fi; \
	done
	@# env -u: the values this Makefile exported at startup were empty and would win over .env.
	@env -u POSTGRES_PASSWORD -u REDIS_PASSWORD -u JWT_SECRET -u GRAFANA_ADMIN_PASSWORD \
	  $(COMPOSE) config --quiet && echo "  .env is complete"

up: ## Build and start the whole stack
	$(COMPOSE) up -d --build

migrate: ## Apply pending migrations (needs no Go on the host)
	$(COMPOSE) run --rm migrate up

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

pull: ## Pull the application images from IMAGE_REPO
	$(COMPOSE) pull back front

deploy: ## Update a server that does not build: pull, migrate, restart
	$(COMPOSE) pull back front
	$(COMPOSE) run --rm migrate up
	$(COMPOSE) up -d --no-build

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

# --- backend: runs from the host against the containerised database ------------

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

back-generate: ## Regenerate the server interface from back/api/v1/openapi.yaml
	@cd back && $$(go env GOPATH)/bin/oapi-codegen -config oapi-codegen.yaml api/v1/openapi.yaml
	@echo "regenerated from back/api/v1/openapi.yaml"

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

back-shell: ## Shell in a debug build of the backend image (the runtime image is scratch)
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
