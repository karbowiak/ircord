# Message search

Search returns messages available to the signed-in account. Retained search is the default.
An optional upstream search requests a bounded set from one conversation.

## Contract

`GET /api/v1/search` requires authentication. The response contains `items` and a nullable `nextCursor`.
Each item contains `message` and `conversation`.

| Parameter | Meaning |
| --- | --- |
| `q` | Literal text phrase, with a word prefix on the last term; at most 200 characters. |
| `from` | Repeatable member IDs. |
| `mentions` | Repeatable member IDs, resolved to their current nicknames. |
| `conversationId` | Repeatable conversation IDs. |
| `has` | Repeatable `link`, `image`, `gif`, `video`, or `file`. |
| `before` | RFC 3339 instant; messages must be older than this instant. |
| `after` | RFC 3339 instant; messages can be at or after this instant. |
| `author` | `user` or `bot`, based on current member roles. |
| `cursor` | Opaque continuation returned by the previous page. |
| `limit` | Page size, from 1 to 100; default 50. |
| `source` | `retained` or `upstream`. Omit this field to use the legacy retained response. |

Values within a filter combine with OR. Different filters combine with AND.
Each list accepts at most 50 values. Without filters, enter at least two characters.
With filters, text is optional, and one-character text is allowed.

Retained pages use descending creation time, then descending message ID. Continue with the same filters and the returned cursor.
Deleted messages are excluded. Ignored senders remain searchable, consistent with retained timeline history and previous search behavior.

New retained cursors bind the owner, source, query, and filters with a bounded fingerprint.
They do not contain the search text. A mismatch returns 400.
The fingerprint detects accidental reuse; it does not authorize access or protect cursor integrity.
Legacy cursors remain accepted and have no scope check.
Retained cursor pages cover indexed retained data, which may omit older messages during backfill.

An explicit source adds a `source` object to the response.
It has `requested`, `actual`, `coverage`, and `completeness` fields.
Coverage is `retained-indexed` or `upstream-bounded`; completeness is always `unknown`.
An upstream response has no continuation cursor. A null cursor does not prove complete upstream history.
An upstream request can return retained results with a `reason`:

| Reason | Meaning |
| --- | --- |
| `unsupported-scope` | The request does not identify one supported conversation. |
| `unsupported-filters` | The requested filter combination is not supported upstream. |
| `capability-unavailable` | The current connection cannot use upstream search. |
| `upstream-unavailable` | The upstream request failed, timed out, or returned an unusable response. |
| `continuation` | A legacy retained cursor forces the request to stay on retained search. |

A fallback cursor retains the fallback reason. Later pages stay on retained search even if capabilities change.
Unknown source values return 422. Caller cancellation, authorization failures, and local storage failures do not become fallback responses.

Each explicit-source item has a `context` object. `available` is true when the retained message can be opened.
`identity` is `retained` or `ephemeral`. An ephemeral ID names only that HTTP result.
Its `reason` is `not-retained` or `missing-msgid`; clients must not open retained context for that hit.
An ephemeral item can contain `senderNick` as display text. It is not an account identity.
Retained messages, including edits and tombstones, take precedence over upstream copies.

An inaccessible conversation returns 404, including when another conversation in the list is accessible.
Inaccessible member IDs contribute no matches. A list containing only inaccessible members returns no results.

Mentions match whole nicknames, with an optional leading `@`. Nickname changes are not tracked historically.
Bot classification uses current roles, rather than roles at posting time.

Content filters cover URLs outside fenced code and IRC monospace, uploaded attachments, and GIF-picker posts.
`link` includes all supported web URLs. Images exclude GIFs. Videos include YouTube and Vimeo links.
`file` includes attachments whose content type is neither image nor video.

## Defaults

Text matching uses SQLite FTS5 with `unicode61 remove_diacritics 2`.
Search syntax characters are token separators, not operators. Quotes, asterisks, hyphens, and `NEAR` cannot change query syntax.
`wor` matches `world`; `orl` does not. Consecutive words form a phrase, with a prefix on its last word.

Search content flags are stored on message rows, so filter queries do not decode JSON per candidate.
Database triggers synchronize text edits, tombstones, and physical deletions with the contentless FTS index.
New messages are indexed in their write transaction.

The existing bounded `message-links` backfill also builds search indexes under extraction version 4.
Checkpoints survive restarts. Results can omit older history while the backfill runs.

## Upstream search

IRCord requests Soju's `soju.im/search` capability. An offer alone does not enable it.
The current ready connection must acknowledge `soju.im/search`, `batch`, `server-time`, `message-tags`, and `labeled-response`.
Capability removal and reconnects can make search unavailable. A server may still reject a negotiated request.

The upstream request requires one explicitly authorized conversation and search text.
Other filters use retained fallback. The server receives a target, text, and a limit of at most 100.
Only one upstream search is admitted per connection. Dispatch and response waits each have a ten-second limit.
Raw `SEARCH` commands are managed by IRCord and are rejected. Other raw commands retain their existing behavior.

Soju's search extension is a work in progress. Its storage drivers can use different text matching rules.
IRCord applies its retained text predicate to returned candidates, but the server can omit valid retained-style matches.
The returned set covers only the server's retained data and has unknown completeness.
Soju's timestamp bounds do not provide a stable continuation across equal timestamps.

Ergo v2.19.1 implements chat history, but its command and capability registries do not offer Soju search.
Chat history retrieves messages by position or time; it is not a text search.
Local retained history and local read markers remain available without upstream history or read-marker capabilities.

## Failure modes

Invalid filter values, reversed date bounds, excessive list lengths, and invalid page sizes return 422.
Malformed cursors return 400. Storage failures return 503.

## Source

- `backend/internal/irc/search.go` resolves account-visible filters and implements the memory fallback.
- `backend/internal/storage/search.go` builds and runs indexed queries.
- `backend/internal/storage/search_migration.go` installs the indexes and synchronization triggers.
- `backend/internal/storage/search_test.go` verifies query plans, paging, edits, deletions, and resumable indexing.
- [Soju search extension](https://github.com/emersion/soju/blob/v0.10.1/doc/ext/search.md) and [Soju v0.11.1 source archive](https://proxy.golang.org/codeberg.org/emersion/soju/@v/v0.11.1.zip).
- [Ergo v2.19.1 command registry](https://github.com/ergochat/ergo/blob/v2.19.1/irc/commands.go) and [capability registry](https://github.com/ergochat/ergo/blob/v2.19.1/irc/caps/defs.go).
- [IRCv3 labeled responses](https://ircv3.net/specs/extensions/labeled-response) and [chat history](https://ircv3.net/specs/extensions/chathistory).

Query-plan regression checks require FTS candidate lookups followed by `SEARCH m USING INTEGER PRIMARY KEY` for text.
Filter-only checks use `messages_search_sender`, `messages_search_page`, or `messages_page`, depending on the filter.
