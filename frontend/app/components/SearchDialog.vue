<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import type { Conversation } from '../../shared/types/chat'
import { searchQuery } from '../queries/chat'
const emit = defineEmits<{ close: []; jump: [conversationId: string, messageId: string] }>()
const chat = useChatStore()
const query = ref('')
const debounced = ref('')
const tab = ref<'conversations' | 'messages'>('conversations')
const error = ref('')
let timer: ReturnType<typeof setTimeout> | undefined
watch(query, (q) => {
  clearTimeout(timer)
  timer = setTimeout(() => (debounced.value = q.trim()), 220)
})
const search = useQuery(() => ({
  ...searchQuery(debounced.value),
  enabled: debounced.value.length >= 2,
}))
const results = computed(() =>
  debounced.value.length >= 2 ? (search.data.value?.items ?? []) : [],
)
const searching = computed(
  () => query.value.trim() !== debounced.value || (search.isLoading.value && !search.data.value),
)
const needle = computed(() => query.value.toLowerCase().replace(/^[#@]/, ''))
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
        run: () => emit('jump', r.conversation.id, r.message.id),
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
  } else if (event.key === 'Tab' && !event.shiftKey && query.value) {
    event.preventDefault()
    tab.value = tab.value === 'conversations' ? 'messages' : 'conversations'
  }
}
onBeforeUnmount(() => clearTimeout(timer))
function open(id: string) {
  chat.activate(id)
  emit('close')
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
  <AppDialog
    title="Search"
    wide
    @close="emit('close')"
    ><SearchField
      v-model="query"
      size="lg"
      hint="esc"
      class="search-field mx-5"
      autofocus
      placeholder="Channels, people or messages"
      aria-label="Search workspace"
      @keydown="keys"
    />
    <UiTabs
      v-model="tab"
      class="contents"
    >
      <UiTabsList
        class="search-tabs gap-4 pt-3 pr-5 pb-0 pl-5 border-b [border-bottom-style:solid] border-b-border"
        aria-label="Search type"
      >
        <UiTabsTrigger value="conversations">
          Conversations <span v-if="query">{{ conversations.length }}</span></UiTabsTrigger
        ><UiTabsTrigger value="messages">
          Messages <span v-if="query.length > 1">{{ results.length }}</span>
        </UiTabsTrigger>
      </UiTabsList>
      <!-- One panel for both tabs, so the results box and its keyboard selection stay put. -->
      <UiTabsContent
        :value="tab"
        as-child
      >
        <div
          ref="resultsBox"
          class="search-results pt-2 pr-3 pb-3 pl-3 min-h-70 max-h-[min(460px,_calc(100dvh_-_260px))] overflow-y-auto"
        >
          <template v-if="tab === 'conversations'"
            ><p
              class="list-label text-[11px] uppercase tracking-[0.05em] font-[600] text-faint pt-2.5 pr-2 pb-1 pl-2"
            >
              {{ query ? 'Conversations' : 'Jump to' }}
            </p>
            <button
              v-for="c in conversations"
              :key="c.id"
              class="search-conversation flex items-center gap-2.5 w-full py-1.5 px-2 rounded-sm text-left"
              :class="{ 'is-selected': isSelected(c.id) }"
              @click="open(c.id)"
            >
              <span
                class="search-result-icon w-9 h-9 flex items-center justify-center rounded-full bg-secondary text-muted-foreground"
                ><AppIcon :name="c.kind === 'channel' ? 'hash' : 'message'" /></span
              ><span
                ><strong>{{ label(c) }}</strong
                ><small
                  >{{ chat.data?.networks.find((n) => n.id === c.networkId)?.name }} ·
                  {{ c.kind === 'channel' ? 'Channel' : 'Direct message' }}</small
                ></span
              ><span
                v-if="c.mentionCount"
                class="mention-badge bg-destructive text-rail text-[11px] min-w-4.5 h-4.5 py-0 px-[5px] flex items-center justify-center rounded-[9px] font-[700]"
                >{{ c.mentionCount }}</span
              ><i
                v-else-if="c.unreadCount"
                class="unread-dot w-[7px] h-[7px] bg-foreground rounded-full mr-0.5"
              /><AppIcon
                name="enter"
                :size="16"
              />
            </button>
            <button
              v-for="n in servers"
              :key="n.id"
              class="search-conversation flex items-center gap-2.5 w-full py-1.5 px-2 rounded-sm text-left"
              :class="{ 'is-selected': isSelected(`server:${n.id}`) }"
              @click="open(`server:${n.id}`)"
            >
              <span
                class="search-result-icon w-9 h-9 flex items-center justify-center rounded-full bg-secondary text-muted-foreground"
                ><AppIcon name="terminal" /></span
              ><span
                ><strong>Server window</strong><small>{{ n.name }}</small></span
              ><AppIcon
                name="enter"
                :size="16"
              />
            </button>
            <p
              v-if="people.length"
              class="list-label text-[11px] uppercase tracking-[0.05em] font-[600] text-faint pt-2.5 pr-2 pb-1 pl-2"
            >
              People
            </p>
            <button
              v-for="person in people"
              :key="person.id"
              class="search-conversation flex items-center gap-2.5 w-full py-1.5 px-2 rounded-sm text-left"
              :class="{ 'is-selected': isSelected(`person:${person.id}`) }"
              @click="direct(person.id)"
            >
              <UserAvatar :member="person" /><span
                ><strong>{{ person.displayName }}</strong
                ><small>@{{ person.nick }}</small></span
              ><AppIcon
                name="message"
                :size="17"
              />
            </button>
            <EmptyState v-if="!conversations.length && !people.length">
              <AppIcon
                name="search"
                :size="27"
              />
              <p>No conversations or people match “{{ query }}”.</p>
            </EmptyState></template
          >
          <template v-else
            ><EmptyState v-if="query.trim().length < 2">
              <AppIcon
                name="message"
                :size="28"
              />
              <p>Type at least two characters to search messages.</p>
            </EmptyState>
            <EmptyState
              as="p"
              v-else-if="searching"
            >
              Searching…
            </EmptyState>
            <EmptyState v-else-if="!results.length">
              <AppIcon
                name="search"
                :size="27"
              />
              <p>No messages match “{{ query }}”.</p>
            </EmptyState>
            <MessageResult
              v-for="result in results"
              :key="result.message.id"
              :class="{ 'is-selected': isSelected(result.message.id) }"
              :time="{
                iso: result.message.createdAt,
                label: new Date(result.message.createdAt).toLocaleDateString(),
              }"
              @click="emit('jump', result.conversation.id, result.message.id)"
            >
              <template #location
                >{{ result.conversation.kind === 'channel' ? '#' : '@'
                }}{{ result.conversation.name }}</template
              ><span
                ><strong>{{ chat.member(result.message.senderId)?.displayName }}</strong
                ><MessageContent :text="result.message.text"
              /></span> </MessageResult
          ></template>
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
      </UiTabsContent>
    </UiTabs>
    <footer
      class="search-footer flex gap-3.5 py-2.5 px-5 border-t [border-top-style:solid] border-t-border bg-sidebar text-[12px] text-faint"
    >
      <span><kbd>↑</kbd> <kbd>↓</kbd> to choose</span><span><kbd>↵</kbd> to open</span
      ><span><kbd>Tab</kbd> messages</span><span><kbd>esc</kbd> to close</span>
    </footer></AppDialog
  >
</template>
