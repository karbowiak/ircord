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
Sign in on other devices with the same username and password.

The backend stores its SQLite database, encryption key, and uploaded files in the `backend-data` volume.
Migrations run before the backend starts. Preserve this volume when updating containers.
The frontend needs no database or persistent volume.

For an HTTPS reverse proxy, set `IRCORD_PUBLIC_URL` to the public origin before starting Compose.
The frontend and backend must use the same public origin.
Set `NUXT_SITE_URL` to that origin when running the frontend outside Compose behind a reverse proxy.
The frontend uses it for absolute share-image URLs and browser request checks.
Without it, request checks use the direct connection's host and protocol; forwarded headers do not establish a trusted origin.
Sign-in limits apply per client address. The frontend adds the address it received each request from to `X-Forwarded-For`. The backend reads that header only from proxies listed in `IRCORD_TRUSTED_PROXIES`.
Your reverse proxy must append the client's address to `X-Forwarded-For`; for nginx, set `proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for`.
Compose trusts only its own network, `172.31.250.0/24`, which includes a reverse proxy on the same host. If your reverse proxy runs elsewhere, add its address range to `IRCORD_TRUSTED_PROXIES`.
Operators can configure network presets and optional trusted WEBIRC gateways with `IRCORD_SERVER_LIST_FILE`.
See [Server presets and WEBIRC](docs/server-presets.md) for the file format and address handling.
Signing in on more than 50 devices signs out the device that signed in longest ago.
Set `NUXT_PUBLIC_KLIPY_API_KEY` to enable GIF search.

Accounts sign in with a username and password (8–128 characters, no spaces), stored as argon2id hashes. With an associated Ergo server (below), the username is also the account and nick there.
Set the backend-only `IRCORD_REGISTRATION_KEY` (or `IRCORD_REGISTRATION_KEY_FILE`) to require a shared key for new accounts.
Set `IRCORD_REQUIRE_REGISTRATION_KEY=true` to fail startup if that key is missing. Signing in never needs the shared key.
To remove this temporary gate, unset the shared key and the requirement flag. `IRCORD_ALLOW_REGISTRATION=false` closes registration entirely.

Message links stay clickable and resolve as their messages become visible.
Direct file links are inspected before storage or previewing. Supported media uses
labeled previews; documents use file cards. Other links use standard oEmbed data
when available, with an information card otherwise. GIFs and muted videos retain
hover, keyboard and touch controls. Code stays literal, and uploads retain their
download links and deletion checks.

## Navigation

Browser Back and Forward retrace pages, dialogs, tabs, drawers and media views.
For example, Back returns from Notifications to Profile before closing Settings and returning to the conversation.
Closing a dialog with its close button or Escape skips that dialog's internal steps.
Unsaved-change guards still apply.

Drafts, passwords, typed searches and ordinary form changes do not create navigation entries or enter browser history state.
Settled confirmations and unavailable views are skipped when they cannot be restored safely.
On mobile, GIF and emoji buttons hide while you edit a nonempty draft, then return when editing ends.
The message field stays editable during connection interruptions; sending resumes when the connection is ready.

## Channels and existing groups

Use the plus button beside Channels to join an existing channel or create one.
Creation offers editable templates for private friends, open discussion and announcements, with the server's supported settings below.
IRCord uses your existing connection and registers the channel to your account when supported.
The server confirms ownership and settings; incomplete changes remain visible with an explanation.
Networks without confirmed-creation support can still use ordinary `/join`.

New group conversations are replaced by ordinary channels. Existing groups remain available until their owner chooses **Convert to channel** in group details.
Conversion retains the channel, privacy settings, participant access and message history, and transfers native ownership to the group owner.
It requires the owner to be connected and authenticated; unresolved prerequisites can be retried.
The legacy controller stops after the last active group converts. New channels never use that controller.
See [Existing group conversations](docs/parties.md) for conversion and retained compatibility behavior.

## Links

The **Links** button in the conversation header lists the links posted in the open channel, or
in all your channels, newest first (`GET /api/v1/links`). Filter by text (address or message),
link type, network and, through the API, host and posting time. Each card can
be opened, copied, or followed back to the exact post; the conversation you leave keeps its draft.

