# IRCord protocol drafts

These references describe IRCord extensions to Internet Relay Chat (IRC).
They document the current implementation and its limits.

## Contract

| Draft                             | Purpose                                                      | Status                       |
| --------------------------------- | ------------------------------------------------------------ | ---------------------------- |
| [Message editing](ircord-edit.md) | Replace a message's text with the `+ircord/edit` client tag. | Implemented IRCord extension |

## Defaults

These drafts are IRCord conventions, not IRCv3 standards.
Each draft states its transport, capability requirements, and fallback behavior.

## Failure modes

A server can block client tags or omit them from history.
A client that does not implement an extension uses the fallback described in its draft.

## Source

- [Message editing overview](../../README.md#editing-messages)
- [Documentation and UI writing style](../STYLE.md)
