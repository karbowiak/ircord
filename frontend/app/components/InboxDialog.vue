<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import type { InboxFilter, InboxItem, InboxReason } from '../../shared/types/chat'
import { INBOX_PAGE, inboxListQuery, inboxRevision } from '../queries/inbox'

/**
 * Your private attention inbox across every network: mentions, replies to your messages and
 * direct messages, newest first. Opening an item jumps to it in its conversation; dismissing
 * one only hides it here — neither marks the conversation read.
 */
const emit = defineEmits<{ close: []; open: [item: InboxItem] }>()
const chat = useChatStore()
const inbox = useInboxStore()
const { full } = useTimeFormat()

const views: { id: InboxFilter; label: string }[] = [
  { id: 'all', label: 'All' },
  { id: 'unread', label: 'Unread' },
  { id: 'mentions', label: 'Mentions' },
  { id: 'replies', label: 'Replies' },
  { id: 'direct', label: 'Direct' },
  { id: 'dismissed', label: 'Dismissed' },
]
const view = ref<InboxFilter>('all')

const first = useQuery(() => inboxListQuery(view.value))
/** Pages after the first, loaded on demand; dropped whenever the inbox may have changed. */
const older = ref<InboxItem[]>([])
const olderCursor = ref<string | null>(null)
const loadingMore = ref(false)
const moreError = ref('')
watch([view, inboxRevision], () => {
  older.value = []
  olderCursor.value = null
  moreError.value = ''
})
const cursor = computed(() =>
  older.value.length ? olderCursor.value : (first.data.value?.nextCursor ?? null),
)
async function loadMore() {
  if (!cursor.value || loadingMore.value) return
  loadingMore.value = true
  moreError.value = ''
  const filter = view.value
  const revision = inboxRevision.value
  try {
    const page = await apiClient().inbox({ filter, cursor: cursor.value, limit: INBOX_PAGE })
    if (filter !== view.value || revision !== inboxRevision.value) return
    const known = new Set(
      [...(first.data.value?.items ?? []), ...older.value].map((i) => i.messageId),
    )
    older.value = [...older.value, ...page.items.filter((i) => !known.has(i.messageId))]
    olderCursor.value = page.nextCursor ?? null
  } catch (error) {
    moreError.value = errorText(error)
  } finally {
    loadingMore.value = false
  }
}

const items = computed(() => [...(first.data.value?.items ?? []), ...older.value])
const loading = computed(() => first.isLoading.value && !first.data.value)
const failed = computed(() => !!first.error.value && !first.data.value)
/** A refresh failed while older results are still shown. */
const stale = computed(() => !!first.error.value && !!first.data.value)
const offline = computed(() => chat.status === 'offline')

const networkName = (item: InboxItem) =>
  chat.data?.networks.find((n) => n.id === item.networkId)?.name ?? 'Removed network'
function place(item: InboxItem) {
  const c = chat.data?.conversations.find((x) => x.id === item.conversationId)
  if (!c || item.status === 'unavailable') return 'Unavailable conversation'
  if (c.kind === 'channel') return `#${c.name}`
  return `@${chat.directPartner(c)?.nick ?? c.name}`
}
const author = (item: InboxItem) => {
  const id = item.senderId
  return id ? (chat.member(id)?.displayName ?? chat.member(id)?.nick ?? 'Someone') : ''
}
const reasonLabel: Record<InboxReason, string> = {
  mention: 'Mention',
  reply: 'Reply',
  direct: 'Direct message',
}
const reasonIcon: Record<InboxReason, string> = { mention: 'at', reply: 'reply', direct: 'message' }

const emptyText: Record<InboxFilter, string> = {
  all: 'You’re all caught up. Mentions, replies to your messages and direct messages land here, even when no device was open.',
  unread: 'Nothing unread. Items stay in All until you dismiss them.',
  mentions: 'No mentions. Messages with your nick or a highlight word show up here.',
  replies: 'No replies. Replies to your messages show up here on networks that support them.',
  direct: 'No direct messages.',
  dismissed: 'Nothing dismissed. Dismissed items can be restored from here.',
}

