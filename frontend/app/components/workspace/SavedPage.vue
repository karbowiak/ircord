<script setup lang="ts" vapor>
import { useQuery } from '@pinia/colada'
import type { SavedItem } from '../../../shared/types/chat'
import {
  patchSavedItem,
  SAVED_PAGE,
  savedListQuery,
  savedPatch,
  savedRevision,
} from '../../queries/saved'
import { useCursorPages } from '../../composables/shared/useCursorPages'

/**
 * Your private saved messages across every network, newest first. Opening one jumps to it in
 * its conversation; removing one only drops your bookmark.
 */
const emit = defineEmits<{ close: []; open: [item: SavedItem] }>()
const chat = useChatStore()
const conversationName = useSidebarOrder().name
const saved = useSavedStore()
const { full, day, time } = useTimeFormat()
const { t, n } = useI18n()
const errorMessage = (value: unknown) => errorText(value, (key) => t(key))
const query = ref('')
const debounced = ref('')
let timer: ReturnType<typeof setTimeout> | undefined
watch(query, (q) => {
  clearTimeout(timer)
  timer = setTimeout(() => (debounced.value = q.trim()), 220)
})
onBeforeUnmount(() => clearTimeout(timer))

const first = useQuery(() => savedListQuery(debounced.value))
/** Pages after the first, loaded on demand; dropped whenever the listing may have changed. */
const {
  older,
  items: listed,
  cursor,
  loadMore,
  loadingMore,
  moreError,
} = useCursorPages<SavedItem>({
  firstPage: first.data,
  resetOn: [debounced, savedRevision],
  itemKey: (item) => item.messageId,
  load: (cursor, signal) =>
    apiClient().savedMessages(
      { cursor, q: debounced.value || undefined, limit: SAVED_PAGE },
      signal,
    ),
})
// An edited or deleted source changes its loaded entry in place; the list keeps your place.
watch(savedPatch, (message) => {
  if (message) older.value = older.value.map((item) => patchSavedItem(item, message))
})

/** Removed bookmarks disappear at once, also when another device removed them. */
const items = computed(() =>
  listed.value.filter((i) => !saved.loaded || saved.isSaved(i.messageId)),
)
const loading = computed(
  () => query.value.trim() !== debounced.value || (first.isLoading.value && !first.data.value),
)
const offline = computed(() => chat.status === 'offline')
const offlineText = computed(() => t('collectionsUi.saved.offline'))
useAnnouncedStatus(() => (offline.value ? offlineText.value : ''))
// The count is read once the filter settles, not on every keypress.
useAnnouncedStatus(() =>
  debounced.value.trim() && !loading.value
    ? t(
        cursor.value ? 'collectionsUi.saved.countFoundMore' : 'collectionsUi.saved.countFound',
        { count: n(items.value.length) },
        items.value.length,
      )
    : '',
)

const networkName = (item: SavedItem) =>
  chat.data?.networks.find((n) => n.id === item.networkId)?.name ??
  t('collectionsUi.saved.networkRemoved')
function place(item: SavedItem) {
  const c = item.conversation
  if (!c) return t('collectionsUi.saved.conversationUnavailable')
  if (c.party) return conversationName(c)
  if (c.kind === 'channel') return `#${c.name}`
  return `@${chat.directPartner(c)?.nick ?? c.name}`
}
const author = (item: SavedItem) => {
  const id = item.message?.senderId
  return id
    ? (chat.member(id, item.networkId)?.displayName ??
        chat.member(id, item.networkId)?.nick ??
        t('collectionsUi.saved.someone'))
    : ''
}