Channel operators can enable **Mark channel as NSFW** in the channel overview.
This classification is shared within one IRCord backend; it is not sent over IRC.
In **Settings → Profile**, choose a server and enable **Show NSFW content** to view its sensitive chat previews and Links cards.
This private account preference defaults to off for each server and syncs across your devices.

With the preference off, sensitive Links cards show a blurred placeholder without loading their preview, thumbnail, title or excerpt.
Chat link previews and media attachments use the same setting, with placeholders that preserve their reserved size.
Click a blurred NSFW card to reveal that item temporarily without enabling other cards or changing your Profile setting.
For a gallery, this reveals the whole card. Leaving the view or reloading clears the temporary reveal.

Cards with unknown classification show a neutral placeholder that you can reveal explicitly.
To show all cards shared across several sources, enable NSFW content for each sensitive source's server.
Changing the preference or channel classification updates cards already on screen.
Ordinary message text remains visible. These settings do not grant channel access or change which messages your account can retrieve.

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

See the [message editing draft](docs/drafts/ircord-edit.md) for the wire format, sender checks, and compatibility limits.
The [protocol draft index](docs/drafts/README.md) lists IRCord extensions.

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
change it. Moderators can delete other people's messages but not edit them. There is no
time limit. An edit that arrives before its original (for example, in history loaded before
scrolling back) shows as an ordinary line until the original is loaded, then folds into it.
Deleting an edited message also deletes its edits on the network, so none of its versions come
back from history.

## Deleting other people's messages

Channel operators and server owners can delete other people's messages for everyone. IRCord
sends an IRCv3 `REDACT` (`draft/message-redaction`), and every IRCord account in the channel
replaces the message with “Message deleted”. Other IRC clients show the deletion only if they
support that extension.

| Who                                  | Where                                                | How it reaches IRC                       |
| ------------------------------------ | ---------------------------------------------------- | ---------------------------------------- |
| The author                           | Any conversation                                     | Their own connection                     |
| A channel operator (`@`, `&` or `~`) | That channel                                         | Their own connection                     |
| A server owner                       | Any channel on the associated server                 | The backend's moderation login           |
| Nobody                               | Other people's direct messages, half-operators (`%`) | Not offered; the IRC server refuses them |

Open **Delete message…** from a message's menu or toolbar. A server owner can add a reason, which
is sent with the deletion, and can also delete the files this server hosts for the message.

To remove one person's recent messages, open their menu in the member list and choose **Delete
recent messages…**. Choose the last hour, 24 hours or 7 days. IRCord counts the messages first,
then deletes them one at a time in the background and reports the result. One run deletes at
most 500 messages. On a channel operator's own connection the IRC server's flood limit applies,
so 100 messages take about a minute.

A message can be deleted for everyone only while the IRC server still keeps it in its history.
Ergo keeps one week by default. Accounts that were disconnected find deletions among the latest
100 messages of each channel when they reconnect.

A server owner who is also an operator in a channel deletes through their own connection there.
They still get the owner options, and their deletions are still recorded.

Server owners' deletions on the associated server are recorded in **Administration** under recent
actions, never with the message text:

- A deleted message records the channel, sender, message ID, reason and outcome.
- Each uploaded file deleted with it records the file ID and the account that uploaded it.
- Deleting recent messages records the channel, sender, time range and reason when the run
  starts, and the counts when it finishes.

Channel operators' deletions are not recorded there, because operators can also delete from any
other IRC client.

## Reading highlights aloud

Settings → Read aloud takes an OpenAI-compatible text-to-speech service: its API base URL (such
as `https://api.openai.com/v1`), a model, a voice and an optional API key. With the speaker
button beside Settings switched on, messages that highlight you are read aloud on that device,
one at a time. The browser calls the service's `/audio/speech` directly; the IRCord server never
contacts it. The address, model and voice are saved with your account's preferences. The API key
stays in that browser, is sent only to the service, and is removed when you log out.

The service must allow requests from browsers (CORS) with an `Authorization` header; OpenAI's API
does. Its address must use HTTPS: an IRCord page served over HTTPS can't call plain HTTP on your
network, so put a LAN speech server behind TLS (for example a reverse proxy). Plain HTTP is
accepted only for a server on the same computer (`http://localhost:8880/v1`), and the browser may
ask you to allow IRCord to access devices on your local network. The page's
Content-Security-Policy allows that one origin in `connect-src` for the signed-in user who saved
it, so after changing the address IRCord asks for a reload.

