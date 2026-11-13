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
  const { conversationId, text } = props.message
  chat.discard(props.message)
  const typed = chat.drafts[conversationId]
  const restored = ircToMarkdown(text)
  // Text typed since stays as it is; the discarded text follows it.
  if (typed) chat.drafts[conversationId] = `${typed}\n${restored}`.slice(0, 4000)
  else if (chat.restoreDraft(conversationId, restored) !== 'restored')
    chat.notify($t('workspaceUi.message.discardLost'), { error: true })
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
        class="reaction flex items-center gap-1.5 h-6 px-[7px] rounded-sm text-[14px] bg-secondary shadow-[inset_0_0_0_1px_transparent] hover:shadow-[inset_0_0_0_1px_var(--border)] disabled:opacity-100 [&_.inline-custom-emoji]:size-4.5 [&.is-reacted]:bg-selection [&.is-reacted]:shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--accent)_55%,transparent)] [&.is-reacted_.reaction-count]:text-primary [@media(pointer:coarse)]:min-h-[44px] [@media(pointer:coarse)]:min-w-[44px]"
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
