<script setup vapor lang="ts">
import type { Message } from '../../../shared/types/chat'
import { ircToMarkdown } from '../../../shared/utils/irc'
/** Link content, file downloads, reactions and delivery state under a message. */
const props = defineProps<{ message: Message }>()
const emit = defineEmits<{ mediaReady: [url: string, ready: boolean] }>()
const chat = useChatStore()
const { t: $t, n: $n } = useI18n()
const reactionSupported = computed(() => chat.messageActionSupported(props.message, 'reactions'))
const reactionReason = computed(() => chat.messageActionReason(props.message, 'reactions'))

/** Discarding returns the text to the message box, so a failed send never costs what was typed. */
function discard() {
  const { conversationId, text, kind } = props.message
  chat.discard(props.message)
  // Put back what was typed, not the wire form: an action needs its /me, and a literal
  // slash message needs its second slash, or Enter would run a command instead.
  const body = ircToMarkdown(text)
  const restored = kind === 'action' ? `/me ${body}` : body.startsWith('/') ? `/${body}` : body
  const result = chat.restoreDraft(conversationId, restored)
  if (result === 'restored') return
  if (result === 'full') return chat.notify($t('workspaceUi.message.discardFull'), { error: true })
  // Text typed since stays as it is; the discarded text follows it.
  const joined = `${chat.drafts[conversationId] ?? ''}\n${restored}`
  if (joined.length > 4000)
    return chat.notify($t('workspaceUi.message.discardTooLong'), { error: true })
  chat.drafts[conversationId] = joined
}
</script>
<template>
  <div class="message-extras pl-13.5 empty:hidden">
    <MessageLinks
      :message="message"
      @media-ready="(url, ready) => emit('mediaReady', url, ready)"
    />
    <MessageAttachments
      v-if="message.attachments?.length"
      :message="message"
      @media-ready="(url, ready) => emit('mediaReady', url, ready)"
    />
    <MessageAttachment
      v-else-if="message.attachment"
      :attachment="message.attachment"
      :message="message"
      @media-ready="(url, ready) => emit('mediaReady', url, ready)"
    />
    <div
      v-if="message.reactions.length"
      class="reactions flex gap-1 mt-1 flex-wrap"
    >
      <component
        :is="reactionSupported ? 'button' : 'span'"
        v-for="reaction in message.reactions"
        :key="reaction.emoji"
        class="reaction flex items-center gap-1.5 h-6.5 px-2.5 rounded-full text-[13px] bg-secondary/75 shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_65%,transparent)] transition-colors hover:bg-accent hover:shadow-[inset_0_0_0_1px_var(--border)] disabled:opacity-100 [&_.inline-custom-emoji]:size-4.5 [&.is-reacted]:[background:linear-gradient(90deg,color-mix(in_srgb,var(--accent)_24%,var(--raised)),color-mix(in_srgb,var(--accent)_10%,var(--raised)))] [&.is-reacted]:shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--accent)_45%,transparent)] [&.is-reacted_.reaction-count]:text-primary [@media(pointer:coarse)]:min-h-[44px] [@media(pointer:coarse)]:min-w-[44px]"
        :class="{ 'is-reacted': reaction.memberIds.includes('you') }"
        :aria-label="
          $t(
            'workspaceUi.message.reactionCount',
            { emoji: reaction.emoji, count: $n(reaction.memberIds.length) },
            reaction.memberIds.length,
          )
        "
        :aria-pressed="reactionSupported ? reaction.memberIds.includes('you') : undefined"
        :disabled="reactionSupported ? !!reactionReason : undefined"
        :title="
          reactionReason ?? reaction.memberIds.map((id) => chat.member(id)?.displayName).join(', ')
        "
        @click="reactionSupported && chat.react(message, reaction.emoji)"
      >
        <MessageContent :text="reaction.emoji" /><span
          class="reaction-count text-[12px] font-[600] text-muted-foreground tabular-nums"
          >{{ $n(reaction.memberIds.length) }}</span
        >
      </component>
    </div>
    <div
      v-if="message.delivery === 'failed'"
      class="delivery-error flex flex-wrap items-center gap-x-2 gap-y-1 pt-1 text-[13px] text-destructive"
    >
      <span>{{ $t('workspaceUi.message.notDelivered') }}</span>
      <UiButton
        variant="secondary"
        size="sm"
        @click="chat.retry(message)"
      >
        {{ $t('workspaceUi.message.retry') }}
      </UiButton>
      <UiButton
        variant="secondary"
        size="sm"
        :title="$t('workspaceUi.message.discardHelp')"
        @click="discard"
      >
        {{ $t('workspaceUi.message.discard') }}
      </UiButton>
    </div>
  </div>
</template>
