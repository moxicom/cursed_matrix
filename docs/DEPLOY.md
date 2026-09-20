# Deploying cursed_matrix on one server

One host, Docker, everything in this repository. The stack builds its own
images, so the server needs no Go and no Node toolchain — only Docker with the
Compose plugin.

## 0. What the server needs

* Docker Engine 24+ with the **Compose v2 plugin** and **buildx**.
* A domain pointing at the host, if the site is to be reached by name.
* Ports 80 and 443 free, for whatever terminates TLS.
* Memory. Measured, not estimated:

  | | |
  |---|---|
  | Postgres, Redis, the API, nginx | **130 MB** |
  | VictoriaMetrics, Grafana, two exporters | **420 MB** |
  | the Go compiler, while building | **up to 700 MB** |

  The application is small; the monitoring costs three times what it watches,
  and the compiler costs more than either. A 1 GB server runs the application
  comfortably, runs it and the monitoring badly, and cannot build it at all
  while serving. See "Building on a small server".

Check before anything else:

```sh
docker compose version     # Docker Compose version v2.x or later
docker buildx version      # github.com/docker/buildx v0.x
```

If the first one answers

```
unknown flag: --profile
Usage:  docker [OPTIONS] COMMAND [ARG...]
```

or `'compose' is not a docker command`, the plugin is missing: the docker CLI
did not recognise `compose` as a subcommand and tried to read the rest as its
own flags. This is what `apt install docker.io` gives you — the engine from the
distribution's repository, without the plugins.

The stack needs both plugins, not out of taste: the compose files use
`profiles:` and `!override`, which are Compose v2, and the images build with
`--mount=type=cache` and `--platform=$BUILDPLATFORM`, which are BuildKit.

Both plugins are single binaries the CLI looks for in
`/usr/local/lib/docker/cli-plugins`. Installing them touches no package, does
not restart the daemon, and does not disturb anything already running — which
matters if this host already serves something from Docker:

```sh
sudo mkdir -p /usr/local/lib/docker/cli-plugins
ARCH=$(dpkg --print-architecture)          # amd64 or arm64

sudo curl -SL "https://github.com/docker/compose/releases/latest/download/docker-compose-linux-$(uname -m)" \
  -o /usr/local/lib/docker/cli-plugins/docker-compose
sudo chmod +x /usr/local/lib/docker/cli-plugins/docker-compose

BUILDX=$(curl -fsSL https://api.github.com/repos/docker/buildx/releases/latest \
  | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')
sudo curl -SL "https://github.com/docker/buildx/releases/download/$BUILDX/buildx-$BUILDX.linux-$ARCH" \
  -o /usr/local/lib/docker/cli-plugins/docker-buildx
sudo chmod +x /usr/local/lib/docker/cli-plugins/docker-buildx

docker compose version && docker buildx version
```

Switching the host to Docker's own packages (`docker-ce`) is the tidier
long-term answer, but it removes and reinstalls the engine: the daemon stops,
and every container on the host stops with it. Do that during a window where
that is acceptable, never as a step in this procedure.

### If something already runs on this host

The stack publishes these ports. Everything but the first is on the loopback,
but a collision still stops the container from starting:

| Port | Service | Bound to |
|---|---|---|
| 8080 | the site (`FRONT_PORT`) | every interface unless you change it |
| 5432 | Postgres | 127.0.0.1 |
| 6379 | Redis | 127.0.0.1 |
| 8428 | VictoriaMetrics | 127.0.0.1 |
| 3000 | Grafana | 127.0.0.1 |

```sh
sudo ss -ltnp | grep -E ':(8080|5432|6379|8428|3000)\b'
```

Anything that answers is a collision; change the matching `*_PORT` in `.env`.
The compose project is named `cursed-matrix`, so its networks and containers
are its own and `docker compose down` here stops nothing else.

