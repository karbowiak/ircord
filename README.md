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

Message links stay clickable and resolve as their messages become visible. Reddit,
Instagram and TikTok use native media cards, inline galleries and player controls,
with an Iframely widget fallback. Direct image, GIF and video URLs and uploaded
media have labeled previews; GIFs and muted videos animate on hover, with keyboard
and touch activation. Other links use Iframely widgets. Code stays literal, and
uploaded files retain their download links.

## Links

The **Links** button in the conversation header lists the links posted in the open channel, or
in all your channels, newest first (`GET /api/v1/links`). Filter by text (address or message),
link type, network and, through the API, host and posting time. Each card can
be opened, copied, or followed back to the exact post; the conversation you leave keeps its draft.

- Links share the galleries' message index: every link of a stored message's text is indexed
  as the message is stored (`msgindex.Links` in `backend/internal/msgindex`). A link is a
  lowercase `http://` or `https://` up to whitespace, an IRC formatting code, `<`, `>` or `"`,
  outside code as the timeline and server previews define it. Trailing sentence punctuation
  and closing brackets the link did not open are dropped, which the timeline's own link
  target does not do. Links with credentials are left out, and a message adds at most 20.
  A direct HTTPS image link the gallery shows is one entry serving both the gallery and the
  link list; image links the gallery leaves out (plain `http://`, past its ten per message, or
  beside a picked GIF) are listed under “Pages”. Upgrading re-indexes
  stored history in the background with the galleries' checkpointed backfill.
- Posts of the same resource form one card with a post count. They group only when the URL
  matches after normalizing the host (lowercase, punycode) and dropping the default port and
  the fragment;
  query strings, signed URLs and path case are kept, so two articles on one site stay two
  cards. YouTube links group by video. Each card opens the newest post's original URL.
- The list comes from your own stored history: links from deleted or edited-away posts,
  ignored senders and channels you closed are left out, and counts follow the filters.
- Nothing is fetched to build it. A card shows a server link preview only when its channel
  already resolved one for that exact link; images follow the device's preview setting like in
  the timeline.
- An open list offers newly posted links with a button instead of moving, and refreshes when a
  listed post is deleted or edited, a channel or network is removed, or someone is ignored.
  While stored history is still being indexed after an upgrade, the list says older links may
  be missing.
- Each page groups the account's matching links in SQLite and reads the cards' posts in one
  query, so its cost grows with the account's history: with 50,000 link posts on one account
  the first page took about 0.23 s (0.27 s with a text filter, 0.03 s with a host filter).

## Server emotes

Every account on one IRCord backend shares a custom emote catalog (Settings → Emotes, and
**Server emoji** in the picker). Messages carry the readable `:shortcode:`, so other IRC
clients and other IRCord deployments see the name as text; only this backend's users see images.

- `IRCORD_EMOTE_MANAGERS` lists the account usernames (comma-separated) that may upload,
  import from 7TV or FrankerFaceZ, rename, disable and remove emotes; `*` allows every account.
  Unset, nobody can change the catalog.
- On first start the backend imports a small seed set from pinned provider IDs (`:kekw:`,
  `:kekl:`, `:kekleo:`, `:oooo:`, `:notlikethis:`), retrying in the background if a provider is
  down. Each seed is applied once, so a removed seed stays removed. `IRCORD_EMOTE_SEED=false`
  skips it.
- Images are validated (PNG, GIF, WebP or JPEG, at most 1 MiB and 1024×1024) and copied into the
  configured file storage (`local` or `s3`; the local uploads directory when file hosting is
  disabled). They belong to the instance, not the uploader, and are never fetched from a
  provider while rendering. Animated emotes keep a still frame for viewers who turn animation
  off. Switching between local and S3 storage does not migrate existing emote images.
