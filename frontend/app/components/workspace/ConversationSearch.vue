<script setup lang="ts" vapor>
import type { Conversation } from '../../../shared/types/chat'
import {
  activeToken,
  emptyFilters,
  filterCount,
  type SearchFilters,
} from '../../../shared/utils/search-filter'

/**
 * Message search within one conversation, in the member list's place, with the Search page's
 * filters except `in:`. Results open in the timeline beside it, so the panel stays for the next
 * match; the Search page searches everywhere.
 */
const props = defineProps<{ conversation: Conversation }>()
const emit = defineEmits<{ close: [] }>()
const chat = useChatStore()
const conversationName = useSidebarOrder().name
const actions = useWorkspaceActions()
const { day, time, full } = useTimeFormat()
const { t, n } = useI18n()
const errorMessage = (value: unknown) => errorText(value, (key) => t(key))
const query = ref('')
const debounced = ref('')
const filters = ref<SearchFilters>(emptyFilters())
const filtered = computed(() => filterCount(filters.value) > 0)
const showFilters = ref(false)
/** What has been typed, without a filter token still being written. */
const typed = computed(() => {
  const token = activeToken(query.value)
  return (token && token.key !== 'in' ? query.value.slice(0, token.start) : query.value).trim()
})
let timer: ReturnType<typeof setTimeout> | undefined
watch(typed, (q) => {
  clearTimeout(timer)
  timer = setTimeout(() => (debounced.value = q), 220)
})
onBeforeUnmount(() => clearTimeout(timer))
const scope = computed(() => ({ ...filters.value, in: [props.conversation.id] }))
const { search, results, cursor, loadMore, loadingMore, moreError } = useMessageSearch(
  debounced,
  scope,
)
const pending = computed(
  () => typed.value !== debounced.value || (search.isLoading.value && !search.data.value),
)
const place = computed(() =>
  props.conversation.party
    ? conversationName(props.conversation)
    : props.conversation.kind === 'channel'
      ? `#${props.conversation.name}`
      : (chat.directPartner(props.conversation)?.displayName ?? props.conversation.name),
)
const resultCount = computed(() =>
  t(
    cursor.value
      ? 'collectionsUi.conversationSearch.resultCountMore'
      : 'collectionsUi.conversationSearch.resultCount',
    { count: n(results.value.length) },
    results.value.length,
  ),
)
// The count is read once a search settles, not on every keypress or when the panel opens.
useAnnouncedStatus(
  () =>
    !pending.value && !search.error.value && (debounced.value || filtered.value)
      ? results.value.length
        ? resultCount.value
        : t('collectionsUi.conversationSearch.noMessages')
      : '',
  { context: () => props.conversation.id },
)

const selected = ref(0)
watch(results, () => (selected.value = 0))
const list = ref<HTMLElement | null>(null)
function open(messageId: string) {
  void actions.reveal(props.conversation.id, messageId)
}
function keys(event: KeyboardEvent) {
  const count = results.value.length
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    if (!count) return
    selected.value = (selected.value + (event.key === 'ArrowDown' ? 1 : -1) + count) % count
    nextTick(() => list.value?.querySelector('.is-selected')?.scrollIntoView({ block: 'nearest' }))
  } else if (event.key === 'Enter') {
    event.preventDefault()
    const result = results.value[selected.value]
    if (result) open(result.message.id)
  }
}

