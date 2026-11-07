# syntax=docker/dockerfile:1
ARG GO_VERSION=1.27.3
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY backend/ ./
RUN mkdir -p /out/data
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags="-s -w -X ircord/backend/internal/buildinfo.Version=${VERSION} -X ircord/backend/internal/buildinfo.Commit=${COMMIT}" \
    -o /out/ircord ./cmd/ircord

FROM gcr.io/distroless/static:nonroot@sha256:25ac8f0c3b2334a9eb05452266bd4697864a8c8ef0ef71e5e6480d7f92868329
LABEL org.opencontainers.image.source="https://github.com/karbowiak/ircord"
COPY --from=build /out/ircord /ircord
COPY --from=build --chown=65532:65532 /out/data /data
ENV IRCORD_LISTEN=0.0.0.0:8080 IRCORD_DATA_DIR=/data
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/ircord"]
