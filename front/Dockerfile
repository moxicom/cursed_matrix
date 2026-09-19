# syntax=docker/dockerfile:1.7

# ---------------------------------------------------------------------------
# Build stage — produces the static bundle in /app/dist.
# ---------------------------------------------------------------------------
FROM node:22-alpine AS build

WORKDIR /app

# Dependencies are copied on their own so this layer is rebuilt only when the
# lockfile changes, not on every source edit.
COPY package.json package-lock.json .npmrc ./
RUN --mount=type=cache,target=/root/.npm npm ci

COPY . .

# `npm run build` is `tsc -b && vite build`: a type error fails the image, which
# is the point — a broken build must not become a running container.
RUN npm run build

# ---------------------------------------------------------------------------
# Runtime stage — nginx serving the bundle.
#
# The unprivileged image runs as uid 101 and listens on 8080: no root in the
# container, and no capability needed to bind the port.
# ---------------------------------------------------------------------------
FROM nginxinc/nginx-unprivileged:1.27-alpine AS runtime

# The image entrypoint runs envsubst over /etc/nginx/templates/*.template into
# /etc/nginx/conf.d/ before nginx starts. Only names that exist in the
# environment are substituted, so nginx's own $uri / $host survive untouched.
COPY --chown=nginx:nginx nginx/default.conf.template /etc/nginx/templates/default.conf.template

# Defaults, so the image is runnable on its own; compose overrides them.
ENV BACKEND_HOST=back \
    BACKEND_PORT=8080
COPY --from=build --chown=nginx:nginx /app/dist /usr/share/nginx/html

EXPOSE 8080

# wget is in busybox on the alpine image; /healthz is answered by nginx itself
# and never touches the backend, so this stays green while the API is down.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
