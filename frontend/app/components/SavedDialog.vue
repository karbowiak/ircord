<script setup lang="ts">
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
const { full } = useTimeFormat()
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
const moreError = ref('')
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
    moreError.value = errorText(error)
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

const networkName = (item: SavedItem) =>
  chat.data?.networks.find((n) => n.id === item.networkId)?.name ?? 'Removed network'
function place(item: SavedItem) {
  const c = item.conversation
  if (!c) return 'Unavailable conversation'
  if (c.kind === 'channel') return `#${c.name}`
  return `@${chat.directPartner(c)?.nick ?? c.name}`
}
const author = (item: SavedItem) => {
  const id = item.message?.senderId
  return id ? (chat.member(id)?.displayName ?? chat.member(id)?.nick ?? 'Someone') : ''
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
  if (item.status === 'unavailable')
    return chat.notify('That conversation isn’t open any more. Rejoin it to see this message.')
  emit('open', item)
}
</script>
<template>
  <AppDialog
    title="Saved messages"
    wide
    @close="emit('close')"
    ><SearchField
      v-model="query"
      size="lg"
      hint="esc"
      class="search-field mx-5"
      autofocus
      placeholder="Filter saved messages"
      aria-label="Filter saved messages"
      @keydown="keys"
    />
    <p
      v-if="offline"
      class="saved-offline mt-2.5 mr-5 mb-0 ml-5 text-[13px] text-muted-foreground"
      role="status"
    >
      You’re offline. Saved messages may be out of date until IRCord reconnects.
    </p>
    <div
      ref="list"
      class="search-results pt-2 pr-3 pb-3 pl-3 min-h-70 max-h-[min(460px,_calc(100dvh_-_260px))] overflow-y-auto saved-results"
    >
      <EmptyState
        as="p"
        v-if="loading"
      >
        Loading saved messages…
      </EmptyState>
      <EmptyState
        v-else-if="first.error.value && !first.data.value"
        role="alert"
      >
        <AppIcon
          name="alert"
          :size="27"
        />
        <p>Couldn’t load your saved messages: {{ errorText(first.error.value) }}</p>
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
          name="bookmark"
          :size="28"
        />
        <p v-if="cursor">
          {{
            debounced
              ? `No saved messages match “${debounced}” here.`
              : 'Nothing left on this page.'
          }}
          Older saved messages may still match.
        </p>
        <p v-else-if="debounced">No saved messages match “{{ debounced }}”.</p>
        <p v-else>
          Nothing saved yet. Choose <strong>Save for later</strong> on a message to keep it here.
          Only you can see your saved messages.
        </p>
      </EmptyState>
      <ul
        v-else
        class="saved-list list-none m-0 p-0"
        aria-label="Saved messages"
      >
        <li
          v-for="(item, index) in items"
          :key="item.messageId"
          class="saved-item flex items-start gap-1 rounded-sm"
          :class="{ 'is-selected': index === selected, 'is-gone': item.status !== 'available' }"
        >
          <MessageResult
            class="saved-open"
            :aria-disabled="item.status === 'unavailable' || undefined"
            :time="
              item.message && {
                iso: item.message.createdAt,
                label: full(item.message.createdAt),
                title: `Sent ${full(item.message.createdAt)}`,
              }
            "
            @click="open(item)"
            @focus="selected = index"
          >
            <template #location>{{ networkName(item) }} · {{ place(item) }}</template
            ><span v-if="item.status === 'available' && item.message"
              ><strong>{{ author(item) }}</strong
              ><MessageContent :text="item.message.text"
            /></span>
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
          </MessageResult>
          <UiButton
            variant="ghost"
            size="icon-sm"
            class="saved-remove flex-none mt-1.5 mr-1 mb-0 ml-0 text-primary"
            :aria-label="`Remove from saved: ${item.status === 'available' ? author(item) : 'unavailable message'}`"
            title="Remove from saved"
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
        v-if="!loading && cursor && !(first.error.value && !first.data.value)"
        class="saved-more block mt-2 mr-auto mb-0 ml-auto"
        :disabled="loadingMore"
        @click="loadMore"
      >
        {{ loadingMore ? 'Loading…' : moreError ? 'Try again' : 'Load older saved messages' }}
      </UiButton>
    </div>
    <footer
      class="search-footer flex gap-3.5 py-2.5 px-5 border-t [border-top-style:solid] border-t-border bg-sidebar text-[12px] text-faint"
    >
      <span><kbd>↑</kbd> <kbd>↓</kbd> to choose</span><span><kbd>↵</kbd> to open</span
      ><span><kbd>esc</kbd> to close</span>
    </footer></AppDialog
  >
</template>
