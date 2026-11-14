<script setup lang="ts" vapor>
import { useQuery } from '@pinia/colada'
import type { LinkCard, LinkKind, LinkSource } from '../../../shared/types/chat'
import type { TagChip, TagKey, TagSuggestion } from '../shared/TagQueryField.vue'
import {
  channelChoiceLabel,
  channelSelectable,
  linkDateRange,
  linkHostFilter,
  linkKinds,
  linkRangeOpen,
} from '../../../shared/utils/link-catalog'
import {
  linkQueryParams,
  linksListed,
  linksQuery,
  linksRevision,
  linksClassificationOverrides,
  updateLinkClassifications,
  postedIn,
  type LinkFilter,
} from '../../queries/links'
import { useCursorPages } from '../../composables/shared/useCursorPages'

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
    { plural },
  )

const channel = computed(() =>
  chat.data?.conversations.find(
    (c) => c.id === props.conversationId && c.kind === 'channel' && !c.party,
  ),
)
const channels = computed(
  () => chat.data?.conversations.filter((c) => c.kind === 'channel' && !c.party) ?? [],
)
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
const debounced = ref('')
const debouncedHost = ref('')
let timer: ReturnType<typeof setTimeout> | undefined
/** What has been typed, without a filter token still being written (`type:im`). */
const typed = computed(() =>
  query.value.replace(/(^|\s)(?:in|network|type|host|since|until):\S*$/i, '').trim(),
)
watch([typed, hostInput], ([q, host]) => {
  clearTimeout(timer)
  timer = setTimeout(() => {
    debounced.value = q
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

/** “#general” or “3 channels”, for the empty state. */
const pickerLabel = computed(() =>
  channelChoiceLabel(
    picked.value.map((id) => conversation(id)?.name ?? ''),
    translate,
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

// Retained sources stay gated until a request started after their update succeeds.
watch(
  [linksRevision, filter],
  async ([revision]) => {
    if (!linksClassificationOverrides.size) return
    try {
      await first.refetch()
      if (!first.error.value && first.data.value)
        updateLinkClassifications(first.data.value, revision)
    } catch {
      // Keep unknown sources gated until a later successful refresh.
    }
  },
  { immediate: true },
)

/** Pages after the first, loaded on demand; dropped whenever the listing may have changed. */
const { items, cursor, loadMore, loadingMore, moreError, reset } = useCursorPages<LinkCard>({
  firstPage: first.data,
  resetOn: [filter, linksRevision],
  itemKey: (card) => card.identity,
  load: async (cursor, signal) => {
    const revision = linksRevision.value
    const page = await apiClient().links(linkQueryParams(filter.value, cursor), signal)
    updateLinkClassifications(page, revision)
    return page
  },
})
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
  reset()
  void first.refetch()
}

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
    typed.value !== debounced.value ||
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

/**
 * The filters as chips in the search box, Search-style: `in:` channels, `network:`, `type:`,
 * `host:`, and `since:`/`until:` days (both included). Each chip maps onto the filter refs.
 */
const field = ref<{ focus: () => void; start: (key: string) => void } | null>(null)
const linkKeys = computed<TagKey[]>(() =>
  [
    { key: 'in', icon: 'hash', show: channels.value.length > 1 },
    { key: 'network', icon: 'globe', show: networks.value.length > 1 && !picked.value.length },
    { key: 'type', icon: 'attachment', show: true },
    { key: 'host', icon: 'link', show: true },
    { key: 'since', icon: 'calendar', show: true, date: true },
    { key: 'until', icon: 'calendar', show: true, date: true },
  ]
    .filter((k) => k.show)
    .map((k) => ({
      key: k.key,
      icon: k.icon,
      label: t(`collectionsUi.links.tags.${k.key}`),
      date: k.date,
    })),
)
const linkChips = computed<TagChip[]>(() => [
  ...picked.value.map((value) => ({ key: 'in', value })),
  ...(network.value && !picked.value.length ? [{ key: 'network', value: network.value }] : []),
  ...kinds.value.map((value) => ({ key: 'type', value })),
  ...(hostInput.value ? [{ key: 'host', value: hostInput.value }] : []),
  ...(from.value ? [{ key: 'since', value: from.value }] : []),
  ...(to.value ? [{ key: 'until', value: to.value }] : []),
])
const kindLabel = (kind: string) => t(`collectionsUi.links.${kindLabels[kind as LinkKind]}`)
const { day } = useTimeFormat()
function linkChipLabel(chip: TagChip) {
  switch (chip.key) {
    case 'in':
      return `#${conversation(chip.value)?.name ?? ''}`
    case 'network':
      return networks.value.find((n) => n.id === chip.value)?.name ?? chip.value
    case 'type':
      return kindLabel(chip.value)
    case 'since':
    case 'until':
      return day(new Date(`${chip.value}T12:00:00`).toISOString())
    default:
      return chip.value
  }
}
function linkSuggest(key: string, q: string): TagSuggestion[] {
  const matches = (label: string) => !q || label.toLowerCase().includes(q)
  switch (key) {
    case 'in':
      return channels.value
        .filter((c) => !picked.value.includes(c.id) && channelSelectable(picked.value, c.id))
        .filter((c) => matches(c.name))
        .slice(0, 8)
        .map((c) => ({
          id: `in-${c.id}`,
          label: `#${c.name}`,
          detail: networks.value.find((n) => n.id === c.networkId)?.name,
          icon: 'hash',
          value: c.id,
        }))
    case 'network':
      return networks.value
        .filter((n) => n.id !== network.value && matches(n.name))
        .map((n) => ({ id: `network-${n.id}`, label: n.name, icon: 'globe', value: n.id }))
    case 'type':
      return linkKinds
        .filter((k) => !kinds.value.includes(k.value) && matches(kindLabel(k.value)))
        .map((k) => ({
          id: `type-${k.value}`,
          label: kindLabel(k.value),
          icon: k.icon,
          value: k.value,
        }))
    case 'host': {
      // Sites already listed, then exactly what was typed.
      const hosts = [...new Set(items.value.map((card) => card.host))].filter(
        (host) => host && matches(host),
      )
      return [...(q && !hosts.includes(q) ? [q] : []), ...hosts]
        .slice(0, 8)
        .map((host) => ({ id: `host-${host}`, label: host, icon: 'link', value: host }))
    }
    default:
      return []
  }
}
function addChip(chip: TagChip) {
  if (chip.key === 'in') {
    if (channelSelectable(picked.value, chip.value)) chosen.value = [...picked.value, chip.value]
  } else if (chip.key === 'network') network.value = chip.value
  else if (chip.key === 'type') kinds.value = [...kinds.value, chip.value as LinkKind]
  else if (chip.key === 'host') hostInput.value = chip.value
  else if (chip.key === 'since') from.value = chip.value
  else if (chip.key === 'until') to.value = chip.value
}
function removeChip(chip: TagChip) {
  if (chip.key === 'in') chosen.value = picked.value.filter((id) => id !== chip.value)
  else if (chip.key === 'network') network.value = ''
  else if (chip.key === 'type') kinds.value = kinds.value.filter((k) => k !== chip.value)
  else if (chip.key === 'host') hostInput.value = ''
  else if (chip.key === 'since') from.value = ''
  else if (chip.key === 'until') to.value = ''
}

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
        <TagQueryField
          ref="field"
          v-model:text="query"
          class="basis-full"
          :keys="linkKeys"
          :chips="linkChips"
          :suggest="linkSuggest"
          :chip-label="linkChipLabel"
          :menu="false"
          :placeholder="t('collectionsUi.links.filter')"
          :label="t('collectionsUi.links.filterLabel')"
          @add="addChip"
          @remove="removeChip"
          @clear="((chosen = []), clearFilters())"
          @keydown="keys"
        />
        <div
          class="links-quick-filters flex basis-full flex-wrap gap-1.5"
          role="group"
          :aria-label="t('collectionsUi.links.filters')"
        >
          <button
            v-for="k in linkKeys"
            :key="k.key"
            type="button"
            class="inline-flex h-7 items-center gap-1.5 rounded-full bg-secondary/45 px-3 text-[length:var(--ui-caption)] font-[550] text-muted-foreground shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)] transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring [@media(pointer:coarse)]:h-9"
            @click="field?.start(k.key)"
          >
            <AppIcon
              :name="k.icon"
              :size="13"
            />{{ k.label }}
          </button>
        </div>
        <!-- Computed while typing; announced once settled. -->
        <p
          v-if="problem"
          class="m-0 basis-full text-[length:var(--ui-caption)] text-destructive"
        >
          {{ problem }}
        </p>
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
          class="links-list m-0 list-none gap-4 p-0 [columns:calc(280px*var(--interface-scale,1))] [&>li]:mb-4 [&>li]:break-inside-avoid"
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
