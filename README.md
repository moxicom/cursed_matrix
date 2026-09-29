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
about 130 MB. The build does not fit next to it, so the server pulls images
that GitHub Actions (`.github/workflows/images.yml`) has already built and
pushed to `ghcr.io/moxicom/cursed_matrix/{back,front}` on every push to `main`.

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

### 2. Configure

```sh
git clone <remote> cursed_matrix && cd cursed_matrix
make init && make secrets
```

Then in `.env`:

```ini
IMAGE_REPO=ghcr.io/moxicom/cursed_matrix
IMAGE_TAG=latest           # or sha-<commit>, or a v* tag
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

### 3. Deploy

```sh
make deploy     # pull back+front, migrate, up -d --no-build
```

Never run `docker compose build` or a plain `up` there: with an image that
cannot be pulled, `up` falls back to building. Updates are `git pull` (compose
files and config only) followed by `make deploy` once the images workflow has
finished for that commit.

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

### 5. Verify

```sh
docker compose ps                              # every service healthy
curl -sf https://cursed.example.com/healthz    # ok
free -m                                        # swap used should stay near 0
```

Register an account and complete one task: that exercises the database,
Redis, the cookie scheme and the XP ledger at once.

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
| `billing.enabled` | `false`: buying a plan grants it outright for `granted_period`, nobody is charged |
| `billing.trial_period` | 14 days for new accounts |
| `billing.prices` | per locale, not converted: 2.39 USD for EN, 199 RUB for RU |

Every secret is named indirectly: a `*_key` field holds the environment
variable that carries the value, so an operator repoints a credential without
touching the image.

Things that are enforced by the API, not the page: free-plan quotas (35 active
tasks, 25 links), input limits (title 100, description 2000, tag 24), XP and
level rules. The frontend only renders.

## Memory footprint

Measured, not estimated. See `docs/DEPLOY.md` for the method.

| | |
|---|---|
| Postgres, Redis, API, nginx | 130 MB |
| Monitoring profile | 420 MB |
| Frontend build | 256 MB |
| Backend build | 768 MB, or 512 MB with `GO_BUILD_LOWMEM=1` |

The one runtime spike is login: argon2id at 64 MiB per hash, bounded by the
rate limits in `config.yaml`.

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
