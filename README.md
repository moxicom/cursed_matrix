# cursed_matrix

Eisenhower-matrix task manager with a force-directed graph of your tasks, XP,
levels, streaks, achievements and a leaderboard. React frontend, Go backend,
Postgres, Redis. One repository, one `docker compose up`.


```
ATTENTION!! This repository is my first attempt to use real, raw vibecode to implement a system that's useful to me.
```


Full spec: [`docs/SPEC.md`](docs/SPEC.md). HTTP contract:
[`docs/API.md`](docs/API.md). Deployment in depth: [`docs/DEPLOY.md`](docs/DEPLOY.md).

```
front/    React + TypeScript + Vite, served by nginx in production
back/     Go 1.27 API, goose migrations, scratch image
infra/    Postgres init, VictoriaMetrics scrape config, Grafana provisioning
docs/     SPEC, API, DEPLOY, STATUS
```

## Requirements

Docker Engine 24+ with the Compose v2 and buildx plugins. Nothing else: the
images build their own Go and Node toolchains. `make` lists every target.

## Run locally

```sh
make init       # .env from .env.example
make secrets    # fill every empty secret with openssl rand -hex 32
make up         # build and start everything
make migrate    # apply migrations
make health     # every service should answer
```

Site at http://localhost:8080. Grafana at http://localhost:3000 (admin /
`GRAFANA_ADMIN_PASSWORD`), VictoriaMetrics at http://localhost:8428/vmui.

Frontend with hot reload instead of nginx:

```sh
make dev        # Vite on http://localhost:5173, source bind-mounted
```

Backend from the host against the containerised database:

```sh
make back-test              # unit tests, no database
make back-test-integration  # creates <db>_test, runs the tagged tests
make back-migrate-up        # migrations via go run
make back-fmt               # go fix + gofumpt, run before committing
```

`make down` keeps the data volumes. `make nuke` deletes them and asks first.

## Run on a small VPS

Tested target: 1 CPU, 1 GB RAM, 20 GB disk, KVM. The running stack takes
about 130 MB. The build does not fit next to it (the Go compiler alone wants
512-768 MB), so the images are built on your own machine and shipped to the
server over SSH. The server never compiles anything.

### 1. Swap

Docker on 1 GB without swap means the kernel OOM killer picks the largest
process, which is Postgres. A KVM VPS allows a swap file; only OpenVZ/LXC do
not (`systemd-detect-virt` tells you which).

```sh
sudo fallocate -l 2G /swapfile && sudo chmod 600 /swapfile
sudo mkswap /swapfile && sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
echo 'vm.swappiness=10' | sudo tee /etc/sysctl.d/99-swap.conf && sudo sysctl -p /etc/sysctl.d/99-swap.conf
```

### 2. Configure the server

The server needs the compose files and the config, not the source. Clone the
repository anyway: it is the simplest way to keep them in sync.

```sh
git clone <remote> cursed_matrix && cd cursed_matrix
make init && make secrets
```

Then in `.env`:

```ini
IMAGE_REPO=cursed-matrix   # the default: names the images you will load, nothing is pulled
IMAGE_TAG=prod
APP_ENV=production         # anything else issues cookies without Secure
FRONT_PORT=127.0.0.1:8080  # only the TLS terminator reaches the container
COMPOSE_PROFILES=          # no monitoring: it costs 3x the application

REDIS_MAXMEMORY=64mb
POSTGRES_MEM_LIMIT=256m    # hard caps per container, 0 = none
BACK_MEM_LIMIT=256m
REDIS_MEM_LIMIT=96m
FRONT_MEM_LIMIT=32m
BACK_GOMEMLIMIT=192MiB     # Go GC tightens here, before the cap kills it
```

### 3. Build locally, ship over SSH

On your machine, in the repository:

```sh
# Build for the server's architecture. Almost every VPS is amd64; on an
# Apple Silicon or other arm64 laptop this line is required, otherwise you
# ship arm64 images the server cannot run. Go cross-compiles natively; the
# Node stage runs under emulation and takes a few minutes.
export DOCKER_DEFAULT_PLATFORM=linux/amd64

IMAGE_TAG=prod docker compose build back front

docker save cursed-matrix/back:prod cursed-matrix/front:prod \
  | gzip | ssh user@server 'gunzip | docker load'
```

`docker load` on the server needs no memory to speak of; it unpacks layers to
disk.

On the server, with `IMAGE_TAG=prod` in `.env`:

```sh
cd cursed_matrix
docker compose up -d postgres redis
docker compose run --rm migrate up
docker compose up -d --no-build
```

`--no-build` matters: if an image is missing, a plain `up` would try to build
it there, which is the thing being avoided.

