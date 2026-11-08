<script setup lang="ts">
/** Settings → Notifications → Across your devices: which device is notified when, and auto-away. */
import { scopeLabels } from '../../shared/settings-index'
import {
  defaultNotificationSettings,
  formatDuration,
  notificationLimits,
} from '../../shared/notifications'
const notifications = useNotificationStore()
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
</script>
<template>
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
    <SettingNote
      as="p"
      data-testid="routing-summary"
    >
      <AppIcon
        name="info"
        :size="15"
      /><span
        >While you’re using IRCord anywhere, no device pings or pops up; unread counts still update.
        <template v-if="lastOnly > 0"
          >Only the device you used last is notified for {{ formatDuration(lastOnly) }}, then every
          device is.</template
        ><template v-else>Every device is notified at once.</template>
      </span>
    </SettingNote>
    <SettingRow
      data-setting="notify-reset"
      label="Reset device timing"
      :description="`Back to ${formatDuration(defaultNotificationSettings.lastDeviceDelay)} for the last-used device and ${formatDuration(defaultNotificationSettings.allDevicesDelay)} for all.`"
    >
      <UiButton
        variant="secondary"
        size="sm"
        type="button"
        :disabled="routingDefaults"
        @click="resetRouting"
      >
        Reset to defaults
      </UiButton>
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
      <span class="param-controls flex gap-1.5 items-center">
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
</template>
