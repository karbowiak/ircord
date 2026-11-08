import { test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { chmodSync, existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
// @ts-expect-error -- plain JavaScript build script without type declarations
import {
  buildChangelog,
  changelogNotes,
  changelogSections,
  compareVersions as scriptCompare,
} from '../scripts/release-changelog.mjs'
import { compareVersions, parseNotes } from '../shared/utils/changelog'

const NOW = '2026-11-08T12:00:00.000Z'
const gh = (tag: string, extra: Record<string, unknown> = {}) => ({
  tag_name: tag,
  name: tag,
  draft: false,
  prerelease: false,
  published_at: '2026-11-07T22:00:00Z',
  body: `## What's Changed\n* ${tag} change`,
  ...extra,
})
const tags = (changelog: { releases: { version: string }[] }) =>
  changelog.releases.map((r) => r.version)

test('an existing release (CLI workflow finished first) keeps its published notes', () => {
  const edited = gh('v0.1.4', { body: 'Hand-edited notes', name: 'IRCord v0.1.4' })
  const changelog = buildChangelog({
    tag: 'v0.1.4',
    releases: [edited, gh('v0.1.3')],
    generated: { name: 'v0.1.4', body: 'generated' },
    now: NOW,
  })
  assert.equal(changelog.version, 'v0.1.4')
  assert.deepEqual(changelog.releases[0], {
    version: 'v0.1.4',
    name: 'IRCord v0.1.4',
    date: '2026-11-07T22:00:00Z',
    notes: 'Hand-edited notes',
  })
})

test('a tag without a release yet (containers first) uses the generated notes', () => {
  const changelog = buildChangelog({
    tag: 'v0.1.5',
    releases: [gh('v0.1.4'), gh('v0.1.3')],
    generated: { name: 'v0.1.5', body: '## What’s Changed\n* new' },
    now: NOW,
  })
  assert.deepEqual(tags(changelog), ['v0.1.5', 'v0.1.4', 'v0.1.3'])
  assert.deepEqual(changelog.releases[0], {
    version: 'v0.1.5',
    name: 'v0.1.5',
    date: NOW,
    notes: '## What’s Changed\n* new',
  })
})

test('newer tags, other drafts and prereleases never reach a stable build', () => {
  const changelog = buildChangelog({
    tag: 'v0.1.3',
    releases: [
      gh('v0.2.0'),
      gh('v0.1.4'),
      gh('v0.1.3-rc.1', { prerelease: true }),
      gh('v0.1.2', { draft: true }),
      // A plain tag flagged as a prerelease on GitHub.
      gh('v0.1.0', { prerelease: true }),
      gh('v0.1.3'),
      gh('v0.1.1'),
      gh('nightly'),
    ],
    now: NOW,
  })
  assert.deepEqual(tags(changelog), ['v0.1.3', 'v0.1.1'])
})

test('a prerelease build keeps earlier prereleases and its own draft', () => {
  const changelog = buildChangelog({
    tag: 'v0.2.0-rc.2',
    releases: [
      gh('v0.2.0-rc.2', { draft: true, published_at: null, created_at: '2026-11-08T10:00:00Z' }),
      gh('v0.2.0-rc.1', { prerelease: true }),
      gh('v0.1.4'),
    ],
    now: NOW,
  })
  assert.deepEqual(tags(changelog), ['v0.2.0-rc.2', 'v0.2.0-rc.1', 'v0.1.4'])
  assert.equal(changelog.releases[0].date, '2026-11-08T10:00:00Z')
})

/** Runs the script against a stub `gh` that serves fixture responses and logs its calls. */
function runScript(
  fixture: { releases: unknown[][] | 'fail'; generated?: unknown },
  tag = 'v0.1.5',
  changelog?: string,
) {
  const dir = mkdtempSync(join(tmpdir(), 'ircord-changelog-'))
  try {
    writeFileSync(join(dir, 'fixture.json'), JSON.stringify(fixture))
    const stub = join(dir, 'gh')
    writeFileSync(
      stub,
      `#!/usr/bin/env node
const fs = require('node:fs')
const fixture = JSON.parse(fs.readFileSync(${JSON.stringify(join(dir, 'fixture.json'))}, 'utf8'))
const args = process.argv.slice(2)
fs.appendFileSync(${JSON.stringify(join(dir, 'calls.log'))}, args.join(' ') + '\\n')
if (fixture.releases === 'fail') { console.error('HTTP 502'); process.exit(1) }
if (args.some((arg) => arg.endsWith('/generate-notes'))) process.stdout.write(JSON.stringify(fixture.generated))
else process.stdout.write(JSON.stringify(fixture.releases))
`,
    )
    chmodSync(stub, 0o755)
    const out = join(dir, 'out', 'changelog.json')
    const args = ['scripts/release-changelog.mjs', out]
    if (changelog !== undefined) {
      writeFileSync(join(dir, 'CHANGELOG.md'), changelog)
      args.push(join(dir, 'CHANGELOG.md'))
    }
    const log = execFileSync(process.execPath, args, {
      encoding: 'utf8',
      env: {
        ...process.env,
        PATH: `${dir}:${process.env.PATH}`,
        GH_REPO: 'owner/repo',
        RELEASE_TAG: tag,
      },
    })
    return {
      log,
      changelog: JSON.parse(readFileSync(out, 'utf8')),
      calls: existsSync(join(dir, 'calls.log'))
        ? readFileSync(join(dir, 'calls.log'), 'utf8').trim().split('\n')
        : [],
    }
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
}

test('the workflow step reads every page of releases and generates notes only when missing', () => {
  const missing = runScript({
    releases: [[gh('v0.1.4')], [gh('v0.1.3')]],
    generated: { name: 'v0.1.5', body: 'generated' },
  })
  assert.deepEqual(tags(missing.changelog), ['v0.1.5', 'v0.1.4', 'v0.1.3'])
  assert.equal(missing.changelog.releases[0].notes, 'generated')
  assert.deepEqual(missing.calls, [
    'api --paginate --slurp repos/owner/repo/releases?per_page=100',
    'api -X POST repos/owner/repo/releases/generate-notes -f tag_name=v0.1.5',
  ])

  const existing = runScript({ releases: [[gh('v0.1.5', { body: 'published' }), gh('v0.1.4')]] })
  assert.equal(existing.changelog.releases[0].notes, 'published')
  assert.equal(existing.calls.length, 1)
})

test('GitHub being unreachable leaves a build without notes instead of failing it', () => {
  const failed = runScript({ releases: 'fail' })
  assert.deepEqual(failed.changelog, { version: 'v0.1.5', releases: [] })
  assert.match(failed.log, /::warning::Release notes unavailable for v0\.1\.5/)
})

test('a v* tag that is not a release version builds without notes instead of blocking the images', () => {
  for (const tag of ['v1.2', 'v2026.11.08.1']) {
    const result = runScript({ releases: [[gh('v0.1.4')]] }, tag)
    assert.deepEqual(result.changelog, { version: tag, releases: [] })
    assert.match(result.log, /::warning::.* is not a release version/)
    assert.deepEqual(result.calls, [])
  }
})

test('the build script and the client order versions the same way', () => {
  const corpus = [
    'v0.1.0',
    'v0.1.4',
    'v0.2.0-rc.1',
    'v0.2.0-rc.2',
    'v0.2.0-rc.10',
    'v0.2.0-beta',
    'v0.2.0-1',
    'v0.2.0',
    'v0.2.0+build.5',
    '1.2.3',
    'v0.10.0',
    'v1.0.0-alpha.beta',
    'v1.0.0-alpha.1',
    'v1.0.0',
    'nightly',
    '',
  ]
  for (const a of corpus)
    for (const b of corpus)
      assert.equal(Math.sign(scriptCompare(a, b)), Math.sign(compareVersions(a, b)), `${a} vs ${b}`)
})

const CHANGELOG = `# Changelog

Intro text that belongs to no release.

## Unreleased

### New

- Something not released yet.

## v0.1.5 — 2026-11-09

### New

- Delete your own messages from the message menu.

### Fixed

- The member list no longer jumps while you scroll.

\`\`\`
## v9.9.9 — inside a code block, not a heading
\`\`\`

## v0.1.4 - 2026-11-08

<!-- nothing user-facing yet -->

## v0.1.3

- Behind-the-scenes improvements.

## Notes

Not a release.
`

test('changelog sections are read per version; Unreleased, intro and other headings are not', () => {
  const sections = changelogSections(CHANGELOG)
  assert.deepEqual([...sections.keys()], ['v0.1.5', 'v0.1.4', 'v0.1.3'])
  assert.deepEqual(sections.get('v0.1.5'), {
    version: 'v0.1.5',
    date: '2026-11-09',
    notes:
      '### New\n\n- Delete your own messages from the message menu.\n\n### Fixed\n\n- The member list no longer jumps while you scroll.\n\n```\n## v9.9.9 — inside a code block, not a heading\n```',
  })
  assert.equal(sections.get('v0.1.4')?.date, '2026-11-08')
  assert.equal(changelogNotes(CHANGELOG, 'v0.1.4'), '', 'a section of only comments is empty')
  assert.equal(changelogNotes(CHANGELOG, 'v0.1.3'), '- Behind-the-scenes improvements.')
  assert.equal(changelogNotes(CHANGELOG, 'v0.1.2'), '', 'a missing section is empty')
  assert.equal(changelogNotes(CHANGELOG, 'Unreleased'), '')
  assert.equal(changelogNotes('', 'v0.1.5'), '')
  assert.equal(changelogNotes('## v0.1.5\r\n\r\n- one\r\n- two\r\n', 'v0.1.5'), '- one\n- two')
})

test('a changelog section replaces GitHub notes; releases without one keep theirs', () => {
  const changelog = buildChangelog({
    tag: 'v0.1.5',
    releases: [gh('v0.1.5'), gh('v0.1.4'), gh('v0.1.3'), gh('v0.1.2')],
    sections: changelogSections(CHANGELOG),
    now: NOW,
  })
  const notes = Object.fromEntries(changelog.releases.map((r) => [r.version, r.notes]))
  assert.match(notes['v0.1.5'], /^### New\n\n- Delete your own messages/)
  // Empty section and no section both fall back to the published notes.
  assert.equal(notes['v0.1.4'], "## What's Changed\n* v0.1.4 change")
  assert.equal(notes['v0.1.3'], '- Behind-the-scenes improvements.')
  assert.equal(notes['v0.1.2'], "## What's Changed\n* v0.1.2 change")
})

test('a section counts as a release before GitHub has one, but never past the tag being built', () => {
  const changelog = buildChangelog({
    tag: 'v0.1.4',
    releases: [gh('v0.1.2')],
    generated: { name: 'v0.1.4', body: 'generated' },
    sections: changelogSections(CHANGELOG),
    now: NOW,
  })
  assert.deepEqual(tags(changelog), ['v0.1.4', 'v0.1.3', 'v0.1.2'])
  assert.equal(changelog.releases[0].notes, 'generated', 'an empty section falls back')
  assert.deepEqual(changelog.releases[1], {
    version: 'v0.1.3',
    name: 'v0.1.3',
    date: NOW,
    notes: '- Behind-the-scenes improvements.',
  })
})

test('a section never brings back a draft or a hand-marked prerelease', () => {
  const sections = changelogSections(
    '## v0.1.4\n\n- Draft notes.\n\n## v0.1.3\n\n- Prerelease notes.\n\n## v0.1.2\n\n- Kept.\n',
  )
  const changelog = buildChangelog({
    tag: 'v0.1.5',
    releases: [gh('v0.1.5'), gh('v0.1.4', { draft: true }), gh('v0.1.3', { prerelease: true })],
    sections,
    now: NOW,
  })
  assert.deepEqual(tags(changelog), ['v0.1.5', 'v0.1.2'])
})

test("the workflow step uses the tag's section without asking the generator", () => {
  const result = runScript({ releases: [[gh('v0.1.4')]] }, 'v0.1.5', CHANGELOG)
  assert.deepEqual(tags(result.changelog), ['v0.1.5', 'v0.1.4', 'v0.1.3'])
  assert.match(result.changelog.releases[0].notes, /Delete your own messages/)
  assert.equal(result.changelog.releases[0].date, '2026-11-09T00:00:00Z')
  assert.deepEqual(result.calls, ['api --paginate --slurp repos/owner/repo/releases?per_page=100'])

  // GitHub down: the changelog's own sections still ship.
  const failed = runScript({ releases: 'fail' }, 'v0.1.5', CHANGELOG)
  assert.deepEqual(tags(failed.changelog), ['v0.1.5', 'v0.1.3'])
})

test('changelog text renders as plain data: markup and script links stay text', () => {
  const blocks = parseNotes(
    changelogNotes(
      '## v0.1.5\n\n### Fixed\n\n- <img src=x onerror=alert(1)> and [x](javascript:alert(1))\n',
      'v0.1.5',
    ),
  )
  assert.deepEqual(blocks, [
    { type: 'heading', level: 3, content: [{ text: 'Fixed' }] },
    {
      type: 'list',
      ordered: false,
      items: [{ content: [{ text: '<img src=x onerror=alert(1)> and [x](javascript:alert(1))' }] }],
    },
  ])
})

/** Runs publish-cli.sh against stub `gh` and `sha256sum`-ready assets; returns the release notes it sent. */
function publishCli(changelog: string | null, tag = 'v0.1.5') {
  const dir = mkdtempSync(join(tmpdir(), 'ircord-publish-'))
  try {
    const assets = join(dir, 'assets')
    execFileSync('mkdir', ['-p', assets])
    for (const os of ['darwin', 'linux', 'windows'])
      for (const arch of ['amd64', 'arm64'])
        writeFileSync(
          join(assets, `ircordctl_${tag}_${os}_${arch}.${os === 'windows' ? 'zip' : 'tar.gz'}`),
          'x',
        )
    if (changelog !== null) writeFileSync(join(dir, 'CHANGELOG.md'), changelog)
    const stub = join(dir, 'gh')
    writeFileSync(
      stub,
      `#!/usr/bin/env node
const fs = require('node:fs')
const args = process.argv.slice(2)
fs.appendFileSync(${JSON.stringify(join(dir, 'calls.log'))}, args.join(' ') + '\\n')
if (args[0] === 'release' && args[1] === 'view') process.exit(1)
if (args.some((arg) => arg.endsWith('/generate-notes')))
  process.stdout.write("## What's Changed\\n* x\\n\\n\\n**Full Changelog**: https://github.com/owner/repo/compare/v0.1.4...${tag}\\n")
const file = args[args.indexOf('--notes-file') + 1]
if (args.includes('--notes-file')) fs.writeFileSync(${JSON.stringify(join(dir, 'notes.md'))}, fs.readFileSync(file))
`,
    )
    chmodSync(stub, 0o755)
    execFileSync(
      'bash',
      [
        join(process.cwd(), '..', '.github/scripts/publish-cli.sh'),
        assets,
        join(dir, 'CHANGELOG.md'),
      ],
      {
        encoding: 'utf8',
        env: {
          ...process.env,
          PATH: `${dir}:${process.env.PATH}`,
          GH_REPO: 'owner/repo',
          RELEASE_TAG: tag,
        },
      },
    )
    const calls = readFileSync(join(dir, 'calls.log'), 'utf8').trim().split('\n')
    const create = calls.find((call) => call.startsWith('release create')) ?? ''
    const notes = existsSync(join(dir, 'notes.md'))
      ? readFileSync(join(dir, 'notes.md'), 'utf8')
      : null
    return { create, notes }
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
}

test("a new GitHub release takes the tag's changelog section and keeps the compare link", () => {
  const { create, notes } = publishCli(CHANGELOG)
  assert.doesNotMatch(create, /--generate-notes/)
  assert.equal(
    notes,
    '### New\n\n- Delete your own messages from the message menu.\n\n### Fixed\n\n- The member list no longer jumps while you scroll.\n\n```\n## v9.9.9 — inside a code block, not a heading\n```\n\n**Full Changelog**: https://github.com/owner/repo/compare/v0.1.4...v0.1.5\n',
  )
})

test('a tag without a changelog section (or without the file) gets generated notes', () => {
  for (const [changelog, tag] of [
    [CHANGELOG, 'v0.1.4'],
    [CHANGELOG, 'v0.1.6'],
    [null, 'v0.1.5'],
  ] as const) {
    const { create, notes } = publishCli(changelog, tag)
    assert.match(create, /--generate-notes/, `${tag}`)
    assert.equal(notes, null)
  }
})

test('the repository changelog: every ## heading is Unreleased or a dated release, v0.1.12 backfilled', () => {
  const text = readFileSync(join(process.cwd(), '..', 'CHANGELOG.md'), 'utf8')
  assert.match(text, /^## Unreleased$/m)
  const sections = changelogSections(text)
  // A mistyped release heading would silently fall back to generated notes.
  const headings = [...text.matchAll(/^## (.*)$/gm)].map((m) => m[1]!)
  assert.deepEqual(
    headings.filter((h) => h !== 'Unreleased'),
    [...sections.keys()].map((v) => `${v} — ${sections.get(v)!.date}`),
  )
  assert.ok(sections.get('v0.1.12')?.notes, 'v0.1.12 is backfilled')
  for (const section of sections.values()) {
    assert.match(section.date, /^\d{4}-\d{2}-\d{2}$/, `${section.version} has a date`)
    assert.ok(section.notes, `${section.version} has notes`)
    assert.doesNotMatch(
      section.notes,
      /#\d+|\bPR\b|\.(ts|go|vue|md)\b/,
      `${section.version} speaks to users`,
    )
    for (const heading of section.notes.matchAll(/^### (.*)$/gm))
      assert.ok(['New', 'Improved', 'Fixed'].includes(heading[1]!), heading[1])
  }
})
