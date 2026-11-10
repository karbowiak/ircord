// Run in the development browser. Uses disposable DOM and real intersection observers;
// it does not change the account, timeline data, preferences or remote services.
import { createApp, defineComponent, h, ref } from 'vue'
import { useEmbedVisibility } from '../app/composables/useEmbedVisibility'

function assert(condition, message) {
  if (!condition) throw new Error(message)
}
async function until(check) {
  const deadline = performance.now() + 2000
  while (!check()) {
    if (performance.now() > deadline) throw new Error('Visibility update timed out')
    await new Promise((resolve) => setTimeout(resolve, 20))
  }
}

export async function verifyEmbedVisibility() {
  const root = document.createElement('div')
  root.className = 'timeline-scroll'
  Object.assign(root.style, {
    position: 'fixed',
    top: '100px',
    left: '20px',
    width: 'calc(100vw - 40px)',
    height: '220px',
    overflow: 'auto',
    opacity: '0',
    pointerEvents: 'none',
    contain: 'strict',
  })
  document.body.append(root)
  const states = []
  const Row = defineComponent({
    props: { index: { type: Number, required: true } },
    setup(props) {
      const target = ref(null)
      const visibility = useEmbedVisibility(target)
      states[props.index] = visibility
      return () => h('div', { ref: target, style: { height: '200px' } }, String(props.index))
    },
  })
  // All links share one long message wrapper, reproducing the previous fan-out case.
  const app = createApp({
    render: () =>
      h(
        'div',
        Array.from({ length: 12 }, (_, index) => h(Row, { index })),
      ),
  })
  try {
    app.mount(root)
    const initialHeight = root.scrollHeight
    await until(() => states[0]?.visible.value && states[3]?.preload.value)
    assert(!states[3].visible.value, 'Preloading metadata must not mount a distant player')
    assert(!states[6].preload.value, 'Wide scroll regions must not expand the pixel lookahead')
    assert(!states[6].visible.value, 'One visible link must not activate the whole message')
    root.scrollTop = 1200
    await until(() => states[6].visible.value && !states[0].preload.value)
    assert(!states[0].visible.value, 'A departed player must leave the render zone')
    assert(root.scrollHeight === initialHeight, 'Visibility must not change reserved geometry')
    return {
      rows: 12,
      checks: [
        'independent links',
        'metadata ahead of players',
        'bounded wide layout',
        'unload',
        'geometry',
      ],
    }
  } finally {
    app.unmount()
    root.remove()
  }
}
