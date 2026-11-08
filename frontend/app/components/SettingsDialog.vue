<script setup lang="ts">
import {
  sectionName,
  searchSettings,
  settingsDestination,
  settingsSections,
  type SettingResult,
  type SettingsAccess,
  type SettingsSectionId,
} from '../../shared/settings-index'

/** `initialTab` is a section id or a setting id from shared/settings-index.ts. */
const props = withDefaults(defineProps<{ initialTab?: string }>(), { initialTab: 'appearance' })
const emit = defineEmits<{ close: []; addNetwork: []; editNetwork: [id: string] }>()
const auth = useAuthStore()
const media = useMediaStore()
const notifications = useNotificationStore()
const appearance = useAppearanceStore()
const install = useInstallPrompt()

const access = computed<SettingsAccess>(() => ({
  emoteManager: !!media.instanceQuery.data?.canManage,
  push: notifications.mode === 'push',
  quietHours: notifications.settings.quietHours.enabled,
  install: notifications.needsInstall || install.available,
}))

const tab = ref<SettingsSectionId>('appearance')
const current = computed(() => settingsSections.find((t) => t.id === tab.value)!)
const content = ref<HTMLElement | null>(null)
const searchInput = ref<HTMLInputElement | null>(null)

/** Opens a section or a setting: the one path for direct links and search results. */
function open(target: string) {
  const destination = settingsDestination(target, access.value)
  tab.value = destination.section
  if (destination.setting) void reveal(destination.setting)
  else content.value?.scrollTo({ top: 0 })
}

let revealing = 0
/**
 * Scrolls to a setting's row, focuses it and flashes it. Only the row takes focus, never its
 * control, so landing on a setting can't change it. Panels that load their content wait briefly.
 */
async function reveal(id: string) {
  const attempt = ++revealing
  await nextTick()
  const deadline = performance.now() + 1500
  let row: HTMLElement | null = null
  while (attempt === revealing) {
    row = content.value?.querySelector<HTMLElement>(`[data-setting="${CSS.escape(id)}"]`) ?? null
    if (row || performance.now() > deadline) break
    await new Promise((resolve) => requestAnimationFrame(resolve))
  }
  if (!row || attempt !== revealing) return
  if (!row.hasAttribute('tabindex')) row.tabIndex = -1
  row.scrollIntoView({ block: 'center', behavior: appearance.reducedMotion ? 'auto' : 'smooth' })
  row.focus({ preventScroll: true })
  row.classList.remove('setting-found')
  void row.offsetWidth // Restart the flash when the same row is found twice.
  row.classList.add('setting-found')
  row.addEventListener('blur', () => row.classList.remove('setting-found'), { once: true })
}

// A setting given as the destination is revealed once its panel has rendered.
open(props.initialTab)

// Search
const query = ref('')
const active = ref(0)
const listId = useId()
const found = computed(() => searchSettings(query.value, access.value))
const results = computed<SettingResult[]>(() => found.value.results)
const truncated = computed(() => found.value.total > results.value.length)
const searching = computed(() => query.value.trim().length > 0)
const announcement = computed(() =>
  !searching.value
    ? ''
    : truncated.value
      ? `Showing ${results.value.length} of ${found.value.total} settings`
      : results.value.length
        ? `${results.value.length} ${results.value.length === 1 ? 'setting' : 'settings'} found`
        : 'No settings found',
)
watch(query, (value) => {
  active.value = 0
  // Whether you can add emotes comes from the server catalog: fetch it once, on first use.
  if (value.trim()) media.needCustom()
})
const context = (result: SettingResult) =>
  [sectionName(result.entry.section), result.entry.group].filter(Boolean).join(' › ')

function choose(result: SettingResult | undefined) {
  if (!result) return
  query.value = ''
  open(result.entry.id)
}
function clear() {
  query.value = ''
  searchInput.value?.focus()
}
function keydown(event: KeyboardEvent) {
  if (event.isComposing) return
  const count = results.value.length
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    if (!count) return
    event.preventDefault()
    active.value = (active.value + (event.key === 'ArrowDown' ? 1 : count - 1)) % count
    document.getElementById(`${listId}-${active.value}`)?.scrollIntoView({ block: 'nearest' })
  } else if (event.key === 'Enter') {
    // Never let the key reach whatever takes focus next.
    event.preventDefault()
    choose(results.value[active.value])
  } else if (event.key === 'Escape' && query.value) {
    // The first Escape clears the search; the next one closes Settings as usual.
    event.preventDefault()
    event.stopPropagation()
    query.value = ''
  }
}
</script>
<template>
  <AppDialog
    title="Settings"
    panel
    @close="emit('close')"
    ><div class="settings-layout">
      <div
        class="settings-search"
        role="search"
      >
        <AppIcon
          name="search"
          :size="15"
        />
        <input
          ref="searchInput"
          v-model="query"
          type="search"
          role="combobox"
          aria-label="Search settings"
          placeholder="Search settings"
          autocomplete="off"
          spellcheck="false"
          maxlength="80"
          :aria-expanded="searching && results.length > 0"
          :aria-controls="searching && results.length ? listId : undefined"
          aria-autocomplete="list"
          :aria-activedescendant="searching && results.length ? `${listId}-${active}` : undefined"
          @keydown="keydown"
        />
        <button
          v-if="query"
          type="button"
          class="settings-search-clear"
          aria-label="Clear search"
          @click="clear"
        >
          <AppIcon
            name="close"
            :size="14"
          />
        </button>
        <span
          class="sr-only"
          role="status"
          >{{ announcement }}</span
        >
      </div>
      <nav
        class="settings-nav"
        aria-label="Settings"
      >
        <button
          v-for="item in settingsSections"
          :key="item.id"
          :class="{ active: !searching && tab === item.id }"
          :aria-current="!searching && tab === item.id ? 'page' : undefined"
          @click="((query = ''), open(item.id))"
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
      <div
        v-show="searching"
        class="settings-results"
      >
        <h3 class="settings-title">Search results</h3>
        <ul
          v-if="results.length"
          :id="listId"
          role="listbox"
          aria-label="Matching settings"
        >
          <li
            v-for="(result, index) in results"
            :id="`${listId}-${index}`"
            :key="result.entry.id"
            role="option"
            :aria-selected="index === active"
            :class="{ active: index === active, unavailable: result.unavailable }"
            @mousedown.prevent
            @mousemove="active = index"
            @click="choose(result)"
          >
            <strong>{{ result.entry.label }}</strong>
            <small>{{ context(result) }}</small>
            <small
              v-if="result.unavailable"
              class="settings-result-note"
              >{{ result.unavailable }}</small
            >
          </li>
        </ul>
        <p
          v-if="truncated"
          class="settings-description settings-results-more"
        >
          Showing the best {{ results.length }} of {{ found.total }}. Keep typing to narrow them.
        </p>
        <p
          v-else-if="!results.length"
          class="settings-description"
        >
          No settings match “{{ query.trim() }}”. Try another word, such as “font” or “sound”.
        </p>
      </div>
      <!-- Hidden, not unmounted, while searching; forms keep what you typed across sections. -->
      <div
        v-show="!searching"
        ref="content"
        class="settings-content"
      >
        <h3 class="settings-title">{{ current.name }}</h3>
        <KeepAlive
          :include="['ProfileSettings', 'NotificationSettings', 'EmoteSettings', 'AccountSettings']"
        >
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
          <AboutSettings v-else />
        </KeepAlive>
      </div></div
  ></AppDialog>
</template>
