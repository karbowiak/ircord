<script setup vapor lang="ts">
import { useQuery, useQueryCache } from '@pinia/colada'
import type { Message, MessagePage } from '../../../shared/types/chat'
import { stripFormatting } from '../../../shared/utils/irc'
import type { MessageLink } from '../../../shared/utils/message-links'
import { messagesQuery } from '../../queries/chat'
import { readMessages } from '../../utils/offline-archive'

/**
 * A pasted link to a message in this app: who wrote it, where, when, and how it starts. The
 * message comes from the reader's own history, so a reader who never received it sees that it
 * is not available. The card keeps one height in every state.
 */
const props = defineProps<{ link: MessageLink; url: string }>()
const chat = useChatStore()
const actions = useWorkspaceActions()
const queryCache = useQueryCache()
const conversationName = useSidebarOrder().name
const { time, full, day } = useTimeFormat()
const { t } = useI18n()

const conversation = computed(() =>
  chat.data?.conversations.find((c) => c.id === props.link.conversationId),
)
const linked = useQuery(() => ({
  key: ['message-link', props.link.conversationId, props.link.messageId],
  enabled: !!conversation.value,
  query: async ({ signal }): Promise<Message | null> => {
    const { conversationId, messageId } = props.link
    const loaded = queryCache
      .getQueryData<MessagePage>(messagesQuery(conversationId).key)
      ?.items.find((m) => m.id === messageId)
    if (loaded) return loaded
    try {
      const page = await readMessages(conversationId, { around: messageId, limit: 1 }, signal)
      return page.items.find((m) => m.id === messageId) ?? null
    } catch (error) {
      if ((error as { status?: number }).status === 404) return null
      throw error
    }
  },
  staleTime: 60_000,
  gcTime: 10 * 60_000,
  refetchOnWindowFocus: false,
}))
const message = computed(() => (conversation.value ? linked.data.value : null))
const pending = computed(() => !!conversation.value && linked.status.value === 'pending')
const author = computed(() => {
  const m = message.value
  if (!m) return ''
  const member = chat.member(m.senderId, conversation.value?.networkId)
  return member?.displayName ?? member?.nick ?? t('workspaceUi.messageLink.someone')
})
const place = computed(() => {
  const c = conversation.value
  if (!c) return ''
  if (c.party) return conversationName(c)
  if (c.kind === 'channel') return `#${c.name}`
  return `@${chat.directPartner(c)?.nick ?? c.name}`
})
const when = computed(() =>
  message.value ? `${day(message.value.createdAt)} ${time(message.value.createdAt)}` : '',
)
const excerpt = computed(() => {
  const m = message.value
  if (!m) return ''
  if (m.deleted) return t('workspaceUi.messageLink.deleted')
  const text = stripFormatting(m.text).replace(/\s+/g, ' ').trim()
  return text.length > 160 ? `${text.slice(0, 159)}…` : text
})
const unavailable = computed(() => !pending.value && !message.value)

/** A plain click jumps in place; a modified click still opens a new tab. */
function open(event: MouseEvent) {
  if (event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return
  event.preventDefault()
  void actions.reveal(props.link.conversationId, props.link.messageId)
}
</script>
<template>
  <a
    class="message-link-card mt-2 flex h-[58px] max-w-[480px] min-w-0 items-stretch gap-2.5 overflow-hidden rounded-lg border border-solid border-border bg-secondary py-2 ps-0 pe-3 text-foreground no-underline hover:bg-accent"
    :href="url"
    :aria-busy="pending || undefined"
    :title="message ? full(message.createdAt) : undefined"
    @click="open"
  >
    <span
      class="w-[3px] shrink-0 rounded-e-full bg-primary/60"
      aria-hidden="true"
    />
    <span class="grid min-w-0 flex-1 grid-rows-[18px_20px] content-center gap-0.5">
      <template v-if="pending">
        <span class="h-3 w-40 self-center rounded bg-muted" />
        <span class="h-3 w-64 max-w-full self-center rounded bg-muted" />
      </template>
      <template v-else-if="unavailable">
        <span
          class="flex min-w-0 items-center gap-1.5 truncate text-[length:var(--ui-caption)] leading-[18px] text-muted-foreground"
        >
          <AppIcon
            name="link"
            :size="12"
          />{{ t('workspaceUi.messageLink.title') }}
        </span>
        <span class="truncate text-[13px] leading-5 text-muted-foreground">
          {{ t('workspaceUi.messageLink.unavailable') }}
        </span>
      </template>
      <template v-else>
        <span
          class="flex min-w-0 items-baseline gap-1.5 truncate text-[length:var(--ui-caption)] leading-[18px] text-muted-foreground"
        >
          <b class="truncate font-[600] text-foreground">{{ author }}</b>
          <span class="truncate">{{ place }}</span>
          <time
            class="shrink-0"
            :datetime="message?.createdAt"
            >{{ when }}</time
          >
        </span>
        <span
          dir="auto"
          class="truncate text-[13px] leading-5"
          :class="{ 'text-muted-foreground italic': message?.deleted }"
          >{{ excerpt }}</span
        >
      </template>
    </span>
  </a>
</template>
