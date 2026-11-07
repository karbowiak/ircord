# Working on IRCord

Use GitHub issues to coordinate work. The shared backlog is
https://github.com/karbowiak/ircord/issues/23. Local planning files are reference
material; contributors must be able to understand an issue without them.

## Claim work before implementation

1. Read the issue, its dependencies, assignees, comments, and related pull requests.
2. Check current work and the checkout before editing. Preserve unrelated changes.
3. Create a scoped issue if the requested work has no issue yet.
4. Assign the issue to your authenticated GitHub account and add `status: in-progress`.
5. Remove `status: ready` or `status: coordinate` when you start the claimed work.
6. Post a claim comment identifying the agent/person, branch or worktree, scope,
   and intended result. Do not publish private filesystem paths or secrets.

Several agents can use the same GitHub account. The assignee alone does not
identify the worker. Read the claim comment before taking an assigned issue.
Do not overwrite another worker's claim or edit overlapping implementation areas
without coordination. An explicit instruction from the user can change ownership.

`status: needs-design` means the design needs agreement before implementation.
`status: blocked` means a linked dependency must be resolved. Research can be
claimed, but claiming an issue does not approve an undecided protocol or design.

## Keep changes scoped

- Own the deliverable described by your issue. Record newly discovered work as
  separate issues instead of expanding a change without limits.
- Coordinate shared files before editing them. In particular, message editing
  and deletion share timeline/composer code; playback and previews share media code.
- Use a separate branch/worktree when needed. Never reset, discard, or stage
  another worker's changes. Fetch current remote state before pushing.
- Keep issue bodies and comments current when scope, ownership, or dependencies change.
- Follow direct user instructions when they override these workflow defaults.

## Implementation and verification

- Nuxt/Vue lives in `frontend/`; Go/Huma and upstream IRC live in `backend/`.
- HTTP carries authenticated mutations. Private WebSockets deliver live updates.
  Preserve account isolation, bounded queues, durable replay, and credential redaction.
- Generate contract changes with `make api-generate`. Keep OpenAPI, the typed
  frontend client, and WebSocket validation aligned with Go.
- Use relevant local checks from `README.md`. Include meaningful regressions for
  behavior changes and verify UI behavior in a browser when it changes.
- Use disposable identities and private test channels. Do not send test messages
  to users or run load tests against shared infrastructure without coordination.
- Keep the local frontend and backend available when the user asks for them.

## Complete and release

- Reference the issue in the commit or pull request. Record what changed and the
  actual validation results before closing it.
- Mark complete only when the issue's acceptance criteria are satisfied.
  Distinguish source changes, local verification, and deployed behavior.
- If work stops unfinished, leave its state and next steps visible. Release the
  claim when someone else should take it; do not leave misleading ownership.
- Builds and publication run only for pushed version tags such as `v0.1.0`.
  Flux follows stable `vX.Y.Z` images. Branch pushes and PRs do not build or deploy.
- Create a release tag only within work authorized to release/deploy. Use the
  latest stable Go version in builds.

## Private material

Keep credentials, account keys, local data, planning/handover Markdown, and
`docs/` out of Git and image contexts. `README.md` and this `AGENTS.md` are the
tracked Markdown exceptions. Never put a secret value in an issue, example,
frontend configuration, claim comment, or diagnostic output.

Deploy cluster configuration through the homelab GitOps repository. Follow its
own `AGENTS.md`; use its secret manager for secret values and commit references only.
