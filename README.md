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

Set the backend-only `IRCORD_REGISTRATION_KEY` (or `IRCORD_REGISTRATION_KEY_FILE`) to require a shared key for new accounts.
Set `IRCORD_REQUIRE_REGISTRATION_KEY=true` to fail startup if that key is missing. Existing account logins use their personal keys.
To remove this temporary gate, unset the shared key and the requirement flag. `IRCORD_ALLOW_REGISTRATION=false` closes registration entirely.

## Link previews

By default each browser previews only direct image, GIF and video links, following the user's
“Image and video previews” setting (off, trusted hosts, or all links). Nothing else is fetched.

A channel or direct message can opt in to **Server link previews** in its settings. The backend
then fetches the first HTTPS link in each ordinary text message and the browser shows the result:

- Ordinary websites get a card with their title, summary and share image (Open Graph,
  Twitter card, `<title>` and `description` metadata).
- Tenor (`tenor.com`) and GIPHY (`giphy.com`) share pages show their GIF inline. That includes
  short links that redirect to them and GIPHY's non-canonical URLs (such as a bare GIF id),
  whose stub pages forward with an immediate meta refresh. The GIF must be served from the
  provider's own domain; if it is missing or fails to load, the card is shown instead.

Server fetches accept only public HTTPS destinations, check every redirect and same-site meta
refresh (at most three in total) and the resolved addresses against private and reserved
ranges, send no cookies or credentials, read at most 1 MiB of HTML, run at most four at a time
and cache results for ten minutes. Images and GIFs in previews load directly from the site's
image or media host and are hidden while the device's preview setting is off. GIF and video
playback preferences do not change which links resolve.

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
make test-read-markers
```

`make api-generate` updates the OpenAPI schema, TypeScript client types, and WebSocket validator from Go.
Integration tests start an isolated Ergo container.

## Containers

GitHub Actions builds and publishes only when a version tag such as `v0.1.0` is pushed:

- `ghcr.io/karbowiak/ircord-backend:v0.1.0`
- `ghcr.io/karbowiak/ircord-frontend:v0.1.0`

Each image also has a commit tag. Flux follows stable `vX.Y.Z` releases.
Create a release with `git tag v0.1.0` and `git push origin v0.1.0`.