If `migrate` fails with `password authentication failed for user "cursed"`,
the Postgres volume was initialised with a different password than `.env` now
holds. The password is applied once, when the volume is empty. On a fresh
server, `docker compose down -v` and start again; with data in place, change
it inside the container, where local connections need no password:

```sh
docker compose exec -T postgres psql -U cursed -d postgres \
  -c "ALTER ROLE cursed PASSWORD '$(grep -E '^POSTGRES_PASSWORD=' .env | cut -d= -f2-)'"
```

### 4. TLS

The stack speaks plain HTTP on 127.0.0.1:8080. Put Caddy in front:

```
cursed.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

The proxy must send `X-Forwarded-Proto` (enables HSTS) and `X-Real-IP`
(rate limits are per address; without it every visitor shares one counter).
Caddy does both by default; an nginx example is in `docs/DEPLOY.md`.

#### Without TLS

Fine for a first smoke test, not for users. In `.env`:

```ini
FRONT_PORT=8080            # every interface, not just the loopback
APP_ENV=development
```

and open `http://<server-ip>:8080`. HSTS stays off on its own, nginx sends it
only when a terminator forwards `X-Forwarded-Proto: https`. Postgres, Redis and
the monitoring remain on `127.0.0.1` either way.

**Symptom of getting this wrong:** the login succeeds, and the first change
you make answers `CSRF_TOKEN_INVALID` / "The session has expired. Sign in
again." With `APP_ENV=production` the three session cookies carry the `Secure`
flag, and a browser refuses to store a `Secure` cookie that arrived over plain
HTTP (except from `localhost`). The login response is a 200, but no cookie is
kept; the next unsafe request finds no `cm_csrf` to echo in `X-CSRF-Token`,
and the API refuses it. Check DevTools → Application → Cookies: if `cm_csrf` is
missing, this is it. Fix on the server:

```sh
sed -i 's/^APP_ENV=.*/APP_ENV=development/' .env
docker compose up -d back      # env changes apply only when the container is recreated
```

then clear the site's cookies and sign in again.

If `cm_csrf` is present and the error persists, check the server clock with
`timedatectl`. The access and CSRF cookies expire 15 minutes after issue, by an
absolute `Expires` stamped with the server's clock; a server more than 15
minutes behind hands out cookies the browser considers already expired.

### 5. Verify

```sh
docker compose ps                              # every service healthy
curl -sf https://cursed.example.com/healthz    # ok
free -m                                        # swap used should stay near 0
```

Register an account and complete one task: that exercises the database,
Redis, the cookie scheme and the XP ledger at once.

### Metrics over SSH

The monitoring profile costs 420 MB, three times the application, so on this
host it stays off. The API still serves Prometheus metrics on `/metrics`, and
the `back` service publishes its port on the server's loopback only
(`BACK_PORT`, 9090 by default). Nothing on the network can reach it; an SSH
tunnel can.

From your machine:

```sh
ssh -N -L 9090:127.0.0.1:9090 user@server &
curl -s localhost:9090/metrics | grep -E '^(http_|go_goroutines|process_resident)'
```

That is the raw text: request rate and latency per endpoint, Go heap and
goroutines, process memory. Enough for a look; for graphs, run the monitoring
where the memory is, on your machine, and let it scrape through the tunnel.
With Docker locally:

```sh
cat > scrape.yml <<'YAML'
scrape_configs:
  - job_name: cursed-vps
    scrape_interval: 15s
    static_configs:
      - targets: ["host.docker.internal:9090"]
YAML

docker run -d --name vm -p 8428:8428 \
  -v "$PWD/scrape.yml:/etc/scrape.yml:ro" -v vm_data:/victoria-metrics-data \
  victoriametrics/victoria-metrics:v1.152.0 \
  -promscrape.config=/etc/scrape.yml -retentionPeriod=30d
```

Open `http://localhost:8428/vmui` and query, or point a local Grafana at it as
a Prometheus datasource. The dashboards under `infra/grafana/dashboards/` work
unchanged for the backend panels; the Postgres and Redis panels stay empty,
their exporters are part of the profile that is off.

Keep the tunnel alive across drops with `autossh -M 0 -N -L 9090:127.0.0.1:9090
user@server`, or as a systemd user service. The tunnel is the only path in:
never publish `BACK_PORT` on `0.0.0.0` and never proxy `/metrics` through
nginx. It lists every endpoint with its latency and the runtime's state, and
the API behind the same port skips nginx's rate limiting.

### Updating

Same three steps. On your machine:

```sh
IMAGE_TAG=prod docker compose build back front
docker save cursed-matrix/back:prod cursed-matrix/front:prod \
  | gzip | ssh user@server 'gunzip | docker load'
```

