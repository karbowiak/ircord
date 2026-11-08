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

Without server previews, each browser previews links by their URL alone, following the user's
“Image and video previews” setting (off, trusted hosts, or all links):

- Direct images (`.png .jpg .jpeg .webp .avif`) and GIFs (`.gif .apng`, animated WebP/AVIF on
  GIF hosts). Two or more in one message form a gallery (up to ten; four tiles and “+N”) that
  the image viewer pages through with arrows, ←/→ or a swipe.
- Direct video (`.mp4 .webm .mov .m4v`) and audio (`.mp3 .m4a .aac .ogg .oga .opus .wav .flac`)
  in the browser's own player. Audio loads nothing until played.
- YouTube links: `youtube.com/watch?v=` (also `m.` and `music.`), `youtu.be/`, `/shorts/`,
  `/embed/`, `/live/` and `youtube-nocookie.com/embed/`, with an optional `t=`/`start=` offset of
  up to a day. Each video shows once per message as a card with its thumbnail from `i.ytimg.com`
  (a trusted host). The `www.youtube-nocookie.com` player, built from the video ID alone, loads
  only when someone presses play and is removed when scrolled out of view.

At most ten media links and three players are shown per message. Under “trusted”, direct media
links load only from the built-in well-known hosts (including the GIF providers' media hosts),
the app's own uploads, YouTube thumbnails and the channel's extra trusted media hosts; images in
server link previews follow the server preview setting below. “Off” loads nothing. A thumbnail
or file that fails to load leaves a link to the original.

A channel's operators (@ and higher) can list **Extra trusted media hosts** in its settings,
for example the image host a bot posts to. Under “trusted”, direct media links from those hosts
then show in that channel exactly like the built-in hosts' (still until previewed, with the same
autoplay rules); other channels, “off” and “all” are unaffected. As with any preview, a listed
host sees the IP address of every member whose device loads its media, so only list hosts the
channel trusts with that. Every IRCord member of the channel gets the list; ordinary members see
it read-only. Each entry is an exact host name such as `images.example.com` (subdomains are not
included, at most 20). Schemes, ports, paths, wildcards, IP addresses (including shorthand such
as `127.1`), bare public suffixes and names outside the public DNS (anything not under a
[public suffix](https://publicsuffix.org/), such as `.local`, `.internal` or `.corp`) are
refused with the reason.

A channel's operators (@ and higher) can turn on **Server link previews** for everyone in the
channel from its settings; ordinary members see the setting read-only. The backend then fetches
the first HTTPS link in each ordinary text message and the browser shows the result:

- Ordinary websites get a card with their title, summary and share image (Open Graph,
  Twitter card, `<title>` and `description` metadata).
- GitHub issue, pull request and repository links are ordinary websites: the card shows the
  page's Open Graph title, description and share image (`opengraph.githubassets.com`, a
  trusted host). No GitHub API is used; if GitHub does not serve the page to the server, no
  card is shown.
- Tenor (`tenor.com`) and GIPHY (`giphy.com`) share pages show their GIF inline. That includes
  short links that redirect to them and GIPHY's non-canonical URLs (such as a bare GIF id),
  whose stub pages forward with an immediate meta refresh. The GIF must be served from the
  provider's own domain; if it is missing or fails to load, the card is shown instead.
- YouTube links (the variants above, also behind redirects) are not fetched as pages: the
  backend asks YouTube's oEmbed endpoint (`www.youtube.com/oembed`, at most 64 KiB) for the
  title and channel. Videos whose owner disallows embedding keep their card with a link to
  YouTube and no player; deleted, private or unknown videos show “Video unavailable” with the
  link. No markup or player URL from the provider is used.
- Articles (`og:type=article` or `article:*` metadata) get a card with headline, publisher,
  summary and the lead image in its declared shape; a missing image or summary is left out.
- Album pages (an `/album/`, `/albums/`, `/gallery/`, `/galleries/`, `/a/` or `/sets/` path)
  that declare at least two `og:image`s show them as a gallery, at most ten, each checked like
  any preview image. Other pages declaring several images keep an ordinary card.
- Pages that advertise a JSON or XML [oEmbed](https://oembed.com) document with a
  `<link rel="alternate">` (JSON preferred) have it fetched under the same rules as the page,
  at most 64 KiB within three seconds. Its title, author, provider name and thumbnail (for
  photos, the picture) fill the ordinary card; Open Graph fills whatever it leaves out, keeps
  its image when the thumbnail is refused, and is used alone when the document is missing,
  unreadable, slow or refused. The document's `html`
  snippet is never used, so players stay limited to YouTube. GIF share pages keep their GIF.

Server fetches accept only public HTTPS destinations, check every redirect and same-site meta
refresh (at most three in total) and the resolved addresses against private and reserved
ranges, send no cookies or credentials, read at most 1 MiB of HTML, run at most four at a time
and cache results for ten minutes. Images and GIFs in previews load directly from the site's
image or media host and are hidden while the device's preview setting is off. GIF and video
playback preferences do not change which links resolve.

## Stored linked media

Set `IRCORD_MEDIA_STORAGE=local` or `s3` to download and host copies of linked JPEG,
PNG/APNG, GIF, WebP, MP4 and WebM media, plus page-preview thumbnails. It is disabled
by default. The backend announces loading/ready/failed states through private
message events; configured clients load the stored copy and keep the original
message link. PDFs, office documents and unsupported formats are not ingested.

- `IRCORD_MEDIA_PUBLIC_URL`: public media URL prefix, such as `https://cdn.example.com/media`.
  Local storage defaults to `${IRCORD_PUBLIC_URL}/media`.
- `IRCORD_MEDIA_MAX_FILE_BYTES`, `IRCORD_MEDIA_MAX_TOTAL_BYTES`, and
  `IRCORD_MEDIA_RETENTION_DAYS` default to 50 MiB, 10 GiB, and 30 days.
- S3 uses `IRCORD_MEDIA_S3_ENDPOINT`, `IRCORD_MEDIA_S3_BUCKET`,
  `IRCORD_MEDIA_S3_REGION` (default `us-east-1`), `IRCORD_MEDIA_S3_ACCESS_KEY`, and
  `IRCORD_MEDIA_S3_SECRET_KEY`. Set `IRCORD_MEDIA_S3_INSECURE=true` only for a trusted
  internal/loopback HTTP endpoint. Create the bucket first; object keys use `media/`.
  A direct public CDN needs GET/HEAD access to that prefix, without listing or write access.

Operators host actual copies of the linked content. Content hashes deduplicate
files; size limits and retention bound storage. Deleting a message releases its
references, and unreferenced copies are removed. Operator-authenticated
`DELETE /api/v1/admin/media/{hash}` removes a copy globally. Durable tombstones
prevent automatic refetching of removed content and known source URLs. Storage
failures are retried by maintenance; browser/CDN caches can retain a copy for up
to 60 seconds. Keep public cache lifetimes at or below that bound.

With hosting enabled, **Off** hides linked media; **Trusted** also permits stored
images/GIFs/videos from other public hosts, and **All** retains its broader support
for other link kinds. Failed stored media never falls back to an automatic request
to its original host. GIF still frames, preview gestures and opt-in autoplay remain.
Provider players still contact their provider after an explicit Play action. Existing
uploads retain their separate upload/storage lifecycle.

## Galleries

The **Gallery** button in a conversation's header shows every image and GIF posted there, newest
post first: uploads, direct image links (the same kinds the timeline previews, outside code), GIFs
from the picker, and the GIF or album images of server link previews. Each tile names who posted
it and when, and jumps to the post; opening one shows it in the image viewer, which pages through
the gallery.

- The backend catalogues images as it stores each message (live, from history catch-up or a
  history fetch), so the gallery needs no client to be open. It keeps only the original URL and,
  for uploads to this server, the file ID; images are never downloaded or copied. A link that stops working
  shows as unavailable with a link to the original.
- Edits replace a message's images and deletions remove them. Storing the same message again
  (replays, reconnects, retries) never adds a second copy.
- A gallery covers the history IRCord has stored for your account in that conversation, the same
  messages your timeline can scroll to. History stored before galleries existed is indexed in the
  background after upgrading, in batches that resume after a restart; until that finishes the
  gallery says older images may be missing. Nothing is fetched from IRC to fill a gallery.
- Galleries are private to each account: they list only your own stored copy of a conversation
  you have open, without messages from people you ignore. The same image posted twice appears
  once per post.
- Thumbnails follow the device's “Image and video previews” setting exactly like the timeline:
  under “trusted”, images from other hosts stay links and are not requested. GIFs from the picker
  show the provider's small preview, held on a still frame like in the timeline; the viewer opens
  the full GIF.
- An open gallery follows new posts without losing your place; edits, deletions and ignoring
  someone refresh it.

## Links

The **Links** button in the conversation header lists the links posted in the open channel, or
in all your channels, newest first (`GET /api/v1/links`). Filter by text (address, message or
preview title), link type, network and, through the API, host and posting time. Each card can
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

| You type                                 | Sent on IRC as                       | Other IRC clients see          |
| ---------------------------------------- | ------------------------------------ | ------------------------------ |
| `**bold**`                               | IRC bold (`^B`)                      | bold                           |
| `*italic*` or `_italic_`                 | IRC italic (`^]`)                    | italic                         |
| `__underline__`                          | IRC underline (`^_`)                 | underline                      |
| `~~strike~~`                             | IRC strikethrough (`^^`)             | strikethrough, where supported |
| `` `inline code` ``                      | IRC monospace (`^Q`)                 | monospace, where supported     |
| ` ```lang ` … ` ``` ` on their own lines | the lines unchanged, fences included | the block as plain text        |

Formatting codes received from other clients, including colours, are shown as sent. Nothing
inside inline or fenced code is rendered as formatting, links, link previews, emoji, mentions
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
copies the code as shown, without the fences or language tag.

Messages with line breaks, such as code blocks, travel as one IRCv3 `draft/multiline` batch
within the byte and line limits the network negotiated. Where `draft/multiline` is not
negotiated or a message exceeds those limits, IRCord keeps the draft and names the limit
instead of splitting or altering the message.

## Develop

Use the latest stable Go, Bun 1.4.4, and Docker. The frontend uses Bun for dependency
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

On Linux use `sha256sum --ignore-missing -c` instead. A new release gets generated notes; tags with a prerelease suffix are marked as prereleases. If the release already exists, the workflow uploads/replaces only its CLI assets and preserves its notes, draft state, and prerelease state.

### Container images

The container workflow publishes:

- `ghcr.io/karbowiak/ircord-backend:v0.1.0`
- `ghcr.io/karbowiak/ircord-frontend:v0.1.0`

Each image also has a commit tag. Flux follows stable `vX.Y.Z` releases.

Create a release with `git tag v0.1.0` and `git push origin v0.1.0`.

### In-client release notes

The frontend image carries the GitHub Release notes of its own tag and every earlier release, shown in the update banner's **What’s new** dialog and under Settings → About. Before the image builds, `frontend/scripts/release-changelog.mjs` reads the tag's release; if the CLI workflow has not created it yet, it takes the same notes from GitHub's release-notes generator. Newer tags, drafts and (for stable builds) prereleases are left out. Notes edited after an image was built appear from the next release's image on. Builds without a tag (local and development builds) show no notes.
