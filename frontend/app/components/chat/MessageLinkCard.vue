<script setup vapor lang="ts">
import { useQuery, useQueryCache } from '@pinia/colada'
import type { Message, MessagePage } from '../../../shared/types/chat'
import { channelTarget, stripFormatting } from '../../../shared/utils/irc'
import { messageAttachments } from '../../../shared/utils/link-media'
import { isLinkedMessage, type MessageLink } from '../../../shared/utils/message-links'
import { messagesQuery } from '../../queries/chat'

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
const root = ref<HTMLElement | null>(null)
const { preload } = useEmbedVisibility(root)

const conversation = (id: string | undefined) =>
  id ? chat.data?.conversations.find((c) => c.id === id) : undefined
/** The live copy in a loaded timeline, so edits and deletions show at once. */
function loaded(conversationId: string | undefined, id: string) {
  const c = conversation(conversationId)
  if (!c) return undefined
  const page = queryCache.get(messagesQuery(c.id).key)?.state.value.data as MessagePage | undefined
  return page?.items.find((m) => isLinkedMessage(c.networkId, m, id))
}
const hinted = computed(() => loaded(props.link.conversationId, props.link.messageId))
// Only cards near the screen ask the server, and each different link asks once.
const located = useQuery(() => ({
  key: ['message-link', props.link.messageId],
  enabled: preload.value && !hinted.value,
  query: async ({ signal }): Promise<Message | null> => {
    try {
      return await apiClient().locateMessage(props.link.messageId, signal)
    } catch (error) {
      if (errorStatus(error) === 404) return null
      throw error
    }
  },
  staleTime: 60_000,
  gcTime: 10 * 60_000,
  refetchOnWindowFocus: false,
}))
const message = computed(() => {
  if (hinted.value) return hinted.value
  const found = located.data.value
  if (!found || !conversation(found.conversationId)) return null
  return loaded(found.conversationId, found.id) ?? found
})
const state = computed(() =>
  message.value
    ? 'found'
    : located.status.value === 'error'
      ? 'failed'
      : located.status.value === 'success'
        ? 'unavailable'
        : 'pending',
)
const author = computed(() => {
  const m = message.value
  if (!m) return ''
  const member = chat.member(m.senderId, conversation(m.conversationId)?.networkId)
  return member?.displayName ?? member?.nick ?? t('workspaceUi.message.unknownSender')
})
const place = computed(() => {
  const c = conversation(message.value?.conversationId)
  if (!c) return ''
  if (c.party) return conversationName(c)
  if (c.kind === 'channel') return channelTarget(c)
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
  if (text) return text.length > 160 ? `${text.slice(0, 159)}…` : text
  return m.gif?.title || messageAttachments(m)[0]?.name || t('workspaceUi.messageLink.noText')
})

/** A plain click jumps in place; a modified click still opens a new tab. */
function open(event: MouseEvent) {
  if (event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return
  event.preventDefault()
  void actions.openMessageLink(props.link)
}
</script>
<template>
  <a
    ref="root"
    class="message-link-card mt-2 flex h-[58px] max-w-[480px] min-w-0 items-stretch gap-2.5 overflow-hidden rounded-lg border border-solid border-border bg-secondary py-2 ps-0 pe-3 text-foreground no-underline hover:bg-accent"
    :href="url"
    :aria-busy="state === 'pending' || undefined"
    :aria-label="state === 'pending' ? t('workspaceUi.messageLink.title') : undefined"
    :title="message ? full(message.createdAt) : undefined"
    :data-state="state"
    @click="open"
  >
    <span
      class="w-[3px] shrink-0 rounded-e-full bg-primary/60"
      aria-hidden="true"
    />
    <span class="grid min-w-0 flex-1 grid-rows-[18px_20px] content-center gap-0.5">
      <template v-if="state === 'pending'">
        <span class="h-3 w-40 self-center rounded bg-muted" />
        <span class="h-3 w-64 max-w-full self-center rounded bg-muted" />
      </template>
      <template v-else-if="state !== 'found'">
        <span
          class="flex min-w-0 items-center gap-1.5 truncate text-[length:var(--ui-caption)] leading-[18px] text-muted-foreground"
        >
          <AppIcon
            name="link"
            :size="12"
          />{{ t('workspaceUi.messageLink.title') }}
        </span>
        <span class="truncate text-[13px] leading-5 text-muted-foreground">
          {{
            state === 'failed'
              ? t('workspaceUi.messageLink.failed')
              : t('workspaceUi.messageLink.unavailable')
          }}
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
