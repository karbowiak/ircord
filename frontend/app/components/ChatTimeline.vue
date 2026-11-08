<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import { defaultRangeExtractor, useVirtualizer } from '@tanstack/vue-virtual'
import type { ViewerItem, Message } from '../../shared/types/chat'
import { messagesQuery } from '../queries/chat'
import { stripFormatting } from '../../shared/utils/irc'
import { isChatter } from '../../shared/utils/events'
import { selectionLog } from '../../shared/utils/log'
import { attractClock, offscreen, resolveAttract } from '../../shared/utils/attract'
const props = defineProps<{ highlightId?: string }>()
const emit = defineEmits<{
  reply: [message: Message]
  profile: [id: string]
  image: [url: string, alt: string, set?: ViewerItem[]]
}>()
const chat = useChatStore()
const appearance = useAppearanceStore()
const { day, time } = useTimeFormat()
const history = useQuery(() => messagesQuery(chat.activeId))
const messages = computed(() =>
  [...(history.data.value?.items ?? []), ...(chat.outbox[chat.activeId] ?? [])].filter(
    (message) => message.kind !== 'system',
  ),
)
const preferences = usePreferencesStore()
const notifications = useNotificationStore()
const scroller = ref<HTMLElement | null>(null)
const atBottom = ref(true)
/** The timeline is a window around an older message (a saved one), not the live end. */
const detached = computed(() => !!history.data.value?.detached)
async function jumpToPresent() {
  if (detached.value) await chat.returnToPresent(chat.activeId)
  scrollLatest()
}
const unseen = ref(0)

// ---- Rows: messages, single events, and runs of join/part/quit/nick collapsed together -------

type Row =
  | { kind: 'message'; message: Message; key: string }
  | { kind: 'events'; events: Message[]; key: string }
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
  return result
})
const firstOf = (row: Row) => (row.kind === 'message' ? row.message : row.events[0]!)
const sameDay = (a: Message, b: Message) =>
  new Date(a.createdAt).toDateString() === new Date(b.createdAt).toDateString()
function dayBreak(index: number) {
  const a = rows.value[index - 1]
  const b = rows.value[index]
  return !!b && (!a || !sameDay(firstOf(a), firstOf(b)))
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
  return !!markerId.value && !!row && firstOf(row).id === markerId.value
}

// Rows holding the ends of a selection in the timeline stay rendered while scrolled away, so
// the browser keeps its selection boundaries in them and a copy knows where the selection starts.
const selectedKeys = ref<string[]>([])
const pinned = computed(() =>
  selectedKeys.value
    .map((key) => rows.value.findIndex((row) => row.key === key))
    .filter((i) => i >= 0),
)
function rowKeyAt(node: Node | null) {
  const el = (node instanceof Element ? node : node?.parentElement)?.closest<HTMLElement>(
    '.virtual-row',
  )
  if (!el || !scroller.value?.contains(el)) return
  return rows.value[Number(el.dataset.index)]?.key
}
function onSelectionChange() {
  const selection = document.getSelection()
  const keys = selection?.rangeCount
    ? [...new Set([rowKeyAt(selection.anchorNode), rowKeyAt(selection.focusNode)])].filter(
        (key): key is string => !!key,
      )
    : []
  if (keys.join('\n') !== selectedKeys.value.join('\n')) selectedKeys.value = keys
}
onMounted(() => document.addEventListener('selectionchange', onSelectionChange))
onBeforeUnmount(() => document.removeEventListener('selectionchange', onSelectionChange))
/** The rows the virtualizer would render, plus `extra`. */
const alsoRender = (extra: number[]) => (range: Parameters<typeof defaultRangeExtractor>[0]) =>
  [...new Set([...defaultRangeExtractor(range), ...extra])].sort((a, b) => a - b)