## Develop

See the [backend reference](docs/backend.md) for package ownership, identity, storage, and live-update contracts.
See [Workspace synchronization](docs/workspace-sync.md) for initial state, mobile resume, offline search and files, and repeatable browser measurements.

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

The embedding pipeline lives in `backend/internal/linkmedia` and uses
`backend/internal/cdn` for object storage. `linkmedia.FromEnv()` reads the settings
in `backend/.env.example`. Each visible link resolves through an authenticated
message-scoped endpoint. Every request rechecks access to its retained message.
Reads never rewrite history.

File extensions select a candidate handler; the response bytes determine the actual type.
The backend inspects a bounded prefix while downloading into a temporary file.
It applies size, time and supported-format checks before publishing bytes to the CDN.
Type detection uses `gabriel-vasile/mimetype`; detecting a type is not malware scanning.
Only supported validated images, audio and video receive automatic native previews.
Documents, archives and other downloads use file cards with attachment delivery.
Rejected or unavailable content has a card with known details and an original-link action.
A failed CDN copy never silently becomes an external media preview.

Reddit post links first use the dedicated Reddit API handler when its OAuth credentials are configured.
Photos and ordered galleries use the same validated CDN pipeline, with at most 30 items and
100 MiB downloaded across one post. A failed gallery is discarded before falling back to
generic oEmbed or an information card; original Reddit media URLs are never a delivery fallback.
Reddit's MP4 renditions are preferred for animated gallery items to avoid downloading and
decoding oversized GIF originals. Those renditions still pass normal media validation.
Text posts retain their title, author and description. Reddit videos requiring separate audio
and share-token links currently use the generic fallback.

Other URLs use generic oEmbed discovery and a pinned provider registry.
`artyom/oembed` parses discovery data and responses; the application owns all HTTP requests.
Page requests, discovered endpoints, redirects, thumbnails and downloads use public-destination restrictions.
oEmbed photos and thumbnails follow the same validation and storage pipeline as direct files.
Known YouTube, Vimeo and Spotify players use controlled UI adapters with fixed geometry.
Unknown rich markup and missing oEmbed data produce an information card; oEmbed photos from
unknown page URLs remain bounded card thumbnails. Provider dimensions and resize messages
cannot resize the conversation's preview box.
Embedded players load directly by default. In Settings → Chat, enable **Ask before loading embeds**
to require a **Load embed** action. This choice follows the account across devices.

Embed UI adapters live in `frontend/app/components/embed/`. Each Vue file declares its URL
matches, layout, accepted data and resolver policy in a static `embedProvider` export.
A local Nuxt module extracts these declarations into a small manifest without executing
provider code and supplies lazy imports for the matched renderer. `useEmbed` owns selection,
the existing message-scoped query/cache, expiry and offline state. Adding an adapter requires
one Vue file; a provider-specific Go fetcher is optional when standard oEmbed supplies its data.
New external player origins still require approval in the backend's player trust policy;
declaring a frontend URL match does not authorize an arbitrary iframe origin.
The declaration reserves a height or aspect ratio before fetching anything. Players and cards
use a compact 480px maximum. Image adapters use validated dimensions available before paint,
growing toward 400px tall, or 200px for GIFs, and shrinking to fit the chat column.
Authenticated message responses can include cached dimensions without fetching remote content.
An image without initial dimensions keeps its fixed reservation for that mounted instance.
Single images and GIFs render inline, with download,
copy/open and a compact upload offline button overlaid inside the reserved space.
Loading, success, failure, privacy prompts and refreshes all remain inside that same box.
Unknown URLs keep the fixed fallback card instead of acquiring a different layout after discovery.
See [Embed provider sources and fixtures](docs/embed-providers.md) for upstream provenance
and the optional component-review workflow.

Dropping up to ten files sends one upload-batch message. Its ordered attachment metadata travels
with the message through live updates and history; images/videos share one gallery and other
files keep their own cards within that post. Each upload is checked against the sender and
conversation network before sending. A failed upload leaves the batch available for retry,
reusing files already uploaded. Galleries require a network that can carry their metadata
within its tag and message limits; oversized or unsupported batches fail before being queued.
Deleting a gallery message retains its files. Removing caller-owned uploads requires a separate
confirmation. External links keep one source URL per embed; pasted Reddit links are not merged.

