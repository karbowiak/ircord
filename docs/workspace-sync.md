# Workspace synchronization

The backend maintains one account view shared by its browser connections.
SQLite retains message history, search indexes, account state, and durable events.

## Contract

Each account workspace holds networks, conversations, members, unread counts, and its committed event cursor.
Startup restores that view from SQLite and recalculates unread counts.
Incoming IRC events and authenticated commands update the view as they commit.
Opening a browser does not create another upstream IRC connection or duplicate the account view.

`GET /api/v1/bootstrap` returns a complete account snapshot and its cursor.
The snapshot reads account maps under their lock and makes no history queries.
The client loads recent message pages separately for the selected conversation.
Other conversations load history when opened.

Open `/api/v1/events?sync=1` to request synchronization support.
`session.ready` advertises that support and refreshes application capabilities and the emote catalog version.
Send `session.resume` with the last applied `after` cursor and `sync: true`.
The server attaches a bounded live queue while capturing replay under the account lock.
It sends replay, then `session.synced` with the completed cursor, then queued live events.
An empty replay still has a completion marker.
Older clients retain their existing handshake when they omit these options.

The client shows `Catching up` until it applies the completion marker.
Replay updates state without playing incoming-message alerts or automatically reading conversations.
A short notice reports new messages received during catch-up.
The notice counts retained message events for this device; account read markers remain independent.

Message pages carry the event cursor captured with their read.
The client tags socket updates with their sequence and history messages with their page cursor.
A delayed page cannot replace a newer edit, reaction, or deletion.
Duplicate message IDs merge, and messages retain timestamp, position, and ID ordering.
A history page's cursor never advances the account's applied event cursor.
Windows opened around older messages retain their separate position.

The client saves a versioned, account-bound device checkpoint containing state, recent messages, and the applied cursor.
It flushes batched member updates before saving.
Pending optimistic reads defer checkpoint writes until they finish or roll back.
HTTP commands leave canonical workspace updates to the event stream.
A cached startup renders that checkpoint and resumes directly, without fetching another bootstrap snapshot.
Page hiding and mobile suspension trigger a checkpoint save.
Sign-out and session expiry remove the account's device checkpoint.

## Defaults

- SQLite retains durable replay for 30 days.
- Resume accepts at most 4,096 retained events; larger gaps require a new snapshot.
- Each browser has a 64-event live queue and an account permits up to 16 subscribers.
- The backend message cache holds up to 500 messages per conversation and 5,000 per account.
- The device checkpoint holds up to 100 messages from each of five recent conversations.
- Normal history pages contain 40 messages; older messages remain in SQLite.

Run the synchronization checks from the repository root:

```sh
make test-sync
```

This command builds Nuxt and runs Chromium against disposable Go and SQLite fixtures.
It uses three channels, 46 members, 600 retained messages, and 25 replayed profile changes.
It measures ten starts and ten reconnects after one warm-up run.
Startup includes session lookup, account state, the rendered active conversation, and socket readiness.
The three history requests run in parallel after startup.
Cached mobile starts use a 390 by 844 viewport and verify that bootstrap is not requested.
Results and a mobile screenshot are written under ignored `.data/sync-perf/`.

The browser check also delays history while edits, reactions, deletion, and new messages arrive.
It verifies recovery from an expired replay cursor through one replacement snapshot and a new socket.
The fixture uses retained account state with its upstream network disconnected.
It measures browser synchronization independently of IRC registration, upstream history recovery, or shared infrastructure.
These local timings exclude sign-in, media downloads, and production network latency.
The browser asset cache stays warm, device checkpoints are cleared between normal starts, and service workers are disabled for these measurements.

### Local comparison, November 9, 2026

Values are medians from ten runs of each implementation, measured before and after the synchronization changes.