const field = ref<{ focus: () => void } | null>(null)
onMounted(() => field.value?.focus())
/** Back to the header button that opened the panel. */
function close() {
  emit('close')
  nextTick(() => document.querySelector<HTMLElement>('.conversation-search-toggle')?.focus())
}
</script>
<template>
  <aside
    class="member-sidebar conversation-search relative flex min-h-0 min-w-0 flex-col bg-background"
    :aria-label="t('collectionsUi.conversationSearch.search', { place })"
    @keydown.esc.stop.prevent="close"
  >
    <div class="flex shrink-0 items-start gap-1 px-3 pt-3 pb-2">
      <SearchQueryField
        ref="field"
        v-model:text="query"
        v-model:filters="filters"
        class="min-w-0!"
        :placeholder="t('collectionsUi.conversationSearch.search', { place })"
        :label="t('collectionsUi.conversationSearch.searchMessages', { place })"
        :exclude="['in']"
        @keydown="keys"
        @more="showFilters = true"
      />
      <AppTooltip :text="t('collectionsUi.search.filters')"
        ><UiButton
          variant="ghost"
          size="icon-sm"
          class="relative mt-[3px] shrink-0"
          :aria-label="
            filtered
              ? t(
                  'collectionsUi.search.filtersOn',
                  { count: n(filterCount(filters)) },
                  filterCount(filters),
                )
              : t('collectionsUi.search.filters')
          "
          @click="showFilters = true"
          ><AppIcon
            name="filters"
            :size="16"
          /><span
            v-if="filtered"
            class="absolute -top-0.5 -right-0.5 min-w-3.5 rounded-full bg-primary px-1 text-center text-[length:var(--ui-label)] leading-[14px] text-primary-foreground"
            aria-hidden="true"
            >{{ n(filterCount(filters)) }}</span
          ></UiButton
        ></AppTooltip
      >
      <AppTooltip :text="t('collectionsUi.conversationSearch.close')"
        ><UiButton
          variant="ghost"
          size="icon-sm"
          class="mt-[3px] shrink-0"
          :aria-label="t('collectionsUi.conversationSearch.close')"
          @click="close"
          ><AppIcon
            name="close"
            :size="16" /></UiButton
      ></AppTooltip>
    </div>
    <OfflineArchiveNotice :offline="search.data.value?.offline" />
    <div
      ref="list"
      class="min-h-0 flex-1 overflow-y-auto px-1.5 pb-3"
      role="region"
      :aria-label="t('collectionsUi.conversationSearch.results', { place })"
    >
      <WorkspaceLoading
        v-if="pending"
        :label="t('collectionsUi.conversationSearch.searching')"
      />
      <EmptyState
        v-else-if="search.error.value"
        class="px-3 py-10"
        role="alert"
      >
        <p>
          {{
            t('collectionsUi.conversationSearch.searchFailed', {
              error: errorMessage(search.error.value),
            })
          }}
        </p>
        <UiButton
          variant="secondary"
          size="sm"
          @click="search.refetch()"
          >{{ t('common.tryAgain') }}</UiButton
        >
      </EmptyState>
      <EmptyState
        v-else-if="!results.length"
        class="px-3 py-10"
      >
        <p>
          {{
            debounced
              ? t('collectionsUi.conversationSearch.noMessagesQuery', { place, query: debounced })
              : t('collectionsUi.conversationSearch.noMessagesFilters')
          }}
        </p>
      </EmptyState>
      <template v-else>
        <p class="m-0 px-2 pb-1 text-[length:var(--ui-caption)] text-muted-foreground">
          {{ resultCount }}
        </p>
        <MessageResult
          v-for="(result, index) in results"
          :key="result.message.id"
          :member="chat.member(result.message.senderId, conversation.networkId)"
          :author="chat.member(result.message.senderId, conversation.networkId)?.displayName"
          class="[&_.search-message-location]:flex-nowrap"
          :class="{ 'is-selected': index === selected }"
          :time="{
            iso: result.message.createdAt,
            label: `${day(result.message.createdAt)} ${time(result.message.createdAt)}`,
            title: full(result.message.createdAt),
          }"
          @click="open(result.message.id)"
          @focus="selected = index"
          ><MessageContent :text="result.message.text"
        /></MessageResult>
      </template>
      <UiAlert
        v-if="moreError"
        variant="destructive"
        presentation="inline"
        role="alert"
        >{{
          t('collectionsUi.conversationSearch.loadMoreFailed', { error: errorMessage(moreError) })
        }}</UiAlert
      >
      <UiButton
        v-if="cursor"
        variant="secondary"
        size="sm"
        class="mt-3"
        :disabled="loadingMore"
        @click="loadMore"
      >
        {{
          loadingMore
            ? t('collectionsUi.conversationSearch.loadingOlder')
            : t('collectionsUi.conversationSearch.loadOlder')
        }}
      </UiButton>
    </div>
    <div
      v-if="debounced.length >= 2"
      class="shrink-0 border-t [border-top-style:solid] border-border/40 px-3 py-2"
    >
      <UiButton
        variant="link"
        size="text"
        @click="actions.search(debounced)"
        >{{
          t('collectionsUi.conversationSearch.searchEverywhere', { query: debounced })
        }}</UiButton
      >
    </div>
    <SearchFiltersDialog
      v-if="showFilters"
      :filters="filters"
      scoped
      @apply="filters = { ...$event, in: [] }"
      @close="showFilters = false"
    />
  </aside>
</template>