Validated copies are retained for 90 days. Private database metadata keeps original links
and copy expiry. Cache hits do not extend retention. Expired or missing copies refresh on demand.
Durable results survive backend restarts; browser reloads can request metadata without repeating source extraction.
The browser also keeps a small account-scoped metadata cache across reloads: five minutes fresh,
up to fifteen minutes while refreshing, bounded by each resource's earlier expiry.
Failed previews, IRCord upload capabilities and server widget documents are excluded.
Signing out or losing the session clears this cache; it contains no downloaded media bytes.
Native previews use stored URLs only after validation and storage succeed.
Public object keys contain random bytes. A durable reservation tracks each upload before storage writes begin.
The backend deletes expired objects at startup and every minute, independently of message views.
Each pass has a 45-second budget and handles at most 100 objects; failed deletes retry after five minutes.
Cleanup can lag during outages or a large backlog. Prepare a 90-day bucket lifecycle rule for
`IRCORD_CDN_PREFIX/link-media/` as an independent backstop, preserving unrelated rules.
Versioned storage also needs noncurrent-version and expired-marker cleanup.
The application never changes bucket lifecycle configuration or needs bucket administration permissions.
Drain the old cache prefix before changing its endpoint, bucket, prefix or delivery origin.
See `backend/.env.example` for the scoped rule and configuration.

IRCord-owned file links retain their download tokens and deletion checks.
Optional `w` and `h` parameters on `/files/` request a contained display copy, from 1 to 2048 pixels.
The backend checks the file and token before every response, including cached display copies.
Resizing accepts at most 16 MiB and 16 million input pixels, with two active transformations.
It keeps at most 128 display copies or 32 MiB in memory for ten minutes.
Animated, unsupported, orientation-tagged and color-profile images retain their original bytes.
The same fallback applies when resizing saves no bytes or processing is busy.
File responses remain `no-store`; omit dimensions to retrieve the original.

Instagram posts, galleries and reels, and TikTok video links use bounded yt-dlp extraction.
Install `yt-dlp` for local backend development; the backend image includes `yt-dlp-core`.
Set `IRCORD_YTDLP_BINARY` to another executable path, or `disabled` to stop extractor processes.
Instagram can also read complete public photo metadata when extraction does not provide it.
Native media passes the same file checks and 90-day CDN storage as direct links.
The resolver does not import account cookies or sign in to either service.
Temporary anonymous delivery cookies stay within their exact media origin and are discarded after fetching.
Restricted or unavailable posts retain information cards rather than partial galleries or external media URLs.

Generic yt-dlp provider selection remains inactive.
The backend retains the legacy Iframely payload shape to reject cached embeds.
The application has no active Iframely service dependency or scheduled Iframely image publisher.
Do not remove an older deployment's Iframely service until that deployment has been replaced.

For local access to private CDN storage, keep this forward running:

```sh
kubectl --context k0s-kmininas -n ircord-cdn port-forward --address 127.0.0.1 svc/ircord-cdn 19000:9000
```

```sh
make test check frontend-check
make test-integration
make test-read-markers
make test-sync
make test-offline
```

`make api-generate` updates the OpenAPI schema, TypeScript client types, and WebSocket validator from Go.
Integration tests start an isolated Ergo container.
To test moderator deletion against its own disposable Ergo with a moderation operator, run
`IRCORD_MODERATION_INTEGRATION=1 go test -run TestErgoModeratorsDeleteOthersMessages ./internal/irc`
from `backend/`. It needs Docker.

### Frontend checks and visual review

`make frontend-check` checks the generated API contract, ESLint, TypeScript types and production build.
Run `bun run lint` from `frontend/` for Nuxt-aware correctness linting. Prettier owns formatting.
Linting does not run automatically in the development server.
After building, run `bun run test:workspace` from `frontend/` for the isolated workspace browser check.
It starts its own demo server and uses a disposable account. Go tests, including
`make test-read-markers` and its browser bridge, remain available.

Run `make test-offline` for offline reload, message search, saved images, replay, and account cleanup with disposable fixtures.

