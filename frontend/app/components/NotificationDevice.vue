<script setup lang="ts">
/** Settings → Notifications → This device: whether notifications work here, and how. */
import type { DeviceMode } from '../stores/notifications'
import { scopeLabels } from '../../shared/settings-index'
const notifications = useNotificationStore()
const sound = useAlertSound()
const install = useInstallPrompt()
const modes: { value: DeviceMode; label: string; note: string }[] = [
  { value: 'off', label: 'Off', note: 'No notifications on this device.' },
  {
    value: 'open',
    label: 'While open',
    note: 'Pop-ups only while IRCord is open in a tab or window.',
  },
  {
    value: 'push',
    label: 'Always',
    note: 'Push notifications, even when IRCord is closed.',
  },
]
const blocked = computed(() => notifications.permission === 'denied')
const modeOptions = computed(() =>
  modes.map((m) => ({
    ...m,
    disabled:
      (m.value !== 'off' && (blocked.value || !notifications.supported.notifications)) ||
      (m.value === 'push' &&
        (!notifications.supported.push ||
          notifications.needsInstall ||
          notifications.serverPush === false)),
  })),
)
/** The summary of this device: what is set, whether it works, and the one next step. */
const summary = computed(() => {
  switch (notifications.status) {
    case 'install':
      return {
        icon: 'download',
        tone: 'warn',
        title: 'Needs the Home Screen app',
        detail: 'iPhone and iPad show notifications only in IRCord opened from the Home Screen.',
      }
    case 'unsupported':
      return {
        icon: 'muted',
        tone: 'warn',
        title: 'Not available in this browser',
        detail:
          'This browser can’t show notifications. Try another browser, or install IRCord as an app.',
      }
    case 'blocked':
      return {
        icon: 'muted',
        tone: 'warn',
        title: 'Blocked by your browser',
        detail:
          'Allow notifications for this site in your browser or system settings, then come back here.',
      }
    case 'off':
      return {
        icon: 'muted',
        tone: 'off',
        title: 'Off on this device',
        detail: notifications.pushAvailable
          ? 'Get notified about mentions and direct messages, even when IRCord is closed.'
          : 'Get notified about mentions and direct messages while IRCord is open.',
      }
    case 'permission':
      return {
        icon: 'alert',
        tone: 'warn',
        title: 'Needs your permission again',
        detail: 'This browser no longer has permission to show notifications from IRCord.',
      }
    case 'open':
      return {
        icon: 'bell',
        tone: 'on',
        title: 'On while IRCord is open',
        detail: notifications.pushAvailable
          ? 'Nothing arrives while it’s closed. Choose Always to get push notifications too.'
          : 'Nothing arrives while it’s closed.',
      }
    case 'checking':
      return {
        icon: 'clock',
        tone: 'off',
        title: 'Checking push…',
        detail: 'Confirming this device’s push registration with the server.',
      }
    case 'push':
      return {
        icon: 'bell',
        tone: 'on',
        title: 'On, even when IRCord is closed',
        detail: 'This device is registered for push notifications.',
      }
    case 'repair':
      return {
        icon: 'alert',
        tone: 'warn',
        title: 'Push needs repair',
        detail:
          'This device lost its push registration, so nothing arrives while IRCord is closed.',
      }
    case 'push-unsupported':
      return {
        icon: 'cloudOff',
        tone: 'warn',
        title: 'Push isn’t supported in this browser',
        detail: 'Choose While open to get notifications while IRCord is open.',
      }
    case 'server-unavailable':
      return {
        icon: 'cloudOff',
        tone: 'warn',
        title: 'Push isn’t available on this server',
        detail: 'Choose While open to get notifications while IRCord is open.',
      }
  }
  return { icon: 'info', tone: 'off', title: '', detail: '' }
})
const permissionText = computed(
  () =>
    ({
      granted: 'Allowed',
      denied: 'Blocked',
      default: 'Not asked yet',
      unsupported: 'Not supported',
    })[notifications.permission],
)
const pushText = computed(() => {
  if (notifications.needsInstall) return 'Needs the Home Screen app'
  if (!notifications.supported.push) return 'Not supported'
  if (notifications.serverPush === false) return 'Unavailable on this server'
  if (notifications.mode !== 'push') return 'Not in use'
  return (
    {
      push: 'Registered',
      repair: 'Needs repair',
      checking: 'Checking…',
      blocked: 'Paused: notifications are blocked',
      permission: 'Paused: needs permission',
    }[notifications.status as string] ?? 'Not registered'
  )
})
const testResult = ref<{ ok: boolean; message: string } | null>(null)
const testing = ref(false)
async function sendTest() {
  testing.value = true
  testResult.value = null
  try {
    testResult.value = await notifications.test()
  } finally {
    testing.value = false
  }
}
watch(
  () => notifications.mode,
  () => (testResult.value = null),
)
</script>
<template>
  <SettingGroup
    title="This device"
    :scope="scopeLabels.device"
  >
    <div
      data-setting="notifications-status"
      class="notify-status flex flex-col gap-2.5 rounded-md bg-card px-3.5 py-3 shadow-[inset_0_0_0_1px_var(--border)]"
      :class="`is-${summary.tone}`"
      role="group"
      aria-label="Notifications on this device"
    >
      <div class="notify-status-head flex flex-wrap items-center gap-3">
        <AppIcon
          :name="summary.icon"
          :size="18"
          class="shrink-0"
          :class="
            summary.tone === 'on'
              ? 'text-success'
              : summary.tone === 'warn'
                ? 'text-warning'
                : 'text-faint'
          "
        />
        <span
          class="setting-copy flex min-w-48 flex-1 flex-col gap-[1px]"
          aria-live="polite"
          ><strong>{{ summary.title }}</strong
          ><small>{{ summary.detail }}</small></span
        >
        <UiButton
          v-if="notifications.status === 'off' && !blocked"
          size="sm"
          :disabled="notifications.busy"
          @click="notifications.enable()"
        >
          Enable notifications
        </UiButton>
        <UiButton
          v-else-if="notifications.status === 'permission'"
          size="sm"
          :disabled="notifications.busy"
          @click="notifications.repair()"
        >
          Allow notifications
        </UiButton>
        <UiButton
          v-else-if="notifications.status === 'repair' && notifications.pushAvailable"
          size="sm"
          :disabled="notifications.busy"
          @click="notifications.repair()"
        >
          Repair
        </UiButton>
        <UiButton
          v-else-if="notifications.status === 'blocked'"
          variant="secondary"
          size="sm"
          @click="notifications.refresh()"
        >
          Check again
        </UiButton>
      </div>
      <dl
        class="notify-facts m-0 flex flex-wrap gap-x-5 gap-y-1 text-[12px] [&_dd]:m-0 [&_dd]:text-foreground [&_dt]:text-faint"
      >
        <div>
          <dt>Delivery</dt>
          <dd>{{ modes.find((m) => m.value === notifications.mode)?.label }}</dd>
        </div>
        <div>
          <dt>Browser permission</dt>
          <dd>{{ permissionText }}</dd>
        </div>
        <div>
          <dt>Push</dt>
          <dd>{{ pushText }}</dd>
        </div>
      </dl>
    </div>
    <div
      v-if="notifications.needsInstall"
      data-setting="install-app"
      class="install-hint flex items-center gap-3 py-3 px-3.5 rounded-md bg-selection text-primary"
    >
      <AppIcon
        name="download"
        :size="18"
      />
      <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
        ><strong>Add IRCord to your Home Screen first</strong
        ><small
          >In Safari, tap Share (on iOS 26, under the ⋯ button), choose “Add to Home Screen”, keep
          “Open as Web App” on, and open IRCord from your Home Screen. Notifications need iOS or
          iPadOS 16.4 or later. Set them up again in the app: what this browser tab allowed doesn’t
          carry over.</small
        ></span
      >
    </div>
    <div
      v-else-if="install.available"
      data-setting="install-app"
      class="install-hint flex items-center gap-3 py-3 px-3.5 rounded-md bg-selection text-primary"
    >
      <AppIcon
        name="download"
        :size="18"
      />
      <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
        ><strong>Install IRCord as an app</strong
        ><small
          >Its own window, an unread badge on the icon and more reliable notifications.</small
        ></span
      >
      <UiButton
        variant="secondary"
        size="sm"
        @click="install.prompt()"
      >
        Install
      </UiButton>
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
            : 'While open shows pop-ups only while IRCord is open in a tab or window; Always also uses push when it’s closed.'
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
      description="Sends a test notification to this device only."
    >
      <UiButton
        variant="secondary"
        size="sm"
        :disabled="testing || notifications.status !== 'push'"
        @click="sendTest"
      >
        Send test
      </UiButton>
    </SettingRow>
    <p
      v-if="testResult"
      class="notify-test-result -mt-1 mb-0 text-[12px]"
      :class="testResult.ok ? 'text-success' : 'text-destructive'"
      role="status"
    >
      {{ testResult.message }}
    </p>
    <SettingRow
      data-setting="notification-sound"
      label="Sound"
      description="A short chime while IRCord is open, unless you’re looking at that conversation."
    >
      <div class="param-controls flex gap-1.5 items-center">
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
        <UiButton
          variant="ghost"
          size="icon-sm"
          aria-label="Play the sound"
          title="Preview"
          :disabled="sound.mode.value === 'off'"
          @click="sound.preview()"
        >
          <AppIcon
            name="volume"
            :size="16"
          />
        </UiButton>
      </div>
    </SettingRow>
    <SettingNote as="p">
      <AppIcon
        name="laptop"
        :size="15"
      /><span
        >These choices apply to this browser only. Each signed-in device has its own; see them and
        their push status under <SettingLink to="account" />. When and what notifies you is set
        below, for all your devices.</span
      >
    </SettingNote>
  </SettingGroup>
</template>
