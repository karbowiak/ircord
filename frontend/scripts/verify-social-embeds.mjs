#!/usr/bin/env bun
// Static matching and component compilation; no external requests or application state.
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
  name: 'social-embed-static-manifest',
  setup(builder) {
    builder.onResolve({ filter: /^#embed-manifest$/ }, () => ({
      path: 'manifest',
      namespace: 'social-embed-static',
    }))
    builder.onLoad({ filter: /.*/, namespace: 'social-embed-static' }, () => ({
      contents: `export const embedProviders = ${JSON.stringify(providers)}`,
      loader: 'js',
    }))
  },
})
const { selectEmbed, embedContainerStyle, embedMatches } = await import('../app/utils/embed.ts')
const cases = [
  ['https://www.instagram.com/reel/DfX9cZwS4DI/?xtok=example', 'instagram', 500],
  [
    'https://www.instagram.com/p/DfZcglGlDYM/?utm_source=ig_web_copy_link&rpxt=example',
    'instagram',
    420,
  ],
  [
    'https://www.instagram.com/p/DfTNU4EHeRc/?utm_source=ig_web_copy_link&psln=example',
    'instagram',
    420,
  ],
  ['https://www.instagram.com/p/DXBJTJiDvgY/', 'instagram', 420],
  [
    'https://www.tiktok.com/@zaylento.yt/video/7703024247081291040?is_from_webapp=1&sender_device=pc',
    'tiktok',
    578,
  ],
  [
    'https://www.tiktok.com/@john.erik.wagner1/video/7696385281938754838?is_from_webapp=1&sender_device=pc',
    'tiktok',
    578,
  ],
]
for (const [url, provider, height] of cases) {
  const selected = selectEmbed(url)
  assert.equal(selected.provider.id, provider, url)
  assert.deepEqual(selected.layout, { height }, url)
  assert.deepEqual(embedContainerStyle(selected), { width: '100%', maxWidth: '480px' })
  assert(selected.provider.accepts.includes('media'))
  assert.equal(selected.provider.imageSizing, undefined)
  assert(
    !selected.provider.frames.some((rule) =>
      embedMatches(rule, new URL('https://evil.example/player/v1/123')),
    ),
  )
}
for (const raw of [
  'https://www.instagram.com/accounts/login/',
  'https://www.instagram.com/some.profile/',
  'https://www.instagram.com.evil.example/p/DXBJTJiDvgY/',
  'https://www.tiktok.com/@someone/',
  'https://www.tiktok.com.evil.example/@someone/video/123',
])
  assert.equal(selectEmbed(raw).provider.id, 'fallback')

for (const name of ['EmbedSocialRenderer.vue', 'embed/Instagram.vue', 'embed/TikTok.vue']) {
  const filename = resolve(frontend, 'app/components', name)
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
  'Instagram/TikTok: all six supplied URLs select fixed layouts; three components compile; unrelated hosts and routes remain fallback.',
)
