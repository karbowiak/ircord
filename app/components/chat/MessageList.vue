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
const stickToBottom = ref(true)
let resizeObserver: ResizeObserver | null = null
let delayedScrollTimer: ReturnType<typeof setTimeout> | null = null

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

function isNearBottom(): boolean {
  if (!listEl.value) return true
  const el = listEl.value
  const distance = el.scrollHeight - (el.scrollTop + el.clientHeight)
  return distance <= 72
}

function handleScroll() {
  stickToBottom.value = isNearBottom()
}

function scheduleBottomSnap() {
  if (delayedScrollTimer) {
    clearTimeout(delayedScrollTimer)
    delayedScrollTimer = null
  }

  nextTick(() => {
    requestAnimationFrame(() => {
      if (stickToBottom.value) {
        scrollToBottom()
      }

      delayedScrollTimer = setTimeout(() => {
        if (stickToBottom.value) {
          scrollToBottom()
        }
      }, 220)
    })
  })
}

function handleMediaLoad() {
  if (!stickToBottom.value) return
  scrollToBottom()
}

onMounted(() => {
  if (!listEl.value) return

  listEl.value.addEventListener('scroll', handleScroll, { passive: true })
  listEl.value.addEventListener('load', handleMediaLoad, true)

  resizeObserver = new ResizeObserver(() => {
    if (stickToBottom.value) {
      scrollToBottom()
    }
  })
  resizeObserver.observe(listEl.value)

  scheduleBottomSnap()
})

onBeforeUnmount(() => {
  if (delayedScrollTimer) {
    clearTimeout(delayedScrollTimer)
    delayedScrollTimer = null
  }

  if (listEl.value) {
    listEl.value.removeEventListener('scroll', handleScroll)
    listEl.value.removeEventListener('load', handleMediaLoad, true)
  }

  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }
})

watch(
  () => {
    const list = appStore.messagesList
    const lastId = list.length ? list[list.length - 1].id : ''
    return [appStore.activeChannelId, list.length, lastId]
  },
  (newValue, previousValue) => {
    if (!Array.isArray(newValue)) return
    const channelId = newValue[0]
    const previousChannelId = Array.isArray(previousValue) ? previousValue[0] : undefined
    if (channelId !== previousChannelId) {
      stickToBottom.value = true
    }
    scheduleBottomSnap()
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
