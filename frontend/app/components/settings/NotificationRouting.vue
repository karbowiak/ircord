<script setup vapor lang="ts">
/** Settings → Notifications → Across your devices: which device is notified when, and auto-away. */
import { scopeLabels } from '../../../shared/settings-index'
import {
  defaultNotificationSettings,
  formatDuration,
  notificationLimits,
} from '../../../shared/notifications'
const notifications = useNotificationStore()
const { t, n } = useI18n()
const $t = t
const duration = (seconds: number) =>
  formatDuration(
    seconds,
    (unit, count) => t(`duration.${unit}`, { count: n(count) }, count),
    (parts) =>
      parts.length === 3
        ? t('duration.three', { first: parts[0], second: parts[1], third: parts[2] })
        : parts.length === 2
          ? t('duration.two', { first: parts[0], second: parts[1] })
          : (parts[0] ?? ''),
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
</script>
<template>
  <SettingGroup
    :title="$t('notifications.groups.acrossDevices')"
    :scope="$t(scopeLabels.account)"
  >
    <SettingRow
      data-setting="notify-last-device"
      :label="$t('notifications.routing.lastDevice')"
      :description="$t('notifications.routing.lastDeviceHelp')"
    >
      <DurationInput
        :model-value="notifications.settings.lastDeviceDelay"
        :max="notificationLimits.lastDeviceDelay"
        :label="$t('notifications.routing.lastDevice')"
        @update:model-value="setLastDevice"
      />
    </SettingRow>
    <SettingRow
      data-setting="notify-all-devices"
      :label="$t('notifications.routing.allDevices')"
      :description="$t('notifications.routing.allDevicesHelp')"
    >
      <DurationInput
        :model-value="notifications.settings.allDevicesDelay"
        :max="notificationLimits.allDevicesDelay"
        :label="$t('notifications.routing.allDevices')"
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
        >{{ $t('notifications.routing.anywhere') }}
        <template v-if="lastOnly > 0">{{
          $t('notifications.routing.lastOnly', { duration: duration(lastOnly) })
        }}</template
        ><template v-else>{{ $t('notifications.routing.allAtOnce') }}</template>
      </span>
    </SettingNote>
    <SettingRow
      data-setting="notify-reset"
      :label="$t('notifications.routing.resetLabel')"
      :description="
        $t('notifications.routing.resetHelp', {
          last: duration(defaultNotificationSettings.lastDeviceDelay),
          all: duration(defaultNotificationSettings.allDevicesDelay),
        })
      "
    >
      <UiButton
        variant="secondary"
        size="sm"
        type="button"
        :disabled="routingDefaults"
        @click="resetRouting"
      >
        {{ $t('notifications.routing.resetButton') }}
      </UiButton>
    </SettingRow>
    <SettingRow
      data-setting="auto-away"
      :label="$t('notifications.routing.autoAway')"
      :description="
        notifications.settings.autoAwayDelay
          ? $t('notifications.routing.autoAwayOn', {
              delay: duration(notifications.settings.autoAwayDelay),
            })
          : $t('notifications.routing.autoAwayOff')
      "
    >
      <span class="param-controls flex gap-1.5 items-center">
        <AppSwitch
          :model-value="notifications.settings.autoAwayDelay > 0"
          :aria-label="$t('notifications.routing.autoAwayAria')"
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
          :label="$t('notifications.routing.awayAfter')"
          @update:model-value="notifications.update({ autoAwayDelay: Math.max(60, $event) })"
        />
      </span>
    </SettingRow>
  </SettingGroup>
</template>
