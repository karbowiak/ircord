# Working on IRCord

This file defines how contributors plan, implement, verify, and release changes
to IRCord.

## Contract

### Choose issue tracking

Before implementation, ask whether the user wants a GitHub issue or work without
one. Recommend an issue, but let the user choose.

If the user has already chosen a path, follow that choice without asking again.
A request to work on a specific issue selects issue tracking.

Before editing, check current work and the checkout. Preserve unrelated changes.
Coordinate overlapping work with other workers in either path.

### Work without an issue

Use the scope agreed in the conversation. Proceed without issue creation,
assignment, labels, claim comments, updates, or references.

Report the result, validation, and unfinished work to the user.

### Work with an issue

If an issue exists, read it before implementation. Review its dependencies,
assignees, comments, and related pull requests. If no issue exists, create one
for the agreed scope.

Write issue descriptions that contributors can understand without local planning
files. Use local planning files as reference material.

Several workers can share one GitHub account. An assignee does not identify the
worker. Before taking an assigned issue, read its claim comments.

Before changing another worker's claim or editing overlapping code, coordinate
with that worker. The user can change ownership.

After confirming ownership, claim the issue before implementation:

1. Assign the issue to your authenticated GitHub account.
2. Add `status: in-progress`.
3. Remove `status: ready` or `status: coordinate` when present.
4. Post a claim comment with your identity, branch or worktree, scope, and intended result.

Keep private filesystem paths and secrets out of claim comments.

When scope, ownership, or dependencies change, update the issue body and comments.

### Keep work scoped

- Keep changes within the agreed scope from the conversation or issue.
- Raise newly discovered work with the user.
- Ask before creating additional issues.
- Before editing shared files, coordinate with other workers.
- When needed, use a separate branch or worktree.
- Do not reset, discard, or stage another worker's changes.
- Before pushing, fetch the current remote state.

Message editing and deletion share timeline and composer code. Playback and
previews share media code.

### Write documentation and interface text

Follow [docs/STYLE.md](docs/STYLE.md) for documentation and user interface (UI)
text. Apply the guide to release notes, labels, help text, notifications, empty
states, and error messages.

### Implement and verify changes

Use authenticated Hypertext Transfer Protocol (HTTP) requests for mutations.
Use private WebSockets for live updates. Preserve account isolation, bounded
queues, durable replay, and credential redaction.

After changing application programming interface (API) contracts, run
`make api-generate`. Keep OpenAPI, the typed frontend client, and WebSocket
validation aligned with Go contracts.

Run the relevant local checks from [README.md](README.md). Add meaningful
regression tests for behavior changes. After UI changes, verify behavior in a
browser.

Use disposable identities and private test channels. Without coordination, do
not send test messages to users. Without coordination, do not run load tests
against shared infrastructure.

When the user requests local servers, keep the frontend and backend available.

### Update the changelog

For user-visible changes, add one plain-language line under `## Unreleased` in
`CHANGELOG.md`. Use the **New**, **Improved**, or **Fixed** group. If the required
group is missing, create it.

Describe what users can now do or what behaves differently. Omit issue numbers,
file names, and internal terms.

For refactors, tests, continuous integration, tooling, and documentation changes,
skip the changelog entry.

### Complete work

For work with an issue, reference it in the commit or pull request. Before
closing the issue, record the changes and actual validation results.

Mark work complete only after you satisfy the agreed requirements and any issue
acceptance criteria. Distinguish source changes, local verification, and
deployed behavior.

### Release changes

Backend and frontend builds and publication run only for pushed version tags,
such as `v0.1.0`. Flux follows stable `vX.Y.Z` images. Branch pushes and pull
requests do not build or deploy these images.

The active embedding pipeline uses direct files, the configured Reddit API handler,
and generic oEmbed. Instagram, TikTok and yt-dlp extractors remain inactive for later
adaptation; do not register them by default.

Embed UI adapters belong in `frontend/app/components/embed/`, with a static
`embedProvider` declaration for URL matching, layout and resolver policy. Keep
selection, caching and expiry in `useEmbed`. Reserve geometry before fetching;
loading, error and player states must not resize it. Unknown sites use a fixed
metadata card until they have a controlled adapter. A separate Go extractor is
optional when generic oEmbed already supplies the required data.

Create a release tag only when the user has authorized release or deployment.
Use the latest stable Go version in builds.

Follow [Cutting a release](README.md#cutting-a-release). In the release commit,
move the `## Unreleased` changes in `CHANGELOG.md` into the version's section.

### Protect private material

Keep credentials, account keys, local data, and planning or handover Markdown
out of Git and image build contexts.

You may track shared documentation under `docs/`. Keep `docs/` out of image build
contexts. Track Markdown only in `README.md`, `CHANGELOG.md`, `COLLABORATORS.md`, `AGENTS.md`, and
shared documentation under `docs/`.

Never include secret values in issues, examples, frontend configuration, claim
comments, or diagnostic output.

Deploy cluster configuration through the homelab GitOps repository. Follow that
repository's `AGENTS.md`. Store secret values in its secret manager. Commit only
references to secrets.

### Use file editing tools

**Use the built-in Edit and Write tools to create or change files.** When those
tools have different names, use their provided equivalent, such as `apply_patch`.

Do not rewrite files with ad hoc Perl, Python, PHP, JavaScript, or other language
scripts. Do not use shell substitutions, heredocs, or redirection as editing
shortcuts.

For formatting and generated output, use the project's established formatters
and code generators.

## Defaults

GitHub issues are preferred, but optional. Direct user instructions override
these workflow defaults.

Nuxt and Vue code lives in `frontend/`. Go, Huma, and upstream Internet Relay Chat
(IRC) code lives in `backend/`.

## Failure modes

`status: needs-design` means the design needs agreement before implementation.
`status: blocked` means a linked dependency must be resolved. You may claim
research work. An issue claim does not approve an undecided protocol or design.

If work stops unfinished, report its state and next steps to the user. For work
with an issue, also record that information on the issue. When another worker
should take over, release the claim.

## Source

- [Writing style](docs/STYLE.md)
- [Local checks and release instructions](README.md)
- [User-facing changelog](CHANGELOG.md)
- [Shared backlog](https://github.com/karbowiak/ircord/issues/23)
