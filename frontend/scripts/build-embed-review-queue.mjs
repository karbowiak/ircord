#!/usr/bin/env bun

// Read declarations and fixture data only. Never load Vue components or call lazy loaders.
import { plugin } from 'bun'
import { createHash } from 'node:crypto'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { discoverEmbeds } from '../nuxt-modules/embed-declarations.ts'

const frontend = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const catalog = JSON.parse(
  await readFile(resolve(frontend, 'shared/embed-fixtures/iframely.json'), 'utf8'),
)
const providers = await discoverEmbeds(resolve(frontend, 'app/components/embed'))
const generatedPath = resolve(frontend, '.nuxt/embed-manifest.ts')
plugin({
  name: 'embed-review-manifest',
  setup(builder) {
    builder.onResolve({ filter: /^#embed-manifest$/ }, () => ({ path: generatedPath }))
  },
})
const generated = await import(generatedPath)
if (JSON.stringify(providers) !== JSON.stringify(generated.embedProviders)) {
  throw new Error(
    'Generated embed manifest is stale; allow the development server to refresh it first',
  )
}
const { selectEmbed, embedMatches } = await import('../app/utils/embed.ts')
const hash = async (path) =>
  createHash('sha256')
    .update(await readFile(path))
    .digest('hex')
const layoutKey = (layout) =>
  'height' in layout ? `height-${layout.height}` : `ratio-${layout.ratio.join('-')}`
const sourceFamily = {
  'apple-music': ['itunes.apple.com'],
  bandcamp: ['bandcamp.com'],
  codepen: ['codepen.io'],
  dailymotion: ['dailymotion.com'],
  flickr: ['flickr.com'],
  giphy: ['giphy.com'],
  jsfiddle: ['jsfiddle.net'],
  livecodes: ['livecodes.io'],
  loom: ['loom.com'],
  mixcloud: ['mixcloud.com'],
  slideshare: ['slideshare.net'],
  soundcloud: ['soundcloud.com'],
  speakerdeck: ['speakerdeck.com'],
  spotify: ['spotify.com'],
  ted: ['ted.com'],
  twitch: ['twitch.tv'],
  vimeo: ['vimeo.com'],
  wistia: ['wistia.com'],
  xkcd: ['xkcd.com'],
  youtube: ['youtube.com'],
  art19: ['art19.com'],
  libsyn: ['libsyn.com'],
  simplecast: ['simplecast.com'],
  geogebra: ['geogebra.org'],
  documentcloud: ['documentcloud.org'],
  prezi: ['prezi.com'],
  scribd: ['scribd.com'],
}
const candidates = [...catalog.fixtures, ...catalog.examples, ...catalog.commentedExamples].filter(
  (fixture) => {
    try {
      return !fixture.url.includes('...') && !new URL(fixture.url).pathname.includes('/s-')
    } catch {
      return false
    }
  },
)
const preferred = new Set(catalog.reviewCandidates.map((fixture) => fixture.url))
const score = (fixture) =>
  (preferred.has(fixture.url) ? -100 : 0) +
  (fixture.kind === 'static-test' ? 0 : fixture.kind === 'unit-test-target' ? 5 : 30) +
  (fixture.url.startsWith('http:') ? 3 : 0) +
  (/(?:claude-debussy|back-to-the-80s|lynnandtonic|datareportal)/i.test(fixture.url) ? -5 : 0) +
  (fixture.url.includes('itunes.apple.com') ? 1 : 0)
const sorted = candidates
  .map((fixture, order) => ({ ...fixture, order }))
  .sort((a, b) => score(a) - score(b) || a.order - b.order)
const queue = []
const gaps = []
const coverage = []

function inputFor(provider, fixture) {
  // Gallery is selected from trusted attachment metadata, never from the URL alone.
  return provider.id === 'gallery' ? { attachmentKind: 'gallery' } : {}
}
function selectedCase(provider, fixture) {
  const input = inputFor(provider, fixture)
  const selected = selectEmbed(fixture.url, input.attachmentKind, input.contentType)
  return { input, selected }
}
function variantIndex(provider, url) {
  return (
    provider.variants?.findIndex((variant) =>
      variant.matches.some((rule) => embedMatches(rule, new URL(url))),
    ) ?? -1
  )
}
function addCase(provider, label, variant, predicate = () => true) {
  const fixture = sorted.find((fixture) => {
    if (!predicate(fixture)) return false
    if (provider.id === 'gallery' && !/\.(?:png|jpe?g|webp)(?:$|\?)/i.test(fixture.url))
      return false
    const { selected } = selectedCase(provider, fixture)
    return (
      selected.valid &&
      selected.provider.id === provider.id &&
      variantIndex(provider, fixture.url) === variant
    )
  })
  if (!fixture) {
    gaps.push({
      provider: provider.id,
      component: provider.component,
      case: label,
      reason: 'No actual upstream URL in the data catalog selects this implemented case',
    })
    return null
  }
  const { input, selected } = selectedCase(provider, fixture)
  const record = {
    id: `${provider.id}:${label}`,
    provider: provider.id,
    component: provider.component,
    case: label,
    variantIndex: variant,
    layout: selected.layout,
    layoutKey: layoutKey(selected.layout),
    url: fixture.url,
    input,
    selectedProvider: selected.provider.id,
    postableAsUrl: provider.id !== 'gallery',
    source: {
      path: fixture.source,
      line: fixture.line,
      kind: fixture.kind,
      revision: catalog.provenance.revision,
    },
    contentStatus: 'Unverified; likely subjects are inferred from upstream source context only',
    reviewStatus: 'Pending parent visible review',
    ...(provider.id === 'gallery'
      ? {
          note: 'Requires a trusted gallery attachment. Posting this URL alone selects another adapter.',
        }
      : {}),
    ...(fixture.kind.startsWith('commented')
      ? { note: 'Upstream commented example; review its source context before posting.' }
      : {}),
  }
  queue.push(record)
  return record.id
}

for (const provider of providers) {
  const base = addCase(
    provider,
    'base',
    -1,
    provider.id === 'fallback'
      ? (fixture) => fixture.url === 'https://iframely.com'
      : provider.id === 'file'
        ? (fixture) => /\.pdf(?:$|\?)/i.test(fixture.url)
        : () => true,
  )
  const variants = (provider.variants || []).map((variant, index) => ({
    index,
    layout: variant.layout,
    caseId: addCase(provider, `variant-${index}-${layoutKey(variant.layout)}`, index),
  }))
  const matched = catalog.fixtures.filter(
    (fixture) => selectEmbed(fixture.url).provider.id === provider.id,
  )
  coverage.push({
    provider: provider.id,
    component: provider.component,
    baseLayout: provider.layout,
    baseCaseId: base,
    variants,
    matchingActiveUpstreamFixtures: matched.length,
  })
}

const youtube = providers.find((provider) => provider.id === 'youtube')
if (youtube?.matches?.some((match) => match.paths?.includes('/shorts/*'))) {
  addCase(youtube, 'shorts-route', -1, (fixture) =>
    new URL(fixture.url).pathname.startsWith('/shorts/'),
  )
}
const unmatchedUpstreamCases = catalog.fixtures.flatMap((fixture) => {
  const expected = providers.find((provider) =>
    sourceFamily[provider.id]?.includes(fixture.provider),
  )
  if (!expected) return []
  const selected = selectEmbed(fixture.url)
  if (selected.provider.id === expected.id) return []
  return [
    {
      expectedProvider: expected.id,
      selectedProvider: selected.provider.id,
      url: fixture.url,
      source: fixture.source,
      line: fixture.line,
      reason:
        'Actual source fixture does not select the adapter associated with its upstream family',
    },
  ]
})

const result = {
  schemaVersion: 1,
  provenance: {
    revision: catalog.provenance.revision,
    sourceCatalog: 'frontend/shared/embed-fixtures/iframely.json',
    selector: 'frontend/app/utils/embed.ts',
    selectorSha256: await hash(resolve(frontend, 'app/utils/embed.ts')),
    generatedManifest: 'frontend/.nuxt/embed-manifest.ts',
    generatedManifestSha256: await hash(generatedPath),
    declarations: await Promise.all(
      providers.map(async (provider) => ({
        component: provider.component,
        sha256: await hash(resolve(frontend, 'app/components/embed', provider.component)),
      })),
    ),
  },
  method: {
    declarations:
      'Actual discoverEmbeds static SFC extractor, compared with the existing Nuxt generated manifest',
    selection: 'Actual selectEmbed with a read-only Bun alias to the existing generated manifest',
    componentScriptsExecuted: false,
    lazyLoadersCalled: false,
    upstreamCodeExecuted: false,
    networkRequests: 0,
    note: 'This is a source-selected queue, not a record of functioning providers. Parent owns paced posting and visible verification.',
  },
  summary: {
    providerCount: providers.length,
    providersWithQueuedCases: new Set(queue.map((record) => record.provider)).size,
    queuedCases: queue.length,
    postableUrlCases: queue.filter((record) => record.postableAsUrl).length,
    implementedLayoutCases: providers.reduce(
      (count, provider) => count + 1 + (provider.variants?.length || 0),
      0,
    ),
    missingFixtureProviderIds: coverage
      .filter((record) => !record.baseCaseId && record.variants.every((variant) => !variant.caseId))
      .map((record) => record.provider),
    gapCount: gaps.length,
    unmatchedActiveUpstreamCases: unmatchedUpstreamCases.length,
  },
  queue,
  coverage,
  gaps,
  unmatchedUpstreamCases,
}
const output = resolve(frontend, '../.data/embed-review/queue.json')
await mkdir(dirname(output), { recursive: true })
await writeFile(output, `${JSON.stringify(result, null, 2)}\n`)
console.log(JSON.stringify({ output, ...result.summary, gaps }, null, 2))
