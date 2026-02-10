<template>
  <div v-if="isOpen" class="settings-overlay" @click.self="emit('close')">
    <div class="settings-modal" role="dialog" aria-label="Settings">
      <div class="sidebar">
        <button
          v-for="tab in tabs"
          :key="tab.id"
          type="button"
          class="tab"
          :class="{ active: activeTab === tab.id }"
          @click="activeTab = tab.id"
        >
          {{ tab.label }}
        </button>
      </div>

      <div class="content">
        <div class="header-row">
          <h2 class="title">{{ activeTabLabel }}</h2>
          <button type="button" class="close" @click="emit('close')">x</button>
        </div>

        <div v-if="activeTab === 'media'" class="section">
          <label class="setting-row">
            <span class="setting-text">
              <span class="setting-name">GIF autoplay</span>
              <span class="setting-desc">When disabled, GIFs animate only while hovering previews.</span>
            </span>
            <input
              type="checkbox"
              :checked="appStore.gifAutoplay"
              @change="onGifToggle"
            >
          </label>

          <div class="setting-row stat-row">
            <span class="setting-text">
              <span class="setting-name">GIF favorites</span>
              <span class="setting-desc">Stored locally as part of your client settings profile.</span>
            </span>
            <span class="stat-pill">{{ appStore.gifFavorites.length }}</span>
          </div>

          <div class="setting-row media-scale-row">
            <span class="setting-text">
              <span class="setting-name">Embedded GIF/video scale</span>
              <span class="setting-desc">Scales embedded animated media in chat from 10% to 100%.</span>
            </span>

            <div class="media-scale-control">
              <input
                type="range"
                min="10"
                max="100"
                step="1"
                :value="appStore.mediaScalePercent"
                @input="onMediaScaleInput"
              >
              <span class="stat-pill">{{ appStore.mediaScalePercent }}%</span>
            </div>
          </div>
        </div>

        <div v-else-if="activeTab === 'data'" class="section">
          <div class="tbi data-panel">
            <div class="tbi-title">Settings Data (Scaffold)</div>
            <div class="tbi-desc">
              This profile is client-side now and is shaped to support future backend sync or JSON import/export.
            </div>

            <div class="setting-row data-row">
              <span class="setting-text">
                <span class="setting-name">Export settings JSON</span>
                <span class="setting-desc">ToBeImplemented: download all user settings and GIF favorites.</span>
              </span>
              <button type="button" class="stub-btn" @click="showStubNotice('Export JSON stub ready')">Stub</button>
            </div>

            <div class="setting-row data-row">
              <span class="setting-text">
                <span class="setting-name">Import settings JSON</span>
                <span class="setting-desc">ToBeImplemented: merge uploaded profile into local client state.</span>
              </span>
              <button type="button" class="stub-btn" @click="showStubNotice('Import JSON stub ready')">Stub</button>
            </div>

            <pre class="schema-preview">{{ settingsSchemaPreview }}</pre>
            <div v-if="stubNotice" class="stub-notice">{{ stubNotice }}</div>
          </div>
        </div>

        <div v-else class="section tbi">
          <div class="tbi-title">ToBeImplemented</div>
          <div class="tbi-desc">
            This page is scaffolded for future IRC-compatible metadata features.
          </div>
          <ul class="tbi-list">
            <li>Profile cards and avatars (external API-backed)</li>
            <li>Per-network identity and user descriptions</li>
            <li>Notification routing and mute presets</li>
            <li>Message/theme preferences synchronization</li>
          </ul>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
interface Props {
  isOpen: boolean
}

defineProps<Props>()

const emit = defineEmits<{
  close: []
}>()

const appStore = useAppStore()

const tabs = [
  { id: 'media', label: 'Media' },
  { id: 'data', label: 'Data & Sync' },
  { id: 'profile', label: 'Profile' },
  { id: 'notifications', label: 'Notifications' },
  { id: 'integrations', label: 'Integrations' },
] as const

