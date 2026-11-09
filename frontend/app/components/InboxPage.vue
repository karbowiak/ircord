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
const { t, n } = useI18n()
const errorMessage = (value: unknown) => errorText(value, (key) => t(key))

const viewIds: InboxFilter[] = ['all', 'unread', 'mentions', 'replies', 'direct', 'dismissed']
const views = computed(() => viewIds.map((id) => ({ id, label: t(`collectionsUi.inbox.${id}`) })))
const view = ref<InboxFilter>('all')

const first = useQuery(() => inboxListQuery(view.value))
/** Pages after the first, loaded on demand; dropped whenever the inbox may have changed. */
const older = ref<InboxItem[]>([])
const olderCursor = ref<string | null>(null)
const loadingMore = ref(false)
const moreError = ref<unknown>('')
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
    moreError.value = error
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
const notice = computed(() =>
  offline.value
    ? t('collectionsUi.inbox.offline')
    : stale.value
      ? t('collectionsUi.inbox.stale')
      : '',
)
useAnnouncedStatus(() => notice.value)
// After choosing another view, its count is read once it loads; the first load is not news.
const switched = ref(false)
watch(view, () => (switched.value = true))
useAnnouncedStatus(() =>
  switched.value && !loading.value && !failed.value
    ? t(
        cursor.value ? 'collectionsUi.inbox.viewCountMore' : 'collectionsUi.inbox.viewCount',
        {
          view: views.value.find((option) => option.id === view.value)?.label ?? '',
          count: n(items.value.length),
        },
        items.value.length,
      )
    : '',
)

const networkName = (item: InboxItem) =>
  chat.data?.networks.find((n) => n.id === item.networkId)?.name ??
  t('collectionsUi.inbox.networkRemoved')
function place(item: InboxItem) {
  const c = chat.data?.conversations.find((x) => x.id === item.conversationId)
  if (!c || item.status === 'unavailable') return t('collectionsUi.inbox.unavailableConversation')
  if (c.kind === 'channel') return `#${c.name}`
  return `@${chat.directPartner(c)?.nick ?? c.name}`
}
const author = (item: InboxItem) => {
  const id = item.senderId
  return id
    ? (chat.member(id)?.displayName ?? chat.member(id)?.nick ?? t('collectionsUi.inbox.someone'))
    : ''
}
const reasonKey: Record<InboxReason, string> = {
  mention: 'reasonMention',
  reply: 'reasonReply',
  direct: 'reasonDirect',
}
const reasonLabel = (reason: InboxReason) => t(`collectionsUi.inbox.${reasonKey[reason]}`)
const reasonIcon: Record<InboxReason, string> = { mention: 'at', reply: 'reply', direct: 'message' }

