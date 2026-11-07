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
    ><div class="search-field">
      <AppIcon
        name="search"
        :size="21"
      /><input
        v-model="query"
        autofocus
        placeholder="Channels, people or messages"
        aria-label="Search workspace"
        @keydown="keys"
      /><kbd>esc</kbd>
    </div>
    <div
      class="search-tabs"
      role="tablist"
      aria-label="Search type"
    >
      <button
        :class="{ active: tab === 'conversations' }"
        role="tab"
        :aria-selected="tab === 'conversations'"
        @click="tab = 'conversations'"
      >
        Conversations <span v-if="query">{{ conversations.length }}</span></button
      ><button
        :class="{ active: tab === 'messages' }"
        role="tab"
        :aria-selected="tab === 'messages'"
        @click="tab = 'messages'"
      >
        Messages <span v-if="query.length > 1">{{ results.length }}</span>
      </button>
    </div>
    <div
      ref="resultsBox"
      class="search-results"
      role="tabpanel"
    >
      <template v-if="tab === 'conversations'"
        ><p class="list-label">{{ query ? 'Conversations' : 'Jump to' }}</p>
        <button
          v-for="c in conversations"
          :key="c.id"
          class="search-conversation"
          :class="{ 'is-selected': isSelected(c.id) }"
          @click="open(c.id)"
        >
          <span class="search-result-icon"
            ><AppIcon :name="c.kind === 'channel' ? 'hash' : 'message'" /></span
          ><span
            ><strong>{{ label(c) }}</strong
            ><small
              >{{ chat.data?.networks.find((n) => n.id === c.networkId)?.name }} ·
              {{ c.kind === 'channel' ? 'Channel' : 'Direct message' }}</small
            ></span
          ><span
            v-if="c.mentionCount"
            class="mention-badge"
            >{{ c.mentionCount }}</span
          ><i
            v-else-if="c.unreadCount"
            class="unread-dot"
          /><AppIcon
            name="enter"
            :size="16"
          />
        </button>
        <button
          v-for="n in servers"
          :key="n.id"
          class="search-conversation"
          :class="{ 'is-selected': isSelected(`server:${n.id}`) }"
          @click="open(`server:${n.id}`)"
        >
          <span class="search-result-icon"><AppIcon name="terminal" /></span
          ><span
            ><strong>Server window</strong><small>{{ n.name }}</small></span
          ><AppIcon
            name="enter"
            :size="16"
          />
        </button>
        <p
          v-if="people.length"
          class="list-label"
        >
          People
        </p>
        <button
          v-for="person in people"
          :key="person.id"
          class="search-conversation"
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
        <div
          v-if="!conversations.length && !people.length"
          class="search-empty"
        >
          <AppIcon
            name="search"
            :size="27"
          />
          <p>No conversations or people match “{{ query }}”.</p>
        </div></template
      >
      <template v-else
        ><div
          v-if="query.trim().length < 2"
          class="search-empty"
        >
          <AppIcon
            name="message"
            :size="28"
          />
          <p>Type at least two characters to search messages.</p>
        </div>
        <p
          v-else-if="searching"
          class="search-empty"
        >
          Searching…
        </p>
        <div
          v-else-if="!results.length"
          class="search-empty"
        >
          <AppIcon
            name="search"
            :size="27"
          />
          <p>No messages match “{{ query }}”.</p>
        </div>
        <button
          v-for="result in results"
          :key="result.message.id"
          class="search-message"
          :class="{ 'is-selected': isSelected(result.message.id) }"
          @click="emit('jump', result.conversation.id, result.message.id)"
        >
          <span class="search-message-location"
            >{{ result.conversation.kind === 'channel' ? '#' : '@' }}{{ result.conversation.name
            }}<time>{{ new Date(result.message.createdAt).toLocaleDateString() }}</time></span
          ><span
            ><strong>{{ chat.member(result.message.senderId)?.displayName }}</strong
            ><MessageContent :text="result.message.text"
          /></span></button
      ></template>
      <p
        v-if="error"
        class="form-error"
        role="alert"
      >
        {{ error }}
      </p>
    </div>
    <footer class="search-footer">
      <span><kbd>↑</kbd> <kbd>↓</kbd> to choose</span><span><kbd>↵</kbd> to open</span
      ><span><kbd>Tab</kbd> messages</span><span><kbd>esc</kbd> to close</span>
    </footer></AppDialog
  >
</template>
