<template>
  <div ref="listEl" class="message-list">
    <Message
      v-for="message in appStore.messagesList"
      :key="message.id"
      :message-id="message.id"
      :author="message.author"
      :content="message.content"
      :timestamp="message.timestamp"
      :reply-to="message.replyTo"
      :is-own-message="message.author.id === appStore.currentUser.id"
      :reactions="appStore.getMessageReactions(message.id)"
      @reply="appStore.setReplyTarget"
      @edit="onEditMessage"
      @react="onReactMessage"
    />
  </div>
</template>

<script setup lang="ts">
import Message from '~/components/chat/Message.vue'

const appStore = useAppStore()
const listEl = ref<HTMLDivElement>()

function onEditMessage(payload: { messageId: string, content: string }) {
  appStore.editMockMessage(payload.messageId, payload.content)
}

function onReactMessage(payload: { messageId: string, emoji: string }) {
  appStore.toggleMessageReaction(payload.messageId, payload.emoji)
}

function scrollToBottom() {
  if (!listEl.value) return
  listEl.value.scrollTop = listEl.value.scrollHeight
}

watch(
  () => [appStore.activeChannelId, appStore.messagesList.length],
  async () => {
    await nextTick()
    scrollToBottom()
  },
  { immediate: true }
)
</script>

<style scoped>
.message-list {
  flex: 1;
  overflow-y: auto;
  padding: 16px 0;
}
</style>
