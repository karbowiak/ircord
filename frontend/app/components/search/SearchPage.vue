<script setup lang="ts" vapor>
import type { Conversation, Member } from '../../../shared/types/chat'
import { foldPersonIdentity, matchingPeople, networkPeople } from '../../../shared/utils/people'
import { nicknameError, nicknameLimit } from '../../../shared/utils/nickname'
import {
  activeToken,
  canSearch,
  emptyFilters,
  filterChips,
  filterCount,
  filterKeys,
  rememberSearch,
  sanitizeRecent,
  type RecentSearch,
  type SearchFilters,
} from '../../../shared/utils/search-filter'
import { offlineStatus } from '../../utils/offline-archive'
import {
  isSearchResultJumpable,
  searchCoverageKey,
  searchResultAuthor,
  type SearchSourceMode,
} from '../../composables/search/search-source'
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
const upstreamSearch = ref(false)
const sourceAllowed = computed(() => filters.value.in.length === 1 && !offlineStatus.value.offline)
watch(
  sourceAllowed,
  (allowed) => {
    if (!allowed) upstreamSearch.value = false
  },
  { flush: 'sync' },
)
const requestedSource = computed<SearchSourceMode>(() =>
  upstreamSearch.value ? 'upstream' : 'retained',
)
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
const { search, results, cursor, loadMore, loadingMore, moreError } = useMessageSearch(
  debounced,
  filters,
  requestedSource,
)
const coverageNote = computed(() => {
  const key = searchCoverageKey(search.data.value)
  return key ? t(key) : ''
})
const sourceHelp = computed(() =>
  offlineStatus.value.offline
    ? t('collectionsUi.search.serverHistoryOffline')
    : filters.value.in.length !== 1
      ? t('collectionsUi.search.serverHistoryNeedsScope')
      : '',
)
const boundedSearch = computed(
  () =>
    search.data.value?.source?.actual === 'upstream' &&
    search.data.value.source.coverage === 'upstream-bounded' &&
    search.data.value.source.completeness === 'unknown',
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
const filterLabels = useSearchFilters()
const field = ref<{ start: (key: string) => void } | null>(null)
/** Filters offered as pills under the field; the rest are typed or in the Filters dialog. */
const quickFilters = filterKeys.filter((k) =>
  ['from', 'mentions', 'in', 'has', 'on'].includes(k.key),
)
function recall(entry: RecentSearch) {
  query.value = debounced.value = entry.text
  filters.value = structuredClone(entry.filters)
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

/**
 * One list: conversations and people by name, then messages by text. Filters only apply to
 * messages, so a filtered search shows messages alone; an empty one is a quick switcher.
 */
const showConversations = computed(() => !filtered.value)
const searchesMessages = computed(() => canSearch(typed.value, filters.value))
const shownConversations = computed(() =>
  needle.value ? conversations.value.slice(0, 5) : conversations.value,
)
const shownPeople = computed(() => people.value.slice(0, 4))

/** Everything selectable, in display order, for keyboard navigation. */
type Item = { key: string; run: () => void }
const items = computed<Item[]>(() => [
  ...(showConversations.value
    ? [
        ...shownConversations.value.map((c) => ({ key: c.id, run: () => open(c.id) })),
        ...servers.value.map((n) => ({ key: `server:${n.id}`, run: () => open(`server:${n.id}`) })),
        ...shownPeople.value.map((m) => ({
          key: `person:${m.id}`,
          run: () => void direct(m.nick),
        })),
      ]
    : []),
  ...(searchesMessages.value
    ? results.value.map((r) => ({
        key: r.message.id,
        run: () => {
          if (isSearchResultJumpable(r)) jumpTo(r.conversation.id, r.message.id)
        },
      }))
    : []),
])
const selectedResultNotice = computed(() => {
  const selectedItem = items.value[selected.value]
  const result = results.value.find((item) => item.message.id === selectedItem?.key)
  return result && !isSearchResultJumpable(result)
    ? t('collectionsUi.search.resultNotJumpable')
    : ''
})
// Counts are read once typing settles; an empty query is not a search.
const plural = (key: string, count: number) => t(key, { count: n(count) }, count)
useAnnouncedStatus(() => {
  if (!searchesMessages.value)
    return needle.value ? plural('collectionsUi.search.matchCount', items.value.length) : ''
  if (searching.value || search.error.value || !canSearch(debounced.value, filters.value)) return ''
  const count = boundedSearch.value
    ? t(
        'collectionsUi.search.boundedResultCount',
        { count: n(results.value.length) },
        results.value.length,
      )
    : plural(
        cursor.value
          ? 'collectionsUi.search.messageCountMore'
          : 'collectionsUi.search.messageCount',
        results.value.length,
      )
  return selectedResultNotice.value ? `${count}. ${selectedResultNotice.value}` : count
})
const selected = ref(0)
watch(items, () => (selected.value = 0))
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
  }
}
onBeforeUnmount(() => clearTimeout(timer))
// “Search everywhere” from a conversation's search arrives as ?q=.
const route = useRoute()
watch(
  () => route.query.q,
  (q) => {
    if (typeof q !== 'string' || !q.trim()) return
    query.value = q
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
    :description="t('collectionsUi.search.description')"
    @close="emit('close')"
  >
    <WorkspaceToolbar class="flex-row flex-wrap items-center">
      <SearchQueryField
        ref="field"
        v-model:text="query"
        v-model:filters="filters"
        :menu="false"
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
      <div
        class="search-quick-filters flex basis-full flex-wrap gap-1.5"
        role="group"
        :aria-label="t('collectionsUi.search.filters')"
      >
        <button
          v-for="k in quickFilters"
          :key="k.key"
          type="button"
          class="inline-flex h-7 items-center gap-1.5 rounded-full bg-secondary/45 px-3 text-[length:var(--ui-caption)] font-[550] text-muted-foreground shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)] transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring [@media(pointer:coarse)]:h-9"
          :aria-label="t(`collectionsUi.search.${k.key}Label`)"
          @click="field?.start(k.key)"
        >
          <AppIcon
            :name="k.icon"
            :size="13"
          />{{ t(`collectionsUi.search.${k.key}Short`) }}
        </button>
      </div>
    </WorkspaceToolbar>
    <OfflineArchiveNotice
      v-if="searchesMessages"
      :offline="search.data.value?.offline" />
    <div class="search-results min-h-0 flex-1 overflow-y-auto px-4 pb-4 max-[700px]:px-1.5">
      <div ref="resultsBox">
        <div class="w-full space-y-0.5">
          <template v-if="showConversations"
            ><template v-if="!query && !filtered && recentSearches.length"
              ><div class="flex items-center justify-between pr-1">
                <p :class="listLabel">{{ t('collectionsUi.search.recentSearches') }}</p>
                <UiButton
                  variant="ghost"
                  size="text"
                  class="text-[length:var(--ui-caption)] text-muted-foreground hover:text-foreground"
                  @click="forget"
                  >{{ t('collectionsUi.search.clear') }}</UiButton
                >
              </div>
              <div class="search-recent mb-3 flex flex-wrap gap-1.5 px-3">
                <button
                  v-for="(entry, index) in recentSearches"
                  :key="index"
                  type="button"
                  class="inline-flex max-w-full min-w-0 items-center gap-1.5 rounded-full bg-secondary/45 py-1 pr-3 pl-2.5 text-[length:var(--ui-small)] text-body shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)] transition-colors hover:bg-accent hover:text-foreground"
                  @click="recall(entry)"
                >
                  <AppIcon
                    name="clock"
                    :size="13"
                    class="shrink-0 text-muted-foreground"
                  /><span
                    v-for="chip in filterChips(entry.filters)"
                    :key="`${chip.key}:${chip.value}`"
                    class="shrink-0 text-[length:var(--ui-caption)]"
                    ><span class="text-primary">{{ chip.key }}:</span
                    >{{ filterLabels.chipLabel(chip) }}</span
                  ><span
                    v-if="entry.text"
                    class="min-w-0 truncate"
                    >{{ entry.text }}</span
                  >
                </button>
              </div></template
            >
            <p
              v-if="shownConversations.length || servers.length || !needle"
              :class="listLabel"
            >
              {{
                needle ? t('collectionsUi.search.conversations') : t('collectionsUi.search.jumpTo')
              }}
            </p>
            <ConversationResult
              v-for="c in shownConversations"
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
              v-if="shownPeople.length"
              :class="[listLabel, 'mt-4']"
            >
              {{ t('collectionsUi.search.people') }}
            </p>
            <ConversationResult
              v-for="person in shownPeople"
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
            <FieldHint v-else-if="needle && peopleLookup.state.value.page?.status === 'partial'">{{
              t('collectionsUi.search.peoplePartial')
            }}</FieldHint>
            <EmptyState
              v-if="!searchesMessages && !conversations.length && !people.length"
              icon="search"
            >
              <p>{{ t('collectionsUi.search.noConversations', { query }) }}</p>
            </EmptyState></template
          >
          <template v-if="searchesMessages"
            ><div
              class="mt-4 flex flex-wrap items-center justify-between gap-x-3 gap-y-1 pr-1 first:mt-0"
            >
              <p :class="listLabel">{{ t('collectionsUi.search.messages') }}</p>
              <label
                class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-[length:var(--ui-caption)] text-muted-foreground"
              >
                <span v-if="sourceHelp">{{ sourceHelp }}</span>
                {{ t('collectionsUi.search.serverHistory') }}
                <AppSwitch
                  v-model="upstreamSearch"
                  :disabled="!sourceAllowed"
                  :aria-label="t('collectionsUi.search.serverHistory')"
                />
              </label>
            </div>
            <p
              v-if="coverageNote"
              class="m-0 px-3 pb-1 text-[length:var(--ui-caption)] text-muted-foreground"
              role="note"
            >
              {{ coverageNote }}
            </p>
            <WorkspaceLoading
              v-if="searching"
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
              :disabled-action="!isSearchResultJumpable(result)"
              :member="chat.member(result.message.senderId, result.conversation.networkId)"
              :author="
                searchResultAuthor(
                  result,
                  chat.member(result.message.senderId, result.conversation.networkId),
                )
              "
              :class="{ 'is-selected': isSelected(result.message.id) }"
              :time="{
                iso: result.message.createdAt,
                label: day(new Date(result.message.createdAt).toISOString()),
              }"
              @click="
                isSearchResultJumpable(result) && jumpTo(result.conversation.id, result.message.id)
              "
            >
              <template #location
                >{{
                  result.conversation.party
                    ? ''
                    : result.conversation.kind === 'channel'
                      ? '#'
                      : '@'
                }}{{ label(result.conversation) }}</template
              ><template
                v-if="!isSearchResultJumpable(result)"
                #badges
              >
                <span
                  role="note"
                  :aria-label="t('collectionsUi.search.resultNotJumpable')"
                  :title="t('collectionsUi.search.resultNotJumpable')"
                  class="rounded-sm bg-raised px-1.5 py-0.5 text-[length:var(--ui-label)] text-muted-foreground"
                  >{{ t('collectionsUi.search.resultRemoteBadge') }}</span
                > </template
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
    </div>
    <WorkspaceFooter>
      <span><kbd>↑</kbd> <kbd>↓</kbd> {{ t('collectionsUi.search.keyboardChoose') }}</span
      ><span><kbd>↵</kbd> {{ t('collectionsUi.search.keyboardOpen') }}</span
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
