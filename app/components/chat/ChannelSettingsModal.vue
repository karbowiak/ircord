<template>
  <AppModal :open="isOpen" aria-label="Channel settings" size="xl" @close="$emit('close')">
    <div class="modal">
      <div class="header">
        <h2 class="title">Channel Settings</h2>
        <button type="button" class="close" @click="$emit('close')">x</button>
      </div>

      <div v-if="channel && 'topic' in channel" class="settings-layout">
        <section class="main-column">
          <div class="meta-grid">
            <div class="meta-item">
              <div class="label">Channel</div>
              <div class="value">#{{ channel.name }}</div>
            </div>
            <div class="meta-item">
              <div class="label">Modes</div>
              <div class="value">{{ channel.modes || '(none)' }}</div>
            </div>
          </div>

          <section class="panel">
            <div class="panel-head">
              <h3>Topic</h3>
              <span class="hint">{{ appStore.canEditChannelTopic(channel.id) ? 'You can edit topic' : 'Topic change requires operator privileges' }}</span>
            </div>
            <p class="topic-preview">{{ appStore.getChannelTopic(channel.id) || 'No topic set' }}</p>
            <div class="editor-row">
              <input
                v-model="topicDraft"
                type="text"
                class="text-input"
                placeholder="Set channel topic"
                :disabled="topicSaving || !appStore.canEditChannelTopic(channel.id)"
                @keydown.enter.prevent="saveTopic"
              >
              <button
                type="button"
                class="action"
                :disabled="topicSaving || !appStore.canEditChannelTopic(channel.id)"
                @click="saveTopic"
              >
                {{ topicSaving ? 'Saving...' : 'Save Topic' }}
              </button>
            </div>
          </section>

          <section class="panel">
            <div class="panel-head">
              <h3>Bans</h3>
              <span class="hint">Add masks ahead of time</span>
            </div>
            <div class="editor-row">
              <input
                v-model="banMaskDraft"
                type="text"
                class="text-input"
                placeholder="*!*@example.net or badnick!*@*"
                :disabled="banSaving || !appStore.canManageChannelBans(channel.id)"
                @keydown.enter.prevent="addBanMask"
              >
              <button
                type="button"
                class="action"
                :disabled="banSaving || !appStore.canManageChannelBans(channel.id)"
                @click="addBanMask"
              >
                {{ banSaving ? 'Adding...' : 'Add Ban' }}
              </button>
            </div>

            <div class="bans-list">
              <template v-if="settings?.bans?.length">
                <div v-for="ban in settings.bans" :key="`${ban.mask}-${ban.setBy}-${ban.setAt}`" class="ban-entry">
                  <span class="mask">{{ ban.mask }}</span>
                  <span class="meta">set by {{ ban.setBy || 'unknown' }}{{ ban.setAt ? ` at ${formatBanTime(ban.setAt)}` : '' }}</span>
                </div>
              </template>
              <div v-else class="empty">No bans</div>
            </div>
          </section>

          <section class="panel">
            <div class="panel-head">
              <h3>Modes</h3>
              <span class="hint">Apply raw mode string (example: +nt-k key)</span>
            </div>
            <div class="editor-row">
              <input
                v-model="modeDraft"
                type="text"
                class="text-input"
                placeholder="+nt"
                :disabled="modeSaving || !appStore.canManageChannelBans(channel.id)"
                @keydown.enter.prevent="saveModes"
              >
              <button
                type="button"
                class="action"
                :disabled="modeSaving || !appStore.canManageChannelBans(channel.id)"
                @click="saveModes"
              >
                {{ modeSaving ? 'Saving...' : 'Set Modes' }}
              </button>
            </div>
          </section>

          <section class="panel">
            <div class="panel-head">
              <h3>Rename Channel</h3>
              <span class="hint">Requires operator privileges</span>
            </div>
            <div class="editor-row">
              <input
                v-model="renameDraft"
                type="text"
                class="text-input"
                placeholder="#new-channel-name"
                :disabled="renameSaving || !appStore.canManageChannelBans(channel.id)"
                @keydown.enter.prevent="saveRename"
              >
              <button
                type="button"
                class="action"
                :disabled="renameSaving || !appStore.canManageChannelBans(channel.id)"
                @click="saveRename"
              >
                {{ renameSaving ? 'Renaming...' : 'Rename' }}
              </button>
            </div>
          </section>

          <div class="actions">
            <button type="button" class="refresh" :disabled="Boolean(settings?.loading)" @click="refresh">
              {{ settings?.loading ? 'Refreshing...' : 'Refresh' }}
            </button>
            <span v-if="settings?.error" class="error">{{ settings.error }}</span>
            <span v-else-if="localError" class="error">{{ localError }}</span>
            <span v-else-if="localSuccess" class="ok">{{ localSuccess }}</span>
          </div>
        </section>

        <aside class="service-column panel">
          <div class="panel-head">
            <h3>Service Activity</h3>
            <span class="hint">HistServ/ChanServ</span>
          </div>
          <div class="service-log">
            <template v-if="serviceMessages.length">
              <div v-for="msg in serviceMessages" :key="msg.id" class="service-line">
                <span class="service-time">{{ msg.timestamp }}</span>
                <span class="service-author">{{ msg.author }}</span>
                <span class="service-text">{{ msg.content }}</span>
              </div>
            </template>
            <div v-else class="empty">No service messages yet</div>
          </div>
        </aside>
      </div>

      <div v-else class="settings-layout">
        <div class="empty">No active channel selected.</div>
      </div>
    </div>
  </AppModal>
