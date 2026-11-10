<script setup vapor lang="ts">
import { errorText } from '../../utils/api'
import { useQuery } from '@pinia/colada'
import type { Message } from '../../../shared/types/chat'
import { messagesQuery } from '../../queries/chat'
import { stripFormatting } from '../../../shared/utils/irc'
import { isChatter } from '../../../shared/utils/events'
import { selectionLog } from '../../../shared/utils/log'
import { isEditable } from '../../../shared/utils/focus'
import { attractClock, offscreen, resolveAttract } from '../../../shared/utils/attract'
import { unseenSince, type SeenUpTo } from '../../../shared/utils/unseen'
const props = defineProps<{ highlightId?: string }>()
const emit = defineEmits<{
  reply: [message: Message]
  profile: [id: string]
}>()
const chat = useChatStore()
const conversationName = useSidebarOrder().name
const { t, n, locale } = useI18n()
const { day, time } = useTimeFormat()
const history = useQuery(() => messagesQuery(chat.activeId))
/** Deleted (redacted) messages leave the timeline entirely; replies still quote them as deleted. */
const messages = computed(() =>
  [...(history.data.value?.items ?? []), ...(chat.outbox[chat.activeId] ?? [])].filter(
    (message) => message.kind !== 'system' && !message.deleted,
  ),
)
const preferences = usePreferencesStore()
const appearance = useAppearanceStore()
const notifications = useNotificationStore()
/**
 * The scroll engine: follows the latest message until the reader scrolls away, keeps the reading
 * position when older history arrives above, and jumps to messages. Rows are in normal flow, so
 * the browser's own scroll anchoring keeps the view put while content above changes size.
 */
/** A retained history window follows the reader, even when its current bottom is visible. */
const detached = computed(() => !!history.data.value?.detached || !!history.data.value?.hasNewer)
const timeline = provideMessageScroller({
  get autoScroll() {
    return !detached.value
  },
  scrollEdgeThreshold: 80,
})
const viewport = ref<{ element: HTMLElement | null } | null>(null)
const scroller = computed(() => viewport.value?.element ?? null)
const atBottom = ref(true)
async function jumpToPresent() {
  const id = chat.activeId
  if ((await chat.returnToPresent(id)) && chat.activeId === id) scrollLatest()
}
/** The newest message when the reader left the latest one; the pill counts what came after. */
const seenUpTo = ref<SeenUpTo>()
// History, not the timeline list: your own unsent messages sort after new arrivals, and deleted
// messages leave the list, so either would lose the anchor.
const historyItems = computed(() => history.data.value?.items ?? [])
const unseen = computed(() =>
  atBottom.value
    ? 0
    : unseenSince(
        historyItems.value,
        seenUpTo.value,
        (m) =>
          !m.deleted &&
          (m.kind === 'message' || m.kind === 'action' || m.kind === 'notice') &&
          !notifications.ignored(chat.member(m.senderId)?.nick),
      ),
)
watch(atBottom, (following) => {
  const last = historyItems.value.at(-1)
  seenUpTo.value = following || !last ? undefined : { id: last.id, createdAt: last.createdAt }
})

// ---- Rows: messages, single events, and runs of join/part/quit/nick collapsed together -------

type Row =
  | { kind: 'message'; message: Message; key: string }
  | { kind: 'events'; events: Message[]; key: string }
  /** The top of the timeline: the conversation's beginning, or older history loading. */
  | { kind: 'start' | 'loading' | 'offline' | 'retry'; key: string }
const at = (m: Message) => new Date(m.createdAt).getTime()
/** A run of chatter takes events at most a minute apart, and closes after 5 minutes or 50. */
const continues = (run: Message[], next: Message) =>
  at(next) - at(run.at(-1)!) <= 60_000 &&
  at(next) - at(run[0]!) <= 5 * 60_000 &&
  run.length < 50 &&
  sameDay(run[0]!, next)