const activeTab = ref<(typeof tabs)[number]['id']>('media')
const stubNotice = ref('')

const settingsSchemaPreview = computed(() => {
  const sample = {
    version: 1,
    settings: {
      gifAutoplay: appStore.gifAutoplay,
      gifFavoritesCount: appStore.gifFavorites.length,
      mediaScalePercent: appStore.mediaScalePercent,
    },
  }
  return JSON.stringify(sample, null, 2)
})

const activeTabLabel = computed(() => {
  return tabs.find(tab => tab.id === activeTab.value)?.label || 'Settings'
})

function onGifToggle(event: Event) {
  const target = event.target as HTMLInputElement
  appStore.setGifAutoplay(target.checked)
}

function onMediaScaleInput(event: Event) {
  const target = event.target as HTMLInputElement
  appStore.setMediaScalePercent(Number(target.value))
}

function showStubNotice(message: string) {
  stubNotice.value = message
  setTimeout(() => {
    stubNotice.value = ''
  }, 1500)
}
</script>

<style scoped>
.settings-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.56);
  z-index: 90;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}

.settings-modal {
  width: min(860px, 100%);
  height: min(560px, 100%);
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  overflow: hidden;
  display: grid;
  grid-template-columns: 220px 1fr;
  background: linear-gradient(180deg, rgba(43, 46, 53, 0.98), rgba(37, 40, 47, 0.98));
  box-shadow: 0 26px 50px rgba(0, 0, 0, 0.44);
}

.sidebar {
  border-right: 1px solid rgba(255, 255, 255, 0.08);
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.tab {
  border: 0;
  border-radius: 8px;
  text-align: left;
  background: transparent;
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: 13px;
  padding: 8px 10px;
  cursor: pointer;
}

.tab:hover,
.tab.active {
  color: var(--text-primary);
  background: rgba(88, 101, 242, 0.22);
}

.content {
  padding: 18px;
  overflow-y: auto;
}

.header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
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

.section {
  margin-top: 18px;
}

.setting-row {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.12);
  padding: 12px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.stat-row {
  margin-top: 10px;
}

.media-scale-row {
  margin-top: 10px;
  align-items: flex-start;
}

.media-scale-control {
  display: flex;
  align-items: center;
  gap: 10px;
}

.media-scale-control input[type='range'] {
  width: 180px;
}

.stat-pill {
  font-family: var(--font-mono);
  font-size: 12px;
  color: #e6ebff;
  background: rgba(88, 101, 242, 0.3);
  border-radius: 999px;
  padding: 5px 10px;
}

.data-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.data-row {
  margin-top: 2px;
}

.stub-btn {
  border: 0;
  border-radius: 8px;
  padding: 7px 11px;
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-primary);
  font-family: var(--font-mono);
  font-size: 12px;
  cursor: pointer;
}

.stub-btn:hover {
  background: rgba(88, 101, 242, 0.24);
}

.schema-preview {
  margin: 0;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  padding: 10px;
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-body);
  background: rgba(0, 0, 0, 0.24);
  overflow-x: auto;
}

.stub-notice {
  font-family: var(--font-mono);
  font-size: 12px;
  color: #cfe0ff;
}

.setting-text {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.setting-name {
  font-family: var(--font-mono);
  font-size: 14px;
  color: var(--text-primary);
}

.setting-desc {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-muted);
}

.tbi {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 10px;
  padding: 14px;
  background: rgba(0, 0, 0, 0.13);
}

.tbi-title {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--accent-orange);
}

.tbi-desc {
  margin-top: 8px;
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-muted);
}

.tbi-list {
  margin: 10px 0 0 18px;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 5px;
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-body);
}

@media (max-width: 760px) {
  .settings-modal {
    grid-template-columns: 1fr;
    height: min(88vh, 640px);
  }

  .sidebar {
    border-right: 0;
    border-bottom: 1px solid rgba(255, 255, 255, 0.08);
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
