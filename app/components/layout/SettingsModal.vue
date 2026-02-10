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
  { id: 'profile', label: 'Profile' },
  { id: 'notifications', label: 'Notifications' },
  { id: 'integrations', label: 'Integrations' },
] as const

const activeTab = ref<(typeof tabs)[number]['id']>('media')

const activeTabLabel = computed(() => {
  return tabs.find(tab => tab.id === activeTab.value)?.label || 'Settings'
})

function onGifToggle(event: Event) {
  const target = event.target as HTMLInputElement
  appStore.setGifAutoplay(target.checked)
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