const emptyText = computed<Record<InboxFilter, string>>(() => ({
  all: t('collectionsUi.inbox.emptyAll'),
  unread: t('collectionsUi.inbox.emptyUnread'),
  mentions: t('collectionsUi.inbox.emptyMentions'),
  replies: t('collectionsUi.inbox.emptyReplies'),
  direct: t('collectionsUi.inbox.emptyDirect'),
  dismissed: t('collectionsUi.inbox.emptyDismissed'),
}))

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
    const index = viewIds.indexOf(view.value)
    const next = (index + (event.key === 'ArrowRight' ? 1 : -1) + viewIds.length) % viewIds.length
    view.value = viewIds[next]!
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
  if (item.status === 'unavailable') return chat.notify(t('collectionsUi.inbox.conversationClosed'))
  emit('open', item)
}
function toggle(item: InboxItem) {
  return item.dismissed ? inbox.restore(item.messageId) : inbox.dismiss(item.messageId)
}
</script>
<template>
  <WorkspacePage
    :title="t('collectionsUi.inbox.title')"
    icon="inbox"
    :description="t('collectionsUi.inbox.description')"
    :meta="
      inbox.unread
        ? t('collectionsUi.inbox.metaUnread', { count: n(inbox.unread) })
        : t('collectionsUi.inbox.allCaughtUp')
    "
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
            :aria-label="t('collectionsUi.inbox.view')"
          >
            <UiTabsTrigger
              v-for="v in views"
              :key="v.id"
              :value="v.id"
              variant="pill"
              :autofocus="view === v.id || undefined"
            >
              {{ v.label
              }}<span v-if="v.id === 'unread' && inbox.unread">{{ n(inbox.unread) }}</span>
            </UiTabsTrigger>
          </UiTabsList></WorkspaceToolbar
        >
        <p
          v-if="notice"
          class="saved-offline mx-4 mt-0 mb-2 text-[13px] text-muted-foreground"
        >
          {{ notice }}
          <UiButton
            v-if="stale && !offline"
            variant="link"
            size="text"
            @click="first.refetch()"
          >
            {{ t('common.tryAgain') }}
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
                :label="t('collectionsUi.inbox.loading')"
              />
              <EmptyState
                v-else-if="failed"
                role="alert"
              >
                <AppIcon
                  name="alert"
                  :size="27"
                />
                <p>
                  {{
                    t('collectionsUi.inbox.loadFailed', { error: errorMessage(first.error.value) })
                  }}
                </p>
                <UiButton
                  variant="link"
                  size="text"
                  @click="first.refetch()"
                >
                  {{ t('common.tryAgain') }}
                </UiButton>
              </EmptyState>
              <EmptyState v-else-if="!items.length">
                <AppIcon
                  name="inbox"
                  :size="28"
                />
                <p v-if="cursor">{{ t('collectionsUi.inbox.nothingOnPage') }}</p>
                <p v-else>{{ emptyText[view] }}</p>
              </EmptyState>
              <ul
                v-else
                class="saved-list list-none m-0 p-0 space-y-0.5"
                :aria-label="
                  t('collectionsUi.inbox.list', {
                    view: views.find((v) => v.id === view)?.label ?? '',
                  })
                "
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
                        : t('collectionsUi.inbox.unavailableItem', {
                            status: t(
                              item.status === 'deleted'
                                ? 'collectionsUi.inbox.deletedMessage'
                                : 'collectionsUi.inbox.unavailableMessage',
                            ),
                            place: place(item),
                          })
                    "
                    :time="{
                      iso: item.createdAt,
                      label: `${day(item.createdAt)} · ${time(item.createdAt)}`,
                      title: t('collectionsUi.inbox.sent', { date: full(item.createdAt) }),
                    }"
                    @click="open(item)"
                    @focus="selected = index"
                  >
                    <template #location
                      ><i
                        v-if="item.unread && !item.dismissed"
                        class="unread-dot w-[7px] h-[7px] flex-none bg-foreground rounded-full -mr-0.5"
                        role="img"
                        :aria-label="t('collectionsUi.inbox.unreadLabel')"
                      />{{ place(item) }} · {{ networkName(item) }}</template
                    ><template #badges>
                      <span
                        v-for="reason in item.reasons"
                        :key="reason"
                        class="inbox-reason inline-flex items-center gap-1 self-center rounded-sm bg-secondary px-1.5 py-px text-[11px] text-muted-foreground"
                        ><AppIcon
                          :name="reasonIcon[reason]"
                          :size="11"
                        />{{ reasonLabel(reason) }}</span
                      ></template
                    ><span v-if="item.status === 'available'"
                      ><em v-if="item.kind === 'action'">{{ item.excerpt }}</em
                      ><template v-else>{{ item.excerpt }}</template></span
                    >
                    <span
                      v-else-if="item.status === 'deleted'"
                      class="message-deleted text-muted-foreground italic"
                      ><em>{{ t('collectionsUi.inbox.deletedMessage') }}</em></span
                    >
                    <span
                      v-else
                      class="message-deleted text-muted-foreground italic"
                      ><em>{{ t('collectionsUi.inbox.messageUnavailable') }}</em></span
                    >
                  </MessageResult>
                  <UiButton
                    variant="ghost"
                    size="icon-sm"
                    class="saved-remove flex-none mt-2 mr-2 text-muted-foreground hover:text-primary"
                    :aria-label="
                      t(
                        item.dismissed
                          ? 'collectionsUi.inbox.restore'
                          : 'collectionsUi.inbox.dismiss',
                      )
                    "
                    :title="
                      t(
                        item.dismissed
                          ? 'collectionsUi.inbox.restoreShortcut'
                          : 'collectionsUi.inbox.dismissShortcut',
                      )
                    "
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
                v-if="moreError"
                as="p"
                variant="destructive"
                presentation="inline"
                class="form-error mt-3"
              >
                {{ t('collectionsUi.inbox.loadMoreFailed', { error: errorMessage(moreError) }) }}
              </UiAlert>
              <UiButton
                v-if="!loading && cursor && !failed"
                variant="link"
                size="text"
                class="saved-more block mt-2 mr-auto mb-0 ml-auto"
                :disabled="loadingMore"
                @click="loadMore"
              >
                {{
                  loadingMore
                    ? t('collectionsUi.inbox.loadingMore')
                    : moreError
                      ? t('common.tryAgain')
                      : t('collectionsUi.inbox.loadOlder')
                }}
              </UiButton>
            </div>
          </div>
        </UiTabsContent>
      </UiTabs>
    </div>
    <WorkspaceFooter class="inbox-footer">
      <span><kbd>↑</kbd> <kbd>↓</kbd> {{ t('collectionsUi.inbox.keyboardChoose') }}</span
      ><span><kbd>←</kbd> <kbd>→</kbd> {{ t('collectionsUi.inbox.keyboardViews') }}</span
      ><span><kbd>↵</kbd> {{ t('collectionsUi.inbox.keyboardOpen') }}</span
      ><span><kbd>E</kbd> {{ t('collectionsUi.inbox.keyboardDismiss') }}</span
      ><span
        ><kbd>{{ t('workspaceUi.shortcuts.keys.escape') }}</kbd>
        {{ t('collectionsUi.inbox.keyboardReturn') }}</span
      >
    </WorkspaceFooter></WorkspacePage
  >
</template>
