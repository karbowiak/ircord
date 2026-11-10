#!/usr/bin/env node

// Parse the pinned Iframely checkout as data. Never import or execute its code.
import { createHash } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { babelParse } from '@vue/compiler-sfc'

const revision = '962bf9287f2ab5880ff96d4ac796ebb62217be57'
const repository = 'https://github.com/itteco/iframely'
const frontendRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const sourceRoot = resolve(
  frontendRoot,
  process.argv.find((arg) => arg.startsWith('--source='))?.slice(9) ||
    '../.data/references/iframely',
)
const output = resolve(frontendRoot, 'shared/embed-fixtures/iframely.json')
const check = process.argv.includes('--check')
const git = (...args) =>
  execFileSync('git', ['-C', sourceRoot, ...args], { encoding: 'utf8' }).trim()

if (git('rev-parse', 'HEAD') !== revision) throw new Error(`Expected Iframely revision ${revision}`)
if (git('status', '--porcelain')) throw new Error('The Iframely reference checkout must be clean')

const paths = git('ls-files', '-z').split('\0').filter(Boolean).sort()
const sources = []
const fixtures = []
const declarations = []
const examples = []
const commentedExamples = []
const dynamicTests = []
const dynamicUrls = []
const withheldUrls = []

const familyGroups = [
  [
    'standard-oembed-player',
    'Review the iframe URL, allowed origin, and fixed layout before adapting.',
    'art19.com cartodb.com cnevids.com codepen.io documentcloud.org issuu.com itunes.apple.com livecodes.io loom.com mixcloud.com polldaddy.com slideshare.net soundcloud.com speakerdeck.com spotify.com ted.com vimeo.com widgetic.com wistia.com',
  ],
  [
    'native-photo-or-media',
    'Use a native image or media adapter when a public source URL is available.',
    '500px d.pr flickr.com giphy.com i.giphy.com imageshack.com smugmug.com xkcd.com',
  ],
  [
    'controlled-custom-iframe',
    'URL conversion is reusable; allowlist the player origin and reserve a fixed layout.',
    'abcnews.com cbc.ca dbdiagram.io espn.com geogebra.org google.com hockeydb.com imdb.com jsfiddle.net knightlab libsyn.com maps.yandex.ru momindum.com openstreetmap.org padlet.com pastebin.com piktochart.com prezi.com quizlet.com readymag.com simplecast.com slid.es twitch.tv v.qq.com vbox7.com washingtonpost.com youku.com',
  ],
  [
    'custom-html-or-metadata',
    'Needs reviewed metadata extraction or HTML adaptation; generic metadata remains the fallback.',
    'bandcamp.com buzzfeed.com dailymail.com github.gist kickstarter.com mail.ru npr.org nytimes.video pinterest.com rapgenius.com tmz.com twitter.com usatoday.com',
  ],
  [
    'credentials-or-private-path',
    'Do not copy API credentials, private fetches, or account-specific bypasses.',
    'dailymotion.com facebook.com instagram.com scribd.com tumblr.com youtube.com',
  ],
  [
    'support-only',
    'This source supports routing or policy; it does not define a standalone rich provider.',
    'ignore spotify.com-meta-fallback usatoday-allowEmbedURL vimeo-moogaloop',
  ],
]

