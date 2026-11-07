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

## Backend and administration CLI

The two Go modules have separate responsibilities:

- `backend/` builds `ircord`, the local backend command tree. It owns storage and IRC connections and can call backend services directly.
- `cli/` builds `ircordctl`, a standalone HTTP administration client. Run it on your own machine using the backend URL and an admin credential; it needs no container shell, backend database, or IRC credentials.

`make build` builds both into `.data/bin/`. The client also builds independently with `cd cli && go build -o /tmp/ircordctl ./cmd/ircordctl`.

```sh
.data/bin/ircord serve
.data/bin/ircordctl --help
.data/bin/ircordctl version
.data/bin/ircordctl completion zsh > /tmp/_ircordctl
```

Running `ircord` without arguments still starts the backend for existing containers and supervisors. Running `ircordctl` without arguments shows help. Help, version, and completion work offline. `--json` produces machine-readable output; diagnostics go to stderr. Both commands honor `--no-color`, `NO_COLOR`, and non-terminal output.

### Enable administration

Generate an operator credential into a private file without printing it:

```sh
(umask 077; openssl rand -hex 32 > /private/path/ircord-admin-token)
```

Configure the **backend** with `IRCORD_ADMIN_TOKEN_FILE=/private/path/ircord-admin-token`, or supply the 64-character hexadecimal value through `IRCORD_ADMIN_TOKEN`. Configure exactly one source. An invalid or unreadable credential fails startup; leaving both unset disables administration (admin endpoints return 503). Keep the credential out of Git, command arguments, URLs, and browser configuration.

Copy the credential securely to the operator machine, then use its local file with the **client**:

```sh
IRCORD_ADMIN_TOKEN_FILE=/private/path/ircord-admin-token \
  .data/bin/ircordctl --server https://chat.example.com status
```

Remote connections require HTTPS with normal certificate verification. Plain HTTP is accepted only for loopback development, such as the default `http://127.0.0.1:8080`. The URL must reach the backend's `/api/v1/admin/status` route, either directly or through a reverse proxy; a path prefix is supported. Redirects are refused. Admin bearer authentication is independent of ordinary user session cookies and bot tokens. Existing browser origin/session checks remain in force.

Rotate the admin credential by generating a replacement file, updating the backend secret configuration, and restarting the backend. Then update the operator client's credential. The old credential stops working after restart; files are not hot-reloaded. Never expose the credential in logs or status output.

### Client configuration

Flags override environment variables, which override an optional JSON config file, then defaults. Use `--config` or `IRCORD_CLI_CONFIG` to select a file; no backend `.env` file or data directory is read by the standalone client.

```json
{
  "serverUrl": "https://chat.example.com",
  "adminTokenFile": "./ircord-admin-token",
  "timeout": "10s"
}
```

Relative token-file paths resolve from the config file's directory. Config files contain file references, never inline tokens. The client supports:

| Setting | Flag | Environment | Default |
| --- | --- | --- | --- |
| Backend URL | `--server` | `IRCORD_SERVER_URL` | `http://127.0.0.1:8080` |
| Credential file | `--admin-token-file` | `IRCORD_ADMIN_TOKEN_FILE` | None |
| Credential value | None | `IRCORD_ADMIN_TOKEN` | None |
| Config file | `--config` | `IRCORD_CLI_CONFIG` | None |
| Request timeout | `--timeout` | `IRCORD_CLI_TIMEOUT` | `10s` |

An explicit `--admin-token-file` overrides environment credentials. Otherwise set either credential environment variable, not both. Network/configuration failures do not print credential values or arbitrary backend error bodies. No administrative request is automatically retried by the command.

`status --json` returns sanitized version, commit, Go version, start time, uptime, and database health. Exit codes are 0 for success, 1 for degraded health/command failure, 2 for invalid input/configuration, 3 for rejected authentication, 4 for transport/API failure, and 130 for cancellation. A degraded status still produces its JSON payload on stdout and a diagnostic on stderr.

Administration HTTP operations declare their own bearer scheme in OpenAPI. `make api-generate` regenerates the frontend artifacts and the standalone Go client using the pinned `oapi-codegen` tool; `make api-check` detects stale artifacts. The client module imports no backend packages. Bot provisioning and bot-management commands are tracked separately in #34.

## Containers

GitHub Actions builds and publishes only when a version tag such as `v0.1.0` is pushed:

- `ghcr.io/karbowiak/ircord-backend:v0.1.0`
- `ghcr.io/karbowiak/ircord-frontend:v0.1.0`

Each image also has a commit tag. Flux follows stable `vX.Y.Z` releases.
Create a release with `git tag v0.1.0` and `git push origin v0.1.0`.