const virtualizer = useVirtualizer(
  computed(() => ({
    count: rows.value.length,
    rangeExtractor: alsoRender(pinned.value),
    getScrollElement: () => scroller.value,
    estimateSize: (i: number) => {
      const row = rows.value[i]
      if (row?.kind === 'events') return 28
      const m = row?.message
      if (appearance.settings.messageLayout === 'irc') return m?.attachment || m?.gif ? 240 : 24
      return m?.attachment || m?.gif ? 280 : grouped(i) ? 26 : m?.reactions.length ? 84 : 52
    },
    getItemKey: (index: number) => rows.value[index]?.key ?? index,
    overscan: 10,
    paddingStart: 8,
    paddingEnd: 16,
  })),
)
const virtualRows = computed(() => virtualizer.value.getVirtualItems())
const totalSize = computed(() => virtualizer.value.getTotalSize())
const measure = (el: unknown) => {
  if (el instanceof HTMLElement) virtualizer.value.measureElement(el)
}

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
  const items = virtualRows.value
  return offscreen(
    a.row,
    items.find((r) => r.index === a.row),
    items[0]?.index ?? 0,
    el,
  )
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
    if (scroller.value) {
      scroller.value.scrollTop = scroller.value.scrollHeight
      lastScrollTop = scroller.value.scrollTop
      lastScrollHeight = scroller.value.scrollHeight
    }
    unseen.value = 0
    atBottom.value = true
  })
}
/**
 * `atBottom` means the view follows the conversation. It stops only when the view moves up past
 * the “new messages” threshold. When the bottom drifts away without the view moving up (a soft
 * keyboard or a growing composer shortening the timeline, rows measuring taller), the view goes
 * back to the latest message instead of giving up on following.
 */
const followThreshold = 80
let lastScrollTop = 0
let lastScrollHeight = 0
// Wheel, touch, scrollbar and keys: the reader moving the view, even while messages arrive.
let lastInput = 0
let pointerHeld = false
function onReaderInput(event: Event) {
  lastInput = performance.now()
  if (event.type === 'pointerdown') pointerHeld = true
}
const onPointerUp = () => (pointerHeld = false)
onMounted(() => window.addEventListener('pointerup', onPointerUp))
onBeforeUnmount(() => window.removeEventListener('pointerup', onPointerUp))
function onScroll() {
  const el = scroller.value
  if (!el) return
  // The virtualizer also moves the view up while rows measure, without the reader touching it;
  // that comes with a height change.
  const reader = pointerHeld || performance.now() - lastInput < 1000
  const movedUp =
    el.scrollTop < lastScrollTop - 1 && (reader || el.scrollHeight === lastScrollHeight)
  lastScrollTop = el.scrollTop
  lastScrollHeight = el.scrollHeight
  if (el.scrollHeight - el.scrollTop - el.clientHeight < followThreshold) atBottom.value = true
  else if (movedUp || detached.value || props.highlightId) atBottom.value = false
  else if (atBottom.value) el.scrollTop = el.scrollHeight
  if (attract.value) scrollTick.value++
  if (atBottom.value) unseen.value = 0
  // Load older history before the reader reaches the top.
  if (el.scrollTop < 300 && history.data.value?.nextCursor && !chat.loadingOlder) void older()
}
async function older() {
  const el = scroller.value
  const previous = el?.scrollHeight ?? 0
  await chat.loadOlder(chat.activeId)
  await nextTick()
  if (el) el.scrollTop += el.scrollHeight - previous
}
watch(
  () => chat.activeId,
  () => {
    unseen.value = 0
    scrollLatest()
  },
)
watch(
  () => messages.value.length,
  (length, previous) => {
    // A highlighted message (a jump to it) keeps the view on that message.
    if ((atBottom.value && !props.highlightId) || messages.value.at(-1)?.delivery === 'pending')
      scrollLatest()
    else if (length > previous) unseen.value += length - previous
    // previous is 0 while history first loads: that isn't news.
    if (previous && length > previous && atBottom.value) announce(messages.value.slice(previous))
  },
  { flush: 'post' },
)

