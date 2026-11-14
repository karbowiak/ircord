# IRCord Go backend

The backend owns account identity, IRC connections, durable chat state, and private live updates.
Huma and Chi expose authenticated Hypertext Transfer Protocol (HTTP) commands.
[`girc`](https://github.com/lrstanley/girc) handles Internet Relay Chat (IRC) framing, capabilities, authentication, and connection state.

## Contract

### Package ownership

| Package                                                   | Responsibility                                                                                |
| --------------------------------------------------------- | --------------------------------------------------------------------------------------------- |
| `cmd/ircord`, `internal/cli`                              | Backend entry point, startup commands, and offline help                                       |
| `internal/app`                                            | Service construction, configuration, background workers, and shutdown                         |
| `internal/api`                                            | HTTP routes, request validation, identity boundaries, error responses, and WebSocket delivery |
| `internal/auth`                                           | Sign-up, password verification, sessions, devices, and coordinated credential recovery        |
| `internal/irc`                                            | Account workspaces, upstream connections, commands, history recovery, and protocol events     |
| `internal/storage`                                        | SQLite migrations, encrypted credentials, atomic state changes, replay, and retained history  |
| `internal/model`                                          | Shared domain values and their wire representation                                            |
| `internal/msgindex`                                       | Content extraction and search indexing rules                                                  |
| `internal/activity`, `internal/notify`, `internal/push`   | Device routing, notification policy, and Web Push delivery                                    |
| `internal/filehost`, `internal/emotes`                    | Upload storage, download capabilities, avatars, and the emote catalog                         |
| `internal/linkmedia`, `internal/cdn`                       | Source media resolution, isolated widgets, and optional stored media copies                   |
| `internal/mediafetch`, `internal/publicnet`               | Public media transport and the shared public HTTP address policy                              |
| `internal/admin`, `internal/ergo`                         | Operator authentication and the associated IRC server's administration interface              |

In `internal/api`, domain route files own their request types and handlers.
`server.go` constructs the router; `middleware.go` enforces identity and origin checks.
`events.go` owns replay and workspace event delivery.
`socket.go` shares session watching, bounded writes, and heartbeat checks between the event and trace streams.
`errors.go` maps command failures; `responses.go` defines common response shapes.

In `internal/app`, `config.go` parses configuration before storage or workers start.
`server.go` constructs services and owns their lifetimes; `http.go` handles HTTP startup and graceful shutdown.

`internal/linkmedia` retains the legacy Iframely payload shape to reject cached embeds.
The resolver does not use an Iframely service.

### IRC feature ownership

| Files in `internal/irc`                                                    | Responsibility                                                                              |
| -------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| `receive.go`                                                               | Ordered dispatch of incoming protocol events to domain handlers                             |
| `chat.go`, `message.go`                                                    | Sending, receiving, validating, and reading retained chat messages                          |
| `reactions.go`                                                             | Outgoing reaction commands and incoming reaction state                                      |
| `members.go`, `presence.go`                                                | Member identity, roster synchronization, and membership events                              |
| `metadata.go`, `metadata_sync.go`                                          | Incoming metadata, synchronization, batching, and retry rules                               |
| `user.go`, `network_profile.go`, `profile_publish.go`                       | Per-network profiles, nicknames, away state, real names, and profile publication            |
| `conversations.go`, `channel_settings.go`, `mode_lists.go`                 | Conversation lifecycle, topics, channel settings, and moderation lists                      |
| `channel_create.go`, `party_conversion.go`                                | Confirmed channel creation and explicit conversion of existing groups                       |
| `channel_exchange.go`                                                      | Bounded labeled replies shared by channel creation and legacy group conversion              |
| `channel_directory.go`                                                     | Bounded upstream channel discovery and its cache                                            |
| `people.go`, `people_direct.go`                                            | Scoped people lookup, distinct nickname and account identities, and direct-message targets   |
| `manager.go`, `networks.go`, `network_passwords.go`                        | Connection admission, saved configuration, and network credentials                          |
| `connection.go`, `transport.go`, `network_check.go`                        | Connection lifetime, outgoing queues, dialing, and reachability checks                      |
| `commands.go`, `batches.go`                                                | Raw commands, command failures, labels, and multiline batches                               |
| `history.go`, `history_pages.go`, `history_scheduler.go`                    | Upstream recovery, bounded catch-up scheduling, and history reads                           |
| `offline.go`                                                               | Account-scoped archive export and retained-file validation                                  |
| `workspace.go`, `events.go`, `snapshot.go`                                 | Account state, durable event publication, replay, and complete snapshots                    |
| `edit.go`, `redact.go`                                                     | Message editing, deletion authorization, and removal of derived content                     |
| `moderation.go`, `purge.go`                                                | Server owners' moderation login, and deleting one member's recent messages                  |
| `parties.go`, `party_controller.go`, `party_access.go`, `party_history.go` | Legacy group membership, control, admission checks, retained history, and recovery           |

Keep incoming and outgoing behavior for a feature together.
Add protocol dispatch in `receive.go`; put validation and state changes in the feature's file.
Established feature files also own typing, invitations, saved messages, links, read markers, notifications, diagnostics, and service commands.
See [Existing group conversations](parties.md) for conversion and retained compatibility requirements.

Channel creation uses the requesting account's existing connection and requires labeled replies for confirmed results.
Availability checks are hints, not name reservations. Creation verifies control before applying native modes and topics.
Supported ChanServ registration belongs to the requesting IRC account; unsupported registration returns an explicit warning.
Partial results preserve the actual conversation and never delete channels as an automatic rollback.
Group conversion is an explicit owner action. It stops legacy control after confirmed native ownership transfer while preserving retained data.
New group creation is retired; compatibility code remains for existing groups and their conversion.
Ordinary channel operations and account lookup use the associated-server client independently of that compatibility code.

`Manager.mu` protects workspace ownership and connection admission.
`workspace.mu` protects account state; `workspace.configMu` serializes network configuration changes.
`connection.mu` protects connection protocol state.
When workspace and connection locks are nested, acquire the workspace lock first.
Release those locks before waiting for connection workers to stop.
Event commits deliberately run under the workspace lock to preserve mutation and publication order.

Targeted network, member, and profile reads do not build complete snapshots or recount conversation unread state.
Request contexts cancel history and search reads, including waits for the SQLite connection.
Durable mutations retain their independent lifetimes.

Database diagnostics use read-only metadata queries and connection-pool counters with a one-second metadata deadline.
They retain pool counters when metadata cannot be read and never return database paths or SQL text.
Allocated space includes reusable pages; the write-ahead log size describes file allocation, not pending writes.
Pool wait counts and durations are cumulative since process startup. They do not measure SQLite lock contention or query execution.
The separate database endpoint avoids the account and message count scans used by the administration overview.

Snapshots read the shared account maps and committed cursor without querying history.
Unread counts refresh when messages, reads, policy, or recovered history change, and when an account loads from storage.
Message pages capture their account cursor under the workspace lock.
Clients preserve message updates newer than that page's cursor.

Clients save account state, recent message windows, and the applied cursor in one device checkpoint.
Resume replays changes from that cursor and explicitly finishes before live delivery.
Expired or oversized replay requires a fresh snapshot.
See [Workspace synchronization](workspace-sync.md) for the protocol, limits, and browser measurements.

The independently buildable operator client lives in [`../cli/`](../cli/).
It uses administration HTTP endpoints and imports no backend packages.

### Identity and connections

Accounts sign in with a username and password.
Passwords use Argon2id hashes; session cookies contain random tokens whose hashes are stored in SQLite.
Cookies are HttpOnly and SameSite=Lax, and use Secure when `IRCORD_PUBLIC_URL` uses HTTPS.
Password changes and account deletion coordinate with the associated Ergo server when configured.

Browsers attach to the account's existing upstream connections.
Opening another tab does not create another IRC client.
HTTP commands, retained history, replay, and live updates stay scoped to the authenticated account.
Private WebSockets close when their session ends; heartbeats also recheck session validity and refresh device presence.

Production mutations use authenticated HTTP endpoints.
WebSockets carry live updates and transient typing state.
`DevelopmentSessions` provides a separate identity adapter for disposable fixtures; the application uses `internal/auth`.

Profiles belong to an account and a configured IRC network.
`GET` and `PATCH /api/v1/networks/{id}/profile` read and update that network's profile after checking account ownership.
Display name, avatar, description and name color publish only through the selected network's connection.
`Network.profile` carries the effective self profile in bootstrap and `network.updated` events, including device checkpoints.
The legacy account profile does not publish changes to connected networks.
Own message identifiers stay stable; presentation resolves the profile using the message's network.

Both profile PATCH routes accept partial updates. Omitted fields keep their current values under the workspace lock.
An empty `bio` clears it; `bio: null` is invalid. Empty or null `displayName` clears the explicit name.
Empty or null `nameColor` returns to automatic color. A null `avatarDataUrl` clears the image.
An omitted or null `avatarUrl` keeps the image; an empty `avatarUrl` clears it.
If both avatar fields are supplied, `avatarDataUrl` takes precedence.
Concurrent edits to different fields merge. Concurrent edits to the same field use the last applied value.
An upload that finishes after its account or network identity is replaced does not update the profile.
The completed upload can remain as an unreferenced file; this patch does not change file cleanup.

### HTTP operation errors

Problem `type` values below are stable identifiers for recognized operation outcomes. Older clients can continue using the HTTP status and problem detail.

| Problem type | Status | Meaning |
| --- | ---: | --- |
| `urn:ircord:problem:operation-id-conflict` | 409 | A `clientId` already belongs to a different payload or conversation. |
| `urn:ircord:problem:delivery-uncertain` | 409 | The upstream result is unknown and requires reconciliation. |
| `urn:ircord:problem:operation-busy` | 409 | The send queue cannot accept the operation now. |
| `urn:ircord:problem:connection-unavailable` | 409 | The network connection is unavailable. |
| `urn:ircord:problem:resource-conflict` | 409 | The requested resource operation conflicts with its current state. |
| No stable type | 503 | The request context was canceled. |
| No stable type | 500 | A request deadline expired; public detail remains generic. |

Reconcile uncertain sends with `GET /api/v1/commands/{clientId}` and retained history before retrying. Never resend an uncertain command automatically.
A 503 cancellation does not prove that a mutation failed. Keep its operation identity and reconcile its state before submitting a different operation.
Reuse a `clientId` only for an identical retry. Use a new ID when the payload or conversation changes.

### People lookup

`GET /api/v1/people` requires an owned network and a purpose: `mention`, `command`, `mask`, `history`, `dm`, `services`, or `party`.
Mention, command and mask lookups also require a conversation on that network.
Results distinguish IRC nicknames, registered services accounts and optional local account identifiers.
An IRC ident is not a services account, and a display name is not an identity.

Known results come from the caller's workspace and retained conversation authors, not a complete server roster.
Services and party lookups can also use the associated server's registered account names, including accounts without a local IRCord account.
The public response excludes operator account details, hosts and idents.
Partial or unavailable sources have explicit status values; callers must not present these as a successful search with no matches.
Requests and source caches are bounded, and request cancellation reaches upstream lookups.
Lookup results never replace authorization checks on subsequent actions.

Direct-message creation accepts either `nickname` or `memberId`, alongside the selected `networkId`.
Explicit nicknames bypass member identifier lookup; retained member identifiers must belong to the selected network.
Neither target can be combined with a channel name.

### Device activity and notifications

An authenticated session identifies a device; tabs sharing its cookie share device settings.
Activity reports bind to that session, tab incarnation, current socket, and increasing report sequence.
Genuine focus or input selects the focal device. Renewals maintain a visible, focused reader without selecting it again.
The frontend accumulates input locally and sends active reports no more than once every ten seconds per current socket.
Passive reading renews every 25 seconds. A fresh socket can report immediately, and leaving sends one immediate inactivity release.
Private routing snapshots carry an ordered revision and opaque device identifiers.
These reports do not mark conversations read or replace manually selected IRC away state.

The focal device can sound for eligible conversations outside its viewed conversation while active, without an operating-system notification.
After inactivity, it receives sound-only alerts before the target delay, then operating-system alerts until the all-device delay.
Defaults are 30 seconds and five minutes, measured from the same inactivity origin.
Only after the all-device delay do other eligible devices receive alerts.
Losing a connection does not skip those deadlines; expired leases and bounded reconnect handling end stale activity.

Scheduled catch-up groups use each conversation's latest eligible unread candidate and count.
Delivery rechecks reads, deletion, ignores, quiet hours, channel rules, device settings, and authorization.
Candidate checkpoints and confirmed delivery receipts survive restart; retries retain their presentation identity.
Device preferences remain backend-owned, with revision checks and a one-time import of existing local choices.
Browser permission and registered push are capability facts, not permission that another device can grant.
Page and service-worker presentation share a deduplication ledger; unavailable browser storage limits cross-context deduplication.

### Durable storage

`internal/storage/store.go` owns the database lifetime, encryption, and common entity reads.
`events.go` owns atomic state/event commits and replay; `networks.go` owns saved connection configuration.
`history.go` owns message persistence, paging, cursors, and read state.
`logs.go` owns server-log reads and shared retention rules.
`outbox.go` validates attachment ownership within the send transaction; file links remain shareable bearer URLs.

One backend process owns each data directory, enforced by an operating-system file lock.
Sharing a directory between backend replicas is unsupported.
Back up the SQLite database, `master.key`, `vapid.json` when present, and configured upload storage together.

A state mutation and its durable event commit before publication.
Replay retains 30 days of account-private events and returns at most 4,096 events per resume.
Expired cursors require a new snapshot.
Slow consumers have bounded queues and must resynchronize when they fall behind.
Typing state is transient and never enters durable replay.

Accounts inactive for 90 days are removed with their credentials, history, settings, and IRCord-hosted files.
Authenticated HTTP requests and socket heartbeats count as account activity; incoming IRC traffic does not.

### Generated API

The API defaults to `http://127.0.0.1:8080`.
Scalar documentation is available at `/docs`; OpenAPI is available at `/openapi.json`.

After changing Go request types, response types, or routes, run `make api-generate`.
This command updates the OpenAPI artifact, frontend types, browser WebSocket validator, and standalone operator client.
`make api-check` detects stale generated artifacts.
Keep route extraction and internal refactors compatible with the existing contract.

### Network and media boundaries

Public media and push transports share the `internal/publicnet` address policy.
Each transport validates every resolved address and dials the validated address directly.
Media ports, redirect behavior, and timeouts remain owned by their transport.
IRC uses its own destination policy, including the explicit private-network option for trusted deployments and fixtures.

Link-media requests must reference a retained message accessible to the requesting account.
Provider results can be shared in the resolver cache; authorization and widget tokens remain account-private.
Widget HTML stays in an isolated document and never enters messages or events.

Offline exports use retained account history without fetching upstream messages.
File downloads must reference a visible message and use the public media transport or a verified local download capability.
Downloads share one transport, permit four concurrent requests, and limit each file to 32 MiB.

## Defaults

Use Go 1.27 or newer and the latest stable Go patch release for builds.
The development frontend uses Bun.

Run the normal checks from the repository root:

```sh
make test
make check
make build
```

Start the backend and frontend in separate terminals:

```sh
make backend
make frontend
```

`make backend` requires Air and loads the ignored `backend/.env` when present.
It defaults to `127.0.0.1:8080` and stores development state in `.data/live`.
Nuxt defaults to port 3000 and forwards authenticated requests, downloads, and WebSockets to the private backend.
Set `IRCORD_PUBLIC_URL` to the browser's public origin.

| Variable                                                    | Default or purpose                                                               |
| ----------------------------------------------------------- | -------------------------------------------------------------------------------- |
| `IRCORD_DATA_DIR`                                           | `.data` for direct startup; exclusive database, key, and local upload directory  |
| `IRCORD_LISTEN`                                             | `127.0.0.1:8080`                                                                 |
| `IRCORD_PUBLIC_URL`                                         | `http://127.0.0.1:8080`; public origin and download URL base                     |
| `IRCORD_ALLOWED_ORIGINS`                                    | Local frontend origins on port 3000; comma-separated exact origins               |
| `IRCORD_ALLOW_REGISTRATION`                                 | `true`; controls account creation                                                |
| `IRCORD_REGISTRATION_KEY`, `IRCORD_REGISTRATION_KEY_FILE`   | Optional private sign-up key or key file                                         |
| `IRCORD_REQUIRE_REGISTRATION_KEY`                           | Rejects startup when a required sign-up key is absent                            |
| `IRCORD_TRUSTED_PROXIES`                                    | Proxy ranges allowed to supply client addresses; include the frontend and proxy  |
| `IRCORD_ALLOW_PRIVATE_IRC`                                  | `false`; permits private destinations and plaintext only when explicitly enabled |
| `IRCORD_NETWORK_PASSWORDS`, `IRCORD_NETWORK_PASSWORDS_FILE` | Private shared server passwords, bound to exact TLS endpoints                    |
| `IRCORD_FILE_STORAGE`                                       | `local`, `s3`, or `disabled`                                                     |
| `IRCORD_WEB_PUSH`, `IRCORD_PUSH_SUBJECT`                    | Push enablement and the VAPID contact                                            |

Run optional service checks against disposable local fixtures:

```sh
make test-integration
make test-s3
make test-read-markers
make test-sync
make test-offline
```

Ergo uses the `ircord-ergo-tests` Compose project and loopback ports 16667 and 16697.
Garage uses the `ircord-s3-tests` project and loopback port 19000.
The read-marker check also builds Nuxt and opens disposable browser contexts.
Run `make ergo-down` and `make s3-down` when those fixtures are no longer needed.

## Failure modes

- A workspace storage failure stops durable delivery and causes unavailable responses. Restore storage and restart to reload committed state.
- A send with an uncertain outcome is never resent automatically. Inspect command status and retained history before retrying.
- Offline recovery depends on upstream history retention and negotiated capabilities. Servers without chat history cannot recover messages missed while disconnected.
- Upstream filehosts can lack deletion support. Account deletion removes IRCord metadata but cannot retract those shared upstream URLs.
- Account deletion waits for file storage. If an account's files are in storage that is unreachable or no longer configured, the hourly cleanup retries and logs each file's storage and object key. The account is signed out and hidden meanwhile.
- Push delivery is best effort with a bounded queue. Provider registration does not prove that a browser can still receive notifications.

## Source

- [Repository setup, configuration, checks, and releases](../README.md)
- [Contributor workflow](../AGENTS.md)
- [Writing style](STYLE.md)
- [Retained-history search](search.md)
- [Workspace synchronization](workspace-sync.md)
- [User-facing changes](../CHANGELOG.md)
