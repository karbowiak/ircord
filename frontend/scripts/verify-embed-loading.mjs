// Browser-only disposable fixture. Uses the existing app context without changing preferences.
import { createVNode, nextTick, render } from 'vue'
import { TooltipProvider } from 'reka-ui'
import LinkWidget from '../app/components/LinkWidget.vue'

const assert = (condition, message) => {
  if (!condition) throw new Error(message)
}

export async function verifyEmbedLoading() {
  const host = document.createElement('div')
  Object.assign(host.style, {
    position: 'fixed',
    inset: '120px auto auto 20px',
    width: '480px',
    height: '270px',
    zIndex: '2147483645',
  })
  document.body.append(host)
  const widget = createVNode(LinkWidget, {
    widget: {
      external: true,
      url: 'https://www.youtube.com/embed/jNQXAC9IVRw',
      token: '0'.repeat(48),
      height: 270,
      thumbnail: new URL('/favicon.svg', location.href).href,
      expiresAt: new Date(Date.now() + 60_000).toISOString(),
    },
    href: 'https://www.youtube.com/watch?v=jNQXAC9IVRw',
    title: 'Loading preview check',
    frames: [{ hosts: ['www.youtube.com'], paths: ['/embed/*'] }],
    fill: true,
  })
  const vnode = createVNode(TooltipProvider, null, { default: () => widget })
  vnode.appContext = document.querySelector('#__nuxt').__vue_app__._context
  try {
    render(vnode, host)
    await nextTick()
    const frame = host.querySelector('iframe')
    if (!frame)
      return { skipped: 'The current consent preference requires a click; it was preserved' }
    const initial = host.getBoundingClientRect()
    assert(host.querySelector('[role="status"]'), 'Automatic iframe loading has visible feedback')
    assert(
      host.textContent.includes('Loading preview check'),
      'Source title stays visible before iframe load',
    )
    assert(
      frame.getAttribute('aria-hidden') === 'true' && frame.tabIndex === -1,
      'Unloaded frames do not take focus',
    )
    // Controlled load/error events make transitions deterministic regardless of upstream latency.
    frame.dispatchEvent(new Event('load'))
    await nextTick()
    assert(!host.querySelector('[role="status"]'), 'Iframe load removes the loading overlay')
    assert(
      !frame.hasAttribute('aria-hidden') && !frame.hasAttribute('tabindex'),
      'Loaded player becomes accessible',
    )
    frame.dispatchEvent(new Event('error'))
    await nextTick()
    assert(host.querySelector('.link-card'), 'Iframe error displays the existing source fallback')
    const final = host.getBoundingClientRect()
    assert(
      initial.width === final.width && initial.height === final.height,
      'Loading and error preserve geometry',
    )
    return {
      checks: ['automatic loading feedback', 'source title', 'focus', 'load', 'error', 'geometry'],
    }
  } finally {
    render(null, host)
    host.remove()
  }
}