- Shortcodes are lowercase and unique across names and aliases, ignoring case; a provider's
  original casing (such as `KEKW`) is kept as the name and an alias. Backend emotes take
  precedence over bundled workspace emotes, which take precedence over 7TV globals. A renamed
  emote keeps rendering in history under up to eight recent former names, unless another emote
  or a standard emoji now uses that name; a removed emote's messages show the shortcode as text.

## Message formatting

The composer accepts a Discord-style subset of Markdown. IRC has no Markdown capability, so
IRCord sends either native IRC formatting codes, which other IRC clients already show, or the
text exactly as typed. Every view, live or from history, renders from that same IRC text.

| You type                                       | Sent on IRC as                       | Other IRC clients see          |
| ---------------------------------------------- | ------------------------------------ | ------------------------------ |
| `**bold**`                                     | IRC bold (`^B`)                      | bold                           |
| `*italic*` or `_italic_`                       | IRC italic (`^]`)                    | italic                         |
| `__underline__`                                | IRC underline (`^_`)                 | underline                      |
| `~~strike~~`                                   | IRC strikethrough (`^^`)             | strikethrough, where supported |
| `` `inline code` ``                            | IRC monospace (`^Q`)                 | monospace, where supported     |
| ` ```lang ` … ` ``` ` on their own lines       | the lines unchanged, fences included | the block as plain text        |
| `- item` or `1. item` at a line start          | the lines unchanged                  | the plain list text            |
| `> quote` at a line start, `>>> ` for the rest | the lines unchanged                  | the plain `>` text             |

Formatting codes received from other clients, including colours, are shown as sent. Nothing
inside inline or fenced code is rendered as formatting, links, emoji, mentions
or channel links, and `snake_case` words and URLs outside code are left alone. As in other IRC
clients, your nick and highlight words still notify you when they appear inside code. Emphasis
can wrap inline code, and a URL when a space separates it from the closing marker (`**see
https://example.com **`): a marker touching a URL is read as part of the URL. A fence opens at
the start of a line, may name a language and closes at the next ` ``` `; an unclosed
fence stays ordinary text. Code blocks keep their whitespace, scroll inside a bounded box and are
always rendered as escaped text. A block whose fence names a common language (`js`, `ts`, `py`,
`go`, `rust`, `sh`, `json`, `yaml`, `html`, `css`, `sql`, `c`, `cpp`, `java`, `diff` and more; see
`frontend/shared/utils/highlight.ts`) is syntax highlighted in the colour scheme's palette (very
long blocks stay plain); any other block stays plain text. Each block has a copy button that
copies the code as shown, without the fences or language tag. Consecutive `> ` lines show as
one quote and `>>> ` quotes everything after it, code blocks included; `>` without a following
space, or in the middle of a line, stays text.

Messages with line breaks, such as code blocks, travel as one IRCv3 `draft/multiline` batch
within the byte and line limits the network negotiated. Where `draft/multiline` is not
negotiated or a message exceeds those limits, IRCord keeps the draft and names the limit
instead of splitting or altering the message.

## Editing messages

IRC has no agreed way to edit a message yet: the IRCv3 `draft/message-edit` proposal is
unfinished and no server implements it. IRCord sends an edit as an ordinary `PRIVMSG` with the
corrected text and the client-only tag `+ircord/edit=<msgid of the original>`. IRCord clients
fold it into the original message, which keeps its ID, replies and reactions and shows
“(edited)” with its earlier versions. Other IRC clients show the corrected text as a new line,
as when someone types a correction by hand; nothing is hidden from them. The server stores the
tag with its history, so the edit also applies when history is loaded later.

Edit your own message from its toolbar or menu, or press Up in an empty composer to edit your
newest one; Enter saves and Escape cancels. Editing is offered where the network negotiated
`message-tags`, `echo-message` and `account-tag` and `CLIENTTAGDENY` lets the tag through, and
only for messages sent from the account you are logged into. Other IRCord clients accept an edit
only from the original sender and the account the original was sent from, so a message sent
without an account login cannot be edited, and someone who later takes the same nick cannot
change it. Channel operators can delete other people's messages but not edit them. There is no
time limit. An edit that arrives before its original (for example, in history loaded before
scrolling back) shows as an ordinary line until the original is loaded, then folds into it.
Deleting an edited message also deletes its edits on the network, so none of its versions come
back from history.

## Reading highlights aloud

Settings → Read aloud takes an OpenAI-compatible text-to-speech service: its API base URL (such
as `https://api.openai.com/v1`), a model, a voice and an optional API key. With the speaker
button beside Settings switched on, messages that highlight you are read aloud on that device,
one at a time. The browser calls the service's `/audio/speech` directly; the IRCord server never
contacts it. The address, model and voice are saved with your account's preferences. The API key
stays in that browser, is sent only to the service, and is removed when you log out.

