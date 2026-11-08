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
        class="search-tabs inbox-tabs"
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
          {{ v.label
          }}<span v-if="v.id === 'unread' && inbox.unread">{{ inbox.unread }}</span>
        </button>
      </div>
      <p
        v-if="offline || stale"
        class="saved-offline"
        role="status"
      >
        {{
          offline
            ? 'You’re offline. The inbox may be out of date until IRCord reconnects.'
            : 'Couldn’t refresh the inbox; showing what was loaded before.'
        }}
        <button
          v-if="stale && !offline"
          class="text-button"
          @click="first.refetch()"
        >
          Try again
        </button>
      </p>
      <div
        ref="list"
        class="search-results saved-results"
        role="tabpanel"
        :aria-label="views.find((v) => v.id === view)?.label"
      >
        <p
          v-if="loading"
          class="search-empty"
          role="status"
        >
          Loading your inbox…
        </p>
        <div
          v-else-if="failed"
          class="search-empty"
          role="alert"
        >
          <AppIcon
            name="alert"
            :size="27"
          />
          <p>Couldn’t load your inbox: {{ errorText(first.error.value) }}</p>
          <button
            class="text-button"
            @click="first.refetch()"
          >
            Try again
          </button>
        </div>
        <div
          v-else-if="!items.length"
          class="search-empty"
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
          class="saved-list"
          :aria-label="`Inbox: ${views.find((v) => v.id === view)?.label}`"
        >
          <li
            v-for="(item, index) in items"
            :key="item.messageId"
            class="saved-item inbox-item"
            :class="{
              'is-selected': index === selected,
              'is-gone': item.status !== 'available',
              'is-unread': item.unread && !item.dismissed,
            }"
          >
            <button
              class="search-message saved-open"
              :aria-disabled="item.status === 'unavailable' || undefined"
              :aria-label="
                item.status === 'available'
                  ? undefined
                  : `${item.status === 'deleted' ? 'Deleted message' : 'Unavailable message'} in ${place(item)}`
              "
              @click="open(item)"
              @focus="selected = index"
            >
              <span class="search-message-location"
                ><i
                  v-if="item.unread && !item.dismissed"
                  class="unread-dot"
                  role="img"
                  aria-label="Unread"
                />{{ networkName(item) }} · {{ place(item)
                }}<time
                  :datetime="item.createdAt"
                  :title="`Sent ${full(item.createdAt)}`"
                  >{{ full(item.createdAt) }}</time
                ></span
              ><span class="inbox-reasons">
                <span
                  v-for="reason in item.reasons"
                  :key="reason"
                  class="inbox-reason"
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
                class="message-deleted"
                ><strong>{{ author(item) }}</strong
                ><em>Message deleted</em></span
              >
              <span
                v-else
                class="message-deleted"
                ><em>Unavailable — you left this conversation or it was removed.</em></span
              >
            </button>
            <button
              class="icon-button saved-remove"
              :aria-label="item.dismissed ? 'Restore to inbox' : 'Dismiss from inbox'"
              :title="item.dismissed ? 'Restore (E)' : 'Dismiss (E)'"
              :disabled="inbox.isPending(item.messageId)"
              @click="toggle(item)"
            >
              <AppIcon
                :name="item.dismissed ? 'reset' : 'check'"
                :size="17"
              />
            </button>
          </li>
        </ul>
        <p
          v-if="moreError"
          class="form-error"
          role="alert"
        >
          Couldn’t load more: {{ moreError }}
        </p>
        <button
          v-if="!loading && cursor && !failed"
          class="text-button saved-more"
          :disabled="loadingMore"
          @click="loadMore"
        >
          {{ loadingMore ? 'Loading…' : moreError ? 'Try again' : 'Load older items' }}
        </button>
      </div>
    </div>
    <footer class="search-footer inbox-footer">
      <span><kbd>↑</kbd> <kbd>↓</kbd> to choose</span><span><kbd>←</kbd> <kbd>→</kbd> views</span
      ><span><kbd>↵</kbd> to open</span><span><kbd>E</kbd> dismiss</span
      ><span><kbd>esc</kbd> to close</span>
    </footer></AppDialog
  >
</template>
