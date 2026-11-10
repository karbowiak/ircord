import assert from 'node:assert/strict'
import { createRenderer, defineComponent, h, nextTick, Suspense } from 'vue'
import { createEmbedRenderer } from '../app/utils/embed-renderer.ts'

// Vue's real Suspense and async component lifecycle, with an in-memory render host.
const node = (type, text = '') => ({ type, text, children: [], props: {}, parent: null })
const detach = (child) => {
  const siblings = child.parent?.children
  if (siblings) siblings.splice(siblings.indexOf(child), 1)
  child.parent = null
}
const renderer = createRenderer({
  createElement: node,
  createText: (text) => node('text', text),
  createComment: (text) => node('comment', text),
  setText: (target, text) => (target.text = text),
  setElementText: (target, text) => {
    target.text = text
    target.children = []
  },
  insert(child, parent, anchor) {
    detach(child)
    child.parent = parent
    const index = anchor ? parent.children.indexOf(anchor) : -1
    parent.children.splice(index < 0 ? parent.children.length : index, 0, child)
  },
  remove: detach,
  parentNode: (child) => child.parent,
  nextSibling: (child) => child.parent?.children[child.parent.children.indexOf(child) + 1],
  patchProp: (target, key, _previous, value) => (target.props[key] = value),
})
const content = (target) =>
  target.type === 'comment' ? '' : target.text + target.children.map(content).join('')
const flush = async () => {
  for (let i = 0; i < 12; i++) await Promise.resolve()
  await nextTick()
}
const placeholder = defineComponent({
  props: ['embed', 'error'],
  setup: (props) => () => h('div', `${props.error ? 'Failed' : 'Loading'} ${props.embed.title}`),
})
let resolveSlow
let calls = 0
let mounted = 0
const slow = createEmbedRenderer(() => {
  calls++
  return new Promise((resolve) => (resolveSlow = resolve))
}, placeholder)
const prefetched = slow.preload()
assert.equal(slow.preload(), prefetched, 'prefetch shares the provider import')
assert.equal(mounted, 0, 'prefetch must not mount a player')
const root = node('root')
const app = renderer.createApp({
  render: () =>
    h(Suspense, null, {
      default: () =>
        h('section', [h('p', 'Other messages'), h(slow.component, { embed: { title: 'video' } })]),
      fallback: () => h('p', 'Whole page waiting'),
    }),
})
app.mount(root)
await flush()
assert.equal(
  content(root),
  'Other messagesLoading video',
  'a pending adapter only delays its own box',
)
assert.equal(calls, 1, 'mount reuses the prefetched import')
resolveSlow(
  defineComponent({
    setup() {
      mounted++
      return () => h('div', 'Player ready')
    },
  }),
)
await prefetched
await flush()
assert.equal(content(root), 'Other messagesPlayer ready')
assert.equal(mounted, 1)
app.unmount()

// Failed speculative prefetch retries on a visible mount, then reports a local error with props.
const errors = []
let attempts = 0
const failed = createEmbedRenderer(async () => {
  attempts++
  throw new Error('Fixture provider unavailable')
}, placeholder)
await assert.rejects(failed.preload(), /Fixture provider/)
const errorRoot = node('root')
const errorApp = renderer.createApp({
  render: () =>
    h(Suspense, null, {
      default: () =>
        h('section', [
          h('p', 'Other messages'),
          h(failed.component, { embed: { title: 'video' } }),
        ]),
      fallback: () => h('p', 'Whole page waiting'),
    }),
})
errorApp.config.errorHandler = (error) => errors.push(error)
errorApp.mount(errorRoot)
await flush()
assert.equal(attempts, 2)
assert.equal(errors.length, 1, 'visible failures remain reported')
assert.equal(
  content(errorRoot),
  'Other messagesFailed video',
  'failure preserves local source context',
)
errorApp.unmount()
console.log(
  'Provider prefetch deduplication, independent Suspense rendering, source-preserving placeholders and local errors pass.',
)