The service must allow requests from browsers (CORS) with an `Authorization` header; OpenAI's API
does. Its address must use HTTPS, or plain HTTP only for a server on your own computer or network
(such as `http://localhost:8880/v1`). The page's Content-Security-Policy allows that one origin
in `connect-src` for the signed-in user who saved it, so after changing the address IRCord asks
for a reload.

## Develop

Use the latest stable Go, Air, Bun 1.4.4, and Docker. Install Air with
`go install github.com/air-verse/air@v1.67.4` (and put Go's bin directory on `PATH`).
The frontend uses Bun for dependency
installation, development, checks, and production through Nitro's Bun preset.

```sh
cd frontend && bun install --frozen-lockfile && cd ..
make backend
```

In another terminal, run `make frontend`. Run both commands from the repository root;
`cd frontend && bun run dev` also starts the frontend directly.
The frontend loads `frontend/.env` automatically. The backend loads optional shell
assignments from ignored `backend/.env` (see `backend/.env.example`), then applies
development defaults. It keeps using `.data/live` and detects the existing registration
key and network-password files there. `make dev-backend` remains an alias.
`make backend` uses Air to watch backend Go files, SQL migrations, `go.mod` and
`go.sum`, rebuilding and restarting on changes. Failed builds rerun the last
successful binary. Stop the watcher with Ctrl+C; restart it after changing `.env`.

Development builds use the ignored `.data/go-build-cache` directory with a default
2 GiB file budget. The build helper prunes the oldest entries before and after each
build, including failed builds; compilation can temporarily exceed the budget.
Override it with `IRCORD_GO_CACHE_MAX_MB=1024 make backend` (MiB), or set that variable
in `backend/.env`. `make build` remains the one-shot backend and CLI build.
OpenAPI and Scalar documentation are available at [localhost:8080/docs](http://localhost:8080/docs).

Independent Go media clients live in `backend/internal/cdn`, `iframely`, and
`linkmedia`. `linkmedia.FromEnv()` constructs a resolver from the settings in
`backend/.env.example`. The client requests account-scoped metadata for visible
message links through an authenticated HTTP endpoint. Reads never rewrite history
or launch background downloads. Source failures in the browser retry as widgets.
Resolved URLs share a bounded in-memory cache across messages for up to seven days.
Known signed-source expirations shorten that lifetime; transient failures and native
extraction fallbacks retry sooner. Restarting the backend clears this runtime cache.
Per-message widget tokens still recheck the requesting account's access.

Reddit, Instagram and TikTok use custom backend extraction, preferring usable
source URLs, with an Iframely widget fallback. Direct media file URLs use a bounded
512-byte type/dimension probe and remain at their original source. Other URLs go
to Iframely for widget data. Iframely media links are not converted to native media.
Metadata-only Iframely responses render as isolated summary cards with the returned
title, description and thumbnail. Refused or inaccessible posts show a specific
explanation; age restrictions are identified only when the extractor reports them.
Reddit uses OAuth for posts and ordered
galleries; Instagram reads structured photo/carousel data, then Instagram,
TikTok and Reddit video extraction can invoke `yt-dlp` in simulation mode.
Install `yt-dlp` on the backend host's PATH, or configure `IRCORD_YTDLP_BINARY`.
The production image includes a pinned yt-dlp installation and its Python runtime,
and runs both the backend and extractor as the existing unprivileged user.
Unavailable media falls back to the configured Iframely service. Standalone
publisher iframes use their external origin. Standard X and Bluesky script widgets
also use their provider's frame, with validated resize messages and app theme.
Relative inferred iframe URLs retry through Iframely's original oEmbed response.
Other script-based widget HTML lives in an authenticated opaque-origin frame with
its own CSP. HTML never enters the
application DOM. Widget requests recheck access to the retained source message.
Provider wrapper dimensions become responsive iframe sizing without placing
publisher players inside the opaque document. CNN video players stay behind a
load button so their advertising scripts do not start while browsing messages.
When TikTok's video CDN rejects direct playback and Iframely has no player, validated
video links fall back to TikTok's official iframe player without autoplay.
`Resolver.Store` explicitly copies media into optional CDN storage; the CDN client
also provides upload, stat, ranged reads and deletion. On-demand message rendering
uses source URLs or widgets and never stores files as a side effect of reading.

Provider implementations live in `backend/internal/linkmedia/`: `reddit.go`,
`instagram.go`, `tiktok.go` and `files.go`. `handlers.go` registers social providers
and their URL rules, extraction chains and optional provider adaptations.
`resolver.go` selects a handler; `iframely.go` handles the shared widget fallback.
The yt-dlp engine, source delivery and runtime cache remain shared. To add a
native provider, create its Go file, register its handler and wire any required
connector configuration. Unknown providers and extraction failures still use
Iframely automatically.

Iframely widget adapters live separately in `backend/internal/api/link_widget_*.go`:
GitHub summaries, X and Bluesky frames, and CNN player options each have their own
file. Register new adapters in `link_widget_providers.go`; generic sizing, CSP,
authentication and opaque HTML isolation stay in the common widget code.

Focused checks (no running application processes are started):

```sh
cd backend
go test -race ./internal/cdn ./internal/iframely ./internal/linkmedia ./internal/mediafetch
```

Live probes are opt-in: `IRCORD_TEST_CDN_*` targets a disposable object in the
configured bucket (deleted afterwards); `IRCORD_TEST_IFRAMELY_URL` checks both
Iframely endpoints; `IRCORD_TEST_REDDIT_URL` uses `REDDIT_CLIENT_ID` and
`REDDIT_CLIENT_SECRET`; `IRCORD_TEST_EXTRACTOR_URL` invokes the real extractor;
and `IRCORD_TEST_LINKMEDIA_URL` checks the complete resolver using `IRCORD_*`
configuration. Load the ignored environment before running those probes.

For local access to the private cluster services, keep these forwards running:

```sh
kubectl --context k0s-kmininas -n ircord port-forward --address 127.0.0.1 svc/ircord-iframely 18061:8061
kubectl --context k0s-kmininas -n ircord-cdn port-forward --address 127.0.0.1 svc/ircord-cdn 19000:9000
```

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

| Setting          | Flag                 | Environment               | Default                 |
| ---------------- | -------------------- | ------------------------- | ----------------------- |
| Backend URL      | `--server`           | `IRCORD_SERVER_URL`       | `http://127.0.0.1:8080` |
| Credential file  | `--admin-token-file` | `IRCORD_ADMIN_TOKEN_FILE` | None                    |
| Credential value | None                 | `IRCORD_ADMIN_TOKEN`      | None                    |
| Config file      | `--config`           | `IRCORD_CLI_CONFIG`       | None                    |
| Request timeout  | `--timeout`          | `IRCORD_CLI_TIMEOUT`      | `10s`                   |

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
- Channels a user creates on this network, by joining an empty channel from IRCord, are registered to their account with ChanServ. Users can turn this off in Settings → Networks. Joins IRCord replays on reconnect and auto-join channels are never registered, and other networks are never touched. Raise Ergo's `channels.registration.max-channels-per-account` to suit.
- Usernames are never reused. Ergo keeps channel access entries under the account name, so a new owner would inherit them. Deleting an IRCord account, by its owner or after 90 days unused, retires the name. The backend then unregisters the Ergo account, which disconnects it, unregisters the channels it founded, and keeps the name reserved on Ergo. The API can't unregister, so the backend sets a random password, logs in over IRC with SASL as the account, and confirms `NS UNREGISTER`. If Ergo can't be reached, the unregistration is retried with the credential recovery.

| Variable                                 | Purpose                                                                                                                      |
| ---------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `IRCORD_ERGO_ACCOUNTS=false`             | Keeps the association read-only (no account creation)                                                                        |
| `IRCORD_ERGO_IRC_TLS=false`              | Connects the new network, and unregistrations, without TLS (local fixtures only; also needs `IRCORD_ALLOW_PRIVATE_IRC=true`) |
| `IRCORD_HOME_NETWORK_NAME`               | The new network's name (default `IRCord`)                                                                                    |
| `IRCORD_HOME_NETWORK_CHANNELS`           | Comma-separated channels it joins                                                                                            |
| `IRCORD_REGISTER_CREATED_CHANNELS=false` | Turns off registering channels users create (see below)                                                                      |

To make IRCord the only way onto the server, configure Ergo so that only SASL-authenticated accounts can connect, and accounts are only created through the API:

```yaml
accounts:
  registration:
    enabled: false # no NickServ REGISTER or IRCv3 REGISTER; the API's saregister still works
  require-sasl:
    enabled: true # refuse connections that don't log in
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

### Resetting a password

`ircordctl accounts reset-password <username>` sets a new password for an existing IRCord account without its current password. It uses the admin credential; browser sessions can't call it. The backend checks the password against the same rules as Account settings. It then changes the account on the associated IRC server, updates the home network's saved IRC login when that network still signs in as this account (reconnecting it if it is connected), and signs out every IRCord session of that account. Other accounts, networks, history and files are untouched.

```sh
make cli accounts reset-password alice                      # hidden prompt, asked twice
printf '%s\n' "$NEW_PASSWORD" | make cli -- accounts reset-password alice --password-stdin --json
make cli -- accounts reset-password alice --password-file ./new-password
```

The password is never taken from command arguments or printed. The reset is saved encrypted before the IRC server is touched. If the IRC server can't be reached, the command reports `pending` and exits with 1. Sign-in stays blocked, and the backend retries every 30 seconds and at startup until both sides agree. Running the command again with the same password is safe. If the IRC server refuses the change outright, for example because it has no account of that name, nothing is changed and the command fails. `ircordctl accounts reset-password --help` lists the exit codes. Each reset that reaches the backend is recorded in the backend's `admin_audit` table and log with the target, time and outcome, and never with the password.

### Server owners and the Administration panel

Server owners get an **Administration** panel in the client (the shield next to the settings button). Its Status page shows backend health, version, uptime, IRCord's own counts and the associated IRC server's statistics; its Users page lists every account and lets owners add or remove other owners. Owners use their ordinary signed-in session; the backend checks ownership on every Administration request, so removing an owner takes effect immediately. Channel founders and operators are not server owners. Every grant and removal is recorded with who did it, to whom, when, the optional reason and the outcome.

Ownership belongs to an account's immutable ID, never to a name: deleting an account removes its ownership, and a later account can't inherit it.

Designate the first owner in the backend environment:

```sh
IRCORD_OWNERS=karbowiak          # comma-separated IRCord usernames
```

A configured name only ever resolves to an account that already existed when the backend first saw that name in `IRCORD_OWNERS`, and it stays pinned to that account. Someone who registers a configured name later never becomes an owner; the backend logs a warning instead. Removing a name from `IRCORD_OWNERS` removes that ownership at the next start, while owners added in the panel are kept. Owners from `IRCORD_OWNERS` can't be removed from the panel, and the last owner can never be removed.

If the owner's account doesn't exist yet, or no owner is left, use the operator credential instead (`PUT`/`DELETE /api/v1/admin/owners/{username}`, `GET /api/v1/admin/owners`):

```sh
curl -X PUT -H "Authorization: Bearer $IRCORD_ADMIN_TOKEN" https://chat.example.com/api/v1/admin/owners/alice
```

## Releases

GitHub Actions builds and publishes only when a version tag such as `v0.1.0` is pushed. Branch pushes and pull requests do not trigger publication.

Two independent workflows run for the same tag:

- **Containers:** backend and frontend build in parallel after Go version selection; the frontend also waits for its release notes. Each image supports Linux amd64 and arm64.
- **CLI:** tests and checks the standalone client, then cross-builds six targets in parallel using the latest stable Go. Publication waits for the complete CLI matrix, independently of container success.

### Standalone CLI downloads

Download `ircordctl` from the matching [GitHub Release](https://github.com/karbowiak/ircord/releases). Each archive contains the binary and its MIT license:

| Platform         | Architectures                            | Archive   |
| ---------------- | ---------------------------------------- | --------- |
| macOS (`darwin`) | `amd64` (Intel), `arm64` (Apple silicon) | `.tar.gz` |
| Linux            | `amd64`, `arm64`                         | `.tar.gz` |
| Windows          | `amd64`, `arm64`                         | `.zip`    |

Assets use names such as `ircordctl_v0.1.0_darwin_arm64.tar.gz`. Extract the archive and put `ircordctl` (or `ircordctl.exe`) on your `PATH`. These are standalone executables; Docker and Go are not required. The tag and commit are embedded in `ircordctl version`.

The release also includes `ircordctl_v0.1.0_checksums.txt`. Verify a downloaded archive before extracting it, for example on macOS:

```sh
shasum -a 256 --ignore-missing -c ircordctl_v0.1.0_checksums.txt
```

On Linux use `sha256sum --ignore-missing -c` instead. A new release gets the tag's section of `CHANGELOG.md` as its notes, followed by GitHub's compare link; a tag without a section gets generated notes. Tags with a prerelease suffix are marked as prereleases. If the release already exists, the workflow uploads/replaces only its CLI assets and preserves its notes, draft state, and prerelease state.

### Container images

The container workflow publishes:

- `ghcr.io/karbowiak/ircord-backend:v0.1.0`
- `ghcr.io/karbowiak/ircord-frontend:v0.1.0`

Each image also has a commit tag. Flux follows stable `vX.Y.Z` releases.

### Cutting a release

`CHANGELOG.md` is the user-facing changelog. Every pull request with a user-visible change adds one plain-language line under `## Unreleased`, grouped as **New**, **Improved** or **Fixed**; refactors, tests, CI, tooling and docs add nothing.

1. In the release commit, rename `## Unreleased` to `## v0.1.0 — YYYY-MM-DD` (today's date) and add a fresh, empty `## Unreleased` above it. A release with nothing user-facing gets the single line "Behind-the-scenes improvements."
2. Push that commit to `main`, then tag it: `git tag v0.1.0` and `git push origin v0.1.0`.

That section becomes the GitHub Release notes and the in-client notes.

### In-client release notes

The frontend image carries the release notes of its own tag and every earlier release, shown in the update banner's **What’s new** dialog and under Settings → About. Before the image builds, `frontend/scripts/release-changelog.mjs` takes each release's section of `CHANGELOG.md` at the tagged commit. Releases from before the changelog existed keep their GitHub Release notes; for a tag with no section whose release the CLI workflow has not created yet, it takes the notes from GitHub's release-notes generator. Newer tags, drafts and (for stable builds) prereleases are left out, also when they have a changelog section. Notes edited after an image was built appear from the next release's image on. Builds without a tag (local and development builds) show no notes.
