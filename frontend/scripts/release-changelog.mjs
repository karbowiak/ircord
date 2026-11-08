#!/usr/bin/env node
// Prepares the release notes baked into the frontend image (server/assets/changelog.json), run by
// the tag-triggered container workflow before the image builds, and prints one version's notes for
// the GitHub Release the CLI workflow creates.
//
// CHANGELOG.md at the repository root is the user-facing source: a release with a `## vX.Y.Z`
// section there shows that section, in the client and on GitHub. Releases from before the file
// existed have no section and keep their GitHub Release notes.
//
// Container and CLI publication run independently, so the GitHub Release for the tag being built
// may not exist yet: then a tag without a changelog section takes its notes from GitHub's
// release-notes generator, as the CLI workflow does. Releases newer than the tag being built,
// drafts and other prereleases are left out, so a build only ever describes itself and what came
// before it.
//
// Usage: GH_REPO=owner/repo RELEASE_TAG=v1.2.3 bun scripts/release-changelog.mjs <output.json> [CHANGELOG.md]
//        node scripts/release-changelog.mjs --notes v1.2.3 CHANGELOG.md   (prints that section)
// Needs `gh` with a token allowed to read releases and generate notes (contents: write).
// Failure to reach GitHub never fails the build: it writes the changelog sections alone and warns.
import { execFileSync } from 'node:child_process'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
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

/** `## v1.2.3 — 2026-11-08` (any dash, date optional); `## Unreleased` and other headings are not. */
const SECTION =
  /^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?(?=\s|$)(?:\s+[-–—]+\s+(\d{4}-\d{2}-\d{2}))?\s*$/

/**
 * Reads CHANGELOG.md into its release sections: version -> { version, date, notes }. A section
 * runs from its `##` heading to the next `##` heading; its `###` groups stay in the notes. An empty
 * section (or one holding only comments) has notes ''. The first section of a version wins.
 */
export function changelogSections(markdown) {
  const sections = new Map()
  let current = null
  let fence = false
  for (const line of String(markdown ?? '')
    .replace(/\r\n?/g, '\n')
    .split('\n')) {
    if (/^\s*```/.test(line)) fence = !fence
    const heading = fence ? null : /^##(?!#)\s+(.*?)\s*$/.exec(line)
    if (!heading) {
      current?.lines.push(line)
      continue
    }
    current = null
    const match = SECTION.exec(heading[1])
    if (!match) continue
    const version = heading[1].split(/\s/)[0]
    if (sections.has(version)) continue
    current = { version, date: match[1] ?? '', lines: [] }
    sections.set(version, current)
  }
  return new Map(
    [...sections].map(([version, { date, lines }]) => [
      version,
      {
        version,
        date,
        notes: lines
          .join('\n')
          .replace(/<!--[\s\S]*?-->/g, '')
          .trim(),
      },
    ]),
  )
}

/** The notes of `version` in CHANGELOG.md, or '' when it has no (or an empty) section. */
export function changelogNotes(markdown, version) {
  return changelogSections(markdown).get(version)?.notes ?? ''
}

/**
 * Builds the changelog for `tag` from the GitHub releases list, the CHANGELOG.md `sections` (from
 * changelogSections) and, when the tag has no release yet, the generator's notes for it. A
 * non-empty changelog section replaces a release's GitHub notes; a section without a GitHub
 * release still counts as a release. Pure, so the workflow ordering cases can be tested directly.
 */
export function buildChangelog({ tag, releases, generated, now, sections }) {
  const notes = (version, fallback) => sections?.get(version)?.notes || fallback
  const byTag = new Map()
  // Another release's draft is unpublished; our own draft holds the notes being prepared.
  // GitHub's prerelease flag counts too: a plain tag can be marked as a prerelease by hand.
  const included = (version, { draft, prerelease } = {}) =>
    version === tag ||
    (!draft &&
      compareVersions(version, tag) <= 0 &&
      (isPrerelease(tag) || !(prerelease || isPrerelease(version))))
  for (const release of releases ?? []) {
    const version = release?.tag_name
    if (typeof version !== 'string' || !VERSION.test(version)) continue
    if (!included(version, release)) continue
    byTag.set(version, {
      version,
      name: release.name || version,
      date: release.published_at || release.created_at || now,
      notes: notes(version, release.body ?? ''),
    })
  }
  if (!byTag.has(tag) && generated)
    byTag.set(tag, {
      version: tag,
      name: generated.name || tag,
      date: now,
      notes: notes(tag, generated.body ?? ''),
    })
  for (const section of sections?.values() ?? []) {
    if (byTag.has(section.version) || !section.notes || !included(section.version)) continue
    byTag.set(section.version, {
      version: section.version,
      name: section.version,
      date: section.date ? `${section.date}T00:00:00Z` : now,
      notes: section.notes,
    })
  }
  return {
    version: tag,
    releases: [...byTag.values()].sort((a, b) => compareVersions(b.version, a.version)),
  }
}

function gh(args) {
  return JSON.parse(execFileSync('gh', args, { encoding: 'utf8', maxBuffer: 64 << 20 }))
}

function printNotes(tag, file) {
  if (!tag || !file) {
    console.error('Usage: release-changelog.mjs --notes v1.2.3 CHANGELOG.md')
    process.exit(2)
  }
  const text = changelogNotes(existsSync(file) ? readFileSync(file, 'utf8') : '', tag)
  if (text) process.stdout.write(`${text}\n`)
}

function main() {
  if (process.argv[2] === '--notes') return printNotes(process.argv[3], process.argv[4])
  const out = process.argv[2]
  const file = process.argv[3]
  const tag = process.env.RELEASE_TAG ?? ''
  const repo = process.env.GH_REPO ?? ''
  if (!out || !/^[\w.-]+\/[\w.-]+$/.test(repo)) {
    console.error(
      'Usage: GH_REPO=owner/repo RELEASE_TAG=v1.2.3 release-changelog.mjs <output> [CHANGELOG.md]',
    )
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
  const sections =
    file && existsSync(file) ? changelogSections(readFileSync(file, 'utf8')) : new Map()
  if (file && !existsSync(file))
    console.log(`::warning::${file} not found; using GitHub notes only.`)
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
    // A tag with its own changelog section needs nothing generated.
    if (!sections.get(tag)?.notes && !releases.some((release) => release?.tag_name === tag)) {
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
    changelog = buildChangelog({ tag, releases, generated, now, sections })
  } catch (error) {
    console.log(`::warning::Release notes unavailable for ${tag}: ${error.message.split('\n')[0]}`)
    changelog = buildChangelog({ tag, releases: [], now, sections })
  }
  writeFileSync(out, `${JSON.stringify(changelog, null, 2)}\n`)
  console.log(`Wrote ${changelog.releases.length} release(s) to ${out}.`)
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? '').href) main()
