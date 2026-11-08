<script setup lang="ts">
/** Settings → Notifications: when, where and how IRCord alerts you. */
import type { DeviceMode } from '../stores/notifications'
import { scopeLabels } from '../../shared/settings-index'
import {
  defaultNotificationSettings,
  formatDuration,
  notificationLimits,
} from '../../shared/notifications'
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
/** The window in which only the last-used device is notified. */
const lastOnly = computed(
  () => notifications.settings.allDevicesDelay - notifications.settings.lastDeviceDelay,
)
const routingDefaults = computed(
  () =>
    notifications.settings.lastDeviceDelay === defaultNotificationSettings.lastDeviceDelay &&
    notifications.settings.allDevicesDelay === defaultNotificationSettings.allDevicesDelay,
)
function setLastDevice(seconds: number) {
  // Every device is never notified before the last-used one; raise that threshold along.
  notifications.update({
    lastDeviceDelay: seconds,
    allDevicesDelay: Math.max(seconds, notifications.settings.allDevicesDelay),
  })
}
function setAllDevices(seconds: number) {
  notifications.update({
    allDevicesDelay: seconds,
    lastDeviceDelay: Math.min(seconds, notifications.settings.lastDeviceDelay),
  })
}
function resetRouting() {
  notifications.update({
    lastDeviceDelay: defaultNotificationSettings.lastDeviceDelay,
    allDevicesDelay: defaultNotificationSettings.allDevicesDelay,
  })
}
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
      title="Across your devices"
      :scope="scopeLabels.account"
    >
      <SettingRow
        data-setting="notify-last-device"
        label="Notify the last-used device after"
        description="Time without IRCord in use on any device. 0 notifies it as soon as you step away."
      >
        <DurationInput
          :model-value="notifications.settings.lastDeviceDelay"
          :max="notificationLimits.lastDeviceDelay"
          label="Notify the last-used device after"
          @update:model-value="setLastDevice"
        />
      </SettingRow>
      <SettingRow
        data-setting="notify-all-devices"
        label="Notify all devices after"
        description="Then every device is notified, with one reminder of what’s still unread."
      >
        <DurationInput
          :model-value="notifications.settings.allDevicesDelay"
          :max="notificationLimits.allDevicesDelay"
          label="Notify all devices after"
          @update:model-value="setAllDevices"
        />
      </SettingRow>
      <p
        class="setting-note"
        data-testid="routing-summary"
      >
        <AppIcon
          name="info"
          :size="15"
        /><span
          >While you’re using IRCord anywhere, no device pings or pops up; unread counts still
          update.
          <template v-if="lastOnly > 0"
            >Only the device you used last is notified for
            {{ formatDuration(lastOnly) }}, then every device is.</template
          ><template v-else>Every device is notified at once.</template>
</span
        >
      </p>
      <SettingRow
        data-setting="notify-reset"
        label="Reset device timing"
        :description="`Back to ${formatDuration(defaultNotificationSettings.lastDeviceDelay)} for the last-used device and ${formatDuration(defaultNotificationSettings.allDevicesDelay)} for all.`"
      >
        <button
          type="button"
          class="button secondary small"
          :disabled="routingDefaults"
          @click="resetRouting"
        >
          Reset to defaults
        </button>
      </SettingRow>
      <SettingRow
        data-setting="auto-away"
        label="Set me away when no device is connected"
        :description="
          notifications.settings.autoAwayDelay
            ? `Others on IRC see you as away after ${formatDuration(notifications.settings.autoAwayDelay)} with no IRCord device connected, and back as soon as one connects. An away you set yourself is left alone.`
            : 'You stay present on IRC while IRCord keeps your connection open.'
        "
      >
        <span class="param-controls">
          <AppSwitch
            :model-value="notifications.settings.autoAwayDelay > 0"
            aria-label="Set me away automatically"
            @update:model-value="
              notifications.update({
                autoAwayDelay: $event ? defaultNotificationSettings.autoAwayDelay : 0,
              })
            "
          />
          <DurationInput
            v-if="notifications.settings.autoAwayDelay > 0"
            :model-value="notifications.settings.autoAwayDelay"
            :max="notificationLimits.autoAwayDelay"
            label="Set me away after"
            @update:model-value="notifications.update({ autoAwayDelay: Math.max(60, $event) })"
          />
        </span>
      </SettingRow>
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
