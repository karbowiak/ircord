<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import type { LinkCard, LinkKind, LinkSource } from '../../shared/types/chat'
import {
  channelChoiceLabel,
  channelSelectable,
  linkDateRange,
  linkHostFilter,
  linkKinds,
  linkRangeOpen,
  linkTitle,
  linkToday,
  MAX_LINK_CHANNELS,
  openableLink,
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
const { full, day, time } = useTimeFormat()
const when = (iso: string) => `${day(iso)} ${time(iso)}`

const channel = computed(() =>
  chat.data?.conversations.find((c) => c.id === props.conversationId && c.kind === 'channel'),
)
const channels = computed(() => chat.data?.conversations.filter((c) => c.kind === 'channel') ?? [])
/** Chosen channels; none lists every channel. Channels closed meanwhile drop out. */
const chosen = ref<string[]>(channel.value ? [channel.value.id] : [])
const picked = computed(() => chosen.value.filter((id) => channels.value.some((c) => c.id === id)))
const scope = computed<'channel' | 'all' | 'some'>({
  get: () =>
    !picked.value.length
      ? 'all'
      : picked.value.length === 1 && picked.value[0] === channel.value?.id
        ? 'channel'
        : 'some',
  set: (value) => {
    chosen.value = value === 'channel' && channel.value ? [channel.value.id] : []
  },
})
const network = ref('')
const kind = ref<LinkKind | ''>('')
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
  kind: kind.value,
  q: debounced.value,
  host: 'host' in hostFilter.value ? hostFilter.value.host : '',
  since: 'error' in range.value ? undefined : range.value.since,
  until: 'error' in range.value ? undefined : range.value.until,
}))

/** The channel picker: every open channel by network, with a search once there are many. */
const picking = ref(false)
const pickerId = useId()
const channelQuery = ref('')
const pickerButton = ref<HTMLButtonElement | null>(null)
const pickerLabel = computed(() =>
  channelChoiceLabel(picked.value.map((id) => conversation(id)?.name ?? '')),
)
const channelGroups = computed(() => {
  const q = channelQuery.value.trim().toLowerCase().replace(/^#/, '')
  return networks.value
    .map((n) => ({
      network: n,
      channels: channels.value.filter(
        (c) => c.networkId === n.id && (!q || c.name.toLowerCase().includes(q)),
      ),
    }))
    .filter((g) => g.channels.length)
})
const picker = ref<HTMLElement | null>(null)
/** Opens the picker and moves focus into it, so its boxes are next for the keyboard. */
function openPicker() {
  picking.value = true
  nextTick(() =>
    picker.value?.querySelector<HTMLElement>('input, [role="checkbox"]:not([disabled])')?.focus(),
  )
}
function closePicker() {
  picking.value = false
  channelQuery.value = ''
  nextTick(() => pickerButton.value?.focus())
}
function clearFilters() {
  query.value = ''
  debounced.value = ''
  hostInput.value = ''
  debouncedHost.value = ''
  from.value = ''
  to.value = ''
  kind.value = ''
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
      kind.value ||
      filter.value.network ||
      filter.value.host ||
      filter.value.since ||
      filter.value.until
    ),
)

const conversation = (id: string) => chat.data?.conversations.find((c) => c.id === id)
const networkName = (id: string) => networks.value.find((n) => n.id === id)?.name ?? 'Network'
const place = (s: LinkSource) => {
  const c = conversation(s.conversationId)
  return c ? `#${c.name}` : 'a closed channel'
}
const author = (s: LinkSource) =>
  chat.member(s.senderId)?.displayName || chat.member(s.senderId)?.nick || 'Someone'
const kindIcon = (k: LinkKind) => linkKinds.find((x) => x.value === k)?.icon ?? 'link'
const expanded = reactive(new Set<string>())
function toggleSources(card: LinkCard) {
  if (expanded.has(card.identity)) expanded.delete(card.identity)
  else expanded.add(card.identity)
}

