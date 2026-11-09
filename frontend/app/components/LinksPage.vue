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
const { t, n } = useI18n()
const errorMessage = (value: unknown) => errorText(value, (key) => t(key))
const translate = (key: string, values?: Record<string, string | number>, plural?: number) =>
  t(
    key,
    Object.fromEntries(
      Object.entries(values ?? {}).map(([name, value]) => [
        name,
        typeof value === 'number' ? n(value) : value,
      ]),
    ),
    plural,
  )

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
const hostFilter = computed(() => linkHostFilter(debouncedHost.value, translate))
const range = computed(() => linkDateRange(from.value, to.value, translate))
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
  channelChoiceLabel(
    picked.value.map((id) => conversation(id)?.name ?? ''),
    translate,
  ),
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
    ? t('collectionsUi.links.allKinds')
    : kinds.value.length === 1
      ? (kindOptions.value.find((kind) => kind.value === kinds.value[0])?.label ??
        t('collectionsUi.links.allKinds'))
      : t(
          'collectionsUi.links.selectedKinds',
          { count: n(kinds.value.length) },
          kinds.value.length,
        ),
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
const moreError = ref<unknown>('')
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
    moreError.value = error
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
const offlineText = computed(() => t('collectionsUi.links.offline'))
const indexingText = computed(() => t('collectionsUi.links.indexing'))
// Notices, filter problems and counts are read once they settle, not while typing.
useAnnouncedStatus(() => (offline.value ? offlineText.value : ''))
useAnnouncedStatus(() => (first.data.value?.indexing ? indexingText.value : ''))
useAnnouncedStatus(() => problem.value)
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
useAnnouncedStatus(() =>
  filtered.value && !loading.value && !problem.value && !first.error.value
    ? t(
        cursor.value ? 'collectionsUi.links.countFoundMore' : 'collectionsUi.links.countFound',
        { count: n(items.value.length) },
        items.value.length,
      )
    : '',
)

const conversation = (id: string) => chat.data?.conversations.find((c) => c.id === id)
function jump(source: LinkSource) {
  if (!conversation(source.conversationId))
    return chat.notify(t('collectionsUi.links.channelClosed'))
  emit('open', source)
}

