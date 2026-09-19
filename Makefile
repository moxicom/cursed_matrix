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
        front-shell clean nuke

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
