// Signed-out, in-memory fixture using the production pager and normal-flow scroll engine.
import { createApp, defineComponent, h, nextTick, ref, vaporInteropPlugin } from 'vue'
import { provideMessageScroller } from '../app/composables/chat/useMessageScroller'
import MessageScrollerViewport from '../app/components/message-scroller/MessageScrollerViewport.vue'
import MessageScrollerContent from '../app/components/message-scroller/MessageScrollerContent.vue'
import MessageScrollerItem from '../app/components/message-scroller/MessageScrollerItem.vue'
import { createHistoryPagination } from '../app/utils/history-pagination'
import { boundaryMessage, messageBoundary } from '../shared/utils/history-cursor'
import timelineSource from '../app/components/chat/ChatTimeline.vue?raw'

export async function verifyHistoryScroll() {
  const retained = Array.from({ length: 1200 }, (_, index) => ({
    id: `fixture-${String(index).padStart(4, '0')}`,
    conversationId: 'fixture-room',
    senderId: 'fixture-peer',
    kind: 'message',
    text: `Fixture ${index}`,
    createdAt: new Date(index * 1000).toISOString(),
    reactions: [],
  }))
  function read(query) {
    const at = query.around
      ? retained.findIndex((item) => item.id === query.around)
      : query.before
        ? retained.findIndex((item) => item.id === boundaryMessage(query.before))
        : retained.length
    const start = query.around
      ? Math.max(0, at - Math.floor((query.limit - 1) / 2))
      : Math.max(0, at - query.limit)
    const end = query.around
      ? Math.min(retained.length, at + Math.ceil((query.limit - 1) / 2) + 1)
      : at
    return {
      items: retained.slice(start, end),
      nextCursor: start ? messageBoundary(retained[start].id) : null,
      hasNewer: !!query.around && end < retained.length,
      cursor: 1,
    }
  }
  const page = ref(read({ limit: 500 })),
    loading = ref()
  let engine,
    viewport,
    maximumAnchorError = 0,
    oldPages = 0,
    newPages = 0
  const seen = new Set(page.value.items.map((item) => item.id))
  const check = (ok, message) => {
    if (!ok) throw new Error(message)
  }
  const pager = createHistoryPagination({
    pageSize: 40,
    limit: 500,
    get: () => page.value,
    set: (_, value) => {
      page.value = value
      for (const item of value.items) seen.add(item.id)
      check(value.items.length <= 500, 'Timeline exceeded its 500-message bound')
    },
    read: async (_, query) => {
      await nextTick()
      return read(query)
    },
    changed: (direction) => {
      loading.value = direction
    },
    report: (error) => {
      throw error
    },
  })
  const Root = defineComponent({
    setup() {
      engine = provideMessageScroller({
        get autoScroll() {
          return !page.value.detached
        },
        scrollEdgeThreshold: 80,
      })
      return () =>
        h(
          MessageScrollerViewport,
          {
            style: {
              height: '240px',
              width: '500px',
              overflow: 'auto',
              overflowAnchor: 'none',
            },
          },
          () =>
            h(MessageScrollerContent, {}, () => [
              ...(loading.value === 'older'
                ? [
                    h(MessageScrollerItem, {
                      key: 'loading',
                      messageId: 'loading',
                      style: { height: '36px' },
                    }),
                  ]
                : []),
              ...page.value.items.map((item) =>
                h(
                  MessageScrollerItem,
                  {
                    key: item.id,
                    messageId: item.id,
                    style: { height: `${28 + (Number(item.id.slice(-4)) % 3) * 4}px` },
                  },
                  () => item.text,
                ),
              ),
              ...(loading.value === 'newer'
                ? [h('div', { style: { height: '36px' } }, 'Loading')]
                : []),
            ]),
        )
    },
  })
  const host = document.createElement('div')
  Object.assign(host.style, {
    position: 'fixed',
    top: '60px',
    left: '20px',
    zIndex: '2147483640',
    background: '#222',
    color: '#eee',
  })
  document.body.append(host)
  const app = createApp(Root).use(vaporInteropPlugin)
  const settle = async () => {
    await nextTick()
    await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)))
  }
  const firstVisible = () => {
    const top = viewport.getBoundingClientRect().top
    return [...viewport.querySelectorAll('[data-message-id]')].find(
      (row) => row.getBoundingClientRect().bottom > top,
    )
  }
  const position = async (top) => {
    engine.context.userScrollIntent()
    viewport.scrollTop = top
    engine.context.syncAfterScroll()
    await settle()
  }
  async function advance(direction) {
    const anchor = firstVisible(),
      before = anchor.getBoundingClientRect().top
    check(await pager.load('fixture-room', direction), `Could not load ${direction} page`)
    await settle()
    check(anchor.isConnected, 'Paging removed the visible anchor')
    const error = Math.abs(anchor.getBoundingClientRect().top - before)
    maximumAnchorError = Math.max(maximumAnchorError, error)
    check(error <= 1, `${direction} paging moved the visible anchor by ${error}px`)
  }
  try {
    app.mount(host)
    viewport = host.querySelector('[data-slot="message-scroller-viewport"]')
    await settle()
    while (page.value.nextCursor) {
      await position(120)
      await advance('older')
      check(++oldPages < 50, 'Older pagination did not finish')
    }
    check(page.value.items[0].id === retained[0].id, 'Could not reach the retained beginning')
    // Exercise the timeline's actual edge-selection function with the production pager.
    // At the bottom of an older window it must fetch the adjacent page, not hide the gap.
    const edgeStart = timelineSource.indexOf('function pageNearEdge() {')
    const edgeEnd = timelineSource.indexOf('\nasync function older()', edgeStart)
    check(edgeStart >= 0 && edgeEnd > edgeStart, 'Timeline edge selector was not found')
    let pendingNewer
    const pageNearEdge = new Function(
      'scroller',
      'paging',
      'history',
      'newerFailed',
      'pageDirection',
      'newer',
      'older',
      `${timelineSource.slice(edgeStart, edgeEnd)}; return pageNearEdge`,
    )(
      { value: viewport },
      () => !!loading.value,
      { data: page },
      { value: false },
      'newer',
      () => {
        pendingNewer = pager.load('fixture-room', 'newer')
      },
      () => {
        throw new Error('Bottom edge selected older history')
      },
    )
    await position(viewport.scrollHeight)
    const previousLast = page.value.items.at(-1).id
    pageNearEdge()
    check(pendingNewer, 'The older window did not request newer history at its bottom')
    check(await pendingNewer, 'The newer history request did not complete')
    await settle()
    check(page.value.items.at(-1).id > previousLast, 'Newer history did not advance')
    check(
      viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight > 200,
      'Fetching newer history skipped the appended messages',
    )
    newPages++
    while (page.value.hasNewer) {
      await position(viewport.scrollHeight - viewport.clientHeight - 120)
      await advance('newer')
      check(
        viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight > 200,
        'Historical paging jumped past the newly appended messages',
      )
      check(++newPages < 50, 'Newer pagination did not finish')
    }
    check(
      page.value.items.at(-1).id === retained.at(-1).id,
      'Could not scroll back to the latest retained message',
    )
    check(!page.value.detached, 'The authoritative latest page did not reattach')
    check(seen.size === retained.length, 'Bidirectional travel skipped retained messages')
    engine.context.scrollToEnd()
    await settle()
    check(
      viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight <= 1,
      'Following latest failed after reattaching',
    )
    const pause = (ms) => new Promise((resolve) => setTimeout(resolve, ms))
    // A gentle upward swipe must stay detached through a new arrival.
    engine.context.userScrollIntent('up')
    viewport.scrollTop -= 12
    engine.context.syncAfterScroll()
    await pause(250)
    check(engine.context.scrollable.value.end, 'Gentle upward scrolling did not detach')
    const beforeArrival = viewport.scrollTop
    page.value = {
      ...page.value,
      items: [
        ...page.value.items,
        {
          ...retained.at(-1),
          id: 'fixture-arrival',
          createdAt: new Date(1200000).toISOString(),
        },
      ],
    }
    await settle()
    await pause(250)
    check(Math.abs(viewport.scrollTop - beforeArrival) <= 1, 'A new message pulled the reader down')
    // Simulate a resize observer recording the final position before the scroll event, and
    // independently rounded DOM dimensions leaving a subpixel residual at the physical end.
    engine.context.userScrollIntent('down')
    viewport.scrollTop = viewport.scrollHeight
    engine.context.handleResize()
    const scrollHeight = Object.getOwnPropertyDescriptor(Element.prototype, 'scrollHeight').get
    Object.defineProperty(viewport, 'scrollHeight', {
      configurable: true,
      get: () => scrollHeight.call(viewport) + 0.75,
    })
    engine.context.syncAfterScroll()
    await pause(250)
    check(!engine.context.scrollable.value.end, 'Jump control remained at the physical bottom')
    delete viewport.scrollHeight
    const beforeFollowingArrival = viewport.scrollTop
    page.value = {
      ...page.value,
      items: [
        ...page.value.items,
        {
          ...retained.at(-1),
          id: 'fixture-following',
          createdAt: new Date(1201000).toISOString(),
        },
      ],
    }
    await settle()
    await pause(250)
    check(viewport.scrollTop > beforeFollowingArrival, 'A downward return did not resume following')
    check(
      viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight <= 1,
      'Following did not reach the new message',
    )
    return {
      retained: retained.length,
      visited: seen.size,
      oldPages,
      newPages,
      bound: 500,
      maximumAnchorError,
      browserScrollAnchoring: 'disabled',
      followLatest: 'pass',
    }
  } finally {
    pager.cancel()
    app.unmount()
    host.remove()
  }
}