// Screen readers hear new messages from others while you're at the latest one, at most every
// few seconds, so a busy channel becomes “4 new messages, latest from jules: …” instead of a flood.
const announcement = ref('')
let announced = 0
let pendingCount = 0
let announceTimer: ReturnType<typeof setTimeout> | undefined
function announce(added: Message[]) {
  const heard = (m: Message) =>
    m.senderId !== 'you' &&
    m.kind !== 'event' &&
    !notifications.ignored(chat.member(m.senderId)?.nick)
  const incoming = added.filter((m) => heard(m) && !m.delivery)
  if (!incoming.length) return
  pendingCount += incoming.length
  const flush = () => {
    const last = messages.value.findLast(heard)
    if (!last || !pendingCount) return
    const who = chat.member(last.senderId)?.nick ?? 'someone'
    const text = stripFormatting(last.text).slice(0, 140)
    announcement.value =
      pendingCount === 1
        ? `${who}: ${text}`
        : `${pendingCount} new messages, latest from ${who}: ${text}`
    pendingCount = 0
    announced = Date.now()
  }
  clearTimeout(announceTimer)
  const wait = announced + 4000 - Date.now()
  if (wait <= 0) flush()
  else announceTimer = setTimeout(flush, wait)
}
watch(
  () => chat.activeId,
  () => {
    clearTimeout(announceTimer)
    pendingCount = 0
    announcement.value = ''
  },
)
onBeforeUnmount(() => clearTimeout(announceTimer))
watch(
  totalSize,
  () => {
    if (atBottom.value) scrollLatest()
  },
  { flush: 'post' },
)
watch(
  () => [props.highlightId, messages.value.length] as const,
  async ([id]) => {
    if (!id) return
    await nextTick()
    const index = rows.value.findIndex((r) => r.kind === 'message' && r.message.id === id)
    if (index >= 0) virtualizer.value.scrollToIndex(index, { align: 'center' })
  },
)
onMounted(scrollLatest)

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
  // collapsed activity run, in timeline order. Rendered rows are checked against the selection;
  // rows between them that the virtualizer has not rendered are wholly selected.
  const units: { el?: Element; index: number; messages: Message[] }[] = []
  const rendered = [...scroller.value.querySelectorAll<HTMLElement>('.virtual-row')].sort(
    (a, b) => Number(a.dataset.index) - Number(b.dataset.index),
  )
  for (const el of rendered) {
    const index = Number(el.dataset.index)
    const row = rows.value[index]
    if (row?.kind === 'message') {
      const item = el.querySelector('.message-row')
      if (item && overlap(range, item)) units.push({ el: item, index, messages: [row.message] })
    } else if (row) {
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
  // A selection reaching past the rendered rows (from the conversation's start above them, say)
  // covers every loaded row on that side.
  const list = scroller.value.querySelector('.virtual-messages')
  let first = Math.min(...units.map((unit) => unit.index))
  let last = Math.max(...units.map((unit) => unit.index))
  if (list && overlap(range, list)) {
    const whole = document.createRange()
    whole.selectNodeContents(list)
    if (range.compareBoundaryPoints(Range.START_TO_START, whole) <= 0) first = 0
    if (range.compareBoundaryPoints(Range.END_TO_END, whole) >= 0) last = rows.value.length - 1
  }
  const seen = new Set(rendered.map((el) => Number(el.dataset.index)))
  for (let index = Math.max(first, 0); index <= Math.min(last, rows.value.length - 1); index++) {
    const row = rows.value[index]
    if (seen.has(index) || !row) continue
    units.push({ index, messages: row.kind === 'message' ? [row.message] : row.events })
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
    new Date(iso).toLocaleDateString(undefined, {
      weekday: 'long',
      year: 'numeric',
      month: 'long',
      day: 'numeric',
    })
  const picked = units.flatMap((unit) => unit.messages)
  event.clipboardData.setData('text/plain', selectionLog(picked, nick, time, date))
  event.preventDefault()
}
onMounted(() => document.addEventListener('copy', onCopy))
onBeforeUnmount(() => document.removeEventListener('copy', onCopy))
</script>
<template>
  <div class="timeline-wrap">
    <div
      v-if="edgeCue"
      class="attract-edge"
      :style="edgeClock"
      aria-hidden="true"
    />
    <div
      ref="scroller"
      class="timeline-scroll"
      tabindex="0"
      aria-label="Message history"
      @scroll.passive="onScroll"
      @wheel.passive="onReaderInput"
      @touchmove.passive="onReaderInput"
      @pointerdown="onReaderInput"
      @keydown="onReaderInput"
    >
      <div
        v-if="!chat.active"
        class="empty-state"
      >
        <AppIcon
          name="message"
          :size="30"
        />
        <h2>
          {{ chat.data?.networks.length ? 'Choose a conversation' : 'Add your first network' }}
        </h2>
        <p>
          {{
            chat.data?.networks.length
              ? 'Join a channel or open a direct message.'
              : 'Connect to IRC to start chatting.'
          }}
        </p>
        <button
          v-if="!chat.data?.networks.length"
          class="button primary"
          @click="useWorkspaceActions().addNetwork()"
        >
          Add a network
        </button>
      </div>
      <div
        v-else-if="history.isLoading.value && !messages.length"
        class="timeline-skeleton"
        aria-label="Loading messages"
      >
        <div
          v-for="i in 6"
          :key="i"
          class="skeleton-message"
        >
          <i /><span /><span />
        </div>
      </div>
      <div
        v-else-if="history.error.value && !messages.length"
        class="empty-state"
      >
        <AppIcon
          name="offline"
          :size="28"
        />
        <h2>Couldn’t load this conversation</h2>
        <p>{{ errorText(history.error.value) }}</p>
        <button
          class="button primary"
          @click="history.refetch()"
        >
          Try again
        </button>
      </div>
      <template v-else>
        <div
          v-if="!history.data.value?.nextCursor"
          class="conversation-start"
        >
          <span class="welcome-symbol"
            ><AppIcon
              :name="chat.active?.kind === 'direct' ? 'message' : 'hash'"
              :size="24"
          /></span>
          <h1>
            {{
              chat.active?.kind === 'direct'
                ? (chat.directPartner(chat.active)?.displayName ?? chat.active.name)
                : `#${chat.active?.name}`
            }}
          </h1>
          <p>
            {{
              chat.active?.kind === 'direct'
                ? 'This is the beginning of your direct messages.'
                : `This is the beginning of #${chat.active?.name}.`
            }}
            <template v-if="chat.active?.topic && chat.active.kind === 'channel'">
              <br /><MessageContent :text="chat.active.topic"
            /></template>
          </p>
        </div>
        <div
          v-else-if="chat.loadingOlder"
          class="history-load"
        >
          Loading earlier messages…
        </div>
        <div
          class="virtual-messages"
          :style="{ height: `${totalSize}px` }"
        >
          <div
            v-for="row in virtualRows"
            :key="String(row.key)"
            :ref="measure"
            :data-index="row.index"
            class="virtual-row"
            :style="{ transform: `translateY(${row.start}px)` }"
          >
            <template v-if="rows[row.index]">
              <div
                v-if="dayBreak(row.index)"
                class="date-divider"
              >
                <span>{{ day(firstOf(rows[row.index]!).createdAt) }}</span>
              </div>
              <div
                v-if="markerBefore(row.index)"
                class="unread-divider"
                role="separator"
              >
                <span>New</span>
              </div>
              <MessageItem
                v-if="rows[row.index]!.kind === 'message'"
                :message="(rows[row.index] as { message: Message }).message"
                :grouped="grouped(row.index)"
                :highlighted="(rows[row.index] as { message: Message }).message.id === highlightId"
                :attract="attractOf(row.index)"
                :attract-started-at="attract?.startedAt"
                @reply="emit('reply', $event)"
                @profile="emit('profile', $event)"
                @image="(url, alt, set) => emit('image', url, alt, set)"
              />
              <EventRow
                v-else
                :events="(rows[row.index] as { events: Message[] }).events"
                @profile="emit('profile', $event)"
              />
            </template>
          </div>
        </div>
      </template>
    </div>
    <button
      v-if="(!atBottom || detached) && messages.length"
      class="jump-latest"
      :class="{ 'is-attract': pillCue }"
      :style="pillClock"
      @click="jumpToPresent"
    >
      {{
        unseen && !detached
          ? `${unseen} new ${unseen === 1 ? 'message' : 'messages'}`
          : detached
            ? 'Viewing older messages · Jump to present'
            : 'Jump to present'
      }}<AppIcon
        name="down"
        :size="14"
      />
    </button>
    <span
      class="sr-only"
      role="status"
      aria-live="polite"
      >{{ unseen ? `${unseen} new messages` : announcement }}</span
    >
  </div>
</template>
