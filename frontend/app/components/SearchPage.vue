<script setup lang="ts">
import type { Conversation } from '../../shared/types/chat'
import {
  activeToken,
  canSearch,
  emptyFilters,
  filterCount,
  rememberSearch,
  sanitizeRecent,
  type RecentSearch,
  type SearchFilters,
} from '../../shared/utils/search-filter'
const emit = defineEmits<{ close: []; jump: [conversationId: string, messageId: string] }>()
const chat = useChatStore()
const auth = useAuthStore()
const navigation = useConversationNavigation()
const query = ref('')
const debounced = ref('')
const filters = ref<SearchFilters>(emptyFilters())
const filtered = computed(() => filterCount(filters.value) > 0)
const tab = ref<'conversations' | 'messages'>('conversations')
const error = ref('')
let timer: ReturnType<typeof setTimeout> | undefined
// A filter token being typed (from:jul) isn't search text yet.
watch(query, (q) => {
  clearTimeout(timer)
  timer = setTimeout(() => {
    const token = activeToken(q)
    debounced.value = (token ? q.slice(0, token.start) : q).trim()
  }, 220)
})
// Filters only search messages.
watch(filtered, (on) => on && (tab.value = 'messages'))
const { search, results, cursor, loadMore, loadingMore, moreError } = useMessageSearch(
  debounced,
  filters,
)
const showFilters = ref(false)

