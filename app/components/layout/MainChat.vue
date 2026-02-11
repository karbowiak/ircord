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

          <button
            v-if="appStore.activeChannel && 'topic' in appStore.activeChannel"
            type="button"
            class="channel-settings-button"
            @click="openChannelSettings"
          >
            Settings
          </button>
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
      <div class="status-title">Server Status — {{ appStore.activeServer.name }}</div>

      <template v-if="status">
        <div class="metrics-grid">
          <div class="metric-card">
            <div class="metric-label">Connection</div>
            <div class="metric-value connected">Connected</div>
            <div class="metric-sub">since {{ formatConnectedAt(status.connectedAt) }}</div>
          </div>
          <div class="metric-card">
            <div class="metric-label">Round Trip</div>
            <div class="metric-value">{{ status.latencyMs !== null ? `${status.latencyMs} ms` : 'measuring…' }}</div>
            <div class="metric-sub">PING/PONG latency</div>
          </div>
          <div class="metric-card">
            <div class="metric-label">Nickname</div>
            <div class="metric-value">{{ status.nick }}</div>
            <div class="metric-sub">{{ status.userModes ? `mode ${status.userModes}` : 'no user modes' }}</div>
          </div>
        </div>

        <div v-if="status.capabilities.length" class="caps-section">
          <div class="caps-label">Negotiated Capabilities</div>
          <div class="caps-list">
            <span v-for="cap in status.capabilities" :key="cap" class="cap-badge">{{ cap }}</span>
          </div>
        </div>

        <div class="log-section">
          <div class="log-header">Raw IRC Log</div>
          <div ref="logContainerEl" class="status-log">
            <div v-for="(entry, i) in status.rawLog" :key="i" class="log-line" :class="entry.direction">
              <span class="log-time">{{ entry.time }}</span>
              <span class="log-dir">{{ entry.direction === 'out' ? '→' : '←' }}</span>
              <span class="log-text">{{ entry.line }}</span>
            </div>
          </div>
        </div>
      </template>

      <div v-else class="status-line muted">Not connected</div>
    </div>
    <div v-else class="placeholder">
      <div class="placeholder-icon">IRC</div>
      <div class="placeholder-text">Select a channel to start chatting</div>
      <div class="placeholder-hint">Pick a network from the left, then join a channel</div>
    </div>

    <ChannelSettingsModal
      :is-open="isChannelSettingsOpen"
      @close="isChannelSettingsOpen = false"
    />
  </div>
</template>

<script setup lang="ts">
import MessageList from '~/components/chat/MessageList.vue'
import MessageInput from '~/components/chat/MessageInput.vue'
import UserList from '~/components/chat/UserList.vue'
import ChannelSettingsModal from '~/components/chat/ChannelSettingsModal.vue'
import type { Channel } from '~/types'

const appStore = useAppStore()
const topicInputEl = ref<HTMLInputElement>()
const isEditingTopic = ref(false)
const topicDraft = ref('')
const isChannelSettingsOpen = ref(false)

const showUserList = computed(() => {
  if (!appStore.activeChannel || !('members' in appStore.activeChannel)) return false
  return appStore.activeChannelParticipation?.joined !== false
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
  if (!ch || !('topic' in ch)) return false
  return appStore.canEditChannelTopic(ch.id)
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

async function submitTopicEdit() {
  const ch = appStore.activeChannel
  if (!ch || !('topic' in ch)) return

  await appStore.setChannelTopic(ch.id, topicDraft.value)
  isEditingTopic.value = false
}

async function openChannelSettings() {
  const ch = appStore.activeChannel
  if (!ch || !('topic' in ch)) return
  isChannelSettingsOpen.value = true
  await appStore.refreshChannelSettings(ch.id)
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
    isChannelSettingsOpen.value = false
  },
)

const status = computed(() => appStore.activeServerStatus)

const logContainerEl = ref<HTMLElement>()

function formatConnectedAt(iso: string | null): string {
  if (!iso) return 'unknown'
  try {
    return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  } catch {
    return iso
  }
}

watch(
  () => status.value?.rawLog.length,
  () => {
    nextTick(() => {
      const el = logContainerEl.value
      if (el) el.scrollTop = el.scrollHeight
    })
  },
  { immediate: true },
)

watch(
  () => appStore.activeChannelId,
  (channelId) => {
    if (channelId) return
    nextTick(() => {
      const el = logContainerEl.value
      if (el) el.scrollTop = el.scrollHeight
    })
  },
)
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

.channel-settings-button {
  margin-left: 4px;
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 6px;
  background: rgba(0, 0, 0, 0.16);
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: 11px;
  padding: 3px 8px;
  cursor: pointer;
}

.channel-settings-button:hover {
  color: var(--text-body);
  border-color: rgba(255, 255, 255, 0.24);
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
  gap: 16px;
  color: var(--text-body);
  overflow: hidden;
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

.metric-value.connected {
  color: var(--accent-green);
}

.metric-sub {
  margin-top: 4px;
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-muted);
}

.caps-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.caps-label {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-faint);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.caps-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.cap-badge {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-body);
  background-color: rgba(88, 101, 242, 0.15);
  border: 1px solid rgba(88, 101, 242, 0.25);
  border-radius: 4px;
  padding: 2px 8px;
}

.log-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-height: 0;
}

.log-header {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-faint);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  flex-shrink: 0;
}

.status-log {
  flex: 1;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 10px;
  background-color: rgba(0, 0, 0, 0.14);
  padding: 10px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.log-line {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-body);
  display: flex;
  gap: 8px;
  line-height: 1.6;
}

.log-line.out {
  color: var(--text-muted);
}

.log-time {
  color: var(--text-faint);
  flex-shrink: 0;
}

.log-dir {
  flex-shrink: 0;
  width: 12px;
  text-align: center;
}

.log-line.in .log-dir {
  color: var(--accent-green);
}

.log-line.out .log-dir {
  color: var(--accent-orange);
}

.log-text {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
