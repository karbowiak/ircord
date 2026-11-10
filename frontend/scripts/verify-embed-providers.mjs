#!/usr/bin/env bun

// Focused provider compilation and real component rendering; no server or provider requests.
import assert from 'node:assert/strict'
import { build } from 'bun'
import { mkdtemp, readFile, readdir, rm } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { compileScript, compileTemplate, parse } from 'vue/compiler-sfc'
import * as vue from 'vue'
import { renderToString } from 'vue/server-renderer'
import { discoverEmbeds } from '../nuxt-modules/embed-declarations.ts'

const frontend = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const directory = resolve(frontend, 'app/components/embed')
const providers = await discoverEmbeds(directory)
let compiled = 0
function compile(source, filename) {
  const { descriptor, errors } = parse(source, { filename })
  assert.deepEqual(errors, [], filename)
  const script = compileScript(descriptor, { id: filename, genDefaultAs: '__component' })
  const template = compileTemplate({
    filename,
    id: filename,
    source: descriptor.template.content,
    compilerOptions: { bindingMetadata: script.bindings },
  })
  assert.deepEqual(template.errors, [], filename)
  return `${script.content}\n${template.code}\n__component.render = render; export default __component`
}
for (const file of await readdir(directory)) {
  if (!file.endsWith('.vue')) continue
  compile(await readFile(resolve(directory, file), 'utf8'), resolve(directory, file))
  compiled++
}
const temporary = await mkdtemp(resolve(frontend, '.nuxt/embed-check-'))
try {
  const builds = await build({
    entrypoints: [
      'embed-check:LinkWidget.vue',
      'embed-check:LinkCard.vue',
      'embed-check:EmbedPlaceholder.vue',
    ],
    outdir: temporary,
    write: true,
    target: 'bun',
    format: 'esm',
    external: ['vue', '@vueuse/core'],
    plugins: [
      {
        name: 'compile-embed-check',
        setup(builder) {
          builder.onResolve({ filter: /^embed-check:/ }, ({ path }) => ({
            path: path.slice(12),
            namespace: 'embed-check',
          }))
          builder.onLoad({ filter: /.*/, namespace: 'embed-check' }, async ({ path }) => {
            const filename = resolve(frontend, 'app/components/media', path)
            return {
              contents: compile(await readFile(filename, 'utf8'), filename),
              loader: 'ts',
              resolveDir: dirname(filename),
            }
          })
          builder.onResolve({ filter: /^\.\.\/\.\.\/\.\.\/shared\/utils\/link-media$/ }, () => ({
            path: resolve(frontend, 'shared/utils/link-media.ts'),
          }))
          builder.onResolve({ filter: /^\.\.\/\.\.\/utils\/embed$/ }, () => ({
            path: resolve(frontend, 'app/utils/embed.ts'),
          }))
          builder.onResolve({ filter: /^#embed-manifest$/ }, () => ({
            path: 'manifest',
            namespace: 'embed-check-manifest',
          }))
          builder.onLoad({ filter: /.*/, namespace: 'embed-check-manifest' }, () => ({
            contents: `export const embedProviders = ${JSON.stringify(providers)}`,
            loader: 'js',
          }))
        },
      },
    ],
  })
  assert.equal(builds.success, true, String(builds.logs))
  const modules = new Map()
  for (const output of builds.outputs) {
    const loaded = await import(output.path)
    assert.ok(loaded.default, `Missing export: ${Object.keys(loaded)}; ${await output.text()}`)
    modules.set(
      output.path
        .split('/')
        .at(-1)
        .replace(/^embed-check:/, '')
        .replace(/\.vue\.js$|\.js$/, ''),
      loaded.default,
    )
  }
  assert.ok(
    modules.get('LinkWidget'),
    `Missing compiled component: ${[...modules.keys()].join(', ')}`,
  )
  const preferences = vue.reactive({ ready: true, preferences: { askBeforeEmbeds: false } })
  Object.assign(globalThis, {
    ref: vue.ref,
    computed: vue.computed,
    watch: vue.watch,
    onBeforeUnmount: vue.onBeforeUnmount,
    useI18n: () => ({ t: (key) => key }),
    useRuntimeConfig: () => ({ public: { apiBase: '/api/v1' } }),
    usePreferencesStore: () => preferences,
    window: { location: { href: 'http://localhost:3000/channel/review', hostname: 'localhost' } },
  })
  const render = async (name, props) => {
    const app = vue.createSSRApp(modules.get(name), props)
    for (const component of ['AppIcon', 'NuxtImg', 'UiButton', 'LinkCard', 'MediaLinkActions']) {
      app.component(component, {
        props: ['actions'],
        setup:
          (props, { slots }) =>
          () =>
            vue.h(
              'span',
              { 'data-component': component, 'data-actions': String(props.actions) },
              slots.default?.(),
            ),
      })
    }
    return renderToString(app)
  }
  const props = {
    widget: { external: true, token: 'a'.repeat(48), url: 'https://player.vimeo.com/video/123' },
    frames: [{ hosts: ['player.vimeo.com'], paths: ['/video/*'] }],
    title: 'Review',
    href: 'https://vimeo.com/123',
    height: 96,
  }
  let html = await render('LinkWidget', props)
  assert.match(html, /<iframe/)
  assert.match(html, /height:100px/)
  preferences.ready = false
  html = await render('LinkWidget', props)
  assert.doesNotMatch(html, /<iframe|widget-poster/)
  assert.match(html, /role="status"/)
  assert.match(html, /mediaUi.linkPreview.loading/)
  preferences.ready = true
  preferences.preferences.askBeforeEmbeds = true
  html = await render('LinkWidget', props)
  assert.match(html, /widget-poster/)
  assert.doesNotMatch(html, /<iframe/)
  preferences.preferences.askBeforeEmbeds = false
  assert.match(await render('LinkWidget', props), /<iframe/)
  for (const url of [
    'https://evil.example/video/123',
    'http://player.vimeo.com/video/123',
    'https://user:pass@player.vimeo.com/video/123',
    'https://player.vimeo.com:444/video/123',
  ]) {
    assert.doesNotMatch(
      await render('LinkWidget', { ...props, widget: { ...props.widget, url } }),
      /<iframe/,
    )
  }
  assert.doesNotMatch(await render('LinkWidget', { ...props, expired: true }), /<iframe/)
  const twitch = await render('LinkWidget', {
    ...props,
    widget: {
      ...props.widget,
      url: 'https://clips.twitch.tv/embed?clip=public-fixture&parent=127.0.0.1&parent=evil.example',
    },
    frames: [{ hosts: ['clips.twitch.tv'], paths: ['/embed'] }],
  })
  assert.match(twitch, /parent=localhost/)
  assert.doesNotMatch(twitch, /parent=127|parent=evil/)
  const card = await render('LinkCard', {
    card: { title: 'Review', site: 'example.com' },
    url: 'https://example.com',
  })
  assert.match(card, /data-component="MediaLinkActions"/)
  const placeholder = await render('EmbedPlaceholder', {
    embed: { card: { title: 'Review' }, url: 'https://example.com', pending: false },
  })
  assert.match(placeholder, /data-actions="true"/)
  const hidden = await render('EmbedPlaceholder', {
    embed: { card: { title: 'Review' }, url: 'https://example.com' },
    actions: false,
  })
  assert.match(hidden, /data-actions="false"/)
  console.log(
    `Compiled ${compiled} provider SFCs and ${providers.length} static declarations; component readiness, consent, expiry, frame trust, Twitch parent, and default actions passed.`,
  )
} finally {
  await rm(temporary, { recursive: true, force: true })
}