const selected = ref(0)
watch(items, (list) => (selected.value = Math.min(selected.value, Math.max(0, list.length - 1))))
watch(view, () => (selected.value = 0))
const list = ref<HTMLElement | null>(null)
const tabs = ref<HTMLElement | null>(null)
function selectView(id: InboxFilter, focus = false) {
  view.value = id
  if (focus)
    nextTick(() => tabs.value?.querySelector<HTMLElement>('[aria-selected="true"]')?.focus())
}
function keys(event: KeyboardEvent) {
  const count = items.value.length
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    if (!count) return
    selected.value = (selected.value + (event.key === 'ArrowDown' ? 1 : -1) + count) % count
    nextTick(() => list.value?.querySelector('.is-selected')?.scrollIntoView({ block: 'nearest' }))
  } else if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
    event.preventDefault()
    const index = views.findIndex((v) => v.id === view.value)
    const next = (index + (event.key === 'ArrowRight' ? 1 : -1) + views.length) % views.length
    selectView(views[next]!.id, !!(event.target as HTMLElement | null)?.closest('[role="tab"]'))
  } else if (
    event.key === 'Enter' &&
    // A focused item or text button handles Enter itself.
    !(event.target as HTMLElement | null)?.closest('.saved-list button, .text-button')
  ) {
    event.preventDefault()
    const item = items.value[selected.value]
    if (item) open(item)
  } else if (event.key === 'e' || event.key === 'Delete' || event.key === 'Backspace') {
    const item = items.value[selected.value]
    if (!item || event.ctrlKey || event.metaKey || event.altKey) return
    event.preventDefault()
    toggle(item)
  }
}
function open(item: InboxItem) {
  if (item.status === 'unavailable')
    return chat.notify('That conversation isn’t open any more. Rejoin it to see this message.')
  emit('open', item)
}
function toggle(item: InboxItem) {
  return item.dismissed ? inbox.restore(item.messageId) : inbox.dismiss(item.messageId)
}
</script>
<template>
  <AppDialog
    title="Inbox"
    wide
    @close="emit('close')"
    ><div
      class="inbox-body"
      @keydown="keys"
    >
      <div
        ref="tabs"
        class="search-tabs inbox-tabs flex gap-4 pt-3 pr-5 pb-0 pl-5 border-b [border-bottom-style:solid] border-b-border overflow-x-auto [scrollbar-width:none]"
        role="tablist"
        aria-label="Inbox view"
      >
        <button
          v-for="v in views"
          :key="v.id"
          :class="{ active: view === v.id }"
          role="tab"
          :aria-selected="view === v.id"
          :tabindex="view === v.id ? 0 : -1"
          :autofocus="view === v.id || undefined"
          @click="selectView(v.id)"
        >
          {{ v.label }}<span v-if="v.id === 'unread' && inbox.unread">{{ inbox.unread }}</span>
        </button>
      </div>
      <p
        v-if="offline || stale"
        class="saved-offline mt-2.5 mr-5 mb-0 ml-5 text-[13px] text-muted-foreground"
        role="status"
      >
        {{
          offline
            ? 'You’re offline. The inbox may be out of date until IRCord reconnects.'
            : 'Couldn’t refresh the inbox; showing what was loaded before.'
        }}
        <UiButton
          variant="link"
          size="text"
          v-if="stale && !offline"
          @click="first.refetch()"
        >
          Try again
        </UiButton>
      </p>
      <div
        ref="list"
        class="search-results pt-2 pr-3 pb-3 pl-3 min-h-70 max-h-[min(460px,_calc(100dvh_-_260px))] overflow-y-auto saved-results"
        role="tabpanel"
        :aria-label="views.find((v) => v.id === view)?.label"
      >
        <p
          v-if="loading"
          class="search-empty flex flex-col items-center gap-2 py-12 px-4 text-faint text-[13px] text-center"
          role="status"
        >
          Loading your inbox…
        </p>
        <div
          v-else-if="failed"
          class="search-empty flex flex-col items-center gap-2 py-12 px-4 text-faint text-[13px] text-center"
          role="alert"
        >
          <AppIcon
            name="alert"
            :size="27"
          />
          <p>Couldn’t load your inbox: {{ errorText(first.error.value) }}</p>
          <UiButton
            variant="link"
            size="text"
            @click="first.refetch()"
          >
            Try again
          </UiButton>
        </div>
        <div
          v-else-if="!items.length"
          class="search-empty flex flex-col items-center gap-2 py-12 px-4 text-faint text-[13px] text-center"
        >
          <AppIcon
            name="inbox"
            :size="28"
          />
          <p v-if="cursor">Nothing left on this page. Older items may follow.</p>
          <p v-else>{{ emptyText[view] }}</p>
        </div>
        <ul
          v-else
          class="saved-list list-none m-0 p-0"
          :aria-label="`Inbox: ${views.find((v) => v.id === view)?.label}`"
        >
          <li
            v-for="(item, index) in items"
            :key="item.messageId"
            class="saved-item flex items-start gap-1 rounded-sm inbox-item"
            :class="{
              'is-selected': index === selected,
              'is-gone': item.status !== 'available',
              'is-unread': item.unread && !item.dismissed,
            }"
          >
            <button
              class="search-message flex flex-col gap-0.5 w-full p-2 rounded-sm text-left saved-open"
              :aria-disabled="item.status === 'unavailable' || undefined"
              :aria-label="
                item.status === 'available'
                  ? undefined
                  : `${item.status === 'deleted' ? 'Deleted message' : 'Unavailable message'} in ${place(item)}`
              "
              @click="open(item)"
              @focus="selected = index"
            >
              <span
                class="search-message-location flex gap-2 text-[12px] text-muted-foreground font-[600]"
                ><i
                  v-if="item.unread && !item.dismissed"
                  class="unread-dot w-[7px] h-[7px] bg-foreground rounded-full mr-0.5"
                  role="img"
                  aria-label="Unread"
                />{{ networkName(item) }} · {{ place(item)
                }}<time
                  :datetime="item.createdAt"
                  :title="`Sent ${full(item.createdAt)}`"
                  >{{ full(item.createdAt) }}</time
                ></span
              ><span class="inbox-reasons flex flex-wrap gap-1 my-0.5 mx-0">
                <span
                  v-for="reason in item.reasons"
                  :key="reason"
                  class="inbox-reason inline-flex items-center gap-[3px] text-[11px] font-[600] text-muted-foreground bg-accent rounded-[9px] pt-[1px] pr-[7px] pb-[1px] pl-[5px]"
                  ><AppIcon
                    :name="reasonIcon[reason]"
                    :size="12"
                  />{{ reasonLabel[reason] }}</span
                ></span
              ><span v-if="item.status === 'available'"
                ><strong>{{ author(item) }}</strong
                ><em v-if="item.kind === 'action'">{{ item.excerpt }}</em
                ><template v-else>{{ item.excerpt }}</template></span
              >
              <span
                v-else-if="item.status === 'deleted'"
                class="message-deleted text-muted-foreground italic"
                ><strong>{{ author(item) }}</strong
                ><em>Message deleted</em></span
              >
              <span
                v-else
                class="message-deleted text-muted-foreground italic"
                ><em>Unavailable — you left this conversation or it was removed.</em></span
              >
            </button>
            <UiButton
              variant="ghost"
              size="icon-sm"
              class="saved-remove flex-none mt-1.5 mr-1 mb-0 ml-0 text-primary"
              :aria-label="item.dismissed ? 'Restore to inbox' : 'Dismiss from inbox'"
              :title="item.dismissed ? 'Restore (E)' : 'Dismiss (E)'"
              :disabled="inbox.isPending(item.messageId)"
              @click="toggle(item)"
            >
              <AppIcon
                :name="item.dismissed ? 'reset' : 'check'"
                :size="17"
              />
            </UiButton>
          </li>
        </ul>
        <UiAlert
          as="p"
          variant="destructive"
          presentation="inline"
          v-if="moreError"
          class="form-error mt-3"
        >
          Couldn’t load more: {{ moreError }}
        </UiAlert>
        <UiButton
          variant="link"
          size="text"
          v-if="!loading && cursor && !failed"
          class="saved-more block mt-2 mr-auto mb-0 ml-auto"
          :disabled="loadingMore"
          @click="loadMore"
        >
          {{ loadingMore ? 'Loading…' : moreError ? 'Try again' : 'Load older items' }}
        </UiButton>
      </div>
    </div>
    <footer
      class="search-footer flex gap-3.5 py-2.5 px-5 border-t [border-top-style:solid] border-t-border bg-sidebar text-[12px] text-faint inbox-footer"
    >
      <span><kbd>↑</kbd> <kbd>↓</kbd> to choose</span><span><kbd>←</kbd> <kbd>→</kbd> views</span
      ><span><kbd>↵</kbd> to open</span><span><kbd>E</kbd> dismiss</span
      ><span><kbd>esc</kbd> to close</span>
    </footer></AppDialog
  >
</template>
