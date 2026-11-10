<script setup vapor lang="ts">
/** Settings → Notifications → This device: whether notifications work here, and how. */
import type { DeviceMode } from '../stores/notifications'
import { scopeLabels } from '../../shared/settings-index'
const notifications = useNotificationStore()
const install = useInstallPrompt()
const { t } = useI18n()
const $t = t
const modes = computed<{ value: DeviceMode; label: string; note: string }[]>(() => [
  {
    value: 'off',
    label: t('notifications.device.modes.off'),
    note: t('notifications.device.modes.offNote'),
  },
  {
    value: 'open',
    label: t('notifications.device.modes.open'),
    note: t('notifications.device.modes.openNote'),
  },
  {
    value: 'push',
    label: t('notifications.device.modes.push'),
    note: t('notifications.device.modes.pushNote'),
  },
])
const blocked = computed(() => notifications.permission === 'denied')
/** The summary of this device: what is set, whether it works, and the one next step. */
const summary = computed(() => {
  switch (notifications.status) {
    case 'install':
      return {
        icon: 'download',
        tone: 'warn',
        title: t('notifications.device.summary.install.title'),
        detail: t('notifications.device.summary.install.detail'),
      }
    case 'unsupported':
      return {
        icon: 'muted',
        tone: 'warn',
        title: t('notifications.device.summary.unsupported.title'),
        detail: t('notifications.device.summary.unsupported.detail'),
      }
    case 'blocked':
      return {
        icon: 'muted',
        tone: 'warn',
        title: t('notifications.device.summary.blocked.title'),
        detail: t('notifications.device.summary.blocked.detail'),
      }
    case 'off':
      return {
        icon: 'muted',
        tone: 'off',
        title: t('notifications.device.summary.off.title'),
        detail: notifications.pushAvailable
          ? t('notifications.device.summary.off.detailPush')
          : t('notifications.device.summary.off.detailOpen'),
      }
    case 'permission':
      return {
        icon: 'alert',
        tone: 'warn',
        title: t('notifications.device.summary.permission.title'),
        detail: t('notifications.device.summary.permission.detail'),
      }
    case 'open':
      return {
        icon: 'bell',
        tone: 'on',
        title: t('notifications.device.summary.open.title'),
        detail: notifications.pushAvailable
          ? t('notifications.device.summary.open.detailPush')
          : t('notifications.device.summary.open.detail'),
      }
    case 'checking':
      return {
        icon: 'clock',
        tone: 'off',
        title: t('notifications.device.summary.checking.title'),
        detail: t('notifications.device.summary.checking.detail'),
      }
    case 'push':
      return {
        icon: 'bell',
        tone: 'on',
        title: t('notifications.device.summary.push.title'),
        detail: t('notifications.device.summary.push.detail'),
      }
    case 'repair':
      return {
        icon: 'alert',
        tone: 'warn',
        title: t('notifications.device.summary.repair.title'),
        detail: t('notifications.device.summary.repair.detail'),
      }
    case 'push-unsupported':
      return {
        icon: 'cloudOff',
        tone: 'warn',
        title: t('notifications.device.summary.pushUnsupported.title'),
        detail: t('notifications.device.summary.chooseWhileOpen'),
      }
    case 'server-unavailable':
      return {
        icon: 'cloudOff',
        tone: 'warn',
        title: t('notifications.device.summary.serverUnavailable.title'),
        detail: t('notifications.device.summary.chooseWhileOpen'),
      }
  }
  return { icon: 'info', tone: 'off', title: '', detail: '' }
})
const permissionText = computed(() => {
  const labels = {
    granted: t('notifications.device.permission.granted'),
    denied: t('notifications.device.permission.denied'),
    default: t('notifications.device.permission.default'),
    unsupported: t('notifications.device.permission.unsupported'),
  }
  return labels[notifications.permission]
})
const pushText = computed(() => {
  if (notifications.needsInstall) return t('notifications.device.push.needsInstall')
  if (!notifications.supported.push) return t('notifications.device.push.unsupported')
  if (notifications.serverPush === false) return t('notifications.device.push.unavailable')
  if (notifications.mode !== 'push') return t('notifications.device.push.notInUse')
  const labels: Record<string, string> = {
    push: t('notifications.device.push.registered'),
    repair: t('notifications.device.push.repair'),
    checking: t('notifications.device.push.checking'),
    blocked: t('notifications.device.push.pausedBlocked'),
    permission: t('notifications.device.push.pausedPermission'),
  }
  return labels[notifications.status as string] ?? t('notifications.device.push.notRegistered')
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
    :title="$t('notifications.device.title')"
    :scope="$t(scopeLabels.device)"
  >
    <div
      data-setting="notifications-status"
      class="notify-status flex flex-col gap-2.5 rounded-md bg-card px-3.5 py-3 shadow-[inset_0_0_0_1px_var(--border)]"
      :class="`is-${summary.tone}`"
      role="group"
      :aria-label="$t('notifications.device.statusLabel')"
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
          {{ $t('notifications.device.enable') }}
        </UiButton>
        <UiButton
          v-else-if="notifications.status === 'permission'"
          size="sm"
          :disabled="notifications.busy"
          @click="notifications.enable()"
        >
          {{ $t('notifications.device.enable') }}
        </UiButton>
        <UiButton
          v-else-if="notifications.status === 'repair' && notifications.pushAvailable"
          size="sm"
          :disabled="notifications.busy"
          @click="notifications.repair()"
        >
          {{ $t('notifications.device.repair') }}
        </UiButton>
        <UiButton
          v-else-if="notifications.status === 'blocked'"
          variant="secondary"
          size="sm"
          @click="notifications.refresh()"
        >
          {{ $t('notifications.device.checkAgain') }}
        </UiButton>
      </div>
      <dl
        class="notify-facts m-0 flex flex-wrap gap-x-5 gap-y-1 text-[12px] [&_dd]:m-0 [&_dd]:text-foreground [&_dt]:text-faint"
      >
        <div>
          <dt>{{ $t('notifications.device.facts.delivery') }}</dt>
          <dd>{{ modes.find((m) => m.value === notifications.mode)?.label }}</dd>
        </div>
        <div>
          <dt>{{ $t('notifications.device.facts.permission') }}</dt>
          <dd>{{ permissionText }}</dd>
        </div>
        <div>
          <dt>{{ $t('notifications.device.facts.push') }}</dt>
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
        ><strong>{{ $t('notifications.device.homeScreen.title') }}</strong
        ><small>{{ $t('notifications.device.homeScreen.body') }}</small></span
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
        ><strong>{{ $t('notifications.device.installTitle') }}</strong
        ><small>{{ $t('notifications.device.installBody') }}</small></span
      >
      <UiButton
        variant="secondary"
        size="sm"
        @click="install.prompt()"
      >
        {{ $t('common.install') }}
      </UiButton>
    </div>
    <SettingRow
      data-setting="notifications-device"
      :label="$t('notifications.device.notifications')"
      stacked
      :description="
        blocked
          ? $t('notifications.device.blockedHelp')
          : !notifications.supported.notifications
            ? $t('notifications.device.unsupportedHelp')
            : $t('notifications.device.help')
      "
    >
      <AppSegmented
        :model-value="notifications.mode"
        :options="modes"
        :disabled="notifications.busy"
        :label="$t('notifications.device.statusLabel')"
        reselect
        @change="notifications.setMode($event)"
      />
    </SettingRow>
    <SettingRow
      v-if="notifications.mode === 'push'"
      data-setting="notifications-test"
      :label="$t('notifications.device.test')"
      :description="$t('notifications.device.testHelp')"
    >
      <UiButton
        variant="secondary"
        size="sm"
        :disabled="testing || notifications.status !== 'push'"
        @click="sendTest"
      >
        {{ $t('notifications.device.sendTest') }}
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
    <DeviceNotificationPreferences
      v-if="notifications.currentDevice"
      data-setting="notification-sound"
      :device="notifications.currentDevice"
    />
    <SettingNote as="p">
      <AppIcon
        name="laptop"
        :size="15"
      /><span>
        <i18n-t keypath="notifications.device.note">
          <template #link><SettingLink to="devices" /></template>
        </i18n-t>
      </span>
    </SettingNote>
  </SettingGroup>
</template>
