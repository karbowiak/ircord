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

FROM python:3.15.2-alpine3.24@sha256:a0f2e268f2ce8f3709611a90054ec6a8ae31f6251f5896b8648b9bb99835e30a
LABEL org.opencontainers.image.source="https://github.com/karbowiak/ircord"
COPY backend/requirements-yt-dlp.txt /tmp/requirements-yt-dlp.txt
RUN python -m pip install --no-cache-dir --disable-pip-version-check --require-hashes -r /tmp/requirements-yt-dlp.txt \
    && rm /tmp/requirements-yt-dlp.txt
COPY --from=build /out/ircord /ircord
COPY --from=build --chown=65532:65532 /out/data /data
ENV IRCORD_LISTEN=0.0.0.0:8080 IRCORD_DATA_DIR=/data
ENV HOME=/tmp PYTHONDONTWRITEBYTECODE=1
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/ircord"]