/** Items under day headings, by when the message was sent; each keeps its list index. */
const groups = computed(() => {
  const out: { label: string; entries: { item: SavedItem; index: number }[] }[] = []
  items.value.forEach((item, index) => {
    const label = item.message ? day(item.message.createdAt) : t('collectionsUi.saved.undated')
    const last = out.at(-1)
    if (last?.label === label) last.entries.push({ item, index })
    else out.push({ label, entries: [{ item, index }] })
  })
  return out
})
const selected = ref(0)
watch(items, (list) => (selected.value = Math.min(selected.value, Math.max(0, list.length - 1))))
const list = ref<HTMLElement | null>(null)
function keys(event: KeyboardEvent) {
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    const count = items.value.length
    if (!count) return
    selected.value = (selected.value + (event.key === 'ArrowDown' ? 1 : -1) + count) % count
    nextTick(() => list.value?.querySelector('.is-selected')?.scrollIntoView({ block: 'nearest' }))
  } else if (event.key === 'Enter') {
    event.preventDefault()
    const item = items.value[selected.value]
    if (item) open(item)
  }
}
function open(item: SavedItem) {
  if (item.status === 'unavailable') return chat.notify(t('collectionsUi.saved.conversationClosed'))
  emit('open', item)
}
</script>
<template>
  <WorkspacePage
    :title="t('collectionsUi.saved.title')"
    :description="t('collectionsUi.saved.description')"
    :meta="
      items.length
        ? t(
            cursor ? 'collectionsUi.saved.metaMore' : 'collectionsUi.saved.meta',
            { count: n(items.length) },
            items.length,
          )
        : undefined
    "
    @close="emit('close')"
    ><WorkspaceToolbar
      ><SearchField
        v-model="query"
        size="lg"
        class="search-field"
        autofocus
        :placeholder="t('collectionsUi.saved.filter')"
        :aria-label="t('collectionsUi.saved.filter')"
        @keydown="keys"
      />
    </WorkspaceToolbar>
    <p
      v-if="offline"
      class="saved-offline mx-4 mt-0 mb-2 text-[13px] text-muted-foreground"
    >
      {{ offlineText }}
    </p>
    <div
      ref="list"
      class="search-results saved-results min-h-0 flex-1 overflow-y-auto px-4 pb-4 max-[700px]:px-1.5"
    >
      <div class="w-full">
        <WorkspaceLoading
          v-if="loading"
          :label="t('collectionsUi.saved.loading')"
        />
        <EmptyState
          v-else-if="first.error.value && !first.data.value"
          role="alert"
        >
          <AppIcon
            name="alert"
            :size="27"
          />
          <p>
            {{ t('collectionsUi.saved.loadFailed', { error: errorMessage(first.error.value) }) }}
          </p>
          <UiButton
            variant="link"
            size="text"
            @click="first.refetch()"
          >
            {{ t('common.tryAgain') }}
          </UiButton>
        </EmptyState>
        <EmptyState
          v-else-if="!items.length"
          icon="bookmark"
          :title="cursor || debounced ? undefined : t('collectionsUi.saved.emptyTitle')"
        >
          <p v-if="cursor">
            {{
              debounced
                ? t('collectionsUi.saved.noQueryHere', { query: debounced })
                : t('collectionsUi.saved.nothingOnPage')
            }}
            {{ t('collectionsUi.saved.olderMayMatch') }}
          </p>
          <p v-else-if="debounced">{{ t('collectionsUi.saved.noQuery', { query: debounced }) }}</p>
          <p v-else>{{ t('collectionsUi.saved.empty') }}</p>
        </EmptyState>
        <div
          v-else
          class="saved-list"
          role="group"
          :aria-label="t('collectionsUi.saved.list')"
        >
          <section
            v-for="group in groups"
            :key="group.label"
            class="mb-3 last:mb-0"
          >
            <h2
              class="list-label m-0 flex h-8 items-center px-3 text-[length:var(--ui-caption)] font-semibold uppercase tracking-[0.02em] text-muted-foreground"
            >
              {{ group.label }}
            </h2>
            <ul class="m-0 list-none space-y-0.5 p-0">
              <li
                v-for="{ item, index } in group.entries"
                :key="item.messageId"
                class="saved-item group flex items-start gap-1 rounded-md transition-colors hover:not-[.is-selected]:bg-accent/60 [&.is-selected]:bg-selection/70"
                :class="{
                  'is-selected': index === selected,
                  'is-gone': item.status !== 'available',
                }"
              >
                <MessageResult
                  plain
                  :member="
                    item.message ? chat.member(item.message.senderId, item.networkId) : undefined
                  "
                  :author="item.status === 'unavailable' ? undefined : author(item)"
                  class="saved-open min-w-0 flex-1 aria-disabled:cursor-default"
                  :class="{ 'text-muted-foreground': item.status !== 'available' }"
                  :aria-disabled="item.status === 'unavailable' || undefined"
                  :time="
                    item.message && {
                      iso: item.message.createdAt,
                      label: time(item.message.createdAt),
                      title: t('collectionsUi.saved.sent', { date: full(item.message.createdAt) }),
                    }
                  "
                  @click="open(item)"
                  @focus="selected = index"
                >
                  <template #location>{{ place(item) }} · {{ networkName(item) }}</template
                  ><MessageContent
                    v-if="item.status === 'available' && item.message"
                    :text="item.message.text"
                  />
                  <span
                    v-else-if="item.status === 'deleted'"
                    class="message-deleted text-muted-foreground italic"
                    ><em>{{ t('collectionsUi.saved.messageDeleted') }}</em></span
                  >
                  <span
                    v-else
                    class="message-deleted text-muted-foreground italic"
                    ><em>{{ t('collectionsUi.saved.messageUnavailable') }}</em></span
                  >
                </MessageResult>
                <UiButton
                  variant="ghost"
                  size="icon-sm"
                  class="saved-remove flex-none mt-2 mr-2 text-muted-foreground hover:text-primary"
                  :aria-label="
                    t('collectionsUi.saved.removeFromSaved', {
                      author:
                        item.status === 'available'
                          ? author(item)
                          : t('collectionsUi.saved.unavailableMessage'),
                    })
                  "
                  :title="t('collectionsUi.saved.remove')"
                  :disabled="saved.isPending(item.messageId)"
                  @click="saved.remove(item.messageId)"
                >
                  <AppIcon
                    name="bookmarkSaved"
                    :size="17"
                  />
                </UiButton>
              </li>
            </ul>
          </section>
        </div>
        <UiAlert
          v-if="moreError"
          as="p"
          variant="destructive"
          presentation="inline"
          class="form-error mt-3"
        >
          {{ t('collectionsUi.saved.loadMoreFailed', { error: errorMessage(moreError) }) }}
        </UiAlert>
        <UiButton
          v-if="!loading && cursor && !(first.error.value && !first.data.value)"
          variant="link"
          size="text"
          class="saved-more block mt-2 mr-auto mb-0 ml-auto"
          :disabled="loadingMore"
          @click="loadMore"
        >
          {{
            loadingMore
              ? t('collectionsUi.saved.loadingMore')
              : moreError
                ? t('common.tryAgain')
                : t('collectionsUi.saved.loadOlder')
          }}
        </UiButton>
      </div>
    </div>
    <WorkspaceFooter>
      <span><kbd>↑</kbd> <kbd>↓</kbd> {{ t('collectionsUi.saved.keyboardChoose') }}</span
      ><span><kbd>↵</kbd> {{ t('collectionsUi.saved.keyboardOpen') }}</span
      ><span
        ><kbd>{{ t('workspaceUi.shortcuts.keys.escape') }}</kbd>
        {{ t('collectionsUi.saved.keyboardReturn') }}</span
      >
    </WorkspaceFooter></WorkspacePage
  >
</template>