With a reverse proxy already on the host, skip §2 and point the proxy you have
at `FRONT_PORT` instead. What it must send is in §2: `X-Forwarded-Proto`, so
HSTS appears, and `X-Real-IP`, without which every visitor shares one
rate-limit counter.

## 1. Fetch and configure

```sh
git clone <your remote> cursed_matrix && cd cursed_matrix
cp .env.example .env
```

Fill in `.env`. Every secret is generated, never chosen:

```sh
make secrets
```

It writes a fresh value into every secret that is still empty, leaves the ones
already set alone, and then asks compose to read the file back — so the step
that says it worked has checked.

Hex and not base64: the Postgres and Redis passwords are substituted into
connection URLs, where a `/` or a `+` breaks the parse.

### "I filled it in and compose still says it is missing"

Compose prefers an **environment variable** over the value in `.env`. If one of
these names is exported in the shell — empty — it wins and the file is ignored:

```sh
printenv REDIS_PASSWORD        # an empty line means it is exported, and empty
env | grep -E 'POSTGRES_PASSWORD|REDIS_PASSWORD|JWT_SECRET|GRAFANA_ADMIN'
unset REDIS_PASSWORD           # then run compose again
```

The Makefile does this to itself: it reads `.env` at startup and exports what
it found, so a value filled in during the same `make` run is not the one its
later commands see. That is why `make secrets` clears these four names before
checking.

If nothing is exported, look at the line itself:

```sh
grep -n 'REDIS_PASSWORD' .env | cat -A
```

`cat -A` shows what is otherwise invisible: `^M$` at the end means Windows line
endings and a stray carriage return inside the value; a trailing space, or
`KEY = value` with spaces around the `=`, means the name never matched.

Two settings decide whether this is a deployment or a laptop:

```ini
APP_ENV=production
FRONT_PORT=127.0.0.1:8080
```

`APP_ENV` is not decoration. Anything other than exactly `production` issues
the session cookies **without the `Secure` flag**, which means a browser will
send them over plain HTTP. `FRONT_PORT` bound to the loopback keeps the
plain-HTTP container off the network, so the only way in is through the TLS
terminator.

Compose refuses to start if a secret is empty. That is deliberate: a default
password that works is a default password that ships.

## 2. Put TLS in front

The stack serves plain HTTP on 127.0.0.1:8080 and expects something in front
of it to hold the certificate. Caddy is the shortest path, because it obtains
and renews the certificate by itself:

