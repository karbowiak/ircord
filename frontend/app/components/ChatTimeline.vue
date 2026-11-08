<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import type { Message } from '../../shared/types/chat'
import { messagesQuery } from '../queries/chat'
import { stripFormatting } from '../../shared/utils/irc'
import { isChatter } from '../../shared/utils/events'
import { selectionLog } from '../../shared/utils/log'
import { attractClock, offscreen, resolveAttract } from '../../shared/utils/attract'
import { unseenSince, type SeenUpTo } from '../../shared/utils/unseen'
const props = defineProps<{ highlightId?: string }>()
const emit = defineEmits<{
  reply: [message: Message]
  profile: [id: string]
}>()
const chat = useChatStore()
const appearance = useAppearanceStore()
const { day, time } = useTimeFormat()
const history = useQuery(() => messagesQuery(chat.activeId))
/** Deleted (redacted) messages leave the timeline entirely; replies still quote them as deleted. */
const messages = computed(() =>
  [...(history.data.value?.items ?? []), ...(chat.outbox[chat.activeId] ?? [])].filter(
    (message) => message.kind !== 'system' && !message.deleted,
  ),
)
const preferences = usePreferencesStore()
const notifications = useNotificationStore()
/**
 * The scroll engine: follows the latest message until the reader scrolls away, keeps the reading
 * position when older history arrives above, and jumps to messages. Rows are in normal flow, so
 * the browser's own scroll anchoring keeps the view put while content above changes size.
 */
