import { createApp, defineComponent, h, ref, watch } from 'vue'
import { useEmbedVisibility } from '../app/composables/useEmbedVisibility'

export async function measureEmbedScroll(requireDeferred = false) {
  const host = document.createElement('div')
  host.className = 'timeline-scroll'
  Object.assign(host.style, {
    position: 'fixed',
    top: '80px',
    left: '20px',
    width: '700px',
    height: '220px',
    overflow: 'auto',
    zIndex: '2147483645',
    background: '#19191f',
  })
  document.body.append(host)
  const states = []
  const mounts = []
  let moving = false
  const Row = defineComponent({
    props: ['index'],
    setup(props) {
      const target = ref(null)
      const visibility = useEmbedVisibility(target)
      states[props.index] = visibility
      watch(visibility.visible, (active) => {
        if (active) mounts.push({ index: props.index, moving })
      })
      return () =>
        h(
          'div',
          { ref: target, style: { height: '200px', color: '#eee' } },
          visibility.visible.value ? 'Player' : 'Reserved preview',
        )
    },
  })
  const app = createApp({
    render: () =>
      h(
        'div',
        Array.from({ length: 16 }, (_, index) => h(Row, { index })),
      ),
  })
  const until = async (predicate) => {
    const deadline = performance.now() + 4000
    while (!predicate()) {
      if (performance.now() >= deadline) throw new Error('Preview activation timed out')
      await new Promise((resolve) => setTimeout(resolve, 20))
    }
  }
  try {
    app.mount(host)
    await until(() => states[0]?.visible.value)
    const initialHeight = host.scrollHeight
    mounts.length = 0
    moving = true
    for (let index = 1; index <= 8; index++) {
      host.scrollTop = index * 240
      await new Promise((resolve) => setTimeout(resolve, 40))
    }
    moving = false
    await until(() => states[10]?.visible.value)
    const duringScroll = mounts.filter((entry) => entry.moving).length
    if (requireDeferred && duringScroll)
      throw new Error('New players mounted during continuous scrolling')
    if (host.scrollHeight !== initialHeight)
      throw new Error('Scroll admission changed reserved geometry')
    if (states[0].visible.value) throw new Error('Departed players were retained')
    return {
      duringScroll,
      mounted: mounts.length,
      geometryStable: true,
      departedPlayerReleased: true,
    }
  } finally {
    app.unmount()
    host.remove()
  }
}