| Measurement | Before | After |
| --- | ---: | ---: |
| Workspace rendered | 82.4 ms | 78.4 ms |
| Active conversation messages rendered | 82.4 ms | 86.9 ms |
| Workspace, messages, and socket ready | 125.5 ms | 117.3 ms |
| Three parallel history requests | 3.67 ms | 3.01 ms |
| Reconnect and apply 25 changes | 41.9 ms | 38.8 ms |

Cached mobile startup reached full readiness in 111.8 ms without a bootstrap request.
The timings show similar local performance, with a small improvement in total readiness and reconnect.
The synchronization guarantees and consistent device checkpoint are the primary changes.
The readiness check now waits for replay completion.

## Offline history and files

The device checkpoint remains a small, fast startup copy in `localStorage`.
A separate account-scoped IndexedDB archive holds seven days of retained messages, account state, search tokens, and downloaded files.
A dedicated worker performs database writes, retention, and local search without blocking the interface.
This archive uses authorized records rather than copying the server's SQLite database.

`GET /api/v1/offline/snapshot` exports up to 500 messages per page.
The first page includes account state; each page includes the cursor captured with its messages.
The client buffers live events while downloading pages.
It preserves newer message revisions, then applies buffered events and stores state and cursor together.
Incomplete exports resume through a new export.
Later connections use durable replay from the archive cursor.
Expired cursors and sequence gaps require a fresh export.

The client refreshes retention each hour while the page is visible and connected.
Removed conversations and networks lose their local messages and files.
Edits and deletions update the archive through durable events.
Fresh exports revalidate messages kept beyond seven days through `POST /api/v1/offline/pinned`.
Missing or inaccessible messages are removed.
An offline device receives these removals when it reconnects.

Message search uses the server while connected.
While offline, reconnecting, or unable to reach the server, search uses the worker's text index and the same message filters.
Local results support pagination and open the saved conversation around the selected message.
The search notice identifies saved results and incomplete archives.
The service worker caches the app shell and hashed assets so an installed or previously opened app can reload offline.

Images attached to messages, GIFs, and direct image links download automatically in the background.
`Keep offline` saves an attachment and its message beyond normal retention.
Other attachment types download when kept offline.
The authenticated `/api/v1/offline/media` endpoint checks message access before returning bytes.
Each file is limited to 32 MiB; automatic storage has a 256 MiB target and evicts older unpinned downloads first.
Pinned files remain subject to browser storage availability.
External post widgets still require a connection.

Tabs share the account archive and announce file changes and sign-out through a broadcast channel.
Snapshot generations prevent one tab's delayed export from replacing another export.
Sign-out and session expiry stop background work and delete the account database and device checkpoint.
Worker jobs retain their account identity across asynchronous work.
Storage failures leave online messaging available.

The sidebar indicator uses existing client latency samples, socket state, and upstream network state.
It adds no polling and opens the existing server details.

Run the offline browser check from the repository root:

```sh
make test-offline
```

The check uses a disposable account, 601 recent messages, older history, and a generated image.
It verifies offline reload, Unicode search pagination, full-size photo viewing, retained pins, multiple tabs, replay recovery, deletions, and logout cleanup.
Worker checks also cover delayed pages, sequence gaps, superseded exports, and shared-file references.
Desktop and mobile screenshots are written under ignored `.data/offline-review/`.

## Failure modes

- A sequence gap, malformed event, or expired cursor closes the old stream and requests a replacement snapshot.
- A replacement snapshot clears cached message windows before reconnecting from its cursor.
- A fully expired replay log rejects an old cursor even when no events remain.
- A slow consumer must resynchronize when its bounded queue fills.
- An invalid or older device checkpoint falls back to a normal bootstrap.
- Unavailable device storage leaves the live workspace functional without saved resume state.
- If the backend loses its IRC connection, recovery still depends on upstream history retention and capabilities.

## Source

- [Backend ownership and storage](backend.md)
- [Retained-history search](search.md)
- [Contributor checks](../README.md)
