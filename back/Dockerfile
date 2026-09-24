# syntax=docker/dockerfile:1

# BUILDPLATFORM: compile natively and let Go cross-compile to TARGETARCH.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
# Non-empty: one package at a time + heap cap, ~512 MB instead of ~768 (docs/DEPLOY.md).
ARG GO_BUILD_LOWMEM=

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    if [ -n "${GO_BUILD_LOWMEM}" ]; then export GOFLAGS=-p=1 GOMEMLIMIT=128MiB; fi && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/migrate ./cmd/migrate

# Same binaries on alpine with a shell: `make back-shell`.
FROM alpine:3.21 AS debug

RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app

COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/migrate /usr/local/bin/migrate
COPY --from=build /src/config/config.yaml /etc/cursed-matrix/config.yaml

ENV CONFIG_PATH=/etc/cursed-matrix/config.yaml
USER app
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD ["/usr/local/bin/api", "-healthcheck"]
ENTRYPOINT ["/usr/local/bin/api"]

FROM scratch AS runtime

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /etc/passwd /etc/passwd

COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/migrate /usr/local/bin/migrate
COPY --from=build /src/config/config.yaml /etc/cursed-matrix/config.yaml

ENV CONFIG_PATH=/etc/cursed-matrix/config.yaml

USER nobody
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD ["/usr/local/bin/api", "-healthcheck"]

ENTRYPOINT ["/usr/local/bin/api"]