const rows = computed<Row[]>(() => {
  const mode = preferences.preferences.events === 'hide' ? 'hide' : 'collapse'
  const result: Row[] = []
  for (const message of messages.value) {
    // Ignored people's messages and membership noise are hidden entirely.
    if (notifications.ignored(chat.member(message.senderId)?.nick)) continue
    const chatter = isChatter(message)
    if (chatter && mode === 'hide') continue
    const last = result.at(-1)
    // Kicks, bans, modes and topics always stand alone; only chatter is folded together.
    if (
      chatter &&
      mode === 'collapse' &&
      last?.kind === 'events' &&
      isChatter(last.events[0]!) &&
      continues(last.events, message)
    )
      last.events.push(message)
    else if (message.kind === 'event')
      result.push({ kind: 'events', events: [message], key: message.id })
    else result.push({ kind: 'message', message, key: message.clientId ?? message.id })
  }
  if (chat.loadingOlder) result.unshift({ kind: 'loading', key: 'loading' })
  else if (history.data.value?.syncError) result.unshift({ kind: 'retry', key: 'retry' })
  else if (!history.data.value?.nextCursor && !history.data.value?.partial) {
    const kind = history.data.value?.offline ? 'offline' : 'start'
    result.unshift({ kind, key: kind })
  }
  return result
})
const firstOf = (row: Row | undefined) =>
  row?.kind === 'message' ? row.message : row?.kind === 'events' ? row.events[0] : undefined
const sameDay = (a: Message, b: Message) =>
  new Date(a.createdAt).toDateString() === new Date(b.createdAt).toDateString()
function dayBreak(index: number) {
  const a = firstOf(rows.value[index - 1])
  const b = firstOf(rows.value[index])
  return !!b && (!a || !sameDay(a, b))
}
function grouped(index: number) {
  const a = rows.value[index - 1]
  const b = rows.value[index]
  if (a?.kind !== 'message' || b?.kind !== 'message' || markerBefore(index)) return false
  return (
    !b.message.replyTo &&
    !dayBreak(index) &&
    a.message.senderId === b.message.senderId &&
    a.message.kind === b.message.kind &&
    new Date(b.message.createdAt).getTime() - new Date(a.message.createdAt).getTime() < 5 * 60_000
  )
}

// “New messages” line: placed once per visit, before the first message that was unread.
const markerId = ref('')
watch(
  () => [chat.activeId, history.data.value !== undefined] as const,
  ([id, loaded]) => {
    if (!loaded) return
    // A manual “mark as unread” names its message; otherwise count back from the end.
    const anchor = chat.unreadAnchors[id]
    if (anchor && messages.value.some((m) => m.id === anchor)) return (markerId.value = anchor)
    const count = chat.unreadMarkers[id] ?? 0
    const others = messages.value.filter((m) => m.senderId !== 'you' && m.kind !== 'event')
    markerId.value = count ? (others[Math.max(0, others.length - count)]?.id ?? '') : ''
  },
  { immediate: true },
)
watch(
  () => chat.unreadMarkers[chat.activeId],
  (count) => {
    if (!count && !chat.unreadAnchors[chat.activeId]) markerId.value = ''
  },
)
watch(
  () => chat.unreadAnchors[chat.activeId],
  (anchor) => {
    if (anchor) markerId.value = anchor
    else if (!chat.unreadMarkers[chat.activeId]) markerId.value = ''
  },
)
const markerBefore = (index: number) => {
  const row = rows.value[index]
  return !!markerId.value && firstOf(row)?.id === markerId.value
}

/** A row's place in the timeline, for the /attract cues. */
function rowBox(index: number) {
  const el = scroller.value?.querySelector<HTMLElement>(`.timeline-row[data-index="${index}"]`)
  return el ? { start: el.offsetTop, size: el.offsetHeight } : undefined
}
/** Embed boxes already know their size. Keep them in normal flow, including off-screen;
 * their expensive contents are mounted only nearby, independently of the reserved space. */
