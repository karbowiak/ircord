<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import type { LinkCard, LinkKind, LinkSource } from '../../shared/types/chat'
import { linkKinds, linkTitle, openableLink } from '../../shared/utils/link-catalog'
import {
  linkQueryParams,
  linksListed,
  linksQuery,
  linksRevision,
  postedIn,
  type LinkFilter,
} from '../queries/links'

/**
 * Links posted in your channels, grouped by what they point to and newest first: either the
 * channel you opened it from or every channel. Built from your own history on the server, so
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
const scope = ref<'channel' | 'all'>(channel.value ? 'channel' : 'all')
const network = ref('')
const kind = ref<LinkKind | ''>('')
const query = ref('')
const debounced = ref('')
let timer: ReturnType<typeof setTimeout> | undefined
watch(query, (q) => {
  clearTimeout(timer)
  timer = setTimeout(() => (debounced.value = q.trim()), 220)
})
onBeforeUnmount(() => clearTimeout(timer))

const networks = computed(() => chat.data?.networks ?? [])
const filter = computed<LinkFilter>(() => ({
  conversations: scope.value === 'channel' && channel.value ? [channel.value.id] : [],
  network: scope.value === 'all' ? network.value : '',
  kind: kind.value,
  q: debounced.value,
}))
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
const newPosted = computed(() => postedIn(filter.value, networkOf) > postedAtLoad.value)
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
  () => query.value.trim() !== debounced.value || (first.isLoading.value && !first.data.value),
)
const offline = computed(() => chat.status === 'offline')
const filtered = computed(() => !!(debounced.value || kind.value || filter.value.network))

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
    ><div class="search-field">
      <AppIcon
        name="search"
        :size="21"
      /><input
        v-model="query"
        autofocus
        placeholder="Filter by address or message"
        aria-label="Filter links"
        @keydown="keys"
      /><kbd>esc</kbd>
    </div>
    <div class="links-filters">
      <AppSegmented
        v-if="channel"
        v-model="scope"
        label="Which channels"
        :options="[
          { value: 'channel', label: `#${channel.name}` },
          { value: 'all', label: 'All channels' },
        ]"
      />
      <AppSelect
        v-if="scope === 'all' && networks.length > 1"
        v-model="network"
        aria-label="Network"
        :options="networkOptions"
      />
      <AppSelect
        v-model="kind"
        aria-label="Link type"
        :options="kindOptions"
      />
    </div>
    <p
      v-if="offline"
      class="saved-offline"
      role="status"
    >
      You’re offline. Links may be out of date until IRCord reconnects.
    </p>
    <p
      v-if="first.data.value?.indexing"
      class="saved-offline"
      role="status"
    >
      Older history is still being indexed, so some older links may be missing.
    </p>
    <button
      v-else-if="newPosted && !loading"
      class="text-button links-new"
      @click="showNew"
    >
      New links were posted. Show them
    </button>
    <div
      ref="list"
      class="search-results saved-results"
    >
      <p
        v-if="loading"
        class="search-empty"
      >
        Loading links…
      </p>
      <div
        v-else-if="first.error.value && !first.data.value"
        class="search-empty"
        role="alert"
      >
        <AppIcon
          name="alert"
          :size="27"
        />
        <p>Couldn’t load links: {{ errorText(first.error.value) }}</p>
        <button
          class="text-button"
          @click="first.refetch()"
        >
          Try again
        </button>
      </div>
      <div
        v-else-if="!items.length"
        class="search-empty"
      >
        <AppIcon
          name="link"
          :size="28"
        />
        <p v-if="filtered">No links match these filters.</p>
        <p v-else-if="filter.conversations.length">
          No links have been posted in #{{ channel?.name }} yet.
        </p>
        <p v-else>No links have been posted in your channels yet.</p>
      </div>
      <ul
        v-else
        class="links-list"
        aria-label="Links"
      >
        <li
          v-for="(card, index) in items"
          :key="card.identity"
          class="catalog-card"
          :class="{ 'is-selected': index === selected }"
          @focusin="selected = index"
        >
          <div class="catalog-card-main">
            <span
              class="catalog-card-thumb"
              aria-hidden="true"
              ><AppIcon
                :name="kindIcon(card.kind)"
                :size="22"
            /></span>
            <div class="catalog-card-text">
              <a
                v-if="openableLink(card.url)"
                class="catalog-card-title"
                :href="card.url"
                target="_blank"
                rel="noopener noreferrer nofollow"
                >{{ linkTitle(card) }}</a
              ><span
                v-else
                class="catalog-card-title"
                >{{ linkTitle(card) }}</span
              >
              <span class="catalog-card-host">{{ card.host }}</span>
              <span class="catalog-card-source"
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
                class="catalog-card-excerpt"
                >{{ card.sources[0].excerpt }}</span
              >
            </div>
            <div class="catalog-card-actions">
              <button
                class="icon-button"
                :aria-label="`Jump to the message: ${linkTitle(card)}`"
                title="Jump to message"
                @click="jump(card.sources[0]!)"
              >
                <AppIcon
                  name="reply"
                  :size="17"
                />
              </button>
              <button
                class="icon-button"
                :aria-label="`Copy link: ${card.url}`"
                title="Copy link"
                @click="copy(card.url)"
              >
                <AppIcon
                  name="copy"
                  :size="17"
                />
              </button>
              <a
                v-if="openableLink(card.url)"
                class="icon-button"
                :href="card.url"
                target="_blank"
                rel="noopener noreferrer nofollow"
                :aria-label="`Open link: ${card.url}`"
                title="Open link"
                ><AppIcon
                  name="external"
                  :size="17"
              /></a>
            </div>
          </div>
          <button
            v-if="card.count > 1"
            class="text-button catalog-card-count"
            :aria-expanded="expanded.has(card.identity)"
            @click="toggleSources(card)"
          >
            Posted {{ card.count }} times{{ expanded.has(card.identity) ? '' : ' · show posts' }}
          </button>
          <ul
            v-if="expanded.has(card.identity)"
            class="catalog-card-sources"
            :aria-label="`Posts of ${linkTitle(card)}`"
          >
            <li
              v-for="source in card.sources"
              :key="source.messageId"
            >
              <button
                class="search-message"
                @click="jump(source)"
              >
                <span class="search-message-location"
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
              class="catalog-card-more"
            >
              and {{ card.count - card.sources.length }} earlier
              {{ card.count - card.sources.length === 1 ? 'post' : 'posts' }}
            </li>
          </ul>
        </li>
      </ul>
      <p
        v-if="moreError"
        class="form-error"
        role="alert"
      >
        Couldn’t load more: {{ moreError }}
      </p>
      <button
        v-if="!loading && cursor && !(first.error.value && !first.data.value)"
        class="text-button saved-more"
        :disabled="loadingMore"
        @click="loadMore"
      >
        {{ loadingMore ? 'Loading…' : moreError ? 'Try again' : 'Load older links' }}
      </button>
    </div>
    <footer class="search-footer">
      <span><kbd>↑</kbd> <kbd>↓</kbd> to choose</span><span><kbd>↵</kbd> to jump to the post</span
      ><span><kbd>esc</kbd> to close</span>
    </footer></AppDialog
  >
</template>
