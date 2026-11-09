# Message search

Search returns the signed-in account's stored messages, with filters and cursor pagination.

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

Values within a filter combine with OR. Different filters combine with AND.
Each list accepts at most 50 values. Without filters, enter at least two characters.
With filters, text is optional, and one-character text is allowed.

Pages use descending creation time, then descending message ID. Continue with the same filters and the returned cursor.
Deleted messages are excluded. Ignored senders remain searchable, consistent with retained timeline history and previous search behavior.

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

## Failure modes

Invalid filter values, reversed date bounds, excessive list lengths, and invalid page sizes return 422.
Malformed cursors return 400. Storage failures return 503.

## Source

- `backend/internal/irc/search.go` resolves account-visible filters and implements the memory fallback.
- `backend/internal/storage/search.go` builds and runs indexed queries.
- `backend/internal/storage/search_migration.go` installs the indexes and synchronization triggers.
- `backend/internal/storage/search_test.go` verifies query plans, paging, edits, deletions, and resumable indexing.

Query-plan regression checks require FTS candidate lookups followed by `SEARCH m USING INTEGER PRIMARY KEY` for text.
Filter-only checks use `messages_search_sender`, `messages_search_page`, or `messages_page`, depending on the filter.
