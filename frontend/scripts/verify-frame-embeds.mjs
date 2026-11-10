#!/usr/bin/env bun

// Read real pinned fixtures, extract static declarations and compile six SFCs.
// This does not contact providers, execute upstream code or start servers.
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
  name: 'controlled-frame-static-manifest',
  setup(builder) {
    builder.onResolve({ filter: /^#embed-manifest$/ }, () => ({
      path: 'manifest',
      namespace: 'frame-check',
    }))
    builder.onLoad({ filter: /.*/, namespace: 'frame-check' }, () => ({
      contents: `export const embedProviders = ${JSON.stringify(providers)}`,
      loader: 'js',
    }))
  },
})
const { selectEmbed, embedMatches, embedContainerStyle } = await import('../app/utils/embed.ts')
const catalog = JSON.parse(
  await readFile(resolve(frontend, 'shared/embed-fixtures/iframely.json'), 'utf8'),
)
const families = {
  'dbdiagram.io': ['dbdiagram', 'DBDiagram.vue', { ratio: [16, 9] }],
  'slid.es': ['slides', 'Slides.vue', { ratio: [48, 35] }],
  'quizlet.com': ['quizlet', 'Quizlet.vue', { height: 500 }],
  'openstreetmap.org': ['openstreetmap', 'OpenStreetMap.vue', { ratio: [4, 3] }],
  'maps.yandex.ru': ['yandex-maps', 'YandexMaps.vue', { ratio: [4, 3] }],
  'padlet.com': ['padlet', 'Padlet.vue', { height: 608 }],
}
const fixtures = catalog.fixtures.filter((item) => families[item.provider])
assert.equal(fixtures.length, 25)
for (const fixture of fixtures) {
  const [id, component, layout] = families[fixture.provider]
  const selection = selectEmbed(fixture.url)
  assert.equal(selection.provider.id, id, fixture.url)
  assert.equal(selection.provider.component, component)
  assert.deepEqual(selection.layout, layout)
  assert.equal(selection.provider.imageSizing, undefined)
  assert.deepEqual(embedContainerStyle(selection), { width: '100%', maxWidth: '480px' })
  assert.equal(
    selection.provider.frames.some((rule) =>
      embedMatches(rule, new URL('https://evil.example/embed/id')),
    ),
    false,
  )
}
for (const [, component] of Object.values(families)) {
  const filename = resolve(frontend, 'app/components/embed', component)
  const { descriptor, errors } = parse(await readFile(filename, 'utf8'), { filename })
  assert.deepEqual(errors, [])
  const script = compileScript(descriptor, { id: component })
  const template = compileTemplate({
    filename,
    id: component,
    source: descriptor.template.content,
    compilerOptions: { bindingMetadata: script.bindings },
  })
  assert.deepEqual(template.errors, [])
}
for (const url of [
  'https://slides.com.evil.example/user/deck',
  'https://blog.slides.com/post/84828911898',
  'https://dbdiagram.io/account',
  'https://yandex.ru/login',
]) {
  assert.equal(selectEmbed(url).provider.id, 'fallback')
}
console.log(
  'B2: 25 real upstream fixtures select six fixed layouts; 480px bounds, imageSizing exclusion, frame-host policies and six SFC compilations passed (source selection is not live provider parity).',
)
