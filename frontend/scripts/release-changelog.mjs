#!/usr/bin/env node
// Prepares the release notes baked into the frontend image (server/assets/changelog.json), run by
// the tag-triggered container workflow before the image builds.
//
// The tagged GitHub Releases are the only changelog. Container and CLI publication run
// independently, so the release for the tag being built may not exist yet: then the notes come
// from GitHub's release-notes generator, the same one the CLI workflow's
// `gh release create --generate-notes` uses later. An existing release (generated, or written by
// hand) always wins. Releases newer than the tag being built, drafts and other prereleases are
// left out, so a build only ever describes itself and what came before it.
//
// Usage: GH_REPO=owner/repo RELEASE_TAG=v1.2.3 node scripts/release-changelog.mjs <output.json>
// Needs `gh` with a token allowed to read releases and generate notes (contents: write).
// Failure to reach GitHub never fails the build: it writes a changelog without notes and warns.
import { execFileSync } from 'node:child_process'
import { mkdirSync, writeFileSync } from 'node:fs'
import { dirname } from 'node:path'
import { pathToFileURL } from 'node:url'

const VERSION = /^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$/

/** Semantic-version order (kept in step with shared/utils/changelog.ts). */
export function compareVersions(a, b) {
  const x = VERSION.exec(a)
  const y = VERSION.exec(b)
  if (!x || !y) return (x ? 1 : 0) - (y ? 1 : 0)
  for (let i = 1; i <= 3; i++) if (Number(x[i]) !== Number(y[i])) return Number(x[i]) - Number(y[i])
  const p = x[4] ? x[4].split('.') : []
  const q = y[4] ? y[4].split('.') : []
  if (!p.length || !q.length) return q.length - p.length
  for (let i = 0; i < Math.max(p.length, q.length); i++) {
    if (p[i] === undefined || q[i] === undefined) return p[i] === undefined ? -1 : 1
    if (p[i] === q[i]) continue
    const pn = /^\d+$/.test(p[i])
    const qn = /^\d+$/.test(q[i])
    if (pn && qn) return Number(p[i]) - Number(q[i])
    if (pn !== qn) return pn ? -1 : 1
    return p[i] < q[i] ? -1 : 1
  }
  return 0
}

const isPrerelease = (tag) => !!VERSION.exec(tag)?.[4]

/**
 * Builds the changelog for `tag` from the GitHub releases list and, when the tag has no release
 * yet, the generator's notes for it. Pure, so the workflow ordering cases can be tested directly.
 */
export function buildChangelog({ tag, releases, generated, now }) {
  const byTag = new Map()
  for (const release of releases ?? []) {
    const version = release?.tag_name
    if (typeof version !== 'string' || !VERSION.test(version)) continue
    const own = version === tag
    // Another release's draft is unpublished; our own draft holds the notes being prepared.
    if (release.draft && !own) continue
    if (!own && compareVersions(version, tag) > 0) continue
    // GitHub's flag counts too: a plain tag can be marked as a prerelease by hand.
    if (!own && (release.prerelease || isPrerelease(version)) && !isPrerelease(tag)) continue
    byTag.set(version, {
      version,
      name: release.name || version,
      date: release.published_at || release.created_at || now,
      notes: release.body ?? '',
    })
  }
  if (!byTag.has(tag) && generated)
    byTag.set(tag, {
      version: tag,
      name: generated.name || tag,
      date: now,
      notes: generated.body ?? '',
    })
  return {
    version: tag,
    releases: [...byTag.values()].sort((a, b) => compareVersions(b.version, a.version)),
  }
}

function gh(args) {
  return JSON.parse(execFileSync('gh', args, { encoding: 'utf8', maxBuffer: 64 << 20 }))
}

function main() {
  const out = process.argv[2]
  const tag = process.env.RELEASE_TAG ?? ''
  const repo = process.env.GH_REPO ?? ''
  if (!out || !/^[\w.-]+\/[\w.-]+$/.test(repo)) {
    console.error('Usage: GH_REPO=owner/repo RELEASE_TAG=v1.2.3 release-changelog.mjs <output>')
    process.exit(2)
  }
  mkdirSync(dirname(out), { recursive: true })
  // The workflow runs for any v* tag; one that is not a version simply ships without notes
  // rather than blocking the images.
  if (!VERSION.test(tag)) {
    console.log(`::warning::${tag || 'The tag'} is not a release version; building without notes.`)
    writeFileSync(out, `${JSON.stringify({ version: tag, releases: [] }, null, 2)}\n`)
    return
  }
  const now = new Date().toISOString()
  let changelog
  try {
    // --slurp wraps each page in an outer array.
    const releases = gh([
      'api',
      '--paginate',
      '--slurp',
      `repos/${repo}/releases?per_page=100`,
    ]).flat()
    let generated
    if (!releases.some((release) => release?.tag_name === tag)) {
      generated = gh([
        'api',
        '-X',
        'POST',
        `repos/${repo}/releases/generate-notes`,
        '-f',
        `tag_name=${tag}`,
      ])
      console.log(`No release for ${tag} yet; using generated notes.`)
    } else console.log(`Using the published notes of ${tag}.`)
    changelog = buildChangelog({ tag, releases, generated, now })
  } catch (error) {
    console.log(`::warning::Release notes unavailable for ${tag}: ${error.message.split('\n')[0]}`)
    changelog = { version: tag, releases: [] }
  }
  writeFileSync(out, `${JSON.stringify(changelog, null, 2)}\n`)
  console.log(`Wrote ${changelog.releases.length} release(s) to ${out}.`)
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? '').href) main()
