<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import type { LinkCard, LinkKind, LinkSource } from '../../shared/types/chat'
import type { FilterOption } from './FilterMenu.vue'
import {
  channelChoiceLabel,
  channelSelectable,
  MAX_LINK_CHANNELS,
  linkDateRange,
  linkHostFilter,
  linkKinds,
  linkRangeOpen,
  linkToday,
} from '../../shared/utils/link-catalog'
import {
  linkQueryParams,
  linksListed,
  linksQuery,
  linksRevision,
  postedIn,
  type LinkFilter,
} from '../queries/links'

/**
 * Links posted in your channels, grouped by what they point to and newest first: the channel
 * you opened it from, channels you choose, or every channel, narrowed by type, network, host and
 * the days they were posted. Built from your own history on the server, so
 * a card only ever names posts you can read; nothing is fetched to build it.
 */
const props = defineProps<{ conversationId?: string }>()
const emit = defineEmits<{ close: []; open: [source: LinkSource] }>()
const chat = useChatStore()

const channel = computed(() =>
  chat.data?.conversations.find((c) => c.id === props.conversationId && c.kind === 'channel'),
)
const channels = computed(() => chat.data?.conversations.filter((c) => c.kind === 'channel') ?? [])
/** Chosen channels; none lists every channel. Channels closed meanwhile drop out. */
const chosen = ref<string[]>(channel.value ? [channel.value.id] : [])
watch(
  () => props.conversationId,
  () => {
    chosen.value = channel.value ? [channel.value.id] : []
  },
)
const picked = computed(() => chosen.value.filter((id) => channels.value.some((c) => c.id === id)))
const network = ref('')
const kinds = ref<LinkKind[]>([])
const query = ref('')
const hostInput = ref('')
const from = ref('')
const to = ref('')
/** The latest day the inputs offer; refreshed on focus, so a dialog left open past midnight
 * still offers the new day. */
const today = ref(linkToday())
const refreshToday = () => (today.value = linkToday())
const debounced = ref('')
const debouncedHost = ref('')
let timer: ReturnType<typeof setTimeout> | undefined
watch([query, hostInput], ([q, host]) => {
  clearTimeout(timer)
  timer = setTimeout(() => {
    debounced.value = q.trim()
    debouncedHost.value = host.trim()
  }, 220)
})
onBeforeUnmount(() => clearTimeout(timer))
const hostFilter = computed(() => linkHostFilter(debouncedHost.value))
const range = computed(() => linkDateRange(from.value, to.value))
/** Why a typed filter can't be used; the list then ignores it until it is fixed. */
const problem = computed(
  () =>
    ('error' in hostFilter.value && hostFilter.value.error) ||
    ('error' in range.value && range.value.error) ||
    '',
)

const networks = computed(() => chat.data?.networks ?? [])
const filter = computed<LinkFilter>(() => ({
  conversations: picked.value,
  network: picked.value.length ? '' : network.value,
  kinds: kinds.value,
  q: debounced.value,
  host: 'host' in hostFilter.value ? hostFilter.value.host : '',
  since: 'error' in range.value ? undefined : range.value.since,
  until: 'error' in range.value ? undefined : range.value.until,
}))

/** Host and dates, in the Filters popover. */
const extraFilters = computed(() => [hostInput.value, from.value, to.value].filter(Boolean).length)
const pickerLabel = computed(() =>
  channelChoiceLabel(picked.value.map((id) => conversation(id)?.name ?? '')),
)
/** Channels by network; once the limit is reached only ticked ones stay selectable. */
const channelOptions = computed<FilterOption[]>(() =>
  networks.value.flatMap((n) =>
    channels.value
      .filter((c) => c.networkId === n.id)
      .map((c) => ({
        value: c.id,
        label: `#${c.name}`,
        group: networks.value.length > 1 ? n.name : undefined,
        disabled: !channelSelectable(picked.value, c.id),
      })),
  ),
)
const kindsLabel = computed(() =>
  !kinds.value.length
    ? 'All types'
    : kinds.value.length === 1
      ? (linkKinds.find((k) => k.value === kinds.value[0])?.label ?? 'All types')
      : `${kinds.value.length} types`,
)
function clearExtra() {
  hostInput.value = ''
  debouncedHost.value = ''
  from.value = ''
  to.value = ''
}
function clearFilters() {
  query.value = ''
  debounced.value = ''
  clearExtra()
  kinds.value = []
  network.value = ''
}
const first = useQuery(() => linksQuery(filter.value))

