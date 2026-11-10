# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM oven/bun:1.4.4-alpine@sha256:4cf82f5686b196504fe119db3fcd9185b191f82ece82c40680e597228ec6070a AS build
WORKDIR /app
COPY frontend/package.json frontend/bun.lock frontend/bunfig.toml ./
COPY frontend/patches/ ./patches/
RUN bun install --frozen-lockfile --ignore-scripts
COPY frontend/ ./
# Credited under Settings → About; nuxt.config.ts reads it from the repository root.
COPY COLLABORATORS.md /COLLABORATORS.md
# The release tag; the workflow also places that release's notes in server/assets.
ARG VERSION=""
RUN IRCORD_VERSION="$VERSION" bun run build

FROM oven/bun:1.4.4-alpine@sha256:4cf82f5686b196504fe119db3fcd9185b191f82ece82c40680e597228ec6070a
LABEL org.opencontainers.image.source="https://github.com/karbowiak/ircord"
ENV NODE_ENV=production HOST=0.0.0.0 PORT=3000
WORKDIR /app
COPY --from=build --chown=bun:bun /app/.output ./
USER bun
EXPOSE 3000
CMD ["bun", "server/index.mjs"]
