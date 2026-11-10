<script setup lang="ts" vapor>
import { useQuery } from '@pinia/colada'
import type { SavedItem } from '../../shared/types/chat'
import {
  patchSavedItem,
  SAVED_PAGE,
  savedListQuery,
  savedPatch,
  savedRevision,
} from '../queries/saved'

/**
 * Your private saved messages across every network, newest first. Opening one jumps to it in
 * its conversation; removing one only drops your bookmark.
 */
const emit = defineEmits<{ close: []; open: [item: SavedItem] }>()
const chat = useChatStore()
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
const older = ref<SavedItem[]>([])
const olderCursor = ref<string | null>(null)
const loadingMore = ref(false)
const moreError = ref<unknown>('')
// An edited or deleted source changes its loaded entry in place; the list keeps your place.
watch(savedPatch, (message) => {
  if (message) older.value = older.value.map((item) => patchSavedItem(item, message))
})
watch([debounced, savedRevision], () => {
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
  const q = debounced.value
  const revision = savedRevision.value
  try {
    const page = await apiClient().savedMessages({
      cursor: cursor.value,
      q: q || undefined,
      limit: SAVED_PAGE,
    })
    if (q !== debounced.value || revision !== savedRevision.value) return
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

/** Removed bookmarks disappear at once, also when another device removed them. */
const items = computed(() =>
  [...(first.data.value?.items ?? []), ...older.value].filter(
    (i) => !saved.loaded || saved.isSaved(i.messageId),
  ),
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
  if (c.kind === 'channel') return `#${c.name}`
  return `@${chat.directPartner(c)?.nick ?? c.name}`
}
const author = (item: SavedItem) => {
  const id = item.message?.senderId
  return id
    ? (chat.member(id)?.displayName ?? chat.member(id)?.nick ?? t('collectionsUi.saved.someone'))
    : ''
}

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
    icon="bookmark"
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
        <EmptyState v-else-if="!items.length">
          <AppIcon
            name="bookmark"
            :size="28"
          />
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
        <ul
          v-else
          class="saved-list list-none m-0 p-0 space-y-0.5"
          :aria-label="t('collectionsUi.saved.list')"
        >
          <li
            v-for="(item, index) in items"
            :key="item.messageId"
            class="saved-item group flex items-start gap-1 rounded-md transition-colors hover:not-[.is-selected]:bg-accent/60 [&.is-selected]:bg-selection/70"
            :class="{ 'is-selected': index === selected, 'is-gone': item.status !== 'available' }"
          >
            <MessageResult
              plain
              :member="item.message ? chat.member(item.message.senderId) : undefined"
              :author="item.status === 'unavailable' ? undefined : author(item)"
              class="saved-open min-w-0 flex-1 aria-disabled:cursor-default"
              :class="{ 'text-muted-foreground': item.status !== 'available' }"
              :aria-disabled="item.status === 'unavailable' || undefined"
              :time="
                item.message && {
                  iso: item.message.createdAt,
                  label: `${day(item.message.createdAt)} · ${time(item.message.createdAt)}`,
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