</template>

<script setup lang="ts">
import { nextTick } from 'vue'

defineProps<{
  isOpen: boolean
}>()

defineEmits<{
  (e: 'close'): void
}>()

const appStore = useAppStore()

const channel = computed(() => appStore.activeChannel)
const settings = computed(() => appStore.activeChannelSettings)
const serviceMessages = computed(() => appStore.activeChannelServiceActivity.slice().reverse())

const topicDraft = ref('')
const banMaskDraft = ref('')
const modeDraft = ref('')
const renameDraft = ref('')
const topicSaving = ref(false)
const banSaving = ref(false)
const modeSaving = ref(false)
const renameSaving = ref(false)
const localError = ref<string | null>(null)
const localSuccess = ref<string | null>(null)

watch(
  () => channel.value?.id,
  () => {
    const active = channel.value
    if (!active || !('topic' in active)) {
      topicDraft.value = ''
      modeDraft.value = ''
      renameDraft.value = ''
      return
    }
    topicDraft.value = appStore.getChannelTopic(active.id)
    modeDraft.value = active.modes || ''
    renameDraft.value = `#${active.name}`
    localError.value = null
    localSuccess.value = null
  },
  { immediate: true },
)

watch(
  () => appStore.getChannelTopic(channel.value && 'topic' in channel.value ? channel.value.id : ''),
  async () => {
    const active = channel.value
    if (!active || !('topic' in active)) return
    await nextTick()
    if (!topicSaving.value) {
      topicDraft.value = appStore.getChannelTopic(active.id)
    }
  },
)

async function refresh() {
  const active = channel.value
  if (!active || !('topic' in active)) return
  localError.value = null
  localSuccess.value = null
  await appStore.refreshChannelSettings(active.id)
}

async function saveTopic() {
  const active = channel.value
  if (!active || !('topic' in active)) return

  localError.value = null
  localSuccess.value = null
  topicSaving.value = true
  const changed = await appStore.setChannelTopic(active.id, topicDraft.value)
  topicSaving.value = false

  if (!changed) {
    localError.value = 'Unable to set topic (check channel permissions).'
    return
  }

  localSuccess.value = 'Topic updated.'
  await appStore.refreshChannelSettings(active.id)
}

async function addBanMask() {
  const active = channel.value
  if (!active || !('topic' in active)) return
  const mask = banMaskDraft.value.trim()
  if (!mask) return

  localError.value = null
  localSuccess.value = null
  banSaving.value = true
  const added = await appStore.addChannelBan(active.id, mask)
  banSaving.value = false

  if (!added) {
    localError.value = 'Unable to add ban mask (check operator permissions or mask syntax).'
    return
  }

  banMaskDraft.value = ''
  localSuccess.value = 'Ban added.'
}

