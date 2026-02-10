<template>
  <div ref="listEl" class="message-list">
    <Message
      v-for="message in appStore.messagesList"
      :key="message.id"
      :author="message.author"
      :content="message.content"
      :timestamp="message.timestamp"
    />
  </div>
</template>

<script setup lang="ts">
import Message from '~/components/chat/Message.vue'

const appStore = useAppStore()
const listEl = ref<HTMLDivElement>()

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
