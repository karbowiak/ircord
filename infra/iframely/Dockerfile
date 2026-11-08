FROM node:22-alpine AS build
RUN apk add --no-cache g++ make python3 && corepack enable
WORKDIR /iframely
ARG IFRAMELY_REV=f3a1b054c94862f40407604d340ded3363979f31
ARG IFRAMELY_SOURCE_SHA256=6efa4cf6cd12b93eef8f98ac53697da3a871c4e6fc433d5739c011825155832a
ADD --checksum=sha256:${IFRAMELY_SOURCE_SHA256} https://codeload.github.com/itteco/iframely/tar.gz/${IFRAMELY_REV} /tmp/iframely.tar.gz
RUN tar -xzf /tmp/iframely.tar.gz --strip-components=1 -C /iframely && pnpm install --frozen-lockfile --prod
FROM node:22-alpine
LABEL org.opencontainers.image.source="https://github.com/karbowiak/ircord"
LABEL org.opencontainers.image.licenses="MIT"
WORKDIR /iframely
ENV NODE_ENV=production
COPY --from=build --chown=node:node /iframely /iframely
COPY --chown=node:node config.local.js /iframely/config.local.js
USER node
EXPOSE 8061
CMD ["node", "--max-old-space-size=384", "server.js"]