const timeline = provideMessageScroller({ autoScroll: true, scrollEdgeThreshold: 80 })
const viewport = ref<{ element: HTMLElement | null } | null>(null)
const scroller = computed(() => viewport.value?.element ?? null)
const atBottom = ref(true)
/** The timeline is a window around an older message (a saved one), not the live end. */
const detached = computed(() => !!history.data.value?.detached)
async function jumpToPresent() {
  if (detached.value) await chat.returnToPresent(chat.activeId)
  scrollLatest()
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
  | { kind: 'start' | 'loading'; key: string }
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
  if (!history.data.value?.nextCursor) result.unshift({ kind: 'start', key: 'start' })
  else if (chat.loadingOlder) result.unshift({ kind: 'loading', key: 'loading' })
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
/** Rows with link media start taller, so the first layout of history is closer to its final one. */
const intrinsic = (row: Row) =>
  row.kind === 'message' && /https?:\/\//.test(row.message.text) ? 'auto 360px' : 'auto 56px'

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
function onScroll() {
  const el = scroller.value
  if (!el) return
  if (attract.value) scrollTick.value++
  // Load older history before the reader reaches the top; the engine keeps their place.
  if (el.scrollTop < 300 && history.data.value?.nextCursor && !chat.loadingOlder) void older()
}
async function older() {
  await chat.loadOlder(chat.activeId)
  fillUp()
}
/**
 * Older history loads as the reader nears the top, which takes a scroll event. When the loaded
 * messages don't fill the timeline there is nothing to scroll, so keep loading until they do.
 */
function fillUp() {
  nextTick(() => {
    const el = scroller.value
    if (!el || detached.value || chat.loadingOlder || !history.data.value?.nextCursor) return
    if (el.scrollHeight - el.clientHeight < 300) void older()
  })
}
/** Wheel or swipe upwards at the very top (or with nothing to scroll) asks for more too. */
let touchY = 0
function pullOlder(event: WheelEvent | TouchEvent) {
  const el = scroller.value
  if (!el || el.scrollTop > 0) return
  // Not `instanceof TouchEvent`: desktop Firefox has no TouchEvent.
  const up =
    'touches' in event ? (event.touches[0]?.clientY ?? touchY) > touchY + 10 : event.deltaY < 0
  if (up && history.data.value?.nextCursor && !chat.loadingOlder) void older()
}
const touchStart = (event: TouchEvent) => (touchY = event.touches[0]?.clientY ?? 0)
watch(
  () => [chat.activeId, messages.value.length, history.data.value?.nextCursor] as const,
  fillUp,
  { flush: 'post' },
)
watch(
  () => chat.activeId,
  () => scrollLatest(),
)
watch(
  () => messages.value.length,
  (length, previous) => {
    // Sending a message brings you to the latest; otherwise the engine follows while you're there.
    if (messages.value.at(-1)?.delivery === 'pending') scrollLatest()
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
  () => [props.highlightId, messages.value.length] as const,
  async ([id]) => {
    if (!id) return
    await nextTick()
    const row = rows.value.find((r) => r.kind === 'message' && r.message.id === id)
    if (row) timeline.context.scrollToMessage(row.key, { align: 'center' })
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
  <div class="timeline-wrap relative flex-1 min-h-0">
    <div
      v-if="edgeCue"
      class="attract-edge absolute top-0 left-0 right-0 h-7 z-3 pointer-events-none [background:linear-gradient(_to_bottom,_color-mix(in_srgb,_var(--accent)_45%,_transparent),_transparent_)] [animation:attract-edge_calc(var(--attract-ms,_3000ms)_/_3)_ease-out_var(--attract-delay,_0ms)_3]"
      :style="edgeClock"
      aria-hidden="true"
    />
    <MessageScrollerViewport
      ref="viewport"
      class="timeline-scroll [touch-action:pan-y_pinch-zoom]"
      tabindex="0"
      aria-label="Message history"
      @scroll.passive="onScroll"
      @touchstart.passive="touchStart"
      @wheel.passive="pullOlder"
      @touchmove.passive="pullOlder"
    >
      <div
        v-if="!chat.active"
        class="empty-state h-full min-h-70 flex flex-col items-center justify-center p-6 gap-3 text-muted-foreground text-center"
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
        <UiButton
          v-if="!chat.data?.networks.length"
          @click="useWorkspaceActions().addNetwork()"
        >
          Add a network
        </UiButton>
      </div>
      <div
        v-else-if="history.isLoading.value && !messages.length"
        class="timeline-skeleton py-6 px-4"
        aria-label="Loading messages"
      >
        <div
          v-for="i in 6"
          :key="i"
          class="skeleton-message grid [grid-template-columns:40px_1fr] gap-[8px_14px] mb-6"
        >
          <i /><span /><span />
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
        <h2>Couldn’t load this conversation</h2>
        <p>{{ errorText(history.error.value) }}</p>
        <UiButton @click="history.refetch()"> Try again </UiButton>
      </div>
      <MessageScrollerContent
        v-else
        :key="chat.activeId"
        class="timeline-rows flow-root min-h-full pt-2 pr-0 pb-4 pl-0"
        role="log"
        aria-relevant="additions"
      >
        <MessageScrollerItem
          v-for="(row, index) in rows"
          :key="row.key"
          :message-id="row.key"
          :data-index="index"
          class="timeline-row flow-root [content-visibility:auto]"
          :style="{ containIntrinsicSize: intrinsic(row) }"
        >
          <div
            v-if="row.kind === 'start'"
            class="conversation-start pt-8 pr-4 pb-2 pl-4"
          >
            <span
              class="welcome-symbol inline-flex items-center justify-center w-14 h-14 bg-secondary rounded-full text-foreground mb-3"
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
            v-else-if="row.kind === 'loading'"
            class="history-load p-4 text-center text-faint text-[12px]"
          >
            Loading earlier messages…
          </div>
          <template v-else>
            <div
              v-if="dayBreak(index)"
              class="date-divider flex items-center gap-2.5 mt-4 mr-4 mb-1 ml-4 text-faint text-[11px] font-[600] whitespace-nowrap"
            >
              <span>{{ day(firstOf(row)!.createdAt) }}</span>
            </div>
            <div
              v-if="markerBefore(index)"
              class="unread-divider flex items-center mt-2 mr-4 mb-1 ml-4 h-[1px] bg-destructive"
              role="separator"
            >
              <span>New</span>
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
      </MessageScrollerContent>
    </MessageScrollerViewport>
    <button
      v-if="(!atBottom || detached) && messages.length"
      class="jump-latest absolute bottom-3 left-1/2 [transform:translateX(-50%)] flex items-center gap-2 bg-primary text-primary-foreground py-1.5 px-3 rounded-[999px] text-[12px] font-[600] shadow-control whitespace-nowrap"
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
