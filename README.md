# IRCord

IRCord is a self-hosted IRC client with a Nuxt frontend and a Go backend.
The backend keeps IRC connections online and stores messages, profiles, files, and preferences.
Clients use authenticated HTTP APIs and a private WebSocket event stream.

Try the feedback instance at [chat.ircord.com](https://chat.ircord.com).

## Run with Docker

```sh
docker compose up -d --build
```

Open [localhost:3000](http://localhost:3000), create an account, and add an IRC network.
Save your account key; it signs you in on other devices.

The backend stores its SQLite database, encryption key, and uploaded files in the `backend-data` volume.
Migrations run before the backend starts. Preserve this volume when updating containers.
The frontend needs no database or persistent volume.

For an HTTPS reverse proxy, set `IRCORD_PUBLIC_URL` to the public origin before starting Compose.
The frontend and backend must use the same public origin.
Set `NUXT_PUBLIC_KLIPY_API_KEY` to enable GIF search.

## Develop

Use the latest stable Go, Node.js 26, and Docker.

```sh
cd frontend && npm ci && cd ..
make dev-backend
```

In another terminal, run `cd frontend && npm run dev`.
OpenAPI and Scalar documentation are available at [localhost:8080/docs](http://localhost:8080/docs).

```sh
make test check frontend-check
make test-integration
```

`make api-generate` updates the OpenAPI schema, TypeScript client types, and WebSocket validator from Go.
Integration tests start an isolated Ergo container.

## Containers

GitHub Actions tests each change and publishes separate images:

- `ghcr.io/karbowiak/ircord-backend:main`
- `ghcr.io/karbowiak/ircord-frontend:main`

Each published build also has an immutable `main-<build>` tag and a commit tag.
