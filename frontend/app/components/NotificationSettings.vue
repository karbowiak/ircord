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
        detail: 'This browser can’t show notifications. Try another browser, or install IRCord as an app.',
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
  return {
    push: 'Registered',
    repair: 'Needs repair',
    checking: 'Checking…',
  }[notifications.status as string] ?? 'Not registered'
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
        data-setting="notifications-status"
        class="notify-status"
        :class="`is-${summary.tone}`"
        role="group"
        aria-label="Notifications on this device"
      >
        <div class="notify-status-head">
          <AppIcon
            :name="summary.icon"
            :size="18"
          />
          <span
            class="setting-copy"
            aria-live="polite"
            ><strong>{{ summary.title }}</strong><small>{{ summary.detail }}</small></span
          >
          <button
            v-if="notifications.status === 'off' && !blocked"
            class="button primary small"
            :disabled="notifications.busy"
            @click="notifications.enable()"
          >
            Enable notifications
          </button>
          <button
            v-else-if="notifications.status === 'permission'"
            class="button primary small"
            :disabled="notifications.busy"
            @click="notifications.repair()"
          >
            Allow notifications
          </button>
          <button
            v-else-if="notifications.status === 'repair' && notifications.pushAvailable"
            class="button primary small"
            :disabled="notifications.busy"
            @click="notifications.repair()"
          >
            Repair
          </button>
          <button
            v-else-if="notifications.status === 'blocked'"
            class="button secondary small"
            @click="notifications.refresh()"
          >
            Check again
          </button>
        </div>
        <dl class="notify-facts">
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
        class="install-hint"
      >
        <AppIcon
          name="download"
          :size="18"
        />
        <span class="setting-copy"
          ><strong>Add IRCord to your Home Screen first</strong
          ><small
            >In Safari, tap Share (on iOS 26, under the ⋯ button), choose “Add to Home Screen”, keep
            “Open as Web App” on, and open IRCord from your Home Screen. Notifications need iOS or
            iPadOS 16.4 or later. Set them up again in the app: what this browser tab allowed
            doesn’t carry over.</small
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
              : `${modes.find((m) => m.value === notifications.mode)?.note} While open shows pop-ups only while IRCord is open; Always also uses push when it’s closed.`
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
        <button
          class="button secondary small"
          :disabled="testing || notifications.status !== 'push'"
          @click="sendTest"
        >
          Send test
        </button>
      </SettingRow>
      <p
        v-if="testResult"
        class="notify-test-result"
        :class="{ 'is-error': !testResult.ok }"
        role="status"
      >
        {{ testResult.message }}
      </p>
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
          and their push status under <SettingLink to="account" />. When and what notifies you is
          set below, for all your devices.</span
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
<style scoped>
.notify-status {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px 14px;
  border-radius: var(--radius);
  background: var(--panel);
  box-shadow: inset 0 0 0 1px var(--border);
}
.notify-status-head {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.notify-status-head > svg {
  flex-shrink: 0;
  color: var(--faint);
}
.notify-status.is-on .notify-status-head > svg {
  color: var(--mint);
}
.notify-status.is-warn .notify-status-head > svg {
  color: var(--yellow);
}
.notify-status-head .setting-copy {
  min-width: 12rem;
}
.notify-facts {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 20px;
  margin: 0;
  font-size: 12px;
}
.notify-facts dt {
  color: var(--faint);
}
.notify-facts dd {
  margin: 0;
  color: var(--text);
}
.notify-test-result {
  margin: -4px 0 0;
  font-size: 12px;
  color: var(--mint);
}
.notify-test-result.is-error {
  color: var(--danger);
}
</style>
