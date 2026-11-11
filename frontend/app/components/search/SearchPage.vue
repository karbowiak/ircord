<script setup lang="ts" vapor>
import type { Conversation, Member } from '../../../shared/types/chat'
import { foldPersonIdentity, matchingPeople, networkPeople } from '../../../shared/utils/people'
import { nicknameError, nicknameLimit } from '../../../shared/utils/nickname'
import {
  activeToken,
  canSearch,
  emptyFilters,
  filterCount,
  rememberSearch,
  sanitizeRecent,
  type RecentSearch,
  type SearchFilters,
} from '../../../shared/utils/search-filter'
const emit = defineEmits<{ close: []; jump: [conversationId: string, messageId: string] }>()
const chat = useChatStore()
const auth = useAuthStore()
const navigation = useConversationNavigation()
const { t, n } = useI18n()
const { day } = useTimeFormat()
const query = ref('')
const debounced = ref('')
const filters = ref<SearchFilters>(emptyFilters())
const filtered = computed(() => filterCount(filters.value) > 0)
const tab = ref<'conversations' | 'messages'>('conversations')
useViewHistory('search-section', tab)
const error = ref<unknown>()
const errorMessage = (value: unknown) => errorText(value, (key) => t(key))
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
useViewHistory('search-filters-view', showFilters)

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
const peopleLookup = usePeopleLookup(() => ({
  networkId: chat.networkId,
  purpose: 'dm',
  limit: 20,
}))
watch(needle, (value) => peopleLookup.search(value || null))
const { recent } = useRecentConversations()
const label = useSidebarOrder().name

/**
 * Without a query: what needs attention (mentions, then unread DMs, then unread channels), then
 * recently visited. With a query: exact, prefix, then substring matches, ties broken the same way.
 */
