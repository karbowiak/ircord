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

Accounts sign in with a username and password (8–128 characters, no spaces), stored as argon2id hashes. With an associated Ergo server (below), the username is also the account and nick there.
Set the backend-only `IRCORD_REGISTRATION_KEY` (or `IRCORD_REGISTRATION_KEY_FILE`) to require a shared key for new accounts.
Set `IRCORD_REQUIRE_REGISTRATION_KEY=true` to fail startup if that key is missing. Signing in never needs the shared key.
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

## Backend and administration CLI

The two Go modules have separate responsibilities:

- `backend/` builds `ircord`, the local backend command tree. It owns storage and IRC connections and can call backend services directly.
- `cli/` builds `ircordctl`, a standalone HTTP administration client. Run it on your own machine using the backend URL and an admin credential; it needs no container shell, backend database, or IRC credentials.

`make build` builds both into `.data/bin/`. The client also builds independently with `cd cli && go build -o /tmp/ircordctl ./cmd/ircordctl`.

`make cli` builds and launches the standalone client, showing help by default. Pass commands directly, such as `make cli status`. Use `make cli -- status --json` for flags, or keep using `make cli ARGS="status --json"`. When `cli` is the first target, all following targets are client arguments. It automatically uses `.data/admin-client.json` when present, unless `IRCORD_CLI_CONFIG` is already set; an explicit `--config` still takes precedence.

```sh
.data/bin/ircord serve
make cli
make cli status
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

### Optional associated IRC server

A backend can run without its own associated IRC server. In that case status omits the Ergo section, doctor runs backend/database checks only, and CLI help and completion omit the Ergo command group. Normal user-configured IRC networks continue to work independently.

To associate an Ergo server, configure the backend with all of:

```sh
IRCORD_ERGO_SERVER=irc.example.com:6697
IRCORD_ERGO_API_URL=https://ergo-api.ergo.svc.cluster.local:8089
IRCORD_ERGO_API_TLS_SERVER_NAME=irc.example.com
IRCORD_ERGO_API_TOKEN_FILE=/private/path/ergo-api-token
```

`IRCORD_ERGO_API_TOKEN` is an alternative to the token file; configure only one. The credential must be a high-entropy printable ASCII value of 32–512 characters. `IRCORD_ERGO_API_CA_FILE` can add a private CA. Invalid or partial association configuration fails startup. Leaving all association settings unset disables the integration and makes no Ergo requests.

#### Accounts on the associated server

With an associated server, every IRCord account is also an Ergo account:
- Signup creates the Ergo account through the private API (`ns/saregister`) and the IRCord account. Encrypted recovery records let interrupted signup and password changes finish after a retry or restart. Usernames already taken on Ergo are refused while typing.
- Signup also adds a network for the associated server. It uses SASL with the account's own username and password, so the bouncer is logged in from its first connection.
- Password changes update Ergo first (`ns/passwd`), then IRCord and the saved SASL login, and sign out the account's other sessions.
- Deleting an IRCord account doesn't delete its Ergo account.

| Variable | Purpose |
|---|---|
| `IRCORD_ERGO_ACCOUNTS=false` | Keeps the association read-only (no account creation) |
| `IRCORD_ERGO_IRC_TLS=false` | Connects the new network without TLS (local fixtures only; also needs `IRCORD_ALLOW_PRIVATE_IRC=true`) |
| `IRCORD_HOME_NETWORK_NAME` | The new network's name (default `IRCord`) |
| `IRCORD_HOME_NETWORK_CHANNELS` | Comma-separated channels it joins |

To make IRCord the only way onto the server, configure Ergo so that only SASL-authenticated accounts can connect, and accounts are only created through the API:

```yaml
accounts:
    registration:
        enabled: false        # no NickServ REGISTER or IRCv3 REGISTER; the API's saregister still works
    require-sasl:
        enabled: true         # refuse connections that don't log in
        exempted: []
api:
    enabled: true
