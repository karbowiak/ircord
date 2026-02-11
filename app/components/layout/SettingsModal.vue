<template>
  <AppModal :open="isOpen" aria-label="Settings" size="lg" @close="emit('close')">
    <div class="settings-modal">
      <div class="modal-header">
        <div>
          <h2 class="modal-title">Settings</h2>
          <p class="modal-subtitle">Customize client behavior and profile preferences.</p>
        </div>
        <button type="button" class="close" @click="emit('close')">x</button>
      </div>

      <div class="sidebar">
        <div class="tab-list">
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

        <button
          type="button"
          class="tab about-tab"
          :class="{ active: activeTab === 'about' }"
          @click="activeTab = 'about'"
        >
          About
        </button>
      </div>

      <div class="content">
        <div class="header-row">
          <h3 class="title">{{ activeTabLabel }}</h3>
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

        <div v-else-if="activeTab === 'about'" class="section about-section">
          <div class="tbi">
            <div class="tbi-title">IRCord</div>
            <div class="tbi-desc">Discord-style IRC frontend prototype with mocked data and no live IRC connection yet.</div>
          </div>

          <div class="about-card">
            <div class="setting-name">Source Code</div>
            <a href="https://github.com/karbowiak/ircord" target="_blank" rel="noopener noreferrer" class="about-link">
              github.com/karbowiak/ircord
            </a>
          </div>

          <div class="about-card">
            <div class="setting-name">Core Stack</div>
            <ul class="about-list">
              <li>Nuxt 4 + Vue 3</li>
              <li>Pinia state management</li>
              <li>Nitro server routes for metadata/media resolving</li>
              <li>freezeframe hover animation fallback</li>
              <li>Klipy API for GIF search/picker data</li>
            </ul>
          </div>

          <div class="about-card">
            <div class="setting-name">Current Scope</div>
            <ul class="about-list">
              <li>All chat/server data is mock-driven in the client</li>
              <li>No live IRC backend connection is created yet</li>
              <li>Settings are persisted locally and prepared for future import/export</li>
            </ul>
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
  </AppModal>
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

type SettingsTabId = (typeof tabs)[number]['id'] | 'about'

const activeTab = ref<SettingsTabId>('media')
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
  if (activeTab.value === 'about') return 'About'
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
.settings-modal {
  width: 100%;
  height: min(620px, 82vh);
  overflow: hidden;
  display: grid;
  grid-template-rows: auto 1fr;
  grid-template-columns: 220px 1fr;
  gap: 12px;
  align-items: stretch;
}

.modal-header {
  grid-column: 1 / -1;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 10px;
}

.modal-title {
  margin: 0;
  font-family: var(--font-mono);
  color: var(--text-primary);
  font-size: 18px;
}

.modal-subtitle {
  margin: 6px 0 0;
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.4;
}

.sidebar {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.14);
  border-right: 1px solid rgba(255, 255, 255, 0.08);
  padding: 14px;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.tab-list {
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

.about-tab {
  margin-top: auto;
}

.content {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.14);
  padding: 14px;
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
  font-size: 16px;
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

.about-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.about-card {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 10px;
  padding: 12px;
  background: rgba(0, 0, 0, 0.13);
}

.about-link {
  display: inline-block;
  margin-top: 6px;
  color: #c9d5ff;
  font-family: var(--font-mono);
  font-size: 12px;
  text-decoration: none;
}

.about-link:hover {
  text-decoration: underline;
}

.about-list {
  margin: 8px 0 0 18px;
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
    grid-template-rows: auto auto 1fr;
    height: min(88vh, 700px);
  }

  .sidebar {
    border-right: 0;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