const conversations = computed(() => {
  const list = chat.data?.conversations ?? []
  const urgency = (c: Conversation) =>
    c.mentionCount
      ? 0
      : (c.kind === 'direct' || c.party) && c.unreadCount
        ? 1
        : c.unreadCount
          ? 2
          : 3
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
const servers = computed(() => {
  const serverWindow = t('collectionsUi.search.serverWindow').toLowerCase()
  return (chat.data?.networks ?? [])
    .filter((n) => !needle.value || `${serverWindow} ${n.name}`.includes(needle.value))
    .slice(0, needle.value ? 4 : 0)
})
const people = computed(() => {
  if (!needle.value) return []
  const mapping = chat.network?.isupport?.CASEMAPPING
  const fold = (value: string) => foldPersonIdentity(value, mapping)
  const local: {
    id: string
    nick: string
    displayName: string
    member: Pick<Member, 'displayName'> &
      Partial<Pick<Member, 'avatarUrl' | 'color' | 'role' | 'device' | 'status'>>
  }[] = matchingPeople(
    networkPeople(chat.data?.members ?? [], chat.data?.conversations ?? [], chat.networkId),
    needle.value,
    mapping,
    6,
  ).map((member) => ({ id: member.id, nick: member.nick, displayName: member.displayName, member }))
  const seen = new Set(local.map((person) => fold(person.nick)))
  const ownNick = fold(chat.network?.nick ?? '')
  for (const person of peopleLookup.state.value.page?.items ?? []) {
    if (
      !person.eligible ||
      !person.nickname ||
      person.memberId === 'you' ||
      fold(person.nickname) === ownNick ||
      seen.has(fold(person.nickname))
    )
      continue
    seen.add(fold(person.nickname))
    local.push({
      id: person.memberId || person.key,
      nick: person.nickname,
      displayName: person.displayName,
      member: chat.member(person.memberId ?? '') ?? {
        displayName: person.displayName,
        avatarUrl: person.avatarUrl,
      },
    })
  }
  const typedNick = typed.value.replace(/^@/, '')
  if (
    chat.networkId &&
    typedNick &&
    !nicknameError(typedNick, nicknameLimit(chat.network?.isupport)) &&
    fold(typedNick) !== ownNick &&
    !seen.has(fold(typedNick))
  ) {
    if (local.length >= 6) local.pop()
    local.push({
      id: `nickname:${fold(typedNick)}`,
      nick: typedNick,
      displayName: typedNick,
      member: { displayName: typedNick },
    })
  }
  return local.slice(0, 6)
})

/** Everything selectable in the current tab, in display order, for keyboard navigation. */
type Item = { key: string; run: () => void }
const items = computed<Item[]>(() =>
  tab.value === 'conversations'
    ? [
        ...conversations.value.map((c) => ({ key: c.id, run: () => open(c.id) })),
        ...servers.value.map((n) => ({ key: `server:${n.id}`, run: () => open(`server:${n.id}`) })),
        ...people.value.map((m) => ({ key: `person:${m.id}`, run: () => void direct(m.nick) })),
      ]
    : results.value.map((r) => ({
        key: r.message.id,
        run: () => jumpTo(r.conversation.id, r.message.id),
      })),
)
// Counts are read once typing settles; an empty query is not a search.
const plural = (key: string, count: number) => t(key, { count: n(count) }, count)
useAnnouncedStatus(() => {
  if (tab.value === 'conversations')
    return needle.value
      ? plural(
          cursor.value ? 'collectionsUi.search.matchCountMore' : 'collectionsUi.search.matchCount',
          items.value.length,
        )
      : ''
  if (searching.value || search.error.value || !canSearch(debounced.value, filters.value)) return ''
  return plural(
    cursor.value ? 'collectionsUi.search.messageCountMore' : 'collectionsUi.search.messageCount',
    results.value.length,
  )
})
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
async function direct(nickname: string) {
  try {
    await chat.direct.mutateAsync({ networkId: chat.networkId, nickname })
    emit('close')
  } catch (e) {
    error.value = e
  }
}
</script>
<template>
  <WorkspacePage
    :title="t('collectionsUi.search.title')"
    icon="search"
    :description="t('collectionsUi.search.description')"
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
          :placeholder="t('collectionsUi.search.placeholder')"
          @keydown="keys"
          @more="showFilters = true"
          @recall="recall"
          @forget="forget"
        />
        <UiButton
          variant="secondary"
          class="shrink-0"
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
          />{{ t('collectionsUi.search.filters')
          }}<span
            v-if="filtered"
            class="rounded-full bg-selection px-1.5 text-[length:var(--ui-label)] text-primary"
            >{{ n(filterCount(filters)) }}</span
          ></UiButton
        >
        <UiTabsList
          class="search-tabs min-h-9 w-fit max-w-full gap-1 overflow-x-auto rounded-lg bg-sidebar p-0.5 [scrollbar-width:none]"
          :aria-label="t('collectionsUi.search.type')"
        >
          <UiTabsTrigger
            variant="pill"
            value="conversations"
          >
            {{ t('collectionsUi.search.conversations') }}
            <span v-if="query">{{ n(conversations.length) }}</span></UiTabsTrigger
          ><UiTabsTrigger
            variant="pill"
            value="messages"
          >
            {{ t('collectionsUi.search.messages') }}
            <span v-if="canSearch(debounced, filters)">{{ n(results.length) }}</span>
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
                {{
                  query ? t('collectionsUi.search.conversations') : t('collectionsUi.search.jumpTo')
                }}
              </p>
              <ConversationResult
                v-for="c in conversations"
                :key="c.id"
                :class="{ 'is-selected': isSelected(c.id) }"
                :icon="c.party ? 'users' : c.kind === 'channel' ? 'hash' : 'message'"
                :title="label(c)"
                :detail="
                  t('collectionsUi.search.conversationDetail', {
                    network: chat.data?.networks.find((n) => n.id === c.networkId)?.name ?? '',
                    kind: t(
                      c.kind === 'channel' && !c.party
                        ? 'collectionsUi.search.channel'
                        : 'collectionsUi.search.directMessage',
                    ),
                  })
                "
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
                    >{{ n(c.mentionCount) }}</span
                  ><i
                    v-else-if="c.unreadCount"
                    class="unread-dot w-[7px] h-[7px] bg-foreground rounded-full mr-0.5"
                  />
                </template>
              </ConversationResult>
              <ConversationResult
                v-for="server in servers"
                :key="server.id"
                :class="{ 'is-selected': isSelected(`server:${server.id}`) }"
                icon="terminal"
                :title="t('collectionsUi.search.serverWindow')"
                :detail="server.name"
                @click="open(`server:${server.id}`)"
              />
              <p
                v-if="people.length"
                :class="[listLabel, 'mt-4']"
              >
                {{ t('collectionsUi.search.people') }}
              </p>
              <ConversationResult
                v-for="person in people"
                :key="person.id"
                :class="{ 'is-selected': isSelected(`person:${person.id}`) }"
                :title="person.displayName"
                :detail="`@${person.nick}`"
                action="message"
                @click="direct(person.nick)"
              >
                <template #icon><UserAvatar :member="person.member" /></template>
              </ConversationResult>
              <FieldHint
                v-if="
                  needle &&
                  (peopleLookup.state.value.error ||
                    peopleLookup.state.value.page?.status === 'unavailable')
                "
                >{{ t('collectionsUi.search.peopleUnavailable') }}</FieldHint
              >
              <FieldHint
                v-else-if="needle && peopleLookup.state.value.page?.status === 'partial'"
                >{{ t('collectionsUi.search.peoplePartial') }}</FieldHint
              >
              <EmptyState v-if="!conversations.length && !people.length">
                <AppIcon
                  name="search"
                  :size="27"
                />
                <p>{{ t('collectionsUi.search.noConversations', { query }) }}</p>
              </EmptyState></template
            >
            <template v-else
              ><EmptyState v-if="!canSearch(typed, filters)">
                <AppIcon
                  name="message"
                  :size="28"
                />
                <p>
                  {{ t('collectionsUi.search.messagePrompt') }}
                  {{
                    t('collectionsUi.search.syntaxHelp', {
                      from: 'from:',
                      in: 'in:',
                      has: 'has:',
                      mentions: 'mentions:',
                    })
                  }}
                </p>
              </EmptyState>
              <WorkspaceLoading
                v-else-if="searching"
                :label="t('collectionsUi.search.searching')"
              />
              <EmptyState
                v-else-if="search.error.value"
                role="alert"
              >
                <AppIcon
                  name="alert"
                  :size="28"
                />
                <p>
                  {{
                    t('collectionsUi.search.searchFailed', {
                      error: errorMessage(search.error.value),
                    })
                  }}
                </p>
                <UiButton
                  variant="secondary"
                  @click="search.refetch()"
                  >{{ t('common.tryAgain') }}</UiButton
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
                      ? t(
                          filtered
                            ? 'collectionsUi.search.noMessageQueryFiltered'
                            : 'collectionsUi.search.noMessageQuery',
                          { query: debounced },
                        )
                      : t('collectionsUi.search.noMessageFilters')
                  }}
                </p>
              </EmptyState>
              <MessageResult
                v-for="result in results"
                :key="result.message.id"
                :member="chat.member(result.message.senderId, result.conversation.networkId)"
                :author="
                  chat.member(result.message.senderId, result.conversation.networkId)?.displayName
                "
                :class="{ 'is-selected': isSelected(result.message.id) }"
                :time="{
                  iso: result.message.createdAt,
                  label: day(new Date(result.message.createdAt).toISOString()),
                }"
                @click="jumpTo(result.conversation.id, result.message.id)"
              >
                <template #location
                  >{{
                    result.conversation.party
                      ? ''
                      : result.conversation.kind === 'channel'
                        ? '#'
                        : '@'
                  }}{{ label(result.conversation) }}</template
                ><MessageContent :text="result.message.text"
              /></MessageResult>
              <UiAlert
                v-if="moreError"
                variant="destructive"
                presentation="inline"
                role="alert"
                >{{
                  t('collectionsUi.search.loadMoreFailed', { error: errorMessage(moreError) })
                }}</UiAlert
              >
              <UiButton
                v-if="cursor"
                variant="secondary"
                class="mt-3"
                :disabled="loadingMore"
                @click="loadMore"
              >
                {{
                  loadingMore
                    ? t('collectionsUi.search.loadingOlder')
                    : t('collectionsUi.search.loadOlder')
                }}
              </UiButton>
            </template>
            <UiAlert
              v-if="error"
              as="p"
              variant="destructive"
              presentation="inline"
              class="form-error mt-3"
            >
              {{ t('collectionsUi.search.openPersonFailed', { error: errorMessage(error) }) }}
            </UiAlert>
          </div>
        </div>
      </UiTabsContent>
    </UiTabs>
    <WorkspaceFooter>
      <span><kbd>↑</kbd> <kbd>↓</kbd> {{ t('collectionsUi.search.keyboardChoose') }}</span
      ><span><kbd>↵</kbd> {{ t('collectionsUi.search.keyboardOpen') }}</span
      ><span
        ><kbd>{{ t('workspaceUi.shortcuts.keys.tab') }}</kbd>
        {{ t('collectionsUi.search.keyboardMessages') }}</span
      ><span><kbd>⌫</kbd> {{ t('collectionsUi.search.keyboardRemoveFilter') }}</span
      ><span
        ><kbd>{{ t('workspaceUi.shortcuts.keys.escape') }}</kbd>
        {{ t('collectionsUi.search.keyboardReturn') }}</span
      >
    </WorkspaceFooter>
    <SearchFiltersDialog
      v-if="showFilters"
      :filters="filters"
      @apply="filters = $event"
      @close="showFilters = false"
  /></WorkspacePage>
</template>