```
# /etc/caddy/Caddyfile
cursed.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

Caddy sets `X-Forwarded-Proto: https`, which is what makes the application send
`Strict-Transport-Security`. It stays silent without it, so a development host
reached over plain HTTP is not told to refuse plain HTTP for a year.

With nginx on the host instead, the equivalent is:

```nginx
server {
    listen 443 ssl http2;
    server_name cursed.example.com;

    ssl_certificate     /etc/letsencrypt/live/cursed.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/cursed.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

`X-Real-IP` matters more than it looks: the per-address ceilings count by it,
and the container's own nginx overwrites whatever a client sent. A terminator
that does not set it makes every visitor share one counter.

### Building on a small server

The symptom is the host becoming unresponsive, or the build dying, at this
step:

```
=> [back build 6/6] RUN ... go build -trimpath -ldflags=...
```

That is the Go compiler, and it is competing with the stack it is meant to
replace. Confirm it:

```sh
dmesg -T | grep -i 'out of memory\|killed process' | tail
free -m
```

An exit code of 137 says the same thing.

Four ways out, cheapest first:

**Give the host swap.** A build is exactly what swap is for — slow is fine,
dead is not. 4 GB is plenty:

```sh
sudo fallocate -l 4G /swapfile && sudo chmod 600 /swapfile
sudo mkswap /swapfile && sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
```

**Build one package at a time.** The compiler's memory goes with its
parallelism:

```sh
docker compose build --build-arg GO_BUILD_JOBS=1
```

**Stop the stack while building.** Nothing needs to serve during a build:

```sh
docker compose down
docker compose build
docker compose up -d
```

**Leave the monitoring off.** It is opt-in: in `.env`,

```ini
COMPOSE_PROFILES=
```

runs Postgres, Redis, the API and nginx and nothing else — 130 MB instead of
550. Everything the product does works; what you lose is the dashboards.

### On a 1 GB server

Do not build there. 130 MB to run and 700 MB to compile do not fit in the same
gigabyte, and swap turns the build from impossible into merely very slow.
Build where there is room and ship the image:

```sh
# on your own machine, in the repository
IMAGE_TAG=prod docker compose build back front
docker save cursed-matrix/back:prod cursed-matrix/front:prod \
  | gzip | ssh server 'gunzip | docker load'

# on the server
docker compose up -d --no-build
```

Then, in the server's `.env`:

```ini
COMPOSE_PROFILES=          # no monitoring
REDIS_MAXMEMORY=64mb       # 256mb is a quarter of the host
IMAGE_TAG=prod             # the tag you shipped
```

If you would rather build on the server anyway, give it 4 GB of swap, stop
everything first, and pass `GO_BUILD_JOBS=1`. It will take minutes rather than
seconds, and it will finish.

## 3. Build, migrate, start

```sh
docker compose build
docker compose up -d postgres redis
docker compose run --rm migrate up          # needs no Go on the host
docker compose up -d
```

Migrations are a separate step on purpose: the API does not run them at
startup, so an upgrade that needs a schema change fails loudly before the new
code serves a request against the old schema.

`back/config/config.yaml` is mounted rather than baked into the image, so
editing it takes effect on the next `docker compose up -d` — but only then. A
running container keeps the file it parsed at startup, and its health probe,
which re-reads the file, is what will tell you the two have diverged.

## 4. Check it came up

```sh
docker compose ps                                   # every service healthy
curl -sf https://cursed.example.com/healthz         # ok
curl -si https://cursed.example.com/ | grep -i strict-transport
docker compose logs back --tail 20
```

Then register an account through the site and complete one task. That
exercises the database, Redis, the cookie scheme and the XP ledger in one go.

## 5. Updating

```sh
git pull
docker compose build
docker compose run --rm migrate up
docker compose up -d
```

`restart: unless-stopped` brings everything back after a reboot on its own.

## 6. Backups

Nothing here backs the database up. The data worth keeping is one volume:

```sh
docker compose exec -T postgres \
  pg_dump -U cursed -Fc cursed_matrix > cursed_$(date +%F).dump
```

Restoring:

```sh
docker compose exec -T postgres \
  pg_restore -U cursed -d cursed_matrix --clean --if-exists < cursed_2026-09-20.dump
```

Redis holds sessions and rate-limit counters. Losing it signs everyone out and
is otherwise harmless, so it needs no backup.

## 7. Reaching the monitoring

Grafana, VictoriaMetrics, Postgres and Redis publish only on `127.0.0.1`, so
they are not reachable from the network at all. Use an SSH tunnel:

```sh
ssh -N -L 3000:127.0.0.1:3000 you@server     # then open http://localhost:3000
```

Do not "fix" this by publishing the ports. The exporters expose database
internals and Grafana's admin password is the only thing in front of them.

## 8. Things worth knowing

| | |
|---|---|
| Rotating `JWT_SECRET` | signs everyone out immediately; every access token in flight stops verifying |
| Rotating `POSTGRES_PASSWORD` | the running Postgres keeps the old one — it is applied only when the data volume is created. Change it with `ALTER ROLE`, then update `.env` |
| `IMAGE_TAG` | only names the local image. It is not a registry tag and nothing pulls it |
| Billing | `billing.enabled: false` in `back/config/config.yaml` means buying a plan grants it outright for `granted_period`. Nobody is charged, and nothing takes card details |
| Free-plan quotas and every input limit | enforced by the API, not the page. See `docs/STATUS.md` §6 |
