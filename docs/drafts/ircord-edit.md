# Message editing: `+ircord/edit`

This draft documents IRCord message editing over Internet Relay Chat (IRC).
It describes the implemented client tag, sender checks, history behavior, and compatibility limits.

Status: Implemented IRCord extension. This draft is not an IRCv3 standard.

## Contract

### Transport

An edit is an ordinary `PRIVMSG` containing the complete replacement text.
The client-only tag `+ircord/edit` names the original message's server-assigned `msgid`.
Every later edit names that same original message, not the previous edit.
Each edit receives its own `msgid` from the server.

For example, the client sends this correction to `original-1`:

```irc
@+ircord/edit=original-1 PRIVMSG #example :hello world
```

The server delivers a message such as:

```irc
@account=alice;msgid=edit-1;time=2026-11-09T12:00:01.000Z;+ircord/edit=original-1 :Alice!alice@example.test PRIVMSG #example :hello world
```

Here, `original-1` identifies the message containing the typo.
The new `edit-1` identifies the correction as a separate IRC message.
The `account`, `msgid`, and `time` tags come from the server.
Receivers never use a client-supplied account claim to authorize an edit.

IRCord sends action corrections with the normal Client-to-Client Protocol (CTCP) `ACTION` wrapper.
Long or multiline corrections use an IRCv3 `draft/multiline` batch within the negotiated limits.
The opening `BATCH` carries `+ircord/edit`; its `PRIVMSG` lines carry their batch reference.
The receiver reconstructs the batch before it applies the correction.

### Availability

IRCord offers sending edits when all these conditions hold:

- The connection is ready.
- The connection negotiated `message-tags`, `echo-message`, and `account-tag`.
- The server's `CLIENTTAGDENY` policy permits the `ircord/edit` tag.

There is no `ircord/edit` capability to request through IRC `CAP` negotiation.
The Server panel lists message editing under **IRCord extensions**, separately from negotiated capabilities.
Its availability comes from the backend's `network.features.edit` value and the current connection state.
Availability does not mean every other client understands the tag.

The original message must also meet these conditions before IRCord sends an edit:

- It belongs to the caller and has a server-assigned `msgid`.
- It is a text or action message that has not been deleted.
- It contains neither a GIF attachment nor an uploaded file.
- Its recorded sender account matches the account currently logged into IRC.
- For a channel message, the caller remains joined and can speak in the channel.

The correction must differ from the current text.
IRCord rejects empty text, carriage returns, and null characters.

### Sender checks

A receiver resolves the original `msgid` within the same network and conversation.
The original must be an editable text or action message.
The correction must have its own server-assigned `msgid`.

The correction's sender identity must match the original sender identity.
Both messages must name the same nonempty IRC account through server-attributed account tags.
IRCord compares account names without case sensitivity.
An absent account or `account=*` does not authorize an edit.
Reusing the original nickname alone does not authorize an edit.
Channel privileges do not grant permission to edit another sender's message.

### Applying a correction

An accepted correction replaces the displayed text of the original message.
The original keeps its message ID, creation time, replies, and reactions.
Quoted reply excerpts update to the corrected text.
IRCord shows an edited marker and keeps the original text and retained corrections.

IRCord orders retained corrections by their message timestamps.
The correction with the latest timestamp supplies the displayed text and edit time.
Server time takes precedence when present; otherwise, IRCord uses the received event time.
Duplicate retained corrections with the same `msgid` do not add another version.
Equal timestamps retain insertion order; there is no additional tie-break rule.

The sender's stored message changes after the server echoes an accepted correction.
With `labeled-response`, IRCord adds a request label and waits for confirmation or an error.
Without it, the request returns an expected result while the echo applies the stored change.

### History and fallback

A client that does not implement `+ircord/edit` shows the correction as a new message.
A client without `message-tags` receives ordinary corrected text without the tag.
The server does not need a special editing command.

History reconstruction requires the server to retain and replay the client tag and message identifiers.
IRCord applies the same sender checks to live messages and history.
An edit received before its original appears as a separate message until the original loads.
After the original loads, an authorized correction merges into it.
A rejected correction remains a separate message and does not change the original.

### Deletion

Deleting messages uses the separate IRCv3 `draft/message-redaction` extension.
When IRCord deletes an edited message, it also requests deletion of its retained correction messages.
Deletion of correction messages is best effort and remains subject to server permissions.
An accepted correction cannot restore an original that IRCord has already marked as deleted.

A permitted redaction of one retained correction removes that version.
IRCord then displays the latest remaining version, or the original text when no corrections remain.

## Defaults

| Setting                          | Current value                                   |
| -------------------------------- | ----------------------------------------------- |
| Edit time limit                  | None                                            |
| Retained corrections per message | 20, keeping the latest by timestamp             |
| Outgoing text limit              | 4,000 Unicode characters and 4,096 UTF-8 bytes  |
| Network message limits           | Negotiated line and multiline limits also apply |
| Labeled confirmation timeout     | 10 seconds                                      |

## Failure modes

- A blocked tag or missing required capability disables outgoing editing.
- An original without a recorded IRC account cannot be edited.
- A missing original leaves the correction visible as a separate message.
- A mismatched sender or account leaves the original unchanged.
- A missing correction `msgid` prevents it from merging into the original.
- A history server that drops tags cannot reconstruct edits for a fresh client.
- A refused correction deletion can leave corrected text in server history or other clients.
- Corrections beyond the retained limit are unavailable for local version history and correction-deletion requests.
- A confirmation timeout leaves acceptance unknown; a later accepted echo can still apply the correction.

## Source

- [Protocol draft index](README.md)
- [User-facing editing overview](../../README.md#editing-messages)
- [Sending, authorization, version history, and replay](../../backend/internal/irc/edit.go)
- [Feature availability and multiline transport](../../backend/internal/irc/features.go)
- [IRC message processing and deletion](../../backend/internal/irc/)
- [Editing regression tests](../../backend/internal/irc/edit_test.go)
- [Ergo client and history integration test](../../backend/internal/irc/edit_ergo_test.go)
