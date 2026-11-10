#!/usr/bin/env bun

// Real pinned provider URLs; no servers, live provider requests, or message posts.
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { build } from 'bun'
import { compileScript, compileTemplate, parse } from 'vue/compiler-sfc'
import { discoverEmbeds } from '../nuxt-modules/embed-declarations.ts'

const frontend = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const providers = await discoverEmbeds(resolve(frontend, 'app/components/embed'))
const files = ['Droplr.vue', 'DroplrVideo.vue', 'ImageShack.vue', 'SmugMug.vue', 'GiphyClips.vue']
for (const file of files) {
  const filename = resolve(frontend, 'app/components/embed', file)
  const { descriptor, errors } = parse(await readFile(filename, 'utf8'), { filename })
  assert.deepEqual(errors, [], file)
  const script = compileScript(descriptor, { id: filename })
  const template = compileTemplate({
    filename,
    id: filename,
    source: descriptor.template.content,
    compilerOptions: { bindingMetadata: script.bindings },
  })
  assert.deepEqual(template.errors, [], file)
  assert.doesNotMatch(descriptor.template.content, /iframe|v-html|<script|fetch\(/)
}
const bundled = await build({
  entrypoints: [resolve(frontend, 'app/utils/embed.ts')],
  target: 'bun',
  write: false,
  plugins: [
    {
      name: 'native-provider-manifest',
      setup(builder) {
        builder.onResolve({ filter: /^#embed-manifest$/ }, () => ({
          path: 'manifest',
          namespace: 'native-manifest',
        }))
        builder.onLoad({ filter: /.*/, namespace: 'native-manifest' }, () => ({
          contents: `export const embedProviders = ${JSON.stringify(providers)}`,
          loader: 'js',
        }))
      },
    },
  ],
})
assert.equal(bundled.success, true, String(bundled.logs))
const { selectEmbed, embedStyle, embedContainerStyle } = await import(
  `data:text/javascript;base64,${Buffer.from(await bundled.outputs[0].text()).toString('base64')}`
)
const fixtures = [
  ['https://d.pr/i/9jB7', 'droplr', true],
  ['https://d.pr/v/6xCyV3', 'droplr-video', false],
  ['https://imageshack.com/i/eyn2zNjEj', 'imageshack', true],
  ['https://imageshack.com/i/p5PixlVVj', 'imageshack', true],
  ['https://imageshack.com/i/id44d071j', 'imageshack', true],
  ['https://imageshack.com/i/pbzPCsEij', 'imageshack', true],
  ['https://kalyr.smugmug.com/Music/2016/Cambridge-Rock-Festival-2016/i-95w2R4W', 'smugmug', true],
  ['https://giphy.com/videos/kehlani-open-passionate-kBMPSpmc4vbIc3h7Zo', 'giphy-clips', false],
  ['https://giphy.com/clips/kBMPSpmc4vbIc3h7Zo', 'giphy-clips', false],
]
for (const [url, id, imageSizing] of fixtures) {
  const selected = selectEmbed(url)
  assert.equal(selected.provider.id, id, url)
  assert.equal(!!selected.provider.imageSizing, imageSizing, url)
  assert.deepEqual(selected.layout, { height: 400 }, url)
  const cold = embedStyle(selected)
  assert.equal(cold.height, '400px', url)
  if (imageSizing) {
    assert.equal(embedContainerStyle(selected).maxWidth, '100%', url)
    const portrait = embedStyle(selected, 'timeline', { width: 600, height: 900 })
    const landscape = embedStyle(selected, 'timeline', { width: 2200, height: 1103 })
    assert.equal(portrait.width, 'min(100%, 266.6666666666667px)')
    assert.equal(landscape.aspectRatio, '2200 / 1103')
  } else {
    assert.equal(embedContainerStyle(selected).maxWidth, '480px', url)
    assert.deepEqual(embedStyle(selected, 'timeline', { width: 1280, height: 720 }), cold, url)
  }
}
for (const url of [
  'https://giphy.com/gifs/art-artists-on-tumblr-design-uRmDTQDgYxSzS',
  'https://giphy.com/stickers/cindysuenkeys-26AHtOSUIDsTJO7cs',
]) {
  const selected = selectEmbed(url)
  assert.equal(selected.provider.id, 'giphy')
  assert.deepEqual(selected.layout, { height: 200 })
}
const blocked = selectEmbed('https://500px.com/photo/gqZ6Tykaj5')
assert.equal(
  blocked.provider.fallback,
  true,
  '500px has no reviewed native source; do not claim parity',
)
for (const url of ['https://d.pr.evil.test/i/9jB7', 'https://imageshack.com/user/lynnseyography']) {
  assert.equal(selectEmbed(url).provider.fallback, true, url)
}
// Provider host-family rules must not take over explicit media/file URLs.
for (const [url, id] of [
  ['https://2code.d.pr/i/9jB7.png', 'image'],
  [
    'https://kalyr.smugmug.com/Music/2016/Cambridge-Rock-Festival-2016/i-95w2R4W/0/KMVGrnH9KwfBRSXD6V2VmgfLVn56SCksWgW8G6WVS/600x399/DSC01206-600x399.jpg',
    'image',
  ],
  ['https://kalyr.smugmug.com/public/report.pdf', 'file'],
  ['https://d.pr/v/public-fixture.mp4', 'video'],
]) {
  assert.equal(selectEmbed(url).provider.id, id, url)
}
const video = await readFile(resolve(frontend, 'app/components/media/DirectLinkMedia.vue'), 'utf8')
assert.match(video, /<video[\s\S]*?\bcontrols\b/)
assert.match(video, /object-contain/)
console.log(
  `Compiled ${files.length} native provider SFCs; selected ${fixtures.length} real fixture URLs, GIF 200px preservation, photo 400px caps/chat width, fixed video 400px/480px width, and blocked 500px passed.`,
)
