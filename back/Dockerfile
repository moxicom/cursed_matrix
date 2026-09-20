# syntax=docker/dockerfile:1

# ---------------------------------------------------------------- build stage
#
# Pinned to BUILDPLATFORM so a cross-architecture build compiles natively
# instead of running the whole Go toolchain under emulation: building a
# linux/amd64 image on an arm64 laptop otherwise takes minutes rather than
# seconds. Go cross-compiles from TARGETOS/TARGETARCH on its own.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

# How many packages the compiler builds at once. Empty means one per core,
# which is what you want on a build machine and what kills a small server: the
# peak is roughly 700 MB on twelve cores, and a 2 GB host running the stack
# has nowhere near that spare. Pass --build-arg GO_BUILD_JOBS=1 there.
ARG GO_BUILD_JOBS=

WORKDIR /src

# Dependencies first: they change far less often than the code, so this layer
# survives most rebuilds.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

# CGO off gives a static binary; trimpath and -s -w keep the image small and the
# build reproducible.
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    JOBS=${GO_BUILD_JOBS:+-p ${GO_BUILD_JOBS}} && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build $JOBS -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build $JOBS -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/migrate ./cmd/migrate

# ----------------------------------------------------------------- debug stage
#
# Same binaries on alpine, for the times a shell inside the container is worth
# 9 MB: `docker build --target debug`, or `make back-image-debug`.
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

# --------------------------------------------------------------- runtime stage
#
# Nothing but what was copied in: no distribution, no package manager, no shell.
# Everything the service needs is either inside the binary — the timezone
# database via time/tzdata, the health probe as `api -healthcheck` — or copied
# from the build stage: the CA bundle for outbound TLS, and /etc/passwd so the
# container can run as a named unprivileged user.
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
