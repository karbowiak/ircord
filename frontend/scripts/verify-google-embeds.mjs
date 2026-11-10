#!/usr/bin/env bun

// Static declarations, actual selection and SFC compilation only; no provider requests.
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
  name: 'google-embed-static-manifest',
  setup(builder) {
    builder.onResolve({ filter: /^#embed-manifest$/ }, () => ({
      path: 'manifest',
      namespace: 'google-embed-static',
    }))
    builder.onLoad({ filter: /.*/, namespace: 'google-embed-static' }, () => ({
      contents: `export const embedProviders = ${JSON.stringify(providers)}`,
      loader: 'js',
    }))
  },
})
const { selectEmbed, embedMatches, embedContainerStyle } = await import('../app/utils/embed.ts')
const catalog = JSON.parse(
  await readFile(resolve(frontend, 'shared/embed-fixtures/iframely.json'), 'utf8'),
)
const fixtures = catalog.fixtures.filter((item) => item.provider === 'google.com')
assert.equal(fixtures.length, 17)
for (const fixture of fixtures) {
  const url = new URL(fixture.url)
  const selected = selectEmbed(fixture.url)
  const expected = fixture.source.endsWith('google.calendar.js')
    ? 'google-calendar'
    : fixture.source.endsWith('google.custommap.js')
      ? 'google-maps'
      : 'google-documents'
  assert.equal(selected.provider.id, expected, fixture.url)
  const layout = url.pathname.includes('/presentation/')
    ? { ratio: [16, 9] }
    : url.pathname.includes('/spreadsheets/') || expected !== 'google-documents'
      ? { ratio: [4, 3] }
      : { ratio: [3, 4] }
  assert.deepEqual(selected.layout, layout, fixture.url)
  assert.equal(selected.provider.imageSizing, undefined)
  const style = embedContainerStyle(selected)
  assert.equal(style.width, '100%')
  assert.equal(style.maxWidth, '480px')
  assert.equal(
    selected.provider.frames.some((rule) =>
      embedMatches(rule, new URL('https://evil.example/preview')),
    ),
    false,
  )
}
// Synthetic route shapes cover fixture gaps without pretending they are live content IDs.
for (const [url, layout] of [
  ['https://docs.google.com/forms/d/e/synthetic_form/viewform', { height: 640 }],
  ['https://docs.google.com/drawings/d/synthetic_drawing/edit', { ratio: [4, 3] }],
  ['https://drive.google.com/file/d/synthetic_file/view', { ratio: [3, 4] }],
  [
    'https://docs.google.com/a/example.test/presentation/u/0/d/synthetic_deck/edit',
    { ratio: [16, 9] },
  ],
]) {
  assert.equal(selectEmbed(url).provider.id, 'google-documents')
  assert.deepEqual(selectEmbed(url).layout, layout)
}
for (const url of [
  'https://accounts.google.com/ServiceLogin',
  'https://docs.google.com.evil.example/document/d/synthetic_document/preview',
  'https://www.google.com/maps/place/Stanford/',
  'https://www.google.com/maps/embed/v1/place?key=synthetic_key&q=Stanford',
])
  assert.equal(selectEmbed(url).provider.id, 'fallback')
for (const name of ['GoogleDocuments.vue', 'GoogleCalendar.vue', 'GoogleMaps.vue']) {
  const filename = resolve(frontend, 'app/components/embed', name)
  const { descriptor, errors } = parse(await readFile(filename, 'utf8'), { filename })
  assert.deepEqual(errors, [])
  const script = compileScript(descriptor, { id: name })
  const template = compileTemplate({
    filename,
    id: name,
    source: descriptor.template.content,
    compilerOptions: { bindingMetadata: script.bindings },
  })
  assert.deepEqual(template.errors, [])
}
console.log(
  'Google: 17 real upstream fixtures selected; fixed geometry, synthetic missing route shapes, forbidden hosts/API routes and three SFC compilations passed.',
)