/** Pages after the first, loaded on demand; dropped whenever the listing may have changed. */
const older = ref<LinkCard[]>([])
const olderCursor = ref<string | null>(null)
const loadingMore = ref(false)
const moreError = ref('')
watch([filter, linksRevision], () => {
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
  const current = filter.value
  const revision = linksRevision.value
  try {
    const page = await apiClient().links(linkQueryParams(current, cursor.value))
    if (current !== filter.value || revision !== linksRevision.value) return
    const known = new Set(
      [...(first.data.value?.items ?? []), ...older.value].map((c) => c.identity),
    )
    older.value = [...older.value, ...page.items.filter((c) => !known.has(c.identity))]
    olderCursor.value = page.nextCursor ?? null
  } catch (error) {
    moreError.value = errorText(error)
  } finally {
    loadingMore.value = false
  }
}
/**
 * Links posted in the shown channels since the first page loaded: counted when it loads (also
 * from the cache, which then refreshes), so the offer never covers links already listed.
 */
const networkOf = (id: string) => chat.data?.conversations.find((c) => c.id === id)?.networkId
const postedAtLoad = ref(postedIn(filter.value, networkOf))
watch(
  () => first.data.value,
  () => (postedAtLoad.value = postedIn(filter.value, networkOf)),
)
// A range that already ended can't gain new posts, so it never offers them.
const newPosted = computed(
  () => linkRangeOpen(filter.value.until) && postedIn(filter.value, networkOf) > postedAtLoad.value,
)
/** Shows links posted since the list loaded, from the top. */
function showNew() {
  older.value = []
  olderCursor.value = null
  void first.refetch()
}

const items = computed(() => [...(first.data.value?.items ?? []), ...older.value])
// The posts this view lists, so the chat store refreshes it when one of them changes.
watch(
  items,
  (list) => {
    linksListed.clear()
    for (const card of list) for (const source of card.sources) linksListed.add(source.messageId)
  },
  { immediate: true },
)
onBeforeUnmount(() => linksListed.clear())
const loading = computed(
  () =>
    query.value.trim() !== debounced.value ||
    hostInput.value.trim() !== debouncedHost.value ||
    (first.isLoading.value && !first.data.value),
)
const offline = computed(() => chat.status === 'offline')
const filtered = computed(
  () =>
    !!(
      debounced.value ||
      kinds.value.length ||
      filter.value.network ||
      filter.value.host ||
      filter.value.since ||
      filter.value.until
    ),
)

const conversation = (id: string) => chat.data?.conversations.find((c) => c.id === id)
function jump(source: LinkSource) {
  if (!conversation(source.conversationId))
    return chat.notify('That channel isn’t open any more. Rejoin it to see this message.')
  emit('open', source)
}

const kindOptions: FilterOption[] = linkKinds.map((k) => ({
  value: k.value,
  label: k.label,
  icon: k.icon,
}))
const networkOptions = computed<FilterOption[]>(() => [
  { value: '', label: 'All networks' },
  ...networks.value.map((n) => ({ value: n.id, label: n.name })),
])

const selected = ref(0)
watch(items, (list) => (selected.value = Math.min(selected.value, Math.max(0, list.length - 1))))
const list = ref<HTMLElement | null>(null)
function keys(event: KeyboardEvent) {
  // While a filter problem or a pending filter replaces the list, keys act on nothing hidden.
  if (problem.value || loading.value) {
    if (event.key === 'Enter') event.preventDefault()
    return
  }
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    const count = items.value.length
    if (!count) return
    selected.value = (selected.value + (event.key === 'ArrowDown' ? 1 : -1) + count) % count
    nextTick(() => list.value?.querySelector('.is-selected')?.scrollIntoView({ block: 'nearest' }))
  } else if (event.key === 'Enter') {
    event.preventDefault()
    const source = items.value[selected.value]?.sources[0]
    if (source) jump(source)
  }
}
</script>
<template>
  <WorkspacePage
    title="Links"
    icon="link"
    description="The things shared in your channels, gathered in one place."
    :meta="
      items.length
        ? `${items.length}${cursor ? '+' : ''} ${items.length === 1 && !cursor ? 'link' : 'links'}`
        : undefined
    "
    @close="emit('close')"
    ><WorkspaceToolbar>
      <div class="links-filters flex flex-wrap items-center gap-2">
        <SearchField
          v-model="query"
          size="lg"
          class="search-field min-w-[min(calc(240px*var(--interface-scale,1)),100%)] flex-1"
          autofocus
          placeholder="Filter by address or message"
          aria-label="Filter links"
          @keydown="keys"
        />
        <FilterMenu
          v-if="channels.length > 1"
          v-model="chosen"
          label="Channels"
          :display="pickerLabel"
          :options="channelOptions"
          searchable
          search-placeholder="Find a channel"
          clear-label="Any channel"
          :note="
            picked.length >= MAX_LINK_CHANNELS
              ? `Up to ${MAX_LINK_CHANNELS} channels at once.`
              : undefined
          "
        />
        <FilterMenu
          v-if="!picked.length && networks.length > 1"
          v-model="network"
          label="Network"
          :display="networkOptions.find((o) => o.value === network)?.label ?? 'All networks'"
          :options="networkOptions"
        />
        <FilterMenu
          v-model="kinds"
          label="Link type"
          :display="kindsLabel"
          :options="kindOptions"
          clear-label="All types"
        />
        <UiPopover>
          <UiPopoverTrigger as-child>
            <UiButton
              variant="secondary"
              class="links-more data-[state=open]:bg-accent"
              ><AppIcon
                name="settings"
                :size="16"
              />Filters<span
                v-if="extraFilters"
                class="rounded-full bg-selection px-1.5 text-[length:var(--ui-label)] text-primary"
                >{{ extraFilters }}</span
              ></UiButton
            >
          </UiPopoverTrigger>
          <UiPopoverContent
            align="end"
            class="links-advanced grid w-80 gap-3 p-3"
            aria-label="More filters"
          >
            <label
              class="grid gap-1.5 text-[length:var(--ui-caption)] font-medium text-muted-foreground"
              >Host
              <UiInput
                v-model="hostInput"
                class="links-host"
                type="text"
                inputmode="url"
                autocomplete="off"
                spellcheck="false"
                placeholder="example.com"
                aria-label="Host"
                :aria-invalid="'error' in hostFilter || undefined"
              />
              <span class="font-normal text-faint">Subdomains match too.</span>
            </label>
            <fieldset class="m-0 grid min-w-0 gap-1.5 border-0 p-0">
              <legend
                class="mb-1.5 p-0 text-[length:var(--ui-caption)] font-medium text-muted-foreground"
              >
                Posted between
              </legend>
              <div class="links-dates grid grid-cols-2 gap-2">
                <UiInput
                  v-model="from"
                  type="date"
                  :max="to || today"
                  aria-label="Posted from"
                  :aria-invalid="'error' in range || undefined"
                  @focus="refreshToday"
                />
                <UiInput
                  v-model="to"
                  type="date"
                  :min="from || undefined"
                  :max="today"
                  aria-label="Posted until"
                  :aria-invalid="'error' in range || undefined"
                  @focus="refreshToday"
                />
              </div>
            </fieldset>
            <p
              v-if="problem"
              class="m-0 text-[length:var(--ui-caption)] text-destructive"
              role="alert"
            >
              {{ problem }}
            </p>
            <div class="flex justify-end">
              <UiButton
                variant="link"
                size="text"
                :disabled="!extraFilters"
                @click="clearExtra"
                >Clear</UiButton
              >
            </div>
          </UiPopoverContent>
        </UiPopover>
        <UiButton
          v-if="filtered"
          variant="link"
          size="text"
          class="ml-1"
          @click="clearFilters"
          >Clear filters</UiButton
        >
      </div>
    </WorkspaceToolbar>
    <p
      v-if="offline"
      class="saved-offline mx-4 mt-0 mb-2 text-[13px] text-muted-foreground"
      role="status"
    >
      You’re offline. Links may be out of date until IRCord reconnects.
    </p>
    <p
      v-if="first.data.value?.indexing"
      class="saved-offline mx-4 mt-0 mb-2 text-[13px] text-muted-foreground"
      role="status"
    >
      Older history is still being indexed, so some older links may be missing.
    </p>
    <UiButton
      variant="link"
      size="text"
      v-else-if="newPosted && !loading"
      class="links-new mx-4 mt-0 mb-2 self-start"
      @click="showNew"
    >
      New links were posted. Show them
    </UiButton>
    <div
      ref="list"
      class="search-results saved-results min-h-0 flex-1 overflow-y-auto px-4 pb-4 max-[700px]:px-1.5"
    >
      <div class="w-full">
        <EmptyState
          v-if="problem"
          role="alert"
        >
          <AppIcon
            name="alert"
            :size="27"
          />
          <p>{{ problem }}</p>
        </EmptyState>
        <WorkspaceLoading
          v-else-if="loading"
          label="Loading links"
        />
        <EmptyState
          v-else-if="first.error.value && !first.data.value"
          role="alert"
        >
          <AppIcon
            name="alert"
            :size="27"
          />
          <p>Couldn’t load links: {{ errorText(first.error.value) }}</p>
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
            name="link"
            :size="28"
          />
          <template v-if="filtered">
            <p>No links match these filters.</p>
            <UiButton
              variant="link"
              size="text"
              @click="clearFilters"
            >
              Clear filters
            </UiButton>
          </template>
          <p v-else-if="filter.conversations.length === 1">
            No links have been posted in {{ pickerLabel }} yet.
          </p>
          <p v-else-if="filter.conversations.length">
            No links have been posted in these channels yet.
          </p>
          <p v-else>No links have been posted in your channels yet.</p>
        </EmptyState>
        <ul
          v-else
          class="links-list m-0 grid list-none gap-4 p-0 [grid-template-columns:repeat(auto-fill,minmax(min(100%,calc(280px*var(--interface-scale,1))),1fr))]"
          aria-label="Links"
        >
          <LinkCatalogCard
            v-for="(card, index) in items"
            :key="card.identity"
            :card="card"
            :class="{ 'is-selected': index === selected }"
            @focusin="selected = index"
            @jump="jump"
          />
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
          v-if="!problem && !loading && cursor && !(first.error.value && !first.data.value)"
          class="saved-more block mt-2 mr-auto mb-0 ml-auto"
          :disabled="loadingMore"
          @click="loadMore"
        >
          {{ loadingMore ? 'Loading…' : moreError ? 'Try again' : 'Load older links' }}
        </UiButton>
      </div>
    </div>
    <WorkspaceFooter>
      <span><kbd>↑</kbd> <kbd>↓</kbd> to choose</span><span><kbd>↵</kbd> to jump to the post</span
      ><span><kbd>esc</kbd> to return</span>
    </WorkspaceFooter></WorkspacePage
  >
</template>
