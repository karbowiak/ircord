<script setup vapor lang="ts">
import type { Device } from '../../../shared/utils/devices'
import { errorText } from '../../utils/api'

const props = withDefaults(defineProps<{ device: Device; delivery?: boolean; name?: boolean }>(), {
  delivery: false,
  name: false,
})
const notifications = useNotificationStore()
const { t } = useI18n()
const $t = t
const busy = ref(false)
const error = ref('')
const nameValue = ref(props.device.name ?? '')
watch(
  () => props.device.name,
  (value) => (nameValue.value = value ?? ''),
)
const contentOptions = computed(
  () =>
    [
      { value: 'inherit', label: t('devices.preferences.inherit') },
      { value: 'all', label: t('notifications.levels.all') },
      { value: 'mentions', label: t('notifications.levels.mentions') },
      { value: 'none', label: t('notifications.levels.nothing') },
    ] as const,
)
const deliveryOptions = computed(
  () =>
    [
      { value: 'off', label: t('notifications.device.modes.off') },
      { value: 'open', label: t('notifications.device.modes.open') },
      { value: 'push', label: t('devices.preferences.push') },
    ] as const,
)
const soundOptions = computed(
  () =>
    [
      { value: 'off', label: t('notifications.levels.off') },
      { value: 'mentions', label: t('notifications.levels.mentions') },
      { value: 'all', label: t('notifications.levels.all') },
    ] as const,
)

async function save(changes: Partial<Device['notifications']> & { name?: string }) {
  busy.value = true
  error.value = ''
  try {
    await notifications.saveDevice(props.device, changes)
  } catch (cause) {
    if (errorStatus(cause) === 409) {
      error.value = t('devices.preferences.changed')
    } else {
      error.value = t('devices.preferences.saveFailed', {
        error: errorText(cause, (key) => t(key)),
      })
    }
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <div class="grid gap-3">
    <FieldHint v-if="!device.notifications?.initialized">{{
      $t('devices.preferences.notSetUp')
    }}</FieldHint>
    <template v-if="device.notifications">
      <form
        v-if="name"
        class="flex items-end gap-2"
        @submit.prevent="save({ name: nameValue.trim() })"
      >
        <AppInput
          v-model="nameValue"
          class="flex-1"
          :label="$t('devices.preferences.name')"
          maxlength="80"
        />
        <UiButton
          size="sm"
          variant="secondary"
          :disabled="busy || nameValue.trim() === (device.name ?? '')"
        >
          {{ $t('common.save') }}
        </UiButton>
      </form>
      <SettingRow
        :label="$t('devices.preferences.content')"
        :description="$t('devices.preferences.contentHelp')"
        stacked
      >
        <AppSegmented
          :model-value="device.notifications.content"
          :options="contentOptions"
          :label="$t('devices.preferences.content')"
          :disabled="busy"
          @update:model-value="save({ content: $event })"
        />
      </SettingRow>
      <SettingRow
        v-if="delivery"
        :label="$t('devices.preferences.delivery')"
        :description="$t('devices.preferences.deliveryHelp')"
        stacked
      >
        <AppSegmented
          :model-value="device.notifications.delivery"
          :options="deliveryOptions"
          :label="$t('devices.preferences.delivery')"
          :disabled="busy"
          @update:model-value="save({ delivery: $event })"
        />
      </SettingRow>
      <SettingRow
        :label="$t('notifications.device.sound')"
        stacked
      >
        <AppSegmented
          :model-value="device.notifications.sound"
          :options="soundOptions"
          :label="$t('notifications.device.soundAria')"
          :disabled="busy"
          @update:model-value="save({ sound: $event })"
        />
      </SettingRow>
      <SettingRow
        :label="$t('devices.preferences.preview')"
        :description="$t('devices.preferences.previewHelp')"
      >
        <AppSwitch
          :model-value="device.notifications.preview"
          :aria-label="$t('devices.preferences.preview')"
          :disabled="busy"
          @update:model-value="save({ preview: $event })"
        />
      </SettingRow>
    </template>
    <UiAlert
      v-if="error"
      as="p"
      variant="destructive"
      presentation="inline"
      role="alert"
      >{{ error }}</UiAlert
    >
  </div>
</template>
