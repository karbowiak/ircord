<script setup lang="ts">
import {
  sectionName,
  searchSettings,
  settingsDestination,
  settingsGroups,
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
const { t, n } = useI18n()
const messages = useSettingsMessages()

const access = computed<SettingsAccess>(() => ({
  emoteManager: !!media.instanceQuery.data?.canManage,
  push: notifications.mode === 'push',
  quietHours: notifications.settings.quietHours.enabled,
  install: notifications.needsInstall || install.available,
}))

const tab = ref<SettingsSectionId>('appearance')
const current = computed(() => settingsSections.find((t) => t.id === tab.value)!)
const groups = settingsGroups.map((group) => ({
  ...group,
  headingId: useId(),
  sections: settingsSections.filter((s) => s.group === group.id),
}))
const content = ref<HTMLElement | null>(null)
const title = ref<HTMLElement | null>(null)
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

// Links inside a section open through the same path; a section link moves focus to its title.
provide(settingsNavigationKey, (target) => {
  query.value = ''
  open(target)
  if (settingsSections.some((s) => s.id === target))
    nextTick(() => title.value?.focus({ preventScroll: true }))
})

// Search
const query = ref('')
const active = ref(0)
const listId = useId()
const found = computed(() => searchSettings(query.value, access.value, messages))
const results = computed<SettingResult[]>(() => found.value.results)
const truncated = computed(() => found.value.total > results.value.length)
const searching = computed(() => query.value.trim().length > 0)
const announcement = computed(() =>
  !searching.value
    ? ''
    : truncated.value
      ? t('settings.showingOf', {
          shown: n(results.value.length),
          total: n(found.value.total),
        })
      : results.value.length
        ? t('settings.found', { count: n(results.value.length) }, results.value.length)
        : t('settings.noneFound'),
)
watch(query, (value) => {
  active.value = 0
  // Whether you can add emotes comes from the server catalog: fetch it once, on first use.
  if (value.trim()) media.needCustom()
})
const context = (result: SettingResult) =>
  [sectionName(result.entry.section, t), result.text.group].filter(Boolean).join(' › ')

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
    :title="$t('settings.title')"
    panel
    @close="emit('close')"
    ><SettingsLayout :label="$t('settings.title')">
      <template #search>
        <div
          class="settings-search [grid-area:search] relative flex items-center pt-3 pr-2 pb-1 pl-2 bg-sidebar text-muted-foreground max-[700px]:pt-2 max-[700px]:pb-0"
          role="search"
        >
          <AppIcon
            name="search"
            :size="15"
            class="pointer-events-none absolute left-[18px]"
          />
          <input
            ref="searchInput"
            v-model="query"
            class="h-8 w-full rounded-sm border border-solid border-border bg-field pr-7.5 pl-8 text-[14px] text-foreground focus:border-primary [&::-webkit-search-cancel-button]:hidden"
            type="search"
            role="combobox"
            :aria-label="$t('settings.searchLabel')"
            :placeholder="$t('settings.searchLabel')"
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
            class="settings-search-clear absolute right-3 grid [place-items:center] w-6 h-6 rounded-sm text-muted-foreground hover:bg-accent hover:text-foreground"
            :aria-label="$t('settings.clearSearch')"
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
      </template>
      <template #nav>
        <div
          v-for="group in groups"
          :key="group.id"
          class="settings-nav-section contents"
          role="group"
          :aria-labelledby="group.headingId"
        >
          <div
            :id="group.headingId"
            class="settings-nav-group pt-3.5 pr-2.5 pb-1 pl-2.5 text-[11px] font-[650] tracking-[0.05em] uppercase text-faint [.settings-nav-section:first-child>&]:pt-0.5 max-[700px]:sr-only"
          >
            {{ $t(group.name) }}
          </div>
          <SettingsNavItem
            v-for="item in group.sections"
            :key="item.id"
            :icon="item.icon"
            :active="!searching && tab === item.id"
            :data-section="item.id"
            @click="((query = ''), open(item.id))"
            >{{ $t(item.name) }}</SettingsNavItem
          >
        </div>
        <SettingsNavItem
          icon="logout"
          danger
          @click="auth.confirmLogout()"
          >{{ $t('common.logOut') }}</SettingsNavItem
        >
      </template>
      <div
        v-show="searching"
        class="settings-results [grid-area:content] pt-5 pr-7 pb-7 pl-7 overflow-y-auto max-[700px]:p-4"
      >
        <SettingsTitle>{{ $t('settings.searchResults') }}</SettingsTitle>
        <ul
          v-if="results.length"
          :id="listId"
          class="m-0 grid list-none gap-0.5 p-0"
          role="listbox"
          :aria-label="$t('settings.matchingSettings')"
        >
          <li
            v-for="(result, index) in results"
            :id="`${listId}-${index}`"
            :key="result.entry.id"
            role="option"
            class="grid cursor-pointer gap-0.5 rounded-sm px-3 py-2"
            :class="{ 'active bg-selection': index === active, unavailable: result.unavailable }"
            :aria-selected="index === active"
            @mousedown.prevent
            @mousemove="active = index"
            @click="choose(result)"
          >
            <strong :class="{ 'text-muted-foreground': result.unavailable }">{{
              result.text.label
            }}</strong>
            <small class="text-[12px] text-muted-foreground">{{ context(result) }}</small>
            <small
              v-if="result.unavailable"
              class="settings-result-note text-[12px] text-muted-foreground italic"
              >{{ result.unavailable }}</small
            >
          </li>
        </ul>
        <p
          v-if="truncated"
          class="settings-description settings-results-more text-[13px] text-muted-foreground leading-[1.55] mt-3"
        >
          {{
            $t('settings.showingBest', {
              shown: $n(results.length),
              total: $n(found.total),
            })
          }}
        </p>
        <SettingsDescription v-else-if="!results.length">
          {{ $t('settings.noMatches', { query: query.trim() }) }}
        </SettingsDescription>
      </div>
      <!-- Hidden, not unmounted, while searching; forms keep what you typed across sections. -->
      <div
        v-show="!searching"
        ref="content"
        class="settings-content [grid-area:content] pt-5 pr-7 pb-7 pl-7 overflow-y-auto max-[700px]:p-4"
      >
        <SettingsTitle
          :ref="(el) => (title = (el as { $el: HTMLElement } | null)?.$el ?? null)"
          tabindex="-1"
        >
          {{ $t(current.name) }}
        </SettingsTitle>
        <KeepAlive
          :include="[
            'ProfileSettings',
            'MessageSettings',
            'NotificationSettings',
            'ReadAloudSettings',
            'EmoteSettings',
            'AccountSettings',
          ]"
        >
          <AppearanceSettings v-if="tab === 'appearance'" />
          <MessageSettings v-else-if="tab === 'messages'" />
          <ProfileSettings v-else-if="tab === 'profile'" />
          <NetworkSettingsList
            v-else-if="tab === 'networks'"
            @add-network="emit('addNetwork')"
            @edit-network="emit('editNetwork', $event)"
          />
          <NotificationSettings v-else-if="tab === 'notifications'" />
          <ReadAloudSettings v-else-if="tab === 'read-aloud'" />
          <EmoteSettings v-else-if="tab === 'emotes'" />
          <AccountSettings v-else-if="tab === 'account'" />
          <ShortcutList
            v-else-if="tab === 'shortcuts'"
            editable
          />
          <AboutSettings v-else />
        </KeepAlive></div></SettingsLayout
  ></AppDialog>
</template>