// Recent searches stay on this device, per account.
const recentKey = `ircord.${auth.account?.id ?? 'signed-out'}.search-recent`
const recentSearches = ref<RecentSearch[]>(readLocal(recentKey, sanitizeRecent))
function remember() {
  if (!canSearch(debounced.value, filters.value)) return
  recentSearches.value = rememberSearch(recentSearches.value, {
    text: debounced.value,
    filters: structuredClone(toRaw(filters.value)),
  })
  writeLocal(recentKey, recentSearches.value)
}
function forget() {
  recentSearches.value = []
  writeLocal(recentKey, [])
}
function recall(entry: RecentSearch) {
  query.value = debounced.value = entry.text
  filters.value = structuredClone(entry.filters)
  tab.value = 'messages'
}
function jumpTo(conversationId: string, messageId: string) {
  remember()
  emit('jump', conversationId, messageId)
}
/** What has been typed, without a filter token still being written. */
const typed = computed(() => {
  const token = activeToken(query.value)
  return (token ? query.value.slice(0, token.start) : query.value).trim()
})
const searching = computed(
  () => typed.value !== debounced.value || (search.isLoading.value && !search.data.value),
)
const needle = computed(() => typed.value.toLowerCase().replace(/^[#@]/, ''))
const { recent } = useRecentConversations()
const label = (c: Conversation) =>
  c.kind === 'direct' ? (chat.directPartner(c)?.displayName ?? c.name) : c.name

/**
 * Without a query: what needs attention (mentions, then unread DMs, then unread channels), then
 * recently visited. With a query: exact, prefix, then substring matches, ties broken the same way.
 */
const conversations = computed(() => {
  const list = chat.data?.conversations ?? []
  const urgency = (c: Conversation) =>
    c.mentionCount ? 0 : c.kind === 'direct' && c.unreadCount ? 1 : c.unreadCount ? 2 : 3
  const recency = (c: Conversation) => {
    const index = recent.value.indexOf(c.id)
    return index < 0 ? Infinity : index
  }
  const match = (c: Conversation) => {
    const name = label(c).toLowerCase()
    const nick = c.kind === 'direct' ? (chat.directPartner(c)?.nick.toLowerCase() ?? '') : ''
    if (!needle.value) return 0
    if (name === needle.value || nick === needle.value) return 0
    if (name.startsWith(needle.value) || nick.startsWith(needle.value)) return 1
    if (name.includes(needle.value) || nick.includes(needle.value)) return 2
    return -1
  }
  return list
    .map((c) => ({ c, score: match(c) }))
    .filter((x) => x.score >= 0)
    .sort(
      (a, b) =>
        a.score - b.score ||
        urgency(a.c) - urgency(b.c) ||
        recency(a.c) - recency(b.c) ||
        label(a.c).localeCompare(label(b.c)),
    )
    .slice(0, 12)
    .map((x) => x.c)
})
const servers = computed(() =>
  (chat.data?.networks ?? [])
    .filter((n) => !needle.value || `server ${n.name}`.toLowerCase().includes(needle.value))
    .slice(0, needle.value ? 4 : 0),
)
const people = computed(() =>
  needle.value
    ? (chat.data?.members
        .filter(
          (m) =>
            m.id !== 'you' && `${m.displayName} ${m.nick}`.toLowerCase().includes(needle.value),
        )
        .slice(0, 6) ?? [])
    : [],
)

/** Everything selectable in the current tab, in display order, for keyboard navigation. */
type Item = { key: string; run: () => void }
const items = computed<Item[]>(() =>
  tab.value === 'conversations'
    ? [
        ...conversations.value.map((c) => ({ key: c.id, run: () => open(c.id) })),
        ...servers.value.map((n) => ({ key: `server:${n.id}`, run: () => open(`server:${n.id}`) })),
        ...people.value.map((m) => ({ key: `person:${m.id}`, run: () => void direct(m.id) })),
      ]
    : results.value.map((r) => ({
        key: r.message.id,
        run: () => jumpTo(r.conversation.id, r.message.id),
      })),
)
const selected = ref(0)
watch([items, tab], () => (selected.value = 0))
const isSelected = (key: string) => items.value[selected.value]?.key === key
const resultsBox = ref<HTMLElement | null>(null)
function keys(event: KeyboardEvent) {
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    const count = items.value.length
    if (!count) return
    selected.value = (selected.value + (event.key === 'ArrowDown' ? 1 : -1) + count) % count
    nextTick(() =>
      resultsBox.value?.querySelector('.is-selected')?.scrollIntoView({ block: 'nearest' }),
    )
  } else if (event.key === 'Enter') {
    event.preventDefault()
    items.value[selected.value]?.run()
  } else if (event.key === 'Tab' && !event.shiftKey && (query.value || filtered.value)) {
    event.preventDefault()
    tab.value = tab.value === 'conversations' ? 'messages' : 'conversations'
  }
}
onBeforeUnmount(() => clearTimeout(timer))
// “Search everywhere” from a conversation's search arrives as ?q= and opens on messages.
const route = useRoute()
watch(
  () => route.query.q,
  (q) => {
    if (typeof q !== 'string' || !q.trim()) return
    query.value = q
    tab.value = 'messages'
  },
  { immediate: true },
)
/** Group headings, styled like the sidebar's section headings. */
const listLabel =
  'list-label m-0 flex h-7 items-center px-3 text-[length:var(--ui-caption)] font-semibold uppercase tracking-[0.02em] text-muted-foreground'
function open(id: string) {
  void navigation.open(id)
}
async function direct(id: string) {
  try {
    await chat.direct.mutateAsync(id)
    emit('close')
  } catch (e) {
    error.value = errorText(e)
  }
}
</script>
<template>
  <WorkspacePage
    title="Search"
    icon="search"
    description="Find a conversation, someone to talk to, or a message worth revisiting."
    @close="emit('close')"
  >
    <UiTabs
      v-model="tab"
      class="contents"
    >
      <WorkspaceToolbar class="flex-row flex-wrap items-center">
        <SearchQueryField
          v-model:text="query"
          v-model:filters="filters"
          :recent="recentSearches"
          placeholder="Channels, people or messages — try from: or has:"
          @keydown="keys"
          @more="showFilters = true"
          @recall="recall"
          @forget="forget"
        />
        <UiButton
          variant="secondary"
          class="shrink-0"
          :aria-label="filtered ? `Filters, ${filterCount(filters)} on` : 'Filters'"
          @click="showFilters = true"
          ><AppIcon
            name="filters"
            :size="16"
          />Filters<span
            v-if="filtered"
            class="rounded-full bg-selection px-1.5 text-[length:var(--ui-label)] text-primary"
            >{{ filterCount(filters) }}</span
          ></UiButton
        >
        <UiTabsList
          class="search-tabs min-h-9 w-fit max-w-full gap-1 overflow-x-auto rounded-lg bg-sidebar p-0.5 [scrollbar-width:none]"
          aria-label="Search type"
        >
          <UiTabsTrigger
            variant="pill"
            value="conversations"
          >
            Conversations <span v-if="query">{{ conversations.length }}</span></UiTabsTrigger
          ><UiTabsTrigger
            variant="pill"
            value="messages"
          >
            Messages <span v-if="canSearch(debounced, filters)">{{ results.length }}</span>
          </UiTabsTrigger>
        </UiTabsList>
      </WorkspaceToolbar>
      <!-- One panel for both tabs, so the results box and its keyboard selection stay put. -->
      <OfflineArchiveNotice
        v-if="tab === 'messages'"
        :offline="search.data.value?.offline"
      />
      <UiTabsContent
        :value="tab"
        as-child
      >
        <div
          ref="resultsBox"
          class="search-results min-h-0 flex-1 overflow-y-auto px-4 pb-4 max-[700px]:px-1.5"
        >
          <div class="w-full space-y-0.5">
            <template v-if="tab === 'conversations'"
              ><p :class="listLabel">
                {{ query ? 'Conversations' : 'Jump to' }}
              </p>
              <ConversationResult
                v-for="c in conversations"
                :key="c.id"
                :class="{ 'is-selected': isSelected(c.id) }"
                :icon="c.kind === 'channel' ? 'hash' : 'message'"
                :title="label(c)"
                :detail="`${chat.data?.networks.find((n) => n.id === c.networkId)?.name} · ${c.kind === 'channel' ? 'Channel' : 'Direct message'}`"
                @click="open(c.id)"
              >
                <template
                  v-if="c.kind === 'direct'"
                  #icon
                  ><UserAvatar :member="chat.directPartner(c)"
                /></template>
                <template #badge>
                  <span
                    v-if="c.mentionCount"
                    class="mention-badge bg-destructive text-rail text-[11px] min-w-4.5 h-4.5 py-0 px-[5px] flex items-center justify-center rounded-[9px] font-[700]"
                    >{{ c.mentionCount }}</span
                  ><i
                    v-else-if="c.unreadCount"
                    class="unread-dot w-[7px] h-[7px] bg-foreground rounded-full mr-0.5"
                  />
                </template>
              </ConversationResult>
              <ConversationResult
                v-for="n in servers"
                :key="n.id"
                :class="{ 'is-selected': isSelected(`server:${n.id}`) }"
                icon="terminal"
                title="Server window"
                :detail="n.name"
                @click="open(`server:${n.id}`)"
              />
              <p
                v-if="people.length"
                :class="[listLabel, 'mt-4']"
              >
                People
              </p>
              <ConversationResult
                v-for="person in people"
                :key="person.id"
                :class="{ 'is-selected': isSelected(`person:${person.id}`) }"
                :title="person.displayName"
                :detail="`@${person.nick}`"
                action="message"
                @click="direct(person.id)"
              >
                <template #icon><UserAvatar :member="person" /></template>
              </ConversationResult>
              <EmptyState v-if="!conversations.length && !people.length">
                <AppIcon
                  name="search"
                  :size="27"
                />
                <p>No conversations or people match “{{ query }}”.</p>
              </EmptyState></template
            >
            <template v-else
              ><EmptyState v-if="!canSearch(typed, filters)">
                <AppIcon
                  name="message"
                  :size="28"
                />
                <p>
                  Type at least two characters or choose a filter to search messages. Use
                  <strong>from:</strong>, <strong>in:</strong>, <strong>has:</strong>,
                  <strong>mentions:</strong> or a date.
                </p>
              </EmptyState>
              <WorkspaceLoading
                v-else-if="searching"
                label="Searching messages"
              />
              <EmptyState
                v-else-if="search.error.value"
                role="alert"
              >
                <AppIcon
                  name="alert"
                  :size="28"
                />
                <p>Couldn’t search messages. {{ errorText(search.error.value) }}</p>
                <UiButton
                  variant="secondary"
                  @click="search.refetch()"
                  >Try again</UiButton
                >
              </EmptyState>
              <EmptyState v-else-if="!results.length">
                <AppIcon
                  name="search"
                  :size="27"
                />
                <p>
                  {{
                    debounced
                      ? `No messages match “${debounced}”${filtered ? ' with these filters' : ''}.`
                      : 'No messages match these filters.'
                  }}
                </p>
              </EmptyState>
              <MessageResult
                v-for="result in results"
                :key="result.message.id"
                :member="chat.member(result.message.senderId)"
                :author="chat.member(result.message.senderId)?.displayName"
                :class="{ 'is-selected': isSelected(result.message.id) }"
                :time="{
                  iso: result.message.createdAt,
                  label: new Date(result.message.createdAt).toLocaleDateString(),
                }"
                @click="jumpTo(result.conversation.id, result.message.id)"
              >
                <template #location
                  >{{ result.conversation.kind === 'channel' ? '#' : '@'
                  }}{{ result.conversation.name }}</template
                ><MessageContent :text="result.message.text"
              /></MessageResult>
              <UiAlert
                v-if="moreError"
                variant="destructive"
                presentation="inline"
                role="alert"
                >{{ moreError }}</UiAlert
              >
              <UiButton
                v-if="cursor"
                variant="secondary"
                class="mt-3"
                :disabled="loadingMore"
                @click="loadMore"
              >
                {{ loadingMore ? 'Loading older messages…' : 'Load older messages' }}
              </UiButton>
            </template>
            <UiAlert
              as="p"
              variant="destructive"
              presentation="inline"
              v-if="error"
              class="form-error mt-3"
            >
              {{ error }}
            </UiAlert>
          </div>
        </div>
      </UiTabsContent>
    </UiTabs>
    <WorkspaceFooter>
      <span><kbd>↑</kbd> <kbd>↓</kbd> to choose</span><span><kbd>↵</kbd> to open</span
      ><span><kbd>Tab</kbd> messages</span><span><kbd>⌫</kbd> remove a filter</span
      ><span><kbd>esc</kbd> to return</span>
    </WorkspaceFooter>
    <SearchFiltersDialog
      v-if="showFilters"
      :filters="filters"
      @apply="filters = $event"
      @close="showFilters = false"
  /></WorkspacePage>
</template>
