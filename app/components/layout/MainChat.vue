<template>
  <div class="main-chat">
    <div v-if="appStore.activeChannel" class="chat-container">
      <div class="header">
        <div class="channel-info">
          <span class="hash">{{ channelPrefix }}</span>
          <span class="channel-name">{{ channelName }}</span>

          <template v-if="isChannelTopicEditable">
            <span class="topic-divider">|</span>

            <form v-if="isEditingTopic" class="topic-edit" @submit.prevent="submitTopicEdit">
              <input
                ref="topicInputEl"
                v-model="topicDraft"
                class="topic-input"
                type="text"
                maxlength="240"
                placeholder="Set channel topic"
                @keydown.esc.prevent="cancelTopicEdit"
                @blur="submitTopicEdit"
              >
            </form>

            <button v-else type="button" class="topic topic-button" @click="startTopicEdit">
              {{ channelTopic || 'Set channel topic' }}
            </button>
          </template>

          <template v-else-if="channelTopic">
            <span class="topic-divider">|</span>
            <span class="topic">{{ channelTopic }}</span>
          </template>
        </div>
      </div>
      <div class="chat-body">
        <div class="messages-area">
          <MessageList />
          <MessageInput />
        </div>
        <UserList v-if="showUserList" />
      </div>
    </div>
    <div v-else-if="appStore.activeServer" class="status-panel">
      <div class="status-title">Server Status - {{ appStore.activeServer.name }}</div>
      <div class="status-line muted">Mocked live diagnostics (no real IRC connection)</div>

      <div class="metrics-grid">
        <div class="metric-card">
          <div class="metric-label">Connection</div>
          <div class="metric-value">Established</div>
          <div class="metric-sub">TLSv1.3 + SASL PLAIN</div>
        </div>
        <div class="metric-card">
          <div class="metric-label">Round Trip</div>
          <div class="metric-value">{{ mockLatencyMs }} ms</div>
          <div class="metric-sub">from {{ appStore.currentUser.username }} client</div>
        </div>
        <div class="metric-card">
          <div class="metric-label">Nickname</div>
          <div class="metric-value">{{ appStore.currentUser.username }}</div>
          <div class="metric-sub">user mode +i</div>
        </div>
      </div>

      <div class="status-log">
        <div v-for="line in serverLogLines" :key="line" class="log-line">
          {{ line }}
        </div>
      </div>
    </div>
    <div v-else class="placeholder">
      <div class="placeholder-icon">IRC</div>
      <div class="placeholder-text">Select a channel to start chatting</div>
      <div class="placeholder-hint">Pick a network from the left, then join a channel</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import MessageList from '~/components/chat/MessageList.vue'
import MessageInput from '~/components/chat/MessageInput.vue'
import UserList from '~/components/chat/UserList.vue'
import type { Channel } from '~/types'

const appStore = useAppStore()
const topicInputEl = ref<HTMLInputElement>()
const isEditingTopic = ref(false)
const topicDraft = ref('')

const showUserList = computed(() => {
  return appStore.activeChannel && 'members' in appStore.activeChannel
})

const channelName = computed(() => {
  const ch = appStore.activeChannel
  if (!ch) return ''
  if ('name' in ch) return (ch as Channel).name
  return ch.recipient.username
})

const channelPrefix = computed(() => {
  const ch = appStore.activeChannel
  if (!ch) return '#'
  return 'name' in ch ? '#' : '@'
})

const channelTopic = computed(() => {
  const ch = appStore.activeChannel
  if (!ch) return ''
  if ('topic' in ch) return appStore.getChannelTopic(ch.id)
  return ''
})

const isChannelTopicEditable = computed(() => {
  const ch = appStore.activeChannel
  return Boolean(ch && 'topic' in ch)
})

function startTopicEdit() {
  const ch = appStore.activeChannel
  if (!ch || !('topic' in ch)) return

  topicDraft.value = appStore.getChannelTopic(ch.id)
  isEditingTopic.value = true
  nextTick(() => {
    topicInputEl.value?.focus()
    topicInputEl.value?.select()
  })
}