For UI review, start an isolated demo from `frontend/` with `bun run dev:demo`, or build with
`bun run build` and start `bun run preview:demo`. Both demo commands use
`http://127.0.0.1:4173`; use one at a time. Create a disposable demo account on that page to
get the seeded workspace without connecting to Go or a real IRC network. The normal
`make frontend` and `make backend` commands still run the live development app.

With that disposable demo running, check appearance, panel transitions, and browser request protection from `frontend/`:

```sh
bun run test:appearance-motion
BASE=http://127.0.0.1:4173 bun run test:request-origin
```

The request-origin check also uses local ports 4176 and 4177 for a browser attack fixture.
Use these checks only against an isolated demo. They create disposable accounts and change their saved GIFs.
For live-proxy verification, the script supports `MODE=live` and starts a disposable backend on port 4175.
Point an isolated frontend's `NUXT_BACKEND_URL` there, with demo mode disabled, before running that variant.

Agents can use their browser tools or import `chromium`, `firefox` or `webkit` from
`playwright` for one-off inspection. Playwright supports browser inspection, screenshots,
and the isolated workspace regression check.
Install Chromium once if needed with `bun x playwright install chromium`.

From `frontend/`, open the demo and save its browser session on exit, then capture it:

```sh
mkdir -p ../.data/ui-review
bun run ui:open --save-storage ../.data/ui-review/session.json http://127.0.0.1:4173
bun run ui:screenshot --load-storage ../.data/ui-review/session.json --viewport-size '1440,900' --wait-for-selector '.workspace' http://127.0.0.1:4173 ../.data/ui-review/desktop.png
bun run ui:screenshot --load-storage ../.data/ui-review/session.json --viewport-size '390,844' --wait-for-selector '.workspace' http://127.0.0.1:4173 ../.data/ui-review/mobile.png
```

Inspect the screenshots and actual interactions in both themes and at desktop/mobile sizes.
Session files and screenshots stay in ignored `.data/`. `bun run perf:busy` remains an optional
profiler for an isolated demo; set `BASE=http://127.0.0.1:4173` when using these demo commands.

### Frontend styling

