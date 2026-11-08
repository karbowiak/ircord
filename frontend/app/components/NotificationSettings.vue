<script setup lang="ts">
/** Settings → Notifications: when, where and how IRCord alerts you. */
import type { DeviceMode } from '../stores/notifications'
import { scopeLabels } from '../../shared/settings-index'
const notifications = useNotificationStore()
const sound = useAlertSound()
/** Quiet hours follow this device's time zone whenever they're edited here. */
function setQuiet(changes: Partial<{ enabled: boolean; start: string; end: string }>) {
  notifications.update({
    quietHours: {
      ...notifications.settings.quietHours,
      ...changes,
      timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
    },
  })
}
const install = useInstallPrompt()
const word = ref('')
const levelOptions = [
  { value: 'all', label: 'All' },
  { value: 'mentions', label: 'Mentions' },
  { value: 'none', label: 'Nothing' },
] as const
const modes: { value: DeviceMode; label: string; note: string }[] = [
  { value: 'off', label: 'Off', note: 'No notifications on this device.' },
  { value: 'open', label: 'While open', note: 'Only while IRCord is open in a tab or window.' },
  { value: 'push', label: 'Always', note: 'Even when IRCord is closed, through push.' },
]
const blocked = computed(() => notifications.permission === 'denied')
const modeOptions = computed(() =>
  modes.map((m) => ({
    ...m,
    disabled:
      (m.value !== 'off' && (blocked.value || !notifications.supported.notifications)) ||
      (m.value === 'push' && (!notifications.supported.push || notifications.needsInstall)),
  })),
)
function addWord() {
  const value = word.value.trim()
  if (value)
    notifications.update({ highlightWords: [...notifications.settings.highlightWords, value] })
  word.value = ''
}
</script>
<template>
  <div class="appearance-settings">
    <SettingGroup
      title="This device"
      :scope="scopeLabels.device"
    >
      <div
        v-if="notifications.needsInstall"
        data-setting="install-app"
        class="install-hint"
      >
        <AppIcon
          name="download"
          :size="18"
        />
        <span class="setting-copy"
          ><strong>Add IRCord to your Home Screen first</strong
          ><small
            >On iPhone and iPad, notifications only work in the installed app: tap Share, then “Add
            to Home Screen”, and open IRCord from there.</small
          ></span
        >
      </div>
      <div
        v-else-if="install.available"
        data-setting="install-app"
        class="install-hint"
      >
        <AppIcon
          name="download"
          :size="18"
        />
        <span class="setting-copy"
          ><strong>Install IRCord as an app</strong
          ><small
            >Its own window, an unread badge on the icon and more reliable notifications.</small
          ></span
        >
        <button
          class="button secondary small"
          @click="install.prompt()"
        >
          Install
        </button>
      </div>
      <SettingRow
        data-setting="notifications-device"
        label="Notifications"
        stacked
        :description="
          blocked
            ? 'Blocked by your browser. Allow notifications for this site in its settings.'
            : !notifications.supported.notifications
              ? 'This browser can’t show notifications.'
              : modes.find((m) => m.value === notifications.mode)?.note
        "
      >
        <AppSegmented
          :model-value="notifications.mode"
          :options="modeOptions"
          :disabled="notifications.busy"
          label="Notifications on this device"
          reselect
          @change="notifications.setMode($event)"
        />
      </SettingRow>
      <SettingRow
        v-if="notifications.mode === 'push'"
        data-setting="notifications-test"
        label="Test"
        description="Sends a notification to every device using push."
      >
        <button
          class="button secondary small"
          @click="notifications.test()"
        >
          Send test
        </button>
      </SettingRow>
      <SettingRow
        data-setting="notification-sound"
        label="Sound"
        description="A short chime while IRCord is open, unless you’re looking at that conversation."
      >
        <div class="param-controls">
          <AppSegmented
            :model-value="sound.mode.value"
            :options="[
              { value: 'off', label: 'Off' },
              { value: 'mentions', label: 'Mentions' },
              { value: 'all', label: 'All' },
            ]"
            label="Notification sound"
            @update:model-value="sound.setMode($event)"
          />
          <button
            class="icon-button"
            aria-label="Play the sound"
            title="Preview"
            :disabled="sound.mode.value === 'off'"
            @click="sound.preview()"
          >
            <AppIcon
              name="volume"
              :size="16"
            />
          </button>
        </div>
      </SettingRow>
      <p class="setting-note">
        <AppIcon
          name="laptop"
          :size="15"
        /><span
          >These choices apply to this browser only. Each signed-in device has its own; see them
          under <SettingLink to="account" />.</span
        >
      </p>
    </SettingGroup>

    <SettingGroup
      title="Quiet hours"
      :scope="scopeLabels.account"
    >
      <SettingRow
        data-setting="quiet-hours"
        label="Silence notifications"
        :description="`No push, pop-ups or sounds between these times (${notifications.settings.quietHours.timeZone}).`"
      >
        <AppSwitch
          :model-value="notifications.settings.quietHours.enabled"
          aria-label="Quiet hours"
          @update:model-value="setQuiet({ enabled: $event })"
        />
      </SettingRow>
      <div
        v-if="notifications.settings.quietHours.enabled"
        data-setting="quiet-hours-times"
        class="setting-row quiet-times"
      >
        <label
          >From
          <input
            type="time"
            class="text-input"
            :value="notifications.settings.quietHours.start"
            @change="setQuiet({ start: ($event.target as HTMLInputElement).value })" /></label
        ><label
          >until
          <input
            type="time"
            class="text-input"
            :value="notifications.settings.quietHours.end"
            @change="setQuiet({ end: ($event.target as HTMLInputElement).value })"
        /></label>
      </div>
    </SettingGroup>

    <SettingGroup
      title="Notify me about"
      :scope="scopeLabels.account"
    >
      <SettingRow
        data-setting="channel-notifications"
        label="Channels"
        description="Used where a channel’s level is Default. Each channel’s level is set by its operators in channel settings."
      >
        <AppSegmented
          :model-value="notifications.settings.channelDefault"
          :options="levelOptions"
          label="Channel notifications"
          @update:model-value="notifications.update({ channelDefault: $event })"
        />
      </SettingRow>
      <SettingRow
        data-setting="direct-notifications"
        label="Direct messages"
      >
        <AppSegmented
          :model-value="notifications.settings.directDefault"
          :options="levelOptions"
          label="Direct message notifications"
          @update:model-value="notifications.update({ directDefault: $event })"
        />
      </SettingRow>
      <SettingRow
        data-setting="highlight-words"
        label="Highlight words"
        description="Count as a mention, like your nick. Whole words, any case."
        stacked
      >
        <form
          class="chip-input"
          @submit.prevent="addWord"
        >
          <span
            v-for="w in notifications.settings.highlightWords"
            :key="w"
            class="chip"
            >{{ w
            }}<button
              type="button"
              :aria-label="`Remove ${w}`"
              @click="
                notifications.update({
                  highlightWords: notifications.settings.highlightWords.filter((x) => x !== w),
                })
              "
            >
              <AppIcon
                name="close"
                :size="12"
              /></button
          ></span>
          <input
            v-model="word"
            aria-label="Add a highlight word"
            placeholder="Add a word, press Enter"
            maxlength="64"
          />
        </form>
      </SettingRow>
      <p class="setting-note">
        <AppIcon
          name="info"
          :size="15"
        /><span
          >People you ignore never notify you. Manage them under
          <SettingLink to="messages" />.</span
        >
      </p>
    </SettingGroup>
  </div>
</template>
