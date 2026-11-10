#!/usr/bin/env bun

// Static source/layout checks and production Vapor compilation only.
// No provider requests, browser sessions, upstream code or servers are used.
import assert from 'node:assert/strict'
import { plugin } from 'bun'
import { readFile } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { compileScript, compileTemplate, parse } from 'vue/compiler-sfc'
import { discoverEmbeds } from '../nuxt-modules/embed-declarations.ts'

const frontend = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const providers = await discoverEmbeds(resolve(frontend, 'app/components/embed'))
plugin({
  name: 'oembed-apps-manifest',
  setup(builder) {
    builder.onResolve({ filter: /^#embed-manifest$/ }, () => ({
      path: 'manifest',
      namespace: 'oembed-apps',
    }))
    builder.onLoad({ filter: /.*/, namespace: 'oembed-apps' }, () => ({
      contents: `export const embedProviders = ${JSON.stringify(providers)}`,
      loader: 'js',
    }))
  },
})
const { selectEmbed, embedMatches, embedContainerStyle } = await import('../app/utils/embed.ts')
const families = {
  'cnevids.com': ['cnevids', 'CNE.vue', [16, 9]],
  'issuu.com': ['issuu', 'Issuu.vue', [4, 3]],
  'cartodb.com': ['carto', 'Carto.vue', [4, 3]],
}
const catalog = JSON.parse(
  await readFile(resolve(frontend, 'shared/embed-fixtures/iframely.json'), 'utf8'),
)
const fixtures = catalog.fixtures.filter((item) => families[item.provider])
assert.equal(fixtures.length, 11)
for (const fixture of fixtures) {
  const [id, component, ratio] = families[fixture.provider]
  const selected = selectEmbed(fixture.url)
  assert.equal(selected.provider.id, id, fixture.url)
  assert.equal(selected.provider.component, component)
  assert.deepEqual(selected.layout, { ratio })
  assert.equal(selected.provider.imageSizing, undefined)
  assert.deepEqual(embedContainerStyle(selected), { width: '100%', maxWidth: '480px' })
  // This is a layout selection, not a live support claim. The HTTP CNE
  // fixture and CARTO tenant-migration fixture intentionally resolve to cards.
}
const widgetic = selectEmbed('https://widgetic.com/embed/555b5e6509c7e29e718b4569?bp=top-right')
assert.equal(widgetic.provider.id, 'widgetic')
assert.deepEqual(widgetic.layout, { ratio: [1, 1] })
for (const [component, frame, ratio] of [
  [
    'CNE.vue',
    'https://player-backend.cnevids.com/iframe/video/5670377e94c05f43e9000000?autoplay=0',
    [16, 9],
  ],
  [
    'Issuu.vue',
    'https://e.issuu.com/embed.html?u=visitfaroeislands&d=tg25-uk-web_compressed',
    [4, 3],
  ],
  ['Carto.vue', 'https://rpa.carto.com/viz/9a23d690-6e00-4c87-8d06-608d8bf7ca18/embed_map', [4, 3]],
  ['Widgetic.vue', 'https://widgetic.com/embed/555b5e6509c7e29e718b4569?bp=top-right', [1, 1]],
]) {
  const provider = providers.find((p) => p.component === component)
  assert.ok(provider.frames.some((rule) => embedMatches(rule, new URL(frame))))
  assert.equal(
    provider.frames.some((rule) => embedMatches(rule, new URL('https://evil.example/embed/id'))),
    false,
  )
  assert.deepEqual({ ...provider.layout }, { ratio })
  const filename = resolve(frontend, 'app/components/embed', component)
  const source = await readFile(filename, 'utf8')
  const { descriptor, errors } = parse(source, { filename })
  assert.deepEqual(errors, [])
  assert.equal(descriptor.vapor, true)
  assert.match(descriptor.script.content, /import EmbedWidgetRenderer from/)
  const script = compileScript(descriptor, { id: component, isProd: true, inlineTemplate: true })
  assert.match(script.content, /defineVaporComponent/)
  assert.match(script.content, /createComponent\(EmbedWidgetRenderer/)
  assert.doesNotMatch(script.content, /resolveComponent\(["']EmbedWidgetRenderer/)
  const template = compileTemplate({
    source: descriptor.template.content,
    filename,
    id: component,
    isProd: true,
    vapor: true,
    compilerOptions: { bindingMetadata: script.bindings },
  })
  assert.deepEqual(template.errors, [])
}
for (const url of [
  'https://player-other.cnevids.com/iframe/video/5670377e94c05f43e9000000',
  'https://issuu.com/user/account',
  'https://e.issuu.com/embed.html#123/456',
  'https://widgetic.com/widgets/photo/mosaic-gallery/',
  'https://app.widgetic.com/editor',
  'https://clausa.app.carto.com/map/ff76a0cd-fd9c-4893-8c2b-d9c587f2d699',
  'https://rpa.cartodb.com/viz/9a23d690-6e00-4c87-8d06-608d8bf7ca18/embed_map',
  'https://www.kickstarter.com/projects/creator/project',
]) {
  assert.equal(selectEmbed(url).provider.id, 'fallback', url)
}
console.log(
  'B4: 11 pinned URLs and one documented composition select fixed geometry; four production Vapor adapters compile with explicit component bindings; unreviewed families retain fallback layouts.',
)