See [Frontend modules](docs/frontend.md#nuxt-modules) for metadata, fonts, images, VueUse, diagnostics, and localization boundaries.

Tailwind 4 runs through `@tailwindcss/vite`; `app/assets/css/tailwind.css` is the entry point.
Use semantic utilities from `theme.css` such as `bg-background`, `text-foreground`,
`bg-primary`, `text-muted-foreground` and `bg-field`. The existing appearance compiler in
`shared/appearance.ts` owns palette, font and corner values, including the initial HTML's
fallback variables. The store and pre-paint script apply saved settings. Add tokens to this
system rather than introducing another theme provider or copying a light/dark palette.

Tailwind's `accent` is a hover surface; the saved accent maps to `primary`. `input` is a border
token, while `field` is the input background. Use `font-chat` and `font-code` for the existing
message and monospace stacks. Spacing uses 4px units to preserve the current fixed control sizes.
Use `cn` when a reusable component accepts utility overrides, and forward native form attributes.
The transitional input utility is `control-input`: `text-input` would collide with a Tailwind
color utility. Shared native control utilities live in `controls.css`.

Components own ordinary presentation through semantic utilities. The `specialist` layer follows
utilities for shared docking and message layouts, generated content, measured media, and coordinated
motion. There is no legacy layer. Keep these rules with their documented owners; avoid global
presentation overrides. Tailwind Preflight is omitted while the existing normalization remains authoritative.

The shadcn-vue foundation uses Reka UI, `components.json`, and `shadcn-nuxt` with the `Ui` prefix.
Dependencies use the normal `bun add` command; generated primitives live in `app/components/ui`.
Components and the shared `cn` helper use Nuxt auto-imports.
Review generated imports, native form behavior and styling before migrating callers; preserve
`theme.css` and the existing appearance authority when the CLI proposes CSS changes.
Keep domain wrappers in `app/components`, and use primitives for shared interaction behavior.

The frontend uses shared button, field, selection, slider, dialog, menu, tooltip, tab, badge, and
alert primitives from `app/components/ui`. `AppField`/`AppInput` retain labels, descriptions, errors and the exposed focus API;
`AppSwitch`/`AppSegmented` retain controlled models and change/reselect semantics. Keep searchable
`data-setting` anchors outside radio groups so revealing a result focuses its row. File and color
inputs retain native controls; message size and spacing use `UiSlider`. Convert a template ref to the component's
native `$el` when its owner needs DOM identity, selection, measurement or validity methods.
The transitional input utility remains for the native select and file input; standard, icon
and text button styling lives in the shared primitive rather than parallel global utilities.

See [Frontend components and styling](docs/frontend.md) for the component inventory, retained
specialist controls, layout rules, and browser review targets.

See [Message search](docs/search.md) for filters, paging, text matching, and history indexing.

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

Message deletion needs Ergo's history with individual deletion turned on. Own-message deletion
already depends on it:

```yaml
history:
  retention:
    allow-individual-delete: true
```

#### Server owner deletions

Server owners delete other people's messages through one operator login that the backend holds
on the associated server. IRCord server owners are not Ergo operators themselves. The backend
checks the owner list on every request, so removing an owner takes effect at once. Without this
login, server owners delete like everyone else: their own messages, and any message in channels
where they are an operator.

1. Generate a password hash with `ergo genpasswd`.
2. Add an operator class and an operator entry to Ergo's configuration:

   ```yaml
   oper-classes:
     "ircord-moderation":
       title: IRCord moderation
       capabilities:
         - "history" # delete any channel message
         - "nofakelag" # delete many messages without flood delays

   opers:
     ircord-moderation:
       class: "ircord-moderation"
       hidden: true # not shown as an operator in WHOIS
       password: "<hash from ergo genpasswd>"
   ```

3. Store the plain password in a file that only the backend can read.
4. Set both variables on the backend and restart it:

   ```sh
   IRCORD_ERGO_MODERATION_OPERATOR=ircord-moderation
   IRCORD_ERGO_MODERATION_PASSWORD_FILE=/private/path/ergo-moderation-password
   ```

With an associated Ergo server, setting only one of the two variables fails startup. The backend
logs at startup whether server-owner deletion is on, and why when it is off. On first use the
backend creates its own Ergo account through the private API, connects with SASL, and sends
`OPER`. The account name and its password are derived from the operator password, so no extra
secret is stored. The class has only the `history` and `nofakelag` capabilities. After a failed
login, the backend waits 30 seconds before it tries again.

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

Server owners open **Administration** from the shield beside Settings.
Overview shows backend health, version, uptime, account totals, associated IRC server statistics, and recent administration actions.
Database shows SQLite configuration, allocated and reusable space, file sizes, and connection-pool usage and waits.
Its optional ten-second refresh runs only while the Database view is open and visible.
Connection waits measure waiting for a pooled connection, not SQLite lock time or query latency.
The view does not run maintenance, inspect private records, or count messages on each refresh.

Users lists every account and lets owners add or remove other owners.
Owners use their signed-in session; ownership is checked on every request, so revocation takes effect immediately.
Channel founders and operators are not server owners.
Every grant and removal records the actor, target, time, optional reason, and outcome.
Server owners' message deletions are recorded the same way; see [Deleting other people's messages](#deleting-other-peoples-messages).

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

Two independent workflows run for the same tag. Each runs the shared checks (`make test`, `make check`, frontend lint, types and build) and publishes nothing unless they pass:

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
2. Run `make test check frontend-check`. Push that commit to `main`, then tag it: `git tag v0.1.0` and `git push origin v0.1.0`. If the tag's checks fail, nothing is published: fix the problem on `main` and release the next patch version. Don't move the tag, because Flux follows stable tags.

That section becomes the GitHub Release notes and the in-client notes.

### In-client release notes

The frontend image carries the release notes of its own tag and every earlier release, shown in the update banner's **What’s new** dialog and under Settings → About. Before the image builds, `frontend/scripts/release-changelog.mjs` takes each release's section of `CHANGELOG.md` at the tagged commit. Releases from before the changelog existed keep their GitHub Release notes; for a tag with no section whose release the CLI workflow has not created yet, it takes the notes from GitHub's release-notes generator. Newer tags, drafts and (for stable builds) prereleases are left out, also when they have a changelog section. Notes edited after an image was built appear from the next release's image on. Builds without a tag (local and development builds) show no notes.

### Collaborators

`COLLABORATORS.md` at the repository root lists the people who build IRCord, one `- Name` line each, and is the one tracked Markdown file that goes into the frontend image: the build reads it and Settings → About shows the names. Add a line there to be credited; the next build picks it up.
