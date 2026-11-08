import { test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { chmodSync, existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
// @ts-expect-error -- plain JavaScript build script without type declarations
import { buildChangelog, compareVersions as scriptCompare } from '../scripts/release-changelog.mjs'
import { compareVersions } from '../shared/utils/changelog'

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
    const log = execFileSync(process.execPath, ['scripts/release-changelog.mjs', out], {
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
