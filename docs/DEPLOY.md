# Deploying cursed_matrix on one server

One host, Docker, everything in this repository. The stack builds its own
images, so the server needs no Go and no Node toolchain — only Docker with the
Compose plugin.

## 0. What the server needs

* Docker Engine 24+ with `docker compose` v2.
* A domain pointing at the host, if the site is to be reached by name.
* Ports 80 and 443 free, for whatever terminates TLS.
* About 2 GB of RAM: Postgres, Redis, the API, nginx, VictoriaMetrics and
  Grafana. Dropping the monitoring pair frees roughly half of it.

## 1. Fetch and configure

```sh
git clone <your remote> cursed_matrix && cd cursed_matrix
cp .env.example .env
```

Fill in `.env`. Every secret is generated, never chosen:

```sh
for name in POSTGRES_PASSWORD REDIS_PASSWORD JWT_SECRET GRAFANA_ADMIN_PASSWORD; do
  printf '%s=%s\n' "$name" "$(openssl rand -hex 32)"
done
```

Hex and not base64: the Postgres and Redis passwords are substituted into
connection URLs, where a `/` or a `+` breaks the parse.

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

## 3. Build, migrate, start

```sh
docker compose --profile backend build
docker compose up -d postgres redis
docker compose run --rm migrate up          # needs no Go on the host
docker compose --profile backend up -d
```

Migrations are a separate step on purpose: the API does not run them at
startup, so an upgrade that needs a schema change fails loudly before the new
code serves a request against the old schema.

**`--profile backend` is not optional.** A plain `docker compose up -d` starts
everything *except* the API, leaving whatever was already running in place — a
deploy that appears to succeed and changes nothing. Same for the mounted
`back/config/config.yaml`: editing it needs `docker compose --profile backend
up -d` to take effect, not a plain `up`.

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
docker compose --profile backend build
docker compose run --rm migrate up
docker compose --profile backend up -d
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