channels:
    registration:
        max-channels-per-account: 50
```

Ergo refuses passphrases containing whitespace, which is why IRCord passwords can't contain spaces. With `require-sasl`, the shared server password (`IRCORD_NETWORK_PASSWORDS`) is no longer needed for the associated server.

The private API can use the same certificate as the public IRC hostname: its TLS server name selects the certificate identity while the API URL selects the internal Service. Certificate verification remains enabled. Remote plaintext API connections are rejected; loopback HTTP is available for isolated development fixtures. The transport ignores environment proxies, refuses redirects, and bounds concurrency, request duration, and response size.

The deployment keeps Ergo's API on a dedicated ClusterIP Service over verified TLS. A NetworkPolicy permits API connections only from IRCord backend pods in the IRCord namespace, while preserving the existing public IRC listener on 6697. The private API credential is shared through one canonical Infisical location and never sent to the CLI, browser, or public API responses. The CLI always talks to the IRCord administration API, which exposes named read-only operations rather than forwarding arbitrary Ergo requests.

```sh
make cli status
make cli doctor
make cli ergo status
make cli ergo channels list
make cli ergo users show SomeNick
make cli ergo accounts list
make cli ergo accounts show SomeAccount
make cli -- status --json
```

Ergo commands are discovered from the selected backend's authenticated capabilities. Help and completion still work offline; unavailable capability discovery leaves the base command set visible. An associated server that is temporarily unavailable remains a configured feature: status retains backend health/counters, includes the unavailable server check without fabricated statistics, and exits with degraded health. Doctor does the same for individual checks.

Backend overview counts distinguish IRCord accounts, browser sessions, saved networks, stored message copies, allocated SQLite page bytes, and currently connected upstream IRC sessions. Ergo statistics separately report IRC users, operators, peak users, and channels. NickServ account lookup is separate from IRCord account management. These commands do not create users, change moderation state, reload configuration, or inspect message history.

## Releases

GitHub Actions builds and publishes only when a version tag such as `v0.1.0` is pushed. Branch pushes and pull requests do not trigger publication.

Two independent workflows run for the same tag:

- **Containers:** backend and frontend build in parallel after Go version selection. Each image supports Linux amd64 and arm64.
- **CLI:** tests and checks the standalone client, then cross-builds six targets in parallel using the latest stable Go. Publication waits for the complete CLI matrix, independently of container success.

### Standalone CLI downloads

Download `ircordctl` from the matching [GitHub Release](https://github.com/karbowiak/ircord/releases). Each archive contains the binary and its MIT license:

| Platform | Architectures | Archive |
| --- | --- | --- |
| macOS (`darwin`) | `amd64` (Intel), `arm64` (Apple silicon) | `.tar.gz` |
| Linux | `amd64`, `arm64` | `.tar.gz` |
| Windows | `amd64`, `arm64` | `.zip` |

Assets use names such as `ircordctl_v0.1.0_darwin_arm64.tar.gz`. Extract the archive and put `ircordctl` (or `ircordctl.exe`) on your `PATH`. These are standalone executables; Docker and Go are not required. The tag and commit are embedded in `ircordctl version`.

The release also includes `ircordctl_v0.1.0_checksums.txt`. Verify a downloaded archive before extracting it, for example on macOS:

```sh
shasum -a 256 --ignore-missing -c ircordctl_v0.1.0_checksums.txt
```

On Linux use `sha256sum --ignore-missing -c` instead. A new release gets generated notes; tags with a prerelease suffix are marked as prereleases. If the release already exists, the workflow uploads/replaces only its CLI assets and preserves its notes, draft state, and prerelease state.

### Container images

The container workflow publishes:

- `ghcr.io/karbowiak/ircord-backend:v0.1.0`
- `ghcr.io/karbowiak/ircord-frontend:v0.1.0`

Each image also has a commit tag. Flux follows stable `vX.Y.Z` releases.
Create a release with `git tag v0.1.0` and `git push origin v0.1.0`.