function submitTopicEdit() {
  const ch = appStore.activeChannel
  if (!ch || !('topic' in ch)) return

  appStore.setChannelTopic(ch.id, topicDraft.value)
  isEditingTopic.value = false
}

function cancelTopicEdit() {
  isEditingTopic.value = false
  topicDraft.value = ''
}

watch(
  () => appStore.activeChannelId,
  () => {
    isEditingTopic.value = false
    topicDraft.value = ''
  },
)

const mockLatencyMs = computed(() => {
  const base = appStore.activeServer?.id.length || 10
  return 24 + base * 3
})

const serverLogLines = computed(() => {
  const serverName = appStore.activeServer?.name || 'server'
  return [
    `*** Looking up ${serverName.toLowerCase().replace(/\s+/g, '')}.irc.local`,
    '*** Found address 198.51.100.42, connecting on port 6697',
    '*** TLS handshake complete, certificate verified',
    `*** Logged in as ${appStore.currentUser.username} (mode +i)`,
    '*** Message of the day loaded (12 lines)',
    `*** Joined ${appStore.channelsList.length} available channels (mock listing)`,
  ]
})
</script>

<style scoped>
.main-chat {
  flex: 1;
  height: 100vh;
  background-color: var(--bg-primary);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.chat-container {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.header {
  height: 48px;
  padding: 0 16px;
  display: flex;
  align-items: center;
  border-bottom: 1px solid var(--border-dark);
  box-shadow: 0 1px 0 rgba(0, 0, 0, 0.2);
  flex-shrink: 0;
}

.channel-info {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 8px;
}

.hash {
  font-family: var(--font-mono);
  font-size: 22px;
  color: var(--text-muted);
  flex-shrink: 0;
}

.channel-name {
  font-family: var(--font-mono);
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
  flex-shrink: 0;
}

.topic-divider {
  color: var(--text-faint);
  flex-shrink: 0;
}

.topic {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.topic-button {
  border: 0;
  background: transparent;
  cursor: pointer;
  padding: 0;
  text-align: left;
}

.topic-button:hover {
  color: var(--text-body);
}

.topic-edit {
  min-width: 0;
  flex: 1;
}

.topic-input {
  width: min(100%, 520px);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 6px;
  background: rgba(0, 0, 0, 0.2);
  color: var(--text-body);
  font-family: var(--font-mono);
  font-size: 12px;
  padding: 4px 8px;
  outline: none;
}

.topic-input:focus {
  border-color: rgba(88, 101, 242, 0.65);
}

.chat-body {
  flex: 1;
  display: flex;
  overflow: hidden;
}

.messages-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.placeholder {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  gap: 12px;
}

.placeholder-icon {
  font-family: var(--font-mono);
  font-size: 48px;
  font-weight: 700;
  color: var(--text-faint);
  opacity: 0.3;
  letter-spacing: 0.1em;
}

.placeholder-text {
  font-family: var(--font-mono);
  font-size: 16px;
  color: var(--text-muted);
}

.placeholder-hint {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--text-faint);
}

.status-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 24px;
  gap: 12px;
  color: var(--text-body);
}

.status-title {
  font-family: var(--font-mono);
  font-size: 24px;
  font-weight: 700;
}

.status-line {
  font-family: var(--font-mono);
  font-size: 14px;
}

.status-line.muted {
  color: var(--text-muted);
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  gap: 10px;
}

.metric-card {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 10px;
  background-color: rgba(0, 0, 0, 0.16);
  padding: 10px;
}

.metric-label {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-faint);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.metric-value {
  margin-top: 4px;
  font-family: var(--font-mono);
  font-size: 16px;
  color: var(--text-primary);
}

.metric-sub {
  margin-top: 4px;
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-muted);
}

.status-log {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 10px;
  background-color: rgba(0, 0, 0, 0.14);
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.log-line {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-body);
}
</style>