const familyOverrides = {
  'dailymotion.com':
    'Upstream requires a partner Player ID. Its main video tests are commented out for that reason.',
  'facebook.com':
    'Graph oEmbed integration and upstream access workarounds need credentials and separate review.',
  'instagram.com':
    'Graph oEmbed integration and metadata/rate-limit workarounds need separate review. IRCord keeps its extractor inactive.',
  'scribd.com':
    'Public iframe adaptation can be reviewed separately. The error plugin fetches private document access keys; exclude that path.',
  'tumblr.com':
    'The upstream API mixin requires tumblr.consumer_key. Public oEmbed or image handling must avoid that dependency.',
  'youtube.com':
    'The upstream video/channel data path requires youtube.api_key. Public player URL conversion can remain independent.',
  'google.com':
    'Docs, Calendar, and custom maps have public iframe paths. Maps and Street View API paths require an owner-supplied key.',
  'twitter.com':
    'Upstream returns script-backed blockquote HTML. Do not inject it into the IRCord application document.',
  'pinterest.com':
    'Upstream includes a widget template. Review an isolated controlled iframe before adapting.',
  'github.gist':
    'Upstream fetches and transforms Gist HTML. Keep arbitrary scripts out of the application document.',
  '500px':
    'Upstream uses a custom image HTML template and Twitter metadata; prefer a native image adapter.',
  'maps.yandex.ru': 'Upstream uses a custom HTML template. Reserve a fixed map layout if adapting.',
}

function familyFor(path) {
  if (!path.startsWith('plugins/domains/')) return null
  return path
    .slice('plugins/domains/'.length)
    .split('/')[0]
    .replace(/\.(?:js|ejs)$/, '')
}

function classificationFor(path) {
  const family = familyFor(path)
  if (!family) return null
  const group = familyGroups.find(([, , names]) => names.split(' ').includes(family))
  if (!group) throw new Error(`Classify domain family ${family}`)
  return {
    family,
    category: group[0],
    review: familyOverrides[family] || group[1],
    runtimeSupport: 'inventory-only',
  }
}

function keyOf(node) {
  return node?.key?.name || node?.key?.value
}

function walk(node, visit, ancestors = []) {
  if (!node || typeof node !== 'object' || typeof node.type !== 'string') return
  visit(node, ancestors)
  for (const [key, value] of Object.entries(node)) {
    if (
      [
        'loc',
        'comments',
        'leadingComments',
        'trailingComments',
        'innerComments',
        'tokens',
      ].includes(key)
    )
      continue
    if (Array.isArray(value)) value.forEach((child) => walk(child, visit, [...ancestors, node]))
    else if (value && typeof value === 'object') walk(value, visit, [...ancestors, node])
  }
}

function staticValue(node) {
  if (!node) return null
  if (['StringLiteral', 'NumericLiteral', 'BooleanLiteral'].includes(node.type)) return node.value
  if (node.type === 'NullLiteral') return null
  if (node.type === 'RegExpLiteral') return { pattern: node.pattern, flags: node.flags }
  if (node.type === 'TemplateLiteral' && !node.expressions.length)
    return node.quasis[0].value.cooked
  if (node.type === 'ArrayExpression') return node.elements.map(staticValue)
  if (node.type === 'ObjectExpression') {
    return Object.fromEntries(
      node.properties.map((property, index) => [
        keyOf(property) || `dynamic_${index}`,
        staticValue(property.value || property),
      ]),
    )
  }
  if (
    node.type === 'UnaryExpression' &&
    ['+', '-'].includes(node.operator) &&
    node.argument.type === 'NumericLiteral'
  ) {
    return node.operator === '-' ? -node.argument.value : node.argument.value
  }
  return { dynamic: node.type, line: node.loc.start.line, reason: 'Not evaluated' }
}

function urlValue(node) {
  if (node.type === 'StringLiteral') return node.value
  if (node.type === 'TemplateLiteral' && !node.expressions.length)
    return node.quasis[0].value.cooked
  return null
}

