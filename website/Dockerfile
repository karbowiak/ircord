# syntax=docker/dockerfile:1
# The public website at ircord.com: static files built with Nuxt, served by an unprivileged nginx.
FROM --platform=$BUILDPLATFORM oven/bun:1.4.4-alpine@sha256:4cf82f5686b196504fe119db3fcd9185b191f82ece82c40680e597228ec6070a AS build
WORKDIR /app
COPY website/package.json website/bun.lock website/bunfig.toml ./
RUN bun install --frozen-lockfile --ignore-scripts
COPY website/ ./
# The site shows the changelog and the collaborators; app/data.ts reads them one level up.
COPY CHANGELOG.md COLLABORATORS.md /
RUN bun run generate

FROM nginxinc/nginx-unprivileged:1.30-alpine@sha256:f5116ed4606f3eb015dd4a8e28cb9b351476dc0ad4bc6e9f93b19cec130cdf03
LABEL org.opencontainers.image.source="https://github.com/karbowiak/ircord"
COPY website/nginx.conf /etc/nginx/conf.d/default.conf
COPY website/security-headers.conf /etc/nginx/security-headers.conf
COPY --from=build /app/.output/public /usr/share/nginx/html
USER nginx
EXPOSE 8080
