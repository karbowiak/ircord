<script setup lang="ts">
const props = withDefaults(defineProps<{ initialTab?: string }>(), { initialTab: 'appearance' })
const emit = defineEmits<{ close: []; addNetwork: []; editNetwork: [id: string] }>()
const tab = ref(props.initialTab)
const tabs = [
  { id: 'appearance', name: 'Appearance', icon: 'palette' },
  { id: 'profile', name: 'Profile', icon: 'user' },
  { id: 'networks', name: 'Networks', icon: 'globe' },
  { id: 'notifications', name: 'Notifications', icon: 'bell' },
  { id: 'emotes', name: 'Emotes', icon: 'smile' },
  { id: 'account', name: 'Account', icon: 'key' },
  { id: 'shortcuts', name: 'Shortcuts', icon: 'keyboard' },
  { id: 'about', name: 'About', icon: 'info' },
]
const current = computed(() => tabs.find((t) => t.id === tab.value) ?? tabs[0]!)
const auth = useAuthStore()
</script>
<template>
  <AppDialog
    title="Settings"
    panel
    @close="emit('close')"
    ><div class="settings-layout">
      <nav
        class="settings-nav"
        aria-label="Settings"
      >
        <button
          v-for="item in tabs"
          :key="item.id"
          :class="{ active: tab === item.id }"
          @click="tab = item.id"
        >
          <AppIcon
            :name="item.icon"
            :size="17"
          />{{ item.name }}
        </button>
        <button
          class="settings-logout"
          @click="auth.confirmLogout()"
        >
          <AppIcon
            name="logout"
            :size="17"
          />Log out
        </button>
      </nav>
      <div class="settings-content">
        <h3 class="settings-title">{{ current.name }}</h3>
        <AppearanceSettings v-if="tab === 'appearance'" />
        <ProfileSettings v-else-if="tab === 'profile'" />
        <NetworkSettingsList
          v-else-if="tab === 'networks'"
          @add-network="emit('addNetwork')"
          @edit-network="emit('editNetwork', $event)"
        />
        <NotificationSettings v-else-if="tab === 'notifications'" />
        <EmoteSettings v-else-if="tab === 'emotes'" />
        <AccountSettings v-else-if="tab === 'account'" />
        <ShortcutList v-else-if="tab === 'shortcuts'" />
        <div
          v-else
          class="about"
        >
          <BrandMark
            class="about-brand"
            :size="52"
          />
          <p class="settings-description">
            IRCord is a persistent, multi-network IRC client for the web. This build runs against a
            demo server: the people and messages are examples, and Echo repeats what you send.
          </p>
          <ReleaseHistory />
        </div>
      </div></div
  ></AppDialog>
</template>