On the server:

```sh
git pull                                  # compose files and config, not code
docker compose run --rm migrate up
docker compose up -d --no-build
docker image prune -f                     # the previous layers, otherwise they pile up
```

Migrations run as a separate step on purpose: the API does not migrate at
startup, so a schema change fails loudly before new code serves against the
old schema.

Pinning: a fixed tag such as `prod` is replaced on every load. To keep the
previous image around for a rollback, tag by commit instead
(`IMAGE_TAG=$(git rev-parse --short HEAD)` on both sides) and switch `.env`
back to the old tag if the new one misbehaves.

### Alternative: a registry

`.github/workflows/images.yml` builds and pushes `linux/amd64` images to
`ghcr.io/moxicom/cursed_matrix/{back,front}` on every push to `main`. A server
that can reach it sets `IMAGE_REPO=ghcr.io/moxicom/cursed_matrix` and
`IMAGE_TAG=latest` in `.env` and updates with `make deploy` (pull, migrate,
up). Nothing else changes.

## Configuration

Two layers. Secrets and deployment shape live in `.env`; behaviour lives in
`back/config/config.yaml`, which is mounted read-only into the API container
and read at startup.

`.env`, the parts that matter:

| Variable | Meaning |
|---|---|
| `IMAGE_REPO`, `IMAGE_TAG` | with the default repo they name what `build` produces; set to the registry they name what `pull` fetches |
| `APP_ENV` | only exactly `production` sets the `Secure` flag on session cookies |
| `COMPOSE_PROFILES` | `monitoring` adds VictoriaMetrics, Grafana and two exporters (~420 MB); empty runs the four core services |
| `GO_BUILD_LOWMEM` | `1` compiles the backend within ~512 MB instead of ~768, slower |
| `*_MEM_LIMIT` | hard cgroup cap per container; `0` (default) means none |
| `BACK_GOMEMLIMIT` | Go soft heap limit for the API; empty means off; keep it under `BACK_MEM_LIMIT` |
| `REDIS_MAXMEMORY` | Redis runs `noeviction`: at the limit writes fail loudly instead of sessions vanishing |
| `JWT_SECRET` | rotating it signs everyone out immediately |
| `POSTGRES_PASSWORD` | applied only when the data volume is created; change a live one with `ALTER ROLE` |

`back/config/config.yaml`, the parts that matter:

| Key | Meaning |
|---|---|
| `database.max_conns` | 16, deliberately not sized from the CPU count |
| `auth.access_ttl`, `auth.refresh_ttl` | 15 m and 30 d; auth is HttpOnly cookies plus a CSRF header, no tokens in response bodies |
| `auth.rate_limit.*` | per-address and per-account windows for login, register, reads and writes |
| `auth.password_hash.*` | argon2id cost: 32 MiB × 2 passes, 2 hashes at a time (peak 64 MiB); `max_memory_mib` is the ceiling accepted from stored hashes and must never drop below what live passwords were issued with |
| `billing.enabled` | `false`: buying a plan grants it outright for `granted_period`, nobody is charged |
| `billing.trial_period` | 14 days for new accounts |
| `billing.prices` | per locale, not converted: 2.39 USD for EN, 199 RUB for RU |

Every secret is named indirectly: a `*_key` field holds the environment
variable that carries the value, so an operator repoints a credential without
touching the image.

Things that are enforced by the API, not the page: free-plan quotas (5 active
tasks, 5 links), input limits (title 100, description 2000, tag 24), XP and
level rules. The frontend only renders.

## Memory footprint

Measured, not estimated. See `docs/DEPLOY.md` for the method.

| | |
|---|---|
| Postgres, Redis, API, nginx | 130 MB |
| Monitoring profile | 420 MB |
| Frontend build | 256 MB |
| Backend build | 768 MB, or 512 MB with `GO_BUILD_LOWMEM=1` |

The one runtime spike is login: argon2id, 32 MiB per hash and at most two at
once, both set under `auth.password_hash` in `config.yaml`. Every other request
verifies a JWT cookie and hashes nothing.

## Backups

Nothing here backs up the database. The data worth keeping is one volume:

```sh
make pg-dump    # ./backups/<timestamp>.sql
```

Redis holds sessions and rate-limit counters; losing it signs everyone out and
nothing more.

## Monitoring

Opt-in with `COMPOSE_PROFILES=monitoring`. Grafana, VictoriaMetrics, Postgres
and Redis publish only on 127.0.0.1. Reach them over SSH:

```sh
ssh -N -L 3000:127.0.0.1:3000 you@server
```

Do not publish the ports. VictoriaMetrics has no authentication of its own.

## License

Apache 2.0. See [`LICENSE`](LICENSE).