function cleanUrl(value, source, line) {
  if (!/^https?:\/\//i.test(value)) return null
  let parsed
  try {
    parsed = new URL(value)
  } catch {
    return { url: value, urlType: 'incomplete-url-literal' }
  }
  const credentialKey =
    /^(?:api[_-]?key|consumer[_-]?key|access[_-]?key|access[_-]?token|auth[_-]?token|secret(?:[_-]?(?:password|token))?|client[_-]?secret|password|signature|token)$/i
  const credentialKeys = (url, depth = 0) => [
    ...[...url.searchParams.keys()].filter((key) => credentialKey.test(key)),
    ...[...url.searchParams.values()].flatMap((nested) => {
      if (depth >= 3 || !/^https?:\/\//i.test(nested)) return []
      try {
        return credentialKeys(new URL(nested), depth + 1)
      } catch {
        return []
      }
    }),
  ]
  const sensitiveKeys = credentialKeys(parsed)
  if (parsed.username || parsed.password || sensitiveKeys.length) {
    withheldUrls.push({
      source,
      line,
      reason: 'Credential-like URL value omitted',
      queryKeys: sensitiveKeys,
    })
    return null
  }
  const pathname = parsed.pathname.toLowerCase()
  const urlType = /(?:oembed|oembed\.json)/i.test(value)
    ? 'oembed-endpoint'
    : /(?:\/rss\/?$|\/feeds?\/|\.rss$)/i.test(pathname)
      ? 'feed-url'
      : /\.(?:jpe?g|png|webp|gif|avif|svg|mp4|webm|mp3|ogg|m4a|wav)(?:$|[?#])/i.test(value)
        ? 'native-media-url'
        : /(?:\/embed(?:\/|$)|\/player(?:\/|\.|$)|^player\.)/i.test(`${parsed.hostname}${pathname}`)
          ? 'player-url'
          : /(?:^api\.|\/api\/|googleapis\.com)/i.test(`${parsed.hostname}${pathname}`)
            ? 'api-url'
            : /^(?:youtu\.be|flic\.kr|dai\.ly|t\.co|mol\.im|d\.pr|goo\.gl|maps\.app\.goo\.gl)$/.test(
                  parsed.hostname,
                )
              ? 'short-url'
              : 'page-url'
  return { url: value, urlType }
}

function groupFor(path) {
  for (const prefix of [
    'plugins/domains/',
    'plugins/links/',
    'plugins/meta/',
    'plugins/custom/',
    'lib/plugins/system/',
    'lib/plugins/validators/',
    'test/',
    'modules/tests-ui/',
    'whitelist/',
    'docs/',
    'static/',
  ]) {
    if (path.startsWith(prefix)) return prefix.slice(0, -1)
  }
  return 'runtime-and-config'
}

for (const path of paths) {
  const content = await readFile(resolve(sourceRoot, path))
  const source = {
    path,
    sha256: createHash('sha256').update(content).digest('hex'),
    group: groupFor(path),
    ...classificationFor(path),
  }
  sources.push(source)
  if (!path.endsWith('.js') || path.startsWith('static/')) {
    source.extraction = path.startsWith('static/')
      ? 'Runtime assets inventoried; no runtime copied'
      : 'Non-JavaScript source inventoried'
    continue
  }

  const text = content.toString('utf8')
  const ast = babelParse(text, { sourceType: 'unambiguous', attachComment: true })
  source.extraction = 'Parsed with Babel; no source executed'
  const declaration = ast.program.body.find(
    (node) => node.type === 'ExportDefaultDeclaration',
  )?.declaration
  const properties = declaration?.type === 'ObjectExpression' ? declaration.properties : []
  const tests = properties.find((node) => keyOf(node) === 'tests')?.value
  source.mixins = staticValue(properties.find((node) => keyOf(node) === 'mixins')?.value)
  source.matches = staticValue(properties.find((node) => keyOf(node) === 're')?.value)
  source.methods = properties
    .filter(
      (node) =>
        node.type === 'ObjectMethod' ||
        ['FunctionExpression', 'ArrowFunctionExpression'].includes(node.value?.type),
    )
    .map((node) => ({
      name: keyOf(node),
      parameters: (node.params || node.value.params).map(
        (parameter) => parameter.name || parameter.type,
      ),
      line: node.loc.start.line,
    }))
  source.options = []
  source.behaviors = []
  source.testFixtureCount = 0

  if (tests) {
    const entries = tests.type === 'ArrayExpression' ? tests.elements : [tests]
    for (const [index, node] of entries.entries()) {
      if (!node) continue
      const value = urlValue(node)
      if (typeof value === 'string') {
        const url = cleanUrl(value, path, node.loc.start.line)
        if (url) {
          fixtures.push({
            provider: source.family || path,
            source: path,
            line: node.loc.start.line,
            testIndex: index,
            ...url,
            kind: 'static-test',
            liveStatus: 'unverified',
          })
          source.testFixtureCount++
        }
      } else {
        const options = staticValue(node)
        declarations.push({
          provider: source.family || path,
          source: path,
          line: node.loc.start.line,
          testIndex: index,
          value: options,
        })
        const dynamicKeys =
          node.properties
            ?.map(keyOf)
            .filter((key) =>
              ['feed', 'pageWithFeed', 'page', 'getUrl', 'getUrls', 'selector'].includes(key),
            ) || []
        if (dynamicKeys.length)
          dynamicTests.push({
            provider: source.family || path,
            source: path,
            line: node.loc.start.line,
            testIndex: index,
            inputs: options,
            reason: 'Feed, selector, or function requires execution or fetching; not expanded',
          })
      }
    }
  }

  walk(ast, (node, ancestors) => {
    if (node.type === 'CallExpression' && node.callee.type === 'MemberExpression') {
      const name = node.callee.property.name
      if (
        ['getProviderOptions', 'getRequestOptions'].includes(name) &&
        node.arguments[0]?.type === 'StringLiteral'
      ) {
        source.options.push({
          kind: name,
          key: node.arguments[0].value,
          default: staticValue(node.arguments[1]),
          line: node.loc.start.line,
        })
      }
    }
    if (node.type === 'AssignmentExpression' && node.left.type === 'MemberExpression') {
      const property = node.left.property.name || node.left.property.value
      if (
        [
          'headers',
          'followHTTPRedirect',
          'exposeStatusCode',
          'user_agent',
          'userAgent',
          'timeout',
        ].includes(property)
      ) {
        source.behaviors.push({
          property,
          line: node.loc.start.line,
          value: staticValue(node.right),
        })
      }
    }
    if (
      (node.type === 'ObjectProperty' && ['canonical', 'redirect'].includes(keyOf(node))) ||
      (node.type === 'AssignmentExpression' &&
        ['canonical', 'redirect'].includes(node.left.property?.name))
    ) {
      const value = node.value || node.right
      if (urlValue(value) === null)
        dynamicUrls.push({
          source: path,
          line: node.loc.start.line,
          kind: keyOf(node) || node.left.property.name,
          expression: value.type,
          reason: 'Derived URL; not evaluated',
        })
    }
    const value = urlValue(node)
    if (typeof value !== 'string') return
    const inTests = tests && node.start >= tests.start && node.end <= tests.end
    if (inTests && node.type !== 'TemplateLiteral' && ancestors.at(-1) === tests) return
    const url = cleanUrl(value, path, node.loc.start.line)
    if (!url) return
    const property = [...ancestors].reverse().find((parent) => parent.type === 'ObjectProperty')
    const kind = inTests
      ? 'dynamic-test-seed'
      : ['canonical', 'redirect'].includes(keyOf(property))
        ? `${keyOf(property)}-literal`
        : path.startsWith('test/')
          ? 'unit-test-target'
          : 'source-literal'
    examples.push({
      provider: source.family || path,
      source: path,
      line: node.loc.start.line,
      ...url,
      kind,
      liveStatus: 'unverified',
    })
  })

  for (const comment of ast.comments || []) {
    const inTests = tests && comment.start >= tests.start && comment.end <= tests.end
    const commentedTests = /\btests\s*:/.test(comment.value)
    // URLs are extracted from parser-identified comments, never used to parse JavaScript.
    const urls = [...comment.value.matchAll(/https?:\/\/[^\s<>"'`]+/g)]
    for (const match of urls) {
      const commentLine = comment.value.slice(0, match.index).split('\n').length - 1
      const line = comment.loc.start.line + commentLine
      const url = cleanUrl(match[0], path, line)
      if (!url) continue
      const context = comment.value
        .split('\n')
        .slice(Math.max(0, commentLine - 1), commentLine + 2)
        .join('\n')
        .trim()
      const upstreamReason = commentedTests
        ? comment.value
            .split('\n')
            .find((value) => value.trim())
            ?.trim()
        : null
      commentedExamples.push({
        provider: source.family || path,
        source: path,
        line,
        ...url,
        kind:
          inTests || commentedTests
            ? 'commented-test-example'
            : /canonical|redirect/i.test(comment.value)
              ? 'commented-canonical-or-redirect-example'
              : 'commented-source-example',
        context,
        liveStatus: 'unverified',
        reason: 'Comment only; not an active static test',
        ...(upstreamReason && !upstreamReason.startsWith('tests:') ? { upstreamReason } : {}),
      })
    }
  }
}

const registryPath = 'lib/plugins/system/oembed/providers.json'
const registry = JSON.parse(await readFile(resolve(sourceRoot, registryPath), 'utf8'))
const oembedProviders = registry.map((provider, index) => ({
  source: registryPath,
  index,
  ...provider,
  runtimeSupport: 'registry-only; endpoint and iframe must be reviewed before adoption',
  liveStatus: 'unverified',
}))

const reviewCandidateUrls = [
  ['code-demo', 'https://livecodes.io/?template=react'],
  ['code-demo', 'https://jsfiddle.net/pborreli/pJgyu/'],
  ['code-demo', 'http://codepen.io/kevinjannis/pen/pyuix'],
  ['presentation', 'https://speakerdeck.com/lynnandtonic/art-the-web-and-tiny-ux'],
  [
    'presentation',
    'https://www.slideshare.net/DataReportal/digital-2020-global-digital-overview-january-2020-v01-226017535',
  ],
  ['educational-interactive', 'https://www.geogebra.org/m/141300'],
  ['public-document', 'https://www.documentcloud.org/documents/73991-day-three-documents'],
  ['educational-presentation', 'https://prezi.com/hvsanqexuoza/designthinking-vs-leanstartup/'],
  [
    'public-document',
    'https://www.scribd.com/doc/116154615/Australia-Council-Arts-Funding-Guide-2013',
  ],
  ['comic', 'https://xkcd.com/1392/'],
  ['art-gif', 'https://giphy.com/gifs/art-artists-on-tumblr-design-uRmDTQDgYxSzS'],
  ['photo', 'https://www.flickr.com/photos/marshal-banana/23869537421'],
  [
    'product-podcast',
    'https://art19.com/shows/intercom-on-product-management/episodes/5801ff9e-51ba-4e15-a284-5069751c6bed',
  ],
  ['podcast', 'https://directory.libsyn.com/episode/index/id/3252958'],
  [
    'design-podcast',
    'https://tgd.simplecast.com/episodes/dan-blackman-and-robyn-kanner-the-power-7fe152f4',
  ],
  ['music-album', 'https://music.apple.com/jp/album/back-to-the-80s/1458246986'],
  ['video', 'https://appsumo.wistia.com/medias/fudkgyoejs'],
  ['video', 'https://www.loom.com/share/0808e55f94a24992b2de488ac56d3815'],
]
const reviewCandidates = reviewCandidateUrls.map(([subject, url]) => {
  const fixture = fixtures.find((fixture) => fixture.url === url)
  if (!fixture) throw new Error(`Review candidate is absent from static fixtures: ${url}`)
  return {
    ...fixture,
    subject,
    suitability: 'Likely suitable from source context; content and liveness need visible review',
  }
})

function countBy(items, getKey) {
  return Object.fromEntries(
    [
      ...items.reduce(
        (counts, item) => counts.set(getKey(item), (counts.get(getKey(item)) || 0) + 1),
        new Map(),
      ),
    ].sort(([a], [b]) => a.localeCompare(b)),
  )
}

const manifest = {
  schemaVersion: 1,
  provenance: {
    repository,
    revision,
    tree: git('rev-parse', 'HEAD^{tree}'),
    sourceUrl: `${repository}/tree/${revision}`,
    license: 'MIT',
    copyright: 'Copyright (c) 2013-2022 Itteco Software Corp.',
    licenseSource: `${repository}/blob/${revision}/LICENSE`,
    licenseText: await readFile(resolve(sourceRoot, 'LICENSE'), 'utf8'),
  },
  extraction: {
    command: 'node scripts/import-iframely-fixtures.mjs',
    parser: '@vue/compiler-sfc babelParse',
    upstreamCodeExecuted: false,
    networkRequests: 0,
    scope:
      'Every tracked source is inventoried. All non-static JavaScript files are parsed. Static test literals, declarations, other URL literals, parser comments, and the complete known oEmbed registry are retained separately.',
    exclusions: [
      'No upstream npm scripts, plugin functions, feeds, selectors, or getUrls callbacks run.',
      'Static browser assets are inventoried by hash; their runtime is not copied.',
      'Credential-like URL values are omitted.',
      'The hosted QA domain allowlist is absent from the source tree and is not fetched.',
      'Liveness and content suitability are not inferred from source presence.',
    ],
  },
  counts: {
    trackedSources: sources.length,
    parsedJavaScriptSources: sources.filter((source) => source.extraction.startsWith('Parsed'))
      .length,
    domainJavaScriptSources: sources.filter(
      (source) => source.path.startsWith('plugins/domains/') && source.path.endsWith('.js'),
    ).length,
    domainTemplates: sources.filter(
      (source) => source.path.startsWith('plugins/domains/') && source.path.endsWith('.ejs'),
    ).length,
    domainFamilies: new Set(
      sources.filter((source) => source.family).map((source) => source.family),
    ).size,
    staticTestFixtures: fixtures.length,
    distinctStaticTestUrls: new Set(fixtures.map((fixture) => fixture.url)).size,
    sourcesWithStaticTests: sources.filter((source) => source.testFixtureCount > 0).length,
    testDeclarations: declarations.length,
    dynamicTestDeclarations: dynamicTests.length,
    otherUrlLiterals: examples.length,
    commentedUrlExamples: commentedExamples.length,
    commentedTestExamples: commentedExamples.filter(
      (example) => example.kind === 'commented-test-example',
    ).length,
    derivedCanonicalOrRedirectExpressions: dynamicUrls.length,
    withheldUrlValues: withheldUrls.length,
    oembedRegistryRecords: oembedProviders.length,
    verifiedLiveFixtures: 0,
    verifiedDeadFixtures: 0,
    initialReviewCandidates: reviewCandidates.length,
    sourcesByGroup: countBy(sources, (source) => source.group),
    domainSourcesByCategory: countBy(
      sources.filter((source) => source.family),
      (source) => source.category,
    ),
    fixtureUrlTypes: countBy(fixtures, (fixture) => fixture.urlType),
  },
  sources,
  fixtures,
  reviewCandidates,
  testDeclarations: declarations,
  dynamicTests,
  examples,
  commentedExamples,
  derivedUrls: dynamicUrls,
  withheldUrls,
  oembedProviders,
}

const serialized = `${JSON.stringify(manifest, null, 2)}\n`
if (check) {
  if ((await readFile(output, 'utf8')) !== serialized)
    throw new Error('Iframely fixture manifest differs; regenerate it')
  console.log('Iframely fixture manifest matches the pinned source')
} else {
  await mkdir(dirname(output), { recursive: true })
  await writeFile(output, serialized)
  console.log(`Wrote ${fixtures.length} static fixtures and ${sources.length} source records`)
}
console.log(JSON.stringify(manifest.counts, null, 2))
