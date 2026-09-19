# Infrastructure

Everything needed to run cursed_matrix locally or on a single host. This answers
the part of `SPEC.md` Open Question 29 that concerns docker-compose and local
development; the CI pipeline and the production deployment shape are still open.

## Contents

```
docker-compose.yml         frontend, datastores, monitoring; backend behind a profile
docker-compose.dev.yml     overlay: Vite dev server with HMR instead of nginx
.env.example               every variable, with the four secrets left empty
Makefile                   task runner — `make` lists the targets
front/Dockerfile           multi-stage: node build → unprivileged nginx
front/Dockerfile.dev       dev image, dependencies only (source is bind-mounted)
front/nginx/               site config: SPA fallback, /api proxy, cache, CSP
infra/postgres/init/       extensions installed once, when the volume is empty
infra/victoria-metrics/    scrape config (Prometheus format)
infra/grafana/             datasource and dashboard provisioning
```

## Start

```sh
make init      # .env from the template
               # fill in the four empty secrets: openssl rand -hex 32
make up        # build and start
make health    # check that everything answers
```

| Service | URL | Notes |
|---|---|---|
| Frontend | http://localhost:8080 | nginx serving the built bundle |
| Grafana | http://localhost:3000 | admin / `GRAFANA_ADMIN_PASSWORD` |
| VictoriaMetrics | http://localhost:8428 | query UI at `/vmui`, scrape targets at `/targets` |
| Postgres | `127.0.0.1:5432` | `make psql` |
| Redis | `127.0.0.1:6379` | `make redis-cli` |

Everything except the frontend port is bound to `127.0.0.1`. On a host that also
serves the application, the database and the monitoring stack must not be
reachable from the network — and Grafana, holding a datasource that describes
the whole system, is as sensitive as the database it watches. VictoriaMetrics in
particular has no authentication of its own: anything that reaches port 8428 can
read every metric and, through `/api/v1/write`, insert fabricated ones.

## Development with HMR

```sh
make dev       # Vite on http://localhost:5173, source bind-mounted
```

The frontend still runs on mocks (`front/src/shared/mocks`), so no backend is
needed. When `back/` exists, add a proxy to `front/vite.config.ts`:

```ts
server: { port: 5173, proxy: { '/api': 'http://back:8080' } }
```

In the production image the proxy is nginx's job and needs no build-time
configuration — `BACKEND_HOST` and `BACKEND_PORT` are substituted into the
template at container start.

## Adding the backend

`back/` does not exist yet, so its service sits behind a compose profile and the
default `up` skips it. Everything it needs is already wired: `DATABASE_URL`,
`REDIS_URL`, `SESSION_SECRET`, health check, dependency ordering on healthy
datastores, and a scrape job in `infra/victoria-metrics/scrape.yml`.

Once `back/Dockerfile` exists, expose `/healthz` and `/metrics` and run:

```sh
make up-backend
```

nginx resolves the backend per request rather than at startup — deliberately, so
the frontend container comes up whether or not the backend is running, and
`/api` answers `502` instead of nginx refusing to boot.

## Monitoring

**VictoriaMetrics single-node** does both jobs Prometheus and its TSDB did: it
scrapes the targets in `infra/victoria-metrics/scrape.yml` and stores the result,
in one process. The config is Prometheus' own format, and the query API is
Prometheus-compatible, so Grafana uses the stock Prometheus datasource and every
PromQL query works unchanged.

Why it is the better fit at this size:

* **Memory.** Prometheus holds a two-hour head block in RAM regardless of how
  little is being scraped; VM's floor is far lower, and `-memory.allowedPercent`
  caps it explicitly. On a host that also runs Postgres and the application,
  that is the resource that actually runs out.
* **Disk.** VM's compression is substantially better than Prometheus' TSDB on
  the same series, so a longer retention costs less.
* **One process.** No separate TSDB configuration, no `--web.enable-lifecycle`
  dance to reload, no second component to learn before the first metric is
  useful. `-promscrape.configCheckInterval=30s` picks up scrape config edits on
  its own.
* **Query UI included.** `/vmui` at port 8428 is enough to explore metrics
  without opening Grafana (`make metrics`).

What is given up, stated plainly:

* **Rule evaluation.** A single-node VM does not evaluate recording or alerting
  rules. That is `vmalert`, a separate container, which also needs an
  Alertmanager to send anywhere. This is why there is no `rules/` directory
  here: an empty folder would imply rules work, and they do not yet.
* **Ecosystem defaults.** Some published dashboards and operators assume a
  Prometheus HTTP API at `/api/v1/…` on port 9090 and the Prometheus
  configuration surface. VM serves the same API, but a copied setup may need its
  URL adjusted.

Scraped today: VictoriaMetrics itself, the Postgres exporter, the Redis
exporter, and — when it exists — the backend. Grafana is provisioned with the
datasource and one dashboard, `cursed_matrix — infrastructure`, covering target
health, Postgres connections, transaction and deadlock rates, cache hit ratio,
database size, Redis memory against its limit, throughput, evictions, and
placeholders for the backend's HTTP and Go-runtime metrics.

Dashboards are provisioned from a read-only mount, so edits made in the Grafana
UI are not saved. Change `infra/grafana/dashboards/infrastructure.json` and it
reloads within 30 seconds.

What is deliberately not here yet:

* **Log aggregation.** Container logs are capped at 3 × 10 MB per service by the
  compose logging defaults, and `docker compose logs` is enough at this size.
  VictoriaLogs is the consistent next step now that the metrics side is VM.
* **Alerting.** Needs `vmalert` plus somewhere for alerts to go. Worth adding
  once there is someone to receive them — before that they are a folder of
  unread conditions.
* **Tracing.** Useful only once the backend has enough layers to be worth
  tracing through.

## Data and backups

Named volumes hold Postgres, Redis, VictoriaMetrics and Grafana state. `make down`
keeps them; `make nuke` destroys them and asks first.

`make pg-dump` writes a timestamped SQL dump to the repository root. It is a
manual command, not a backup strategy: XP transactions and activity events are
append-only history that `SPEC` §32 requires to stay immutable, so a real
deployment needs scheduled dumps kept off this host.

## Notes on the choices

* **Postgres 16** with `pg_stat_statements` preloaded, checksums enabled, and
  `log_min_duration_statement=200`. The statistics extension cannot be switched
  on after the fact — it needs `shared_preload_libraries` at startup — so it is
  set from the first run rather than added during the first slow query.
* **Extensions** in `infra/postgres/init/`: `pgcrypto`, `citext` for
  case-insensitive username and email, `pg_trgm` for the substring search of
  `SPEC` §9, and `pg_stat_statements`. Schema migrations belong to `back/`; this
  file only installs what migrations depend on.
* **Redis** with `appendonly yes` and `maxmemory-policy noeviction`. Sessions
  and rate-limit counters live here; under memory pressure a loud write error is
  better than users being logged out at random, which is what an LRU policy
  would do.
* **Three networks** (`edge`, `data`, `observability`) so the segments cannot
  reach each other by accident. The frontend never sees Postgres; the backend is
  the only service on more than one.
* **Unprivileged nginx** (uid 101, port 8080). No root in the container and no
  capability needed to bind the port.
* **VictoriaMetrics over Prometheus** for the reasons above; the cost is that
  alerting moves to a separate `vmalert` container when it is needed.
* **Image tags are pinned** to a minor version, not `latest`. `latest` means the
  stack is not reproducible and an unrelated `docker compose pull` can change
  the database version underneath the data.
