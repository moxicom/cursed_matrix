# syntax=docker/dockerfile:1

FROM node:22-alpine AS build

WORKDIR /app

COPY package.json package-lock.json .npmrc ./
RUN --mount=type=cache,target=/root/.npm npm ci

COPY . .

# tsc -b && vite build: a type error fails the image on purpose.
RUN npm run build

# Unprivileged image: uid 101, listens on 8080.
FROM nginxinc/nginx-unprivileged:1.27-alpine AS runtime

# The entrypoint runs envsubst over templates/*.template into conf.d/.
COPY --chown=nginx:nginx nginx/default.conf.template /etc/nginx/templates/default.conf.template

ENV BACKEND_HOST=back \
    BACKEND_PORT=8080
COPY --from=build --chown=nginx:nginx /app/dist /usr/share/nginx/html

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