async function saveModes() {
  const active = channel.value
  if (!active || !('topic' in active)) return
  const spec = modeDraft.value.trim()
  if (!spec) return

  localError.value = null
  localSuccess.value = null
  modeSaving.value = true
  const changed = await appStore.setChannelModes(active.id, spec)
  modeSaving.value = false

  if (!changed) {
    localError.value = 'Unable to set channel modes.'
    return
  }

  localSuccess.value = 'Channel modes updated.'
}

async function saveRename() {
  const active = channel.value
  if (!active || !('topic' in active)) return
  const nextName = renameDraft.value.trim()
  if (!nextName) return

  localError.value = null
  localSuccess.value = null
  renameSaving.value = true
  const renamed = await appStore.renameChannel(active.id, nextName)
  renameSaving.value = false

  if (!renamed) {
    localError.value = 'Unable to rename channel.'
    return
  }

  localSuccess.value = 'Channel renamed.'
}

function formatBanTime(value: string): string {
  try {
    return new Date(value).toLocaleString()
  } catch {
    return value
  }
}
</script>

<style scoped>
.modal {
  width: 100%;
  max-height: min(840px, 88vh);
  overflow-y: auto;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.title {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 18px;
  color: var(--text-primary);
}

.close {
  border: 0;
  border-radius: 8px;
  padding: 7px 10px;
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-muted);
  font-family: var(--font-mono);
  cursor: pointer;
}

.close:hover {
  background: rgba(255, 255, 255, 0.14);
  color: var(--text-primary);
}

.settings-layout {
  margin-top: 14px;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 460px;
  gap: 12px;
  align-items: start;
}

.main-column {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.meta-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.meta-item,
.panel {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.14);
  padding: 12px;
}

.panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.panel-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
}

.panel-head h3 {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--text-primary);
}

.hint {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-faint);
}

.label {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-muted);
  text-transform: uppercase;
}

.value {
  margin-top: 5px;
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--text-body);
}

.topic-preview {
  margin: 0;
  color: var(--text-body);
  font-family: var(--font-mono);
  font-size: 13px;
  word-break: break-word;
}

.editor-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px;
}

.text-input {
  border: 1px solid rgba(255, 255, 255, 0.14);
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.2);
  color: var(--text-body);
  padding: 8px 10px;
  font-family: var(--font-mono);
  font-size: 13px;
}

.text-input:disabled,
.action:disabled,
.refresh:disabled {
  opacity: 0.6;
  cursor: default;
}

.action,
.refresh {
  border: 1px solid rgba(255, 255, 255, 0.16);
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.22);
  color: var(--text-body);
  font-family: var(--font-mono);
  font-size: 12px;
  padding: 7px 10px;
  cursor: pointer;
}

.bans-list,
.service-log {
  display: flex;
  flex-direction: column;
  gap: 8px;
  overflow-y: auto;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  padding: 10px;
  background: rgba(0, 0, 0, 0.16);
}

.bans-list {
  max-height: 260px;
}

.service-log {
  min-height: 520px;
  max-height: 640px;
}

.service-column {
  height: fit-content;
  min-height: 0;
}

.ban-entry {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.mask {
  color: var(--text-primary);
  font-family: var(--font-mono);
}

.meta {
  font-size: 11px;
  color: var(--text-faint);
}

.service-line {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.service-time,
.service-author,
.service-text {
  font-family: var(--font-mono);
  font-size: 12px;
}

.service-time {
  color: var(--text-faint);
}

.service-author {
  color: var(--text-muted);
}

.service-text {
  color: var(--text-body);
  word-break: break-word;
}

.actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.error {
  font-family: var(--font-mono);
  font-size: 11px;
  color: #ff7d7d;
}

.ok {
  font-family: var(--font-mono);
  font-size: 11px;
  color: #8de8b3;
}

.empty {
  color: var(--text-faint);
  font-family: var(--font-mono);
}

@media (max-width: 900px) {
  .settings-layout {
    grid-template-columns: 1fr;
  }

  .service-column {
    order: 3;
  }
}

@media (max-width: 680px) {
  .meta-grid,
  .editor-row {
    grid-template-columns: 1fr;
  }
}
</style>
