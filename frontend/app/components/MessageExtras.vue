<script setup lang="ts">
import type { Message } from '../../shared/types/chat'
/** Link content, file downloads, reactions and delivery state under a message. */
const props = defineProps<{ message: Message }>()
const chat = useChatStore()
const reactionSupported = computed(() => chat.messageActionSupported(props.message, 'reactions'))
const reactionReason = computed(() => chat.messageActionReason(props.message, 'reactions'))
</script>
<template>
  <div class="message-extras">
    <MessageLinks :message="message" />
    <MessageAttachment v-if="message.attachment" :attachment="message.attachment" />
    <div
      v-if="message.reactions.length"
      class="reactions"
    >
      <component
        v-for="reaction in message.reactions"
        :is="reactionSupported ? 'button' : 'span'"
        :key="reaction.emoji"
        class="reaction"
        :class="{ 'is-reacted': reaction.memberIds.includes('you') }"
        :aria-label="`${reaction.emoji}, ${reaction.memberIds.length} reactions`"
        :aria-pressed="reactionSupported ? reaction.memberIds.includes('you') : undefined"
        :disabled="reactionSupported ? !!reactionReason : undefined"
        :title="
          reactionReason ?? reaction.memberIds.map((id) => chat.member(id)?.displayName).join(', ')
        "
        @click="reactionSupported && chat.react(message, reaction.emoji)"
      >
        <MessageContent :text="reaction.emoji" /><span class="reaction-count">{{
          reaction.memberIds.length
        }}</span>
      </component>
    </div>
    <div
      v-if="message.delivery === 'failed'"
      class="delivery-error"
    >
      Not delivered. <button @click="chat.retry(message)">Retry</button> ·
      <button @click="chat.discard(message)">Discard</button>
    </div>
  </div>
</template>