const kindLabels: Record<LinkKind, string> = {
  link: 'linkKindPages',
  image: 'linkKindImages',
  gif: 'linkKindGifs',
  video: 'linkKindVideos',
  audio: 'linkKindAudio',
  youtube: 'linkKindYoutube',
}
const kindOptions = computed<FilterOption[]>(() =>
  linkKinds.map((kind) => ({
    value: kind.value,
    label: t(`collectionsUi.links.${kindLabels[kind.value]}`),
    icon: kind.icon,
  })),
)
const networkOptions = computed<FilterOption[]>(() => [
  { value: '', label: t('collectionsUi.links.allNetworks') },
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
    :title="t('collectionsUi.links.title')"
    icon="link"
    :description="t('collectionsUi.links.description')"
    :meta="
      items.length
        ? t(
            cursor ? 'collectionsUi.links.metaMore' : 'collectionsUi.links.meta',
            { count: n(items.length) },
            items.length,
          )
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
          :placeholder="t('collectionsUi.links.filter')"
          :aria-label="t('collectionsUi.links.filterLabel')"
          @keydown="keys"
        />
        <FilterMenu
          v-if="channels.length > 1"
          v-model="chosen"
          :label="t('collectionsUi.links.channels')"
          :display="pickerLabel"
          :options="channelOptions"
          searchable
          :search-placeholder="t('collectionsUi.links.findChannel')"
          :clear-label="t('collectionsUi.links.anyChannel')"
          :note="
            picked.length >= MAX_LINK_CHANNELS
              ? t('collectionsUi.links.channelLimit', { count: n(MAX_LINK_CHANNELS) })
              : undefined
          "
        />
        <FilterMenu
          v-if="!picked.length && networks.length > 1"
          v-model="network"
          :label="t('collectionsUi.links.network')"
          :display="
            networkOptions.find((o) => o.value === network)?.label ??
            t('collectionsUi.links.allNetworks')
          "
          :options="networkOptions"
        />
        <FilterMenu
          v-model="kinds"
          :label="t('collectionsUi.links.kind')"
          :display="kindsLabel"
          :options="kindOptions"
          :clear-label="t('collectionsUi.links.allKinds')"
        />
        <UiPopover>
          <UiPopoverTrigger as-child>
            <UiButton
              variant="secondary"
              class="links-more data-[state=open]:bg-accent"
              ><AppIcon
                name="settings"
                :size="16"
              />{{ t('collectionsUi.links.filters')
              }}<span
                v-if="extraFilters"
                class="rounded-full bg-selection px-1.5 text-[length:var(--ui-label)] text-primary"
                >{{ n(extraFilters) }}</span
              ></UiButton
            >
          </UiPopoverTrigger>
          <UiPopoverContent
            align="end"
            class="links-advanced grid w-80 gap-3 p-3"
            :aria-label="t('collectionsUi.links.moreFilters')"
          >
            <label
              class="grid gap-1.5 text-[length:var(--ui-caption)] font-medium text-muted-foreground"
              >{{ t('collectionsUi.links.host') }}
              <UiInput
                v-model="hostInput"
                class="links-host"
                type="text"
                inputmode="url"
                autocomplete="off"
                spellcheck="false"
                :placeholder="t('collectionsUi.links.exampleHost')"
                :aria-label="t('collectionsUi.links.host')"
                :aria-invalid="'error' in hostFilter || undefined"
                :aria-describedby="'error' in hostFilter ? 'links-filter-problem' : undefined"
              />
              <span class="font-normal text-faint">{{
                t('collectionsUi.links.subdomainsMatch')
              }}</span>
            </label>
            <fieldset class="m-0 grid min-w-0 gap-1.5 border-0 p-0">
              <legend
                class="mb-1.5 p-0 text-[length:var(--ui-caption)] font-medium text-muted-foreground"
              >
                {{ t('collectionsUi.links.postedBetween') }}
              </legend>
              <div class="links-dates grid grid-cols-2 gap-2">
                <UiInput
                  v-model="from"
                  type="date"
                  :max="to || today"
                  :aria-label="t('collectionsUi.links.postedFrom')"
                  :aria-invalid="'error' in range || undefined"
                  :aria-describedby="'error' in range ? 'links-filter-problem' : undefined"
                  @focus="refreshToday"
                />
                <UiInput
                  v-model="to"
                  type="date"
                  :min="from || undefined"
                  :max="today"
                  :aria-label="t('collectionsUi.links.postedUntil')"
                  :aria-invalid="'error' in range || undefined"
                  :aria-describedby="'error' in range ? 'links-filter-problem' : undefined"
                  @focus="refreshToday"
                />
              </div>
            </fieldset>
            <!-- Computed while typing: the fields describe it, and it is announced once settled. -->
            <p
              v-if="problem"
              id="links-filter-problem"
              class="m-0 text-[length:var(--ui-caption)] text-destructive"
            >
              {{ problem }}
            </p>
            <div class="flex justify-end">
              <UiButton
                variant="link"
                size="text"
                :disabled="!extraFilters"
                @click="clearExtra"
                >{{ t('collectionsUi.links.clear') }}</UiButton
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
          >{{ t('collectionsUi.links.clearFilters') }}</UiButton
        >
      </div>
    </WorkspaceToolbar>
    <p
      v-if="offline"
      class="saved-offline mx-4 mt-0 mb-2 text-[13px] text-muted-foreground"
    >
      {{ offlineText }}
    </p>
    <p
      v-if="first.data.value?.indexing"
      class="saved-offline mx-4 mt-0 mb-2 text-[13px] text-muted-foreground"
    >
      {{ indexingText }}
    </p>
    <UiButton
      v-else-if="newPosted && !loading"
      variant="link"
      size="text"
      class="links-new mx-4 mt-0 mb-2 self-start"
      @click="showNew"
    >
      {{ t('collectionsUi.links.newPosted') }}
    </UiButton>
    <div
      ref="list"
      class="search-results saved-results min-h-0 flex-1 overflow-y-auto px-4 pb-4 max-[700px]:px-1.5"
    >
      <div class="w-full">
        <EmptyState v-if="problem">
          <AppIcon
            name="alert"
            :size="27"
          />
          <p>{{ problem }}</p>
        </EmptyState>
        <WorkspaceLoading
          v-else-if="loading"
          :label="t('collectionsUi.links.loadLinks')"
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
            {{ t('collectionsUi.links.loadFailed', { error: errorMessage(first.error.value) }) }}
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
            name="link"
            :size="28"
          />
          <template v-if="filtered">
            <p>{{ t('collectionsUi.links.noMatches') }}</p>
            <UiButton
              variant="link"
              size="text"
              @click="clearFilters"
            >
              {{ t('collectionsUi.links.clearFilters') }}
            </UiButton>
          </template>
          <p v-else-if="filter.conversations.length === 1">
            {{ t('collectionsUi.links.noLinksInChannel', { channel: pickerLabel }) }}
          </p>
          <p v-else-if="filter.conversations.length">
            {{ t('collectionsUi.links.noLinksInChannels') }}
          </p>
          <p v-else>{{ t('collectionsUi.links.noLinks') }}</p>
        </EmptyState>
        <ul
          v-else
          class="links-list m-0 grid list-none gap-4 p-0 [grid-template-columns:repeat(auto-fill,minmax(min(100%,calc(280px*var(--interface-scale,1))),1fr))]"
          :aria-label="t('collectionsUi.links.list')"
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
          v-if="moreError"
          as="p"
          variant="destructive"
          presentation="inline"
          class="form-error mt-3"
        >
          {{ t('collectionsUi.links.loadMoreFailed', { error: errorMessage(moreError) }) }}
        </UiAlert>
        <UiButton
          v-if="!problem && !loading && cursor && !(first.error.value && !first.data.value)"
          variant="link"
          size="text"
          class="saved-more block mt-2 mr-auto mb-0 ml-auto"
          :disabled="loadingMore"
          @click="loadMore"
        >
          {{
            loadingMore
              ? t('collectionsUi.links.loadingMore')
              : moreError
                ? t('common.tryAgain')
                : t('collectionsUi.links.loadOlder')
          }}
        </UiButton>
      </div>
    </div>
    <WorkspaceFooter>
      <span><kbd>↑</kbd> <kbd>↓</kbd> {{ t('collectionsUi.links.keyboardChoose') }}</span
      ><span><kbd>↵</kbd> {{ t('collectionsUi.links.keyboardJump') }}</span
      ><span
        ><kbd>{{ t('workspaceUi.shortcuts.keys.escape') }}</kbd>
        {{ t('collectionsUi.links.keyboardReturn') }}</span
      >
    </WorkspaceFooter></WorkspacePage
  >
</template>
