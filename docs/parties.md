# Group conversations

Parties are private group conversations between registered accounts on one associated Ergo server.
The Go backend manages an ordinary IRC channel; IRCord presents its groups as direct messages with multiple participants.
Registered users on the same server can also participate through ordinary IRC clients.

## Contract

### Membership and ownership

The backend stores a stable party identifier, name, description, owner, invitations and account-based membership.
Disconnecting or changing a nickname does not remove membership.
Membership binds to the registered account's identity; deleting and recreating its name does not restore the old membership.
Starting a group from a direct message creates a separate conversation and preserves the original direct message.

| Action                                   | Permission                                |
| ---------------------------------------- | ----------------------------------------- |
| Read retained history and send messages  | Accepted member                           |
| Invite another account                   | Accepted member                           |
| Change the name or description           | Accepted member                           |
| Remove another participant               | Owner                                     |
| Transfer ownership to an accepted member | Owner                                     |
| Leave                                    | Each member controls their own membership |

Invitations require explicit acceptance or decline. Receiving an invitation does not join the channel or grant access to messages.
Accepting grants access to available retained history, including retained edits, reactions and available attachments.
Declining suppresses repeated invitations for that party. A former member can receive a fresh invitation after leaving or removal.

When the owner leaves, ownership passes to the longest-standing remaining member.
When the final member leaves, the backend archives the party and revokes its pending invitations.
Archived parties never reuse their IRC channel names.

Removal revokes future access to the conversation and its history, and stops notifications.
The client clears the party from its local archive when it receives the change.
Copies already downloaded or saved outside IRCord cannot be recalled.
Shared file links remain usable after leaving or removal.

### Channel control

Each party uses a randomly named, registered channel on the associated server. Parties do not span IRC networks.
A private backend service account owns the registration and maintains the required channel modes and account grants.
The controller provisions itself when durable party work exists; its credentials are encrypted with the backend's existing storage key.
No additional operator credential is required.
Participants do not receive channel operator or founder privileges, including the group's application owner.
Every accepted member can invite people and edit the name or description through IRCord; Go applies those changes.

The controller applies secret, invite-only, registered-account-only, protected-topic and no-outside-message modes, plus an occupancy limit.
Accepted accounts receive persistent voice for admission without channel-management privileges.
The occupancy limit includes the controller; persistent membership is independent of online channel presence.
The backend confirms channel ownership and protection before admitting members.

Party identity comes from durable records and explicit API metadata, not a broad channel-name prefix.
Ordinary channels such as `#ircord-devclub` retain their existing behavior.
Generic channel-management and raw-command paths cannot replace party membership operations.

### Other IRC clients

Other clients receive ordinary IRC invitations and join the managed channel through their normal controls.
Their messages, file links and chat history follow the IRC server's existing behavior.
The managing backend retains group metadata; other backends do not need a separate metadata or archive synchronization protocol.
Group names, descriptions and owner actions are available in the managing backend's IRCord interface.
Other clients present the underlying IRC channel through their existing interface.

### API and events

Authenticated HTTP operations under `/api/v1/parties` manage creation, metadata, invitations, membership and ownership.
Creation uses a client request identifier for retry deduplication.
`POST /api/v1/invitations/{id}/accept` returns the accepted conversation; declining uses the existing invitation deletion operation.

The conversation retains `kind: channel` for transport and adds `party` metadata for group presentation.
The frontend routes it through `/dm/:id` and hides channel configuration, transport names and operator controls.
Private `party.updated` and `party.removed` events accompany the existing conversation and invitation events.
Monotonic party revisions prevent delayed responses from restoring obsolete membership state.

The backend retains canonical party history and supplies account-scoped copies to accepted members.
Party attachments follow ordinary upload behavior, including when sent through another IRCord backend.
Anyone holding a valid download URL can open the file without signing in or joining the party.
Upload ownership, file validation, download-token checks, deletion and retention follow the existing file service rules.
Parties do not add file-sharing restrictions or separate file-access proofs.

## Defaults

- Party names allow 100 characters; descriptions allow 500 characters.
- Accepted members and pending invitees together have a limit of 32, including the creator.
- An account can have at most 20 pending party invitations.
- Empty names use participant names in the interface.
- Notifications follow direct-message defaults and existing account, conversation and device preferences.
- Canonical history follows the account retention period and has a ceiling of 50,000 messages per party.

Bootstrap exposes `capabilities.parties` only for eligible accounts when associated-server account integration is configured.
The capability supplies the network identifier and field/member limits; the frontend does not infer them from channel names.

## Failure modes

Channel control requires SASL, labeled responses, account tags, account notifications, extended join, batches, server time and chat history.
The server must support channel names of at least 40 characters and allow channel registration.
Its registration and joined-channel limits must accommodate the service account's parties.
Registered-channel history must support persistent storage. The controller enables channel history and verifies its effective persistence before admitting members.

Temporary controller failures retain the feature's availability signal while operations report recovery errors.
Durable intent allows reconciliation after interrupted requests, disconnects or backend restarts.
New admission and protected operations remain unavailable until the backend confirms the required control state.
Members remain members while offline; reconnecting restores their conversation rather than creating a new party.
An accepted member can appear before their IRC join completes. Sending also requires their associated account connection to be authenticated and joined.
Logging out or changing an IRC account triggers removal by the controller; this reaction is asynchronous.

## Source

- [Backend ownership](backend.md)
- [Frontend ownership and routing](frontend.md)
- [Go party API](../backend/internal/api/party_routes.go)
- [Party model](../backend/internal/model/party.go)
- [Ergo user guide: private channels](https://github.com/ergochat/ergo/blob/master/docs/USERGUIDE.md#private-channels)
