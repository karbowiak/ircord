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
const { full, day, time } = useTimeFormat()

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
function keys(event: KeyboardEvent) {
  const count = items.value.length
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    if (!count) return
    selected.value = (selected.value + (event.key === 'ArrowDown' ? 1 : -1) + count) % count
    nextTick(() => list.value?.querySelector('.is-selected')?.scrollIntoView({ block: 'nearest' }))
  } else if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
    // On a tab, the tab list moves focus and selects; elsewhere the arrows still switch views.
    if ((event.target as HTMLElement | null)?.closest('[role="tab"]')) return
    event.preventDefault()
    const index = views.findIndex((v) => v.id === view.value)
    const next = (index + (event.key === 'ArrowRight' ? 1 : -1) + views.length) % views.length
    view.value = views[next]!.id
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
  <WorkspacePage
    title="Inbox"
    icon="inbox"
    description="Your mentions, replies, and direct messages across every network."
    :meta="inbox.unread ? `${inbox.unread} unread` : 'All caught up'"
    @close="emit('close')"
    ><div
      class="inbox-body flex min-h-0 flex-1 flex-col"
      @keydown="keys"
    >
      <UiTabs
        v-model="view"
        class="contents"
      >
        <WorkspaceToolbar
          ><UiTabsList
            class="search-tabs inbox-tabs w-fit max-w-full gap-1 overflow-x-auto rounded-lg bg-sidebar p-1 [scrollbar-width:none]"
            aria-label="Inbox view"
          >
            <UiTabsTrigger
              v-for="v in views"
              :key="v.id"
              :value="v.id"
              variant="pill"
              :autofocus="view === v.id || undefined"
            >
              {{ v.label }}<span v-if="v.id === 'unread' && inbox.unread">{{ inbox.unread }}</span>
            </UiTabsTrigger>
          </UiTabsList></WorkspaceToolbar
        >
        <p
          v-if="offline || stale"
          class="saved-offline mx-4 mt-0 mb-2 text-[13px] text-muted-foreground"
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
        <!-- One panel for every view, so the list and its keyboard selection stay put. -->
        <UiTabsContent
          :value="view"
          as-child
        >
          <div
            ref="list"
            class="search-results saved-results min-h-0 flex-1 overflow-y-auto px-4 pb-4 max-[700px]:px-1.5"
          >
            <div class="w-full">
              <WorkspaceLoading
                v-if="loading"
                label="Loading your inbox"
              />
              <EmptyState
                v-else-if="failed"
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
              </EmptyState>
              <EmptyState v-else-if="!items.length">
                <AppIcon
                  name="inbox"
                  :size="28"
                />
                <p v-if="cursor">Nothing left on this page. Older items may follow.</p>
                <p v-else>{{ emptyText[view] }}</p>
              </EmptyState>
              <ul
                v-else
                class="saved-list list-none m-0 p-0 space-y-0.5"
                :aria-label="`Inbox: ${views.find((v) => v.id === view)?.label}`"
              >
                <li
                  v-for="(item, index) in items"
                  :key="item.messageId"
                  class="saved-item inbox-item group flex items-start gap-1 rounded-md transition-colors hover:not-[.is-selected]:bg-accent/60 [&.is-selected]:bg-selection/70 [&.is-unread_.search-message-location]:text-foreground"
                  :class="{
                    'is-selected': index === selected,
                    'is-gone': item.status !== 'available',
                    'is-unread': item.unread && !item.dismissed,
                  }"
                >
                  <MessageResult
                    plain
                    :member="item.senderId ? chat.member(item.senderId) : undefined"
                    :author="item.status === 'unavailable' ? undefined : author(item)"
                    class="saved-open min-w-0 flex-1 aria-disabled:cursor-default"
                    :class="{ 'text-muted-foreground': item.status !== 'available' }"
                    :aria-disabled="item.status === 'unavailable' || undefined"
                    :aria-label="
                      item.status === 'available'
                        ? undefined
                        : `${item.status === 'deleted' ? 'Deleted message' : 'Unavailable message'} in ${place(item)}`
                    "
                    :time="{
                      iso: item.createdAt,
                      label: `${day(item.createdAt)} · ${time(item.createdAt)}`,
                      title: `Sent ${full(item.createdAt)}`,
                    }"
                    @click="open(item)"
                    @focus="selected = index"
                  >
                    <template #location
                      ><i
                        v-if="item.unread && !item.dismissed"
                        class="unread-dot w-[7px] h-[7px] flex-none bg-foreground rounded-full -mr-0.5"
                        role="img"
                        aria-label="Unread"
                      />{{ place(item) }} · {{ networkName(item) }}</template
                    ><template #badges>
                      <span
                        v-for="reason in item.reasons"
                        :key="reason"
                        class="inbox-reason inline-flex items-center gap-1 self-center rounded-sm bg-secondary px-1.5 py-px text-[11px] text-muted-foreground"
                        ><AppIcon
                          :name="reasonIcon[reason]"
                          :size="11"
                        />{{ reasonLabel[reason] }}</span
                      ></template
                    ><span v-if="item.status === 'available'"
                      ><em v-if="item.kind === 'action'">{{ item.excerpt }}</em
                      ><template v-else>{{ item.excerpt }}</template></span
                    >
                    <span
                      v-else-if="item.status === 'deleted'"
                      class="message-deleted text-muted-foreground italic"
                      ><em>Message deleted</em></span
                    >
                    <span
                      v-else
                      class="message-deleted text-muted-foreground italic"
                      ><em>Unavailable — you left this conversation or it was removed.</em></span
                    >
                  </MessageResult>
                  <UiButton
                    variant="ghost"
                    size="icon-sm"
                    class="saved-remove flex-none mt-2 mr-2 text-muted-foreground hover:text-primary"
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
        </UiTabsContent>
      </UiTabs>
    </div>
    <WorkspaceFooter class="inbox-footer">
      <span><kbd>↑</kbd> <kbd>↓</kbd> to choose</span><span><kbd>←</kbd> <kbd>→</kbd> views</span
      ><span><kbd>↵</kbd> to open</span><span><kbd>E</kbd> dismiss</span
      ><span><kbd>esc</kbd> to return</span>
    </WorkspaceFooter></WorkspacePage
  >
</template>