const hasEmbeds = (row: Row) =>
  row.kind === 'message' &&
  (!!row.message.attachment ||
    !!row.message.attachments?.length ||
    /https?:\/\//.test(row.message.text))

// ---- /attract: a short sparkle on someone's message. It never moves your scroll position. ----

const scrollTick = ref(0)
const attract = computed(() => {
  const target = chat.attention[chat.activeId]
  if (!target) return undefined
  const found = resolveAttract(
    rows.value.map((r) =>
      r.kind === 'message' ? { id: r.message.id, senderId: r.message.senderId } : undefined,
    ),
    target,
    grouped,
  )
  return found && { ...found, startedAt: target.startedAt }
})
function attractOf(index: number) {
  const a = attract.value
  if (!a || (index !== a.row && index !== a.avatar)) return undefined
  return a.row === a.avatar ? 'both' : index === a.row ? 'row' : 'avatar'
}
/**
 * Where the message is when it is out of view: below, the “Jump to present” pill carries the cue;
 * above, a glow along the timeline's top edge does.
 */
const attractOffscreen = computed<'above' | 'below' | undefined>(() => {
  const a = attract.value
  const el = scroller.value
  if (!a || !el) return undefined
  void scrollTick.value
  return offscreen(a.row, rowBox(a.row), 0, el)
})
const pillCue = computed(
  () => attractOffscreen.value === 'below' && (!atBottom.value || detached.value),
)
const edgeCue = computed(() => attractOffscreen.value === 'above')
/** Each cue's clock is set when it appears, never on later re-renders (see `attractClock`). */
const pillClock = ref<Record<string, string>>()
const edgeClock = ref<Record<string, string>>()
watch(
  () => [pillCue.value, attract.value?.startedAt] as const,
  ([on, startedAt]) => (pillClock.value = on && startedAt ? attractClock(startedAt) : undefined),
)
watch(
  () => [edgeCue.value, attract.value?.startedAt] as const,
  ([on, startedAt]) => (edgeClock.value = on && startedAt ? attractClock(startedAt) : undefined),
)

function scrollLatest() {
  nextTick(() => {
    timeline.context.scrollToEnd()
    atBottom.value = true
  })
}
// The engine reports whether there is more below; in follow mode there never is.
watch(
  () => timeline.context.scrollable.value.end,
  (more) => (atBottom.value = !more),
)
let previousTop = 0
let pageDirection: 'older' | 'newer' = 'older'
const newerFailed = ref(false)
// Read only from event callbacks: this flag needs no cached reactive subscription.
const paging = () => chat.loadingOlder || chat.loadingNewer
function onScroll() {
  const el = scroller.value
  if (!el) return
  if (attract.value) scrollTick.value++
  if (el.scrollTop !== previousTop) pageDirection = el.scrollTop < previousTop ? 'older' : 'newer'
  previousTop = el.scrollTop
  pageNearEdge()
}
/** Both edges advance within retained history; the present button is only a shortcut. */
function pageNearEdge() {
  const el = scroller.value
  if (!el || paging()) return
  const distance = Math.max(600, el.clientHeight)
  const nearTop = el.scrollTop < distance
  const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < distance
  if (
    nearBottom &&
    (!nearTop || pageDirection === 'newer') &&
    history.data.value?.hasNewer &&
    !newerFailed.value
  )
    void newer()
  else if (
    nearTop &&
    (!nearBottom || pageDirection === 'older') &&
    history.data.value?.nextCursor &&
    !history.data.value?.syncError
  )
    void older()
}
async function older() {
  const id = chat.activeId
  const loaded = await chat.loadOlder(id)
  if (loaded && id === chat.activeId) {
    fillUp()
    nextTick(pageNearEdge)
  }
}
async function newer() {
  const id = chat.activeId
  newerFailed.value = false
  const loaded = await chat.loadNewer(id)
  if (id !== chat.activeId) return
  if (loaded === false) newerFailed.value = true
  else if (loaded) {
    fillUp()
    nextTick(pageNearEdge)
  }
}
/**
 * Older history loads as the reader nears the top, which takes a scroll event. When the loaded
 * messages don't fill the timeline there is nothing to scroll, so keep loading until they do.
 */
function fillUp() {
  const id = chat.activeId
  nextTick(() => {
    const el = scroller.value
    if (!el || id !== chat.activeId || paging() || el.scrollHeight - el.clientHeight >= 300) return
    if (pageDirection === 'newer' && history.data.value?.hasNewer && !newerFailed.value)
      void newer()
    else if (
      pageDirection === 'older' &&
      history.data.value?.nextCursor &&
      !history.data.value?.syncError
    )
      void older()
  })
}
/** Wheel/swipe intent still pages at an edge where there is nothing left to scroll. */
let touchY = 0
function pullHistory(event: WheelEvent | TouchEvent) {
  stopResume()
  // Not `instanceof TouchEvent`: desktop Firefox has no TouchEvent.
  const delta = 'touches' in event ? touchY - (event.touches[0]?.clientY ?? touchY) : event.deltaY
  if ('touches' in event) touchY = event.touches[0]?.clientY ?? touchY
  if (!delta) return
  pageDirection = delta < 0 ? 'older' : 'newer'
  pageNearEdge()
}
function pullHistoryKey(event: KeyboardEvent) {
  // Keys from a link or button in a row scroll the timeline too; typing in a field doesn't.
  if (isEditable(event.target)) return
  if (['ArrowDown', 'PageDown', 'End'].includes(event.key)) pageDirection = 'newer'
  else if (['ArrowUp', 'PageUp', 'Home'].includes(event.key)) pageDirection = 'older'
  else return
  stopResume()
  pageNearEdge()
}
function touchStart(event: TouchEvent) {
  stopResume()
  touchY = event.touches[0]?.clientY ?? 0
}
watch(() => [chat.activeId, history.data.value] as const, fillUp, { flush: 'post' })
watch(
  () => chat.activeId,
  () => {
    previousTop = 0
    pageDirection = 'older'
    newerFailed.value = false
    scrollLatest()
  },
)
watch(
  () => historyItems.value.at(-1)?.id,
  () => {
    newerFailed.value = false
  },
)
/** Messages already in the list. Only unknown ones after a known one are news. */
let known = new Set(messages.value.map((m) => m.id))
let wasDetached = detached.value

/**
 * Leaving the app (switching apps on a phone, another tab) and coming back: a reader who was
 * following the conversation lands on the newest message, including history that arrives as the
 * connection catches up. A reader who had scrolled up to read history stays where they were.
 */
let followingWhenHidden = false
let resumeUntil = 0
const RESUME_CATCH_UP = 4000
function onVisibility() {
  if (document.visibilityState === 'hidden') {
    followingWhenHidden = atBottom.value && !detached.value
    resumeUntil = 0
    return
  }
  if (!followingWhenHidden) return
  followingWhenHidden = false
  resumeUntil = Date.now() + RESUME_CATCH_UP
  // A thumb already on the screen as the app comes back wins; arrivals catch up after it lifts.
  if (!timeline.context.userInteracting()) scrollLatest()
}
/** The reader's own scrolling ends the catch-up. */
const stopResume = () => (resumeUntil = 0)
useEventListener(document, 'visibilitychange', onVisibility)
useEventListener(window, 'pageshow', (event: PageTransitionEvent) => {
  if (event.persisted) onVisibility()
})
// Sending a message brings you to the latest, once per message; otherwise the engine follows
// while you're there. Other arrivals while it is still pending must not pull you back. This runs
// before the row renders, so the engine glides it in rather than snapping to it first.
watch(
  () => {
    const latest = messages.value.at(-1)
    return latest?.delivery === 'pending' ? (latest.clientId ?? latest.id) : undefined
  },
  (sending) => {
    // Sent while holding or dragging the timeline: the reader's hand wins, the pill shows the way.
    if (!sending || timeline.context.userInteracting()) return
    // A window into older history first goes back to the present.
    if (detached.value) return void jumpToPresent()
    atBottom.value = true
    timeline.context.scrollToEnd({ behavior: appearance.reducedMotion ? 'auto' : 'smooth' })
  },
  { flush: 'pre' },
)
watch(
  messages,
  () => {
    // Back from another app while following: land on the newest message once the history caught up.
    // Only while still following: a reader who scrolled away by any means, even a scrollbar drag
    // no event reports, has left the catch-up.
    if (resumeUntil > Date.now() && atBottom.value && !timeline.context.userInteracting())
      scrollLatest()
    // Nothing is known while history first loads or after switching, and older history arrives
    // above the first known message: neither is news.
    const list = messages.value
    const first = list.findIndex((m) => known.has(m.id))
    const added = first < 0 ? [] : list.slice(first).filter((m) => !known.has(m.id))
    known = new Set(list.map((m) => m.id))
    if (added.length && atBottom.value && !wasDetached && !detached.value) announce(added)
    wasDetached = detached.value
  },
  { flush: 'post' },
)

// Screen readers hear one live region, at most every few seconds. At the latest message a busy
// channel becomes “4 new messages, latest from jules: …”; scrolled up, only the count is read.
// The log itself stays silent, so rows are never read twice and loaded history is never read.
const announcement = ref('')
let announced = 0
let pendingCount = 0
let compose: (() => string) | undefined
let announceTimer: ReturnType<typeof setTimeout> | undefined
const newMessages = (count: number) => t('timeline.newMessages', { count: n(count) }, count)
const heard = (m: Message) =>
  m.senderId !== 'you' &&
  m.kind !== 'event' &&
  !notifications.ignored(chat.member(m.senderId)?.nick)
function flushAnnouncement() {
  const text = compose?.()
  compose = undefined
  if (!text) return
  announcement.value = text
  announced = Date.now()
}
function schedule(next: () => string) {
  compose = next
  clearTimeout(announceTimer)
  const wait = announced + 4000 - Date.now()
  if (wait <= 0) flushAnnouncement()
  else announceTimer = setTimeout(flushAnnouncement, wait)
}
function announce(added: Message[]) {
  const incoming = added.filter((m) => heard(m) && !m.delivery)
  if (!incoming.length) return
  pendingCount += incoming.length
  schedule(() => {
    const last = messages.value.findLast(heard)
    const count = pendingCount
    pendingCount = 0
    if (!last || !count) return ''
    const who = chat.member(last.senderId)?.nick ?? t('timeline.someone')
    const text = stripFormatting(last.text).slice(0, 140)
    return count === 1
      ? t('timeline.latestSingle', { who, text })
      : t('timeline.latestMultiple', { count: newMessages(count), who, text })
  })
}
watch(unseen, (count, previous) => {
  if (count <= previous) return
  pendingCount = 0
  schedule(() => (unseen.value && !atBottom.value ? newMessages(unseen.value) : ''))
})
watch(
  () => chat.activeId,
  () => {
    clearTimeout(announceTimer)
    pendingCount = 0
    compose = undefined
    announcement.value = ''
    known = new Set(messages.value.map((m) => m.id))
  },
)
onBeforeUnmount(() => clearTimeout(announceTimer))
// A highlighted message is jumped to once, as soon as its row exists; later arrivals must not
// pull the reader back to it after they have scrolled on.
let highlighted: string | undefined
watch(
  () => [props.highlightId, messages.value.length] as const,
  async ([id]) => {
    if (id !== highlighted) highlighted = undefined
    if (!id || highlighted) return
    await nextTick()
    const row = rows.value.find((r) => r.kind === 'message' && r.message.id === id)
    if (row && props.highlightId === id) {
      highlighted = id
      timeline.context.scrollToMessage(row.key, { align: 'center' })
    }
  },
)
watch(
  () => chat.activeId,
  () => (highlighted = undefined),
)
onMounted(scrollLatest)

// Keyboard shortcuts for the “New messages” line and the newest message.
const actions = useWorkspaceActions()
function jumpToUnread() {
  const row = markerId.value && rows.value.find((r) => firstOf(r)?.id === markerId.value)
  if (!row) {
    chat.notify(t('workspaceUi.history.nothingUnreadHere'))
    return true
  }
  timeline.context.scrollToMessage(row.key, { align: 'center' })
  return true
}
function jumpToPresentKey() {
  void jumpToPresent()
  return true
}
actions.jumpToUnread = jumpToUnread
actions.jumpToPresent = jumpToPresentKey
onBeforeUnmount(() => {
  if (actions.jumpToUnread === jumpToUnread) actions.jumpToUnread = () => false
  if (actions.jumpToPresent === jumpToPresentKey) actions.jumpToPresent = () => false
})

// Copying a stretch of the timeline gives a plain log, one line per message or event, instead of
// the avatars, reaction buttons and layout gaps the browser would pick up from the rows. A
// selection inside one message or event (a word, a timestamp, a date divider) copies natively.
const controls = '.message-actions, .mobile-message-menu, .avatar-button, .reactions'
const structure = '.message-row, .message-grid, .message-body, .irc-line, .event-row'
/** The part of `range` inside `el`, or null when they only touch at a boundary. */
function overlap(range: Range, el: Element) {
  if (!range.intersectsNode(el)) return null
  const part = document.createRange()
  part.selectNodeContents(el)
  if (range.compareBoundaryPoints(Range.START_TO_START, part) > 0)
    part.setStart(range.startContainer, range.startOffset)
  if (range.compareBoundaryPoints(Range.END_TO_END, part) < 0)
    part.setEnd(range.endContainer, range.endOffset)
  const content = part.cloneContents()
  return part.toString().trim() || content.querySelector('svg, img') ? part : null
}
function onCopy(event: ClipboardEvent) {
  const selection = document.getSelection()
  if (!selection?.rangeCount || selection.isCollapsed || !scroller.value || !event.clipboardData)
    return
  const target = event.target
  if (target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement) return
  const range = selection.getRangeAt(0)
  // Each selected message, each selected line of an expanded activity list, or a whole
  // collapsed activity run, in timeline order.
  const units: { el?: Element; index: number; messages: Message[] }[] = []
  for (const el of scroller.value.querySelectorAll<HTMLElement>('.timeline-row')) {
    const index = Number(el.dataset.index)
    const row = rows.value[index]
    if (row?.kind === 'message') {
      const item = el.querySelector('.message-row')
      if (item && overlap(range, item)) units.push({ el: item, index, messages: [row.message] })
    } else if (row?.kind === 'events') {
      const lines = [...el.querySelectorAll('.event-list li[data-event-id]')].filter((li) =>
        overlap(range, li),
      )
      for (const li of lines) {
        const event = row.events.find((m) => m.id === li.getAttribute('data-event-id'))
        if (event) units.push({ el: li, index, messages: [event] })
      }
      const item = el.querySelector('.event-row')
      if (!lines.length && item && overlap(range, item))
        units.push({ el: item, index, messages: row.events })
    }
  }
  units.sort((a, b) => a.index - b.index)
  if (!units.length) return
  const common = range.commonAncestorContainer
  const within = common instanceof Element ? common : common.parentElement
  const touchesControl = [...scroller.value.querySelectorAll(controls)].some((el) =>
    overlap(range, el),
  )
  if (
    units.length === 1 &&
    !touchesControl &&
    within &&
    units[0]!.el?.contains(within) &&
    !within.matches(structure)
  )
    return
  const nick = (id: string) => chat.member(id)?.nick ?? id
  const date = (iso: string) =>
    new Date(iso).toLocaleDateString(locale.value, {
      weekday: 'long',
      year: 'numeric',
      month: 'long',
      day: 'numeric',
    })
  const picked = units.flatMap((unit) => unit.messages)
  event.clipboardData.setData(
    'text/plain',
    selectionLog(
      picked,
      nick,
      time,
      date,
      (key, values, plural) =>
        values ? (plural === undefined ? t(key, values) : t(key, values, plural)) : t(key),
      locale.value,
    ),
  )
  event.preventDefault()
}
onMounted(() => document.addEventListener('copy', onCopy))
onBeforeUnmount(() => document.removeEventListener('copy', onCopy))
</script>
<template>
  <div class="timeline-wrap relative flex-1 min-h-0">
    <div
      v-if="edgeCue"
      class="attract-edge absolute top-0 left-0 right-0 h-7 z-3 pointer-events-none [background:linear-gradient(_to_bottom,_color-mix(in_srgb,_var(--accent)_45%,_transparent),_transparent_)] [animation:attract-edge_calc(var(--attract-ms,_3000ms)_/_3)_ease-out_var(--attract-delay,_0ms)_3]"
      :style="edgeClock"
      aria-hidden="true"
    />
    <MessageScrollerViewport
      ref="viewport"
      class="timeline-scroll h-full overflow-y-auto [overflow-anchor:auto] overscroll-contain outline-offset-[-2px]! [touch-action:pan-y_pinch-zoom]"
      tabindex="0"
      :aria-label="t('timeline.history')"
      @scroll.passive="onScroll"
      @touchstart.passive="touchStart"
      @pointerdown.passive="stopResume"
      @wheel.passive="pullHistory"
      @touchmove.passive="pullHistory"
      @keydown="pullHistoryKey"
    >
      <div
        v-if="!chat.active"
        class="empty-state h-full min-h-70 flex flex-col items-center justify-center p-6 gap-3 text-muted-foreground text-center"
      >
        <AppIcon
          name="message"
          :size="30"
        />
        <h2 class="text-foreground text-[17px]">
          {{
            chat.data?.networks.length
              ? t('timeline.chooseConversation')
              : t('timeline.firstNetwork')
          }}
        </h2>
        <p>
          {{
            chat.data?.networks.length
              ? t('timeline.chooseConversationHelp')
              : t('timeline.firstNetworkHelp')
          }}
        </p>
        <UiButton
          v-if="!chat.data?.networks.length"
          @click="actions.addNetwork()"
        >
          {{ t('networks.add') }}
        </UiButton>
      </div>
      <div
        v-else-if="history.isLoading.value && !messages.length"
        class="timeline-skeleton py-6 px-4"
        aria-busy="true"
      >
        <span
          class="sr-only"
          role="status"
          >{{ t('timeline.loading') }}</span
        >
        <div
          v-for="i in 6"
          :key="i"
          class="skeleton-message grid [grid-template-columns:40px_1fr] gap-[8px_14px] mb-6 [&>span]:h-3 [&>span]:w-3/5 [&>span]:bg-secondary [&>span]:rounded-[6px] [&>span]:animate-[pulse_1.6s_ease-in-out_infinite] [&>span:first-of-type]:w-30 odd:[&>span:last-child]:w-[85%]"
        >
          <i class="row-span-2 size-10 rounded-full bg-secondary" /><span /><span />
        </div>
      </div>
      <div
        v-else-if="history.error.value && !messages.length"
        class="empty-state h-full min-h-70 flex flex-col items-center justify-center p-6 gap-3 text-muted-foreground text-center"
      >
        <AppIcon
          name="offline"
          :size="28"
        />
        <h2 class="text-foreground text-[17px]">{{ t('timeline.loadFailed') }}</h2>
        <p>{{ errorText(history.error.value, t) }}</p>
        <UiButton @click="history.refetch()"> {{ t('common.tryAgain') }} </UiButton>
      </div>
      <MessageScrollerContent
        v-else
        :key="chat.activeId"
        class="timeline-rows flow-root min-h-full pt-2 pr-0 pb-4 pl-0"
        role="log"
        aria-live="off"
      >
        <MessageScrollerItem
          v-for="(row, index) in rows"
          :key="row.key"
          :message-id="row.key"
          :data-index="index"
          class="timeline-row flow-root"
          :class="{ '[content-visibility:auto]': !hasEmbeds(row) }"
          :style="hasEmbeds(row) ? undefined : { containIntrinsicSize: 'auto 56px' }"
        >
          <div
            v-if="row.kind === 'start'"
            class="conversation-start pt-8 pr-4 pb-2 pl-4"
          >
            <span
              class="welcome-symbol inline-flex items-center justify-center w-14 h-14 bg-secondary rounded-full text-foreground mb-3"
              ><AppIcon
                :name="
                  chat.active?.party ? 'users' : chat.active?.kind === 'direct' ? 'message' : 'hash'
                "
                :size="24"
            /></span>
            <h2 class="text-[26px] font-bold tracking-[-0.02em] mb-1">
              {{
                chat.active?.party
                  ? conversationName(chat.active)
                  : chat.active?.kind === 'direct'
                    ? (chat.directPartner(chat.active)?.displayName ?? chat.active.name)
                    : `#${chat.active?.name}`
              }}
            </h2>
            <p class="text-[14px] text-muted-foreground max-w-[72ch]">
              {{
                chat.active?.party
                  ? t('workspaceUi.party.beginning')
                  : chat.active?.kind === 'direct'
                    ? t('timeline.directBeginning')
                    : t('timeline.channelBeginning', { channel: `#${chat.active?.name}` })
              }}
              <template
                v-if="chat.active?.topic && chat.active.kind === 'channel' && !chat.active.party"
              >
                <br /><MessageContent :text="chat.active.topic"
              /></template>
            </p>
          </div>
          <div
            v-else-if="row.kind === 'retry'"
            class="history-load flex items-center justify-center gap-3 p-4 text-muted-foreground text-[12px]"
            role="status"
          >
            <span>{{ t('timeline.olderLoadFailed') }}</span>
            <UiButton
              size="sm"
              variant="secondary"
              @click="older"
            >
              {{ t('common.tryAgain') }}
            </UiButton>
          </div>
          <div
            v-else-if="row.kind === 'offline'"
            class="history-load p-4 text-center text-muted-foreground text-[12px]"
            role="status"
          >
            {{ t('timeline.savedHistoryEnd') }}
          </div>
          <div
            v-else-if="row.kind === 'loading'"
            class="history-load p-4 text-center text-faint text-[12px]"
          >
            {{ t('timeline.loadingEarlier') }}
          </div>
          <template v-else>
            <div
              v-if="dayBreak(index)"
              class="date-divider flex items-center gap-2.5 mt-4 mr-4 mb-1 ml-4 text-faint text-[11px] font-[600] whitespace-nowrap before:content-[''] before:h-px before:bg-border before:flex-1 after:content-[''] after:h-px after:bg-border after:flex-1"
              role="separator"
              :aria-label="day(firstOf(row)!.createdAt)"
            >
              <span>{{ day(firstOf(row)!.createdAt) }}</span>
            </div>
            <div
              v-if="markerBefore(index)"
              class="unread-divider flex items-center mt-2 mr-4 mb-1 ml-4 h-[1px] bg-destructive"
              role="separator"
              :aria-label="t('timeline.newMessagesLabel')"
            >
              <span
                class="ml-auto px-1.5 rounded-[4px] bg-destructive text-rail text-[10px] font-bold leading-4 uppercase"
                >{{ t('timeline.new') }}</span
              >
            </div>
            <MessageItem
              v-if="row.kind === 'message'"
              :message="row.message"
              :grouped="grouped(index)"
              :highlighted="row.message.id === highlightId"
              :attract="attractOf(index)"
              :attract-started-at="attract?.startedAt"
              @reply="emit('reply', $event)"
              @profile="emit('profile', $event)"
            />
            <EventRow
              v-else-if="row.kind === 'events'"
              :events="row.events"
              @profile="emit('profile', $event)"
            />
          </template>
        </MessageScrollerItem>
        <div
          v-if="chat.loadingNewer || newerFailed"
          class="history-load flex items-center justify-center p-4 text-muted-foreground text-[12px]"
          role="status"
        >
          <UiButton
            v-if="newerFailed"
            size="sm"
            variant="secondary"
            @click="newer"
          >
            {{ t('common.tryAgain') }}
          </UiButton>
          <span v-else>{{ t('timeline.loading') }}</span>
        </div>
      </MessageScrollerContent>
    </MessageScrollerViewport>
    <button
      v-if="(!atBottom || detached) && messages.length"
      class="jump-latest absolute bottom-3 left-1/2 z-10 [transform:translateX(-50%)] flex items-center gap-2 bg-primary text-primary-foreground py-1.5 px-3 rounded-[999px] text-[12px] font-[600] shadow-control whitespace-nowrap"
      :class="{ 'is-attract': pillCue }"
      :style="pillClock"
      @click="jumpToPresent"
    >
      {{
        unseen && !detached
          ? newMessages(unseen)
          : detached
            ? t('timeline.viewingOlder')
            : t('timeline.jumpToPresent')
      }}<AppIcon
        name="down"
        :size="14"
      />
    </button>
    <span
      class="sr-only"
      role="status"
      aria-live="polite"
      >{{ announcement }}</span
    >
  </div>
</template>