async function copy(url: string) {
  try {
    await navigator.clipboard.writeText(url)
    chat.notify('Link copied.')
  } catch {
    chat.notify('Couldn’t copy the link. Your browser blocked clipboard access.')
  }
}
function jump(source: LinkSource) {
  if (!conversation(source.conversationId))
    return chat.notify('That channel isn’t open any more. Rejoin it to see this message.')
  emit('open', source)
}

const kindOptions = computed(() => [
  { value: '' as const, label: 'All types' },
  ...linkKinds.map((k) => ({ value: k.value, label: k.label })),
])
const networkOptions = computed(() => [
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
  <AppDialog
    title="Links"
    wide
    @close="emit('close')"
    ><SearchField
      v-model="query"
      size="lg"
      hint="esc"
      class="search-field mx-5"
      autofocus
      placeholder="Filter by address or message"
      aria-label="Filter links"
      @keydown="keys"
    />
    <div class="links-filters flex flex-wrap gap-2 items-center mt-2.5 mr-5 mb-0 ml-5">
      <AppSegmented
        v-if="channel"
        v-model="scope"
        label="Which channels"
        :options="[
          { value: 'channel', label: `#${channel.name}` },
          { value: 'all', label: 'All channels' },
        ]"
      />
      <button
        v-if="channels.length > 1"
        ref="pickerButton"
        type="button"
        class="app-select inline-flex items-center justify-between gap-2 w-full min-h-9 pt-1.5 pr-2 pb-1.5 pl-2.5 bg-field border border-solid border-border rounded-md text-[13px] leading-[1.5] text-foreground text-left links-channels-button"
        :class="{ open: picking }"
        :aria-expanded="picking"
        :aria-controls="pickerId"
        :aria-label="`Choose channels: ${pickerLabel}`"
        @click="picking ? closePicker() : openPicker()"
      >
        <span
          class="app-select-value flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap"
          >{{ pickerLabel }}</span
        >
        <AppIcon
          name="down"
          :size="15"
          class="app-select-chevron shrink-0 text-muted-foreground [transition:transform_0.15s]"
        />
      </button>
      <AppSelect
        v-if="!picked.length && networks.length > 1"
        v-model="network"
        aria-label="Network"
        :options="networkOptions"
      />
      <AppSelect
        v-model="kind"
        aria-label="Link type"
        :options="kindOptions"
      />
      <UiInput
        v-model="hostInput"
        class="links-host"
        type="text"
        inputmode="url"
        autocomplete="off"
        spellcheck="false"
        placeholder="Host, such as example.com"
        aria-label="Host"
        :aria-invalid="'error' in hostFilter || undefined"
        @keydown="keys"
      />
      <div
        class="links-dates flex flex-wrap gap-2"
        role="group"
        aria-label="Posted between"
      >
        <label class="links-date inline-flex items-center gap-1.5 text-[13px] text-muted-foreground"
          ><span>From</span
          ><UiInput
            v-model="from"
            type="date"
            :max="to || today"
            @focus="refreshToday"
            aria-label="Posted from"
            :aria-invalid="'error' in range || undefined"
        /></label>
        <label class="links-date inline-flex items-center gap-1.5 text-[13px] text-muted-foreground"
          ><span>To</span
          ><UiInput
            v-model="to"
            type="date"
            :min="from || undefined"
            :max="today"
            @focus="refreshToday"
            aria-label="Posted until"
            :aria-invalid="'error' in range || undefined"
        /></label>
      </div>
    </div>
    <div
      v-if="picking"
      :id="pickerId"
      ref="picker"
      class="links-channel-picker grid gap-2 mt-2.5 mr-5 mb-0 ml-5 p-2.5 max-h-[min(260px,_40dvh)] overflow-y-auto bg-field border border-solid border-border rounded-md"
      role="group"
      aria-label="Channels to list"
      @keydown.esc.prevent.stop="closePicker"
    >
      <UiInput
        v-if="channels.length > 8"
        v-model="channelQuery"
        type="search"
        placeholder="Find a channel"
        aria-label="Find a channel"
      />
      <fieldset
        v-for="group in channelGroups"
        :key="group.network.id"
      >
        <legend v-if="networks.length > 1">{{ group.network.name }}</legend>
        <label
          v-for="c in group.channels"
          :key="c.id"
          class="links-channel-option inline-flex items-center gap-1.5 min-h-8 text-[13px] wrap-anywhere"
          ><UiCheckbox
            :model-value="chosen.includes(c.id)"
            :disabled="!channelSelectable(picked, c.id)"
            @update:model-value="
              chosen = $event ? [...chosen, c.id] : chosen.filter((id) => id !== c.id)
            "
          /><span>#{{ c.name }}</span></label
        >
      </fieldset>
      <p
        v-if="!channelGroups.length"
        class="links-channel-none m-0 text-[13px] text-muted-foreground"
      >
        No channel matches.
      </p>
      <p
        v-if="picked.length >= MAX_LINK_CHANNELS"
        class="links-channel-none m-0 text-[13px] text-muted-foreground"
      >
        Up to {{ MAX_LINK_CHANNELS }} channels can be chosen at once.
      </p>
      <div class="links-channel-actions flex justify-end gap-3">
        <UiButton
          variant="link"
          size="text"
          type="button"
          :disabled="!picked.length"
          @click="chosen = []"
        >
          Any channel
        </UiButton>
        <UiButton
          variant="link"
          size="text"
          type="button"
          @click="closePicker"
        >
          Done
        </UiButton>
      </div>
    </div>
    <p
      v-if="offline"
      class="saved-offline mt-2.5 mr-5 mb-0 ml-5 text-[13px] text-muted-foreground"
      role="status"
    >
      You’re offline. Links may be out of date until IRCord reconnects.
    </p>
    <p
      v-if="first.data.value?.indexing"
      class="saved-offline mt-2.5 mr-5 mb-0 ml-5 text-[13px] text-muted-foreground"
      role="status"
    >
      Older history is still being indexed, so some older links may be missing.
    </p>
    <UiButton
      variant="link"
      size="text"
      v-else-if="newPosted && !loading"
      class="links-new block mt-2.5 mr-5 mb-0 ml-5"
      @click="showNew"
    >
      New links were posted. Show them
    </UiButton>
    <div
      ref="list"
      class="search-results pt-2 pr-3 pb-3 pl-3 min-h-70 max-h-[min(460px,_calc(100dvh_-_260px))] overflow-y-auto saved-results"
    >
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
      <EmptyState
        as="p"
        v-else-if="loading"
      >
        Loading links…
      </EmptyState>
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
        class="links-list list-none m-0 p-0"
        aria-label="Links"
      >
        <li
          v-for="(card, index) in items"
          :key="card.identity"
          class="catalog-card p-2 rounded-sm"
          :class="{ 'is-selected': index === selected }"
          @focusin="selected = index"
        >
          <div class="catalog-card-main flex items-start gap-3 min-w-0">
            <span
              class="catalog-card-thumb flex-none grid [place-items:center] w-16 h-16 overflow-hidden rounded-sm bg-accent text-muted-foreground"
              aria-hidden="true"
              ><AppIcon
                :name="kindIcon(card.kind)"
                :size="22"
            /></span>
            <div class="catalog-card-text flex-1 min-w-0 flex flex-col gap-0.5">
              <a
                v-if="openableLink(card.url)"
                class="catalog-card-title text-[14px] font-[600] text-foreground wrap-anywhere [display:-webkit-box] [-webkit-line-clamp:2] [-webkit-box-orient:vertical] overflow-hidden"
                :href="card.url"
                target="_blank"
                rel="noopener noreferrer nofollow"
                >{{ linkTitle(card) }}</a
              ><span
                v-else
                class="catalog-card-title text-[14px] font-[600] text-foreground wrap-anywhere [display:-webkit-box] [-webkit-line-clamp:2] [-webkit-box-orient:vertical] overflow-hidden"
                >{{ linkTitle(card) }}</span
              >
              <span class="catalog-card-host text-[12px] text-muted-foreground">{{
                card.host
              }}</span>
              <span class="catalog-card-source text-[12px] text-muted-foreground"
                ><strong>{{ author(card.sources[0]!) }}</strong> in {{ place(card.sources[0]!) }} ·
                {{ networkName(card.sources[0]!.networkId) }} ·
                <time
                  :datetime="card.lastPostedAt"
                  :title="full(card.lastPostedAt)"
                  >{{ when(card.lastPostedAt) }}</time
                ></span
              >
              <span
                v-if="card.sources[0]?.excerpt"
                class="catalog-card-excerpt text-muted-foreground"
                >{{ card.sources[0].excerpt }}</span
              >
            </div>
            <div class="catalog-card-actions flex-none flex gap-0.5">
              <UiButton
                variant="ghost"
                size="icon-sm"
                :aria-label="`Jump to the message: ${linkTitle(card)}`"
                title="Jump to message"
                @click="jump(card.sources[0]!)"
              >
                <AppIcon
                  name="reply"
                  :size="17"
                />
              </UiButton>
              <UiButton
                variant="ghost"
                size="icon-sm"
                :aria-label="`Copy link: ${card.url}`"
                title="Copy link"
                @click="copy(card.url)"
              >
                <AppIcon
                  name="copy"
                  :size="17"
                />
              </UiButton>
              <UiButton
                as="a"
                variant="ghost"
                size="icon-sm"
                v-if="openableLink(card.url)"
                :href="card.url"
                target="_blank"
                rel="noopener noreferrer nofollow"
                :aria-label="`Open link: ${card.url}`"
                title="Open link"
                ><AppIcon
                  name="external"
                  :size="17"
              /></UiButton>
            </div>
          </div>
          <UiButton
            variant="link"
            size="text"
            v-if="card.count > 1"
            class="catalog-card-count mt-1 mr-0 mb-0 ml-19 text-[12px]"
            :aria-expanded="expanded.has(card.identity)"
            @click="toggleSources(card)"
          >
            Posted {{ card.count }} times{{ expanded.has(card.identity) ? '' : ' · show posts' }}
          </UiButton>
          <ul
            v-if="expanded.has(card.identity)"
            class="catalog-card-sources list-none mt-1 mr-0 mb-0 ml-0 pt-0 pr-0 pb-0 pl-17"
            :aria-label="`Posts of ${linkTitle(card)}`"
          >
            <li
              v-for="source in card.sources"
              :key="source.messageId"
            >
              <button
                class="search-message flex flex-col gap-0.5 w-full p-2 rounded-sm text-left"
                @click="jump(source)"
              >
                <span
                  class="search-message-location flex gap-2 text-[12px] text-muted-foreground font-[600]"
                  >{{ networkName(source.networkId) }} · {{ place(source)
                  }}<time
                    :datetime="source.postedAt"
                    :title="full(source.postedAt)"
                    >{{ full(source.postedAt) }}</time
                  ></span
                ><span
                  ><strong>{{ author(source) }}</strong> {{ source.excerpt }}</span
                >
              </button>
            </li>
            <li
              v-if="card.count > card.sources.length"
              class="catalog-card-more py-1 px-2 text-[12px] text-faint"
            >
              and {{ card.count - card.sources.length }} earlier
              {{ card.count - card.sources.length === 1 ? 'post' : 'posts' }}
            </li>
          </ul>
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
        v-if="!problem && !loading && cursor && !(first.error.value && !first.data.value)"
        class="saved-more block mt-2 mr-auto mb-0 ml-auto"
        :disabled="loadingMore"
        @click="loadMore"
      >
        {{ loadingMore ? 'Loading…' : moreError ? 'Try again' : 'Load older links' }}
      </UiButton>
    </div>
    <footer
      class="search-footer flex gap-3.5 py-2.5 px-5 border-t [border-top-style:solid] border-t-border bg-sidebar text-[12px] text-faint"
    >
      <span><kbd>↑</kbd> <kbd>↓</kbd> to choose</span><span><kbd>↵</kbd> to jump to the post</span
      ><span><kbd>esc</kbd> to close</span>
    </footer></AppDialog
  >
</template>
