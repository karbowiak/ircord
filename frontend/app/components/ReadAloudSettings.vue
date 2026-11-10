<script setup vapor lang="ts">
/**
 * Settings → Read aloud: whether this device reads highlights aloud, and the OpenAI-compatible
 * speech service this browser calls. Its address, model and voice follow the account; the API
 * key is kept on this device only and is sent to nothing but the service itself.
 */
import { scopeLabels } from '../../shared/settings-index'
import {
  emptySpeechService,
  sanitizeSpeechKey,
  speechConfigured,
  speechLimits,
  speechOrigin,
} from '../../shared/speech-service'

const reader = useReadAloud()
const { enabled: readAloudOn, speaking, speechKey } = reader
const preferences = usePreferencesStore()
const saved = computed(() => preferences.preferences.speech)
const configured = computed(() => speechConfigured(saved.value))
const needsReload = computed(() => configured.value && speechNeedsReload(saved.value))
const { confirm } = usePrompt()
const { t } = useI18n()
const $t = t

const baseUrl = ref('')
const baseUrlPlaceholder = 'https://api.openai.com/v1'
const model = ref('')
const voice = ref('')
const apiKey = ref('')
const error = ref('')
const statusKey = ref('')
const testError = ref('')
const testStatusKey = ref('')
const testDescription = computed(
  () =>
    testError.value ||
    (testStatusKey.value ? t(testStatusKey.value) : '') ||
    (dirty.value
      ? t('readAloud.test.saveFirst')
      : needsReload.value
        ? t('readAloud.test.reloadFirst')
        : t('readAloud.test.help')),
)

// Fill the form from the saved service, and follow changes made elsewhere, by value: a field
// the user is editing keeps what they typed (preferences refetch and other settings replace the
// whole object, and must never wipe the form).
const fields = { baseUrl, model, voice }
watch(
  () => ({ ...saved.value }),
  (next, previous) => {
    for (const name of ['baseUrl', 'model', 'voice'] as const) {
      const field = fields[name]
      if (!previous || field.value.trim().replace(/\/+$/, '') === previous[name])
        field.value = next[name]
    }
  },
  { immediate: true, deep: true },
)
const trimmedUrl = computed(() => baseUrl.value.trim().replace(/\/+$/, ''))
const dirty = computed(
  () =>
    !!apiKey.value ||
    trimmedUrl.value !== saved.value.baseUrl ||
    model.value.trim() !== saved.value.model ||
    voice.value.trim() !== saved.value.voice,
)
const urlError = computed(() =>
  trimmedUrl.value && !speechOrigin(trimmedUrl.value) ? t('readAloud.urlError') : '',
)

function save() {
  error.value = statusKey.value = ''
  submitted.value = false
  const next = { baseUrl: trimmedUrl.value, model: model.value.trim(), voice: voice.value.trim() }
  if (!speechOrigin(next.baseUrl) || !next.model || !next.voice) {
    error.value = t('readAloud.invalidFields')
    return
  }
  if (apiKey.value.trim()) {
    if (!sanitizeSpeechKey(apiKey.value)) {
      error.value = t('readAloud.invalidKey')
      return
    }
    reader.setSpeechKey(apiKey.value)
    apiKey.value = ''
  }
  preferences.update({ speech: next })
  submitted.value = true
}
/** Shown once this form saved: only “Saved” when the server has it, since a reload reads it there. */
const submitted = ref(false)
const saveStatus = computed(() => {
  if (!submitted.value) return statusKey.value ? t(statusKey.value) : ''
  if (preferences.saveState === 'saving') return t('common.saving')
  if (preferences.saveState === 'failed') return ''
  return needsReload.value ? t('readAloud.savedReload') : t('readAloud.saved')
})
const saveError = computed(() =>
  submitted.value && preferences.saveState === 'failed' ? t('readAloud.saveFailed') : error.value,
)
/** A reload reads the service from the server, so it waits for the save to land. */
const canReload = computed(
  () => preferences.saveState !== 'saving' && preferences.saveState !== 'failed',
)
async function test() {
  testError.value = testStatusKey.value = ''
  try {
    await reader.test(t('readAloud.sample'))
    testStatusKey.value = 'readAloud.played'
  } catch (e) {
    testError.value =
      (e as Error)?.name === 'NotAllowedError'
        ? t('readAloud.audioBlocked')
        : t('readAloud.sampleFailed', {
            message: (e as Error)?.message ?? t('readAloud.unknownError'),
          })
  }
}
async function remove() {
  if (
    !(await confirm({
      title: t('readAloud.removeConfirm.title'),
      body: t('readAloud.removeConfirm.body'),
      confirm: t('common.remove'),
      danger: true,
    }))
  )
    return
  reader.setEnabled(false)
  reader.setSpeechKey('')
  preferences.update({ speech: { ...emptySpeechService } })
  submitted.value = false
  statusKey.value = 'readAloud.removed'
}
const reload = () => location.reload()
const forgetKey = () => reader.setSpeechKey('')
const { baseUrl: urlMax, name: nameMax, key: keyMax } = speechLimits
</script>
<template>
  <div class="appearance-settings grid gap-6">
    <SettingGroup :title="$t('readAloud.groups.device')">
      <SettingRow
        data-setting="read-aloud-device"
        :label="$t('readAloud.highlights')"
        :description="!configured ? $t('readAloud.setUpFirst') : $t('readAloud.highlightsHelp')"
      >
        <AppSwitch
          :model-value="readAloudOn"
          :aria-label="$t('readAloud.highlights')"
          :disabled="!readAloudOn && !configured"
          @update:model-value="reader.setEnabled($event)"
        />
      </SettingRow>
      <SettingNote as="p"
        ><AppIcon
          name="read-aloud"
          :size="15"
        /><span>{{ $t('readAloud.topBarNote') }}</span></SettingNote
      >
    </SettingGroup>

    <SettingGroup
      data-setting="speech-service"
      :title="$t('readAloud.speechService')"
      :scope="$t(scopeLabels.account)"
    >
      <SettingNote as="p"
        ><AppIcon
          name="lock"
          :size="15"
        /><span>{{ $t('readAloud.serviceNote') }}</span></SettingNote
      >
      <form
        class="speech-service-form grid gap-3.5"
        @submit.prevent="save"
      >
        <AppInput
          id="speech-base-url"
          v-model="baseUrl"
          :label="$t('readAloud.serviceUrl')"
          type="url"
          inputmode="url"
          autocomplete="off"
          spellcheck="false"
          :maxlength="urlMax"
          :placeholder="baseUrlPlaceholder"
          required
          :help="$t('readAloud.serviceUrlHelp')"
          :error="urlError || undefined"
        />
        <AppInput
          id="speech-api-key"
          v-model="apiKey"
          :label="$t('readAloud.apiKey')"
          type="password"
          autocomplete="off"
          spellcheck="false"
          :maxlength="keyMax"
          :placeholder="speechKey ? $t('readAloud.keySaved') : $t('common.optional')"
          :help="$t('readAloud.apiKeyHelp')"
        />
        <AppInput
          id="speech-model"
          v-model="model"
          :label="$t('readAloud.model')"
          autocomplete="off"
          spellcheck="false"
          :maxlength="nameMax"
          placeholder="tts-1"
          required
        />
        <AppInput
          id="speech-voice"
          v-model="voice"
          :label="$t('readAloud.voice')"
          autocomplete="off"
          spellcheck="false"
          :maxlength="nameMax"
          placeholder="alloy"
          required
          :error="saveError || undefined"
          :status="saveStatus || undefined"
        />
        <div class="form-actions flex items-center justify-end gap-2">
          <UiButton
            v-if="speechKey"
            type="button"
            variant="secondary"
            size="sm"
            @click="forgetKey"
          >
            {{ $t('readAloud.forgetKey') }}
          </UiButton>
          <UiButton
            size="sm"
            :disabled="!dirty || !!urlError || !baseUrl.trim() || !model.trim() || !voice.trim()"
          >
            {{ $t('common.save') }}
          </UiButton>
        </div>
      </form>
      <SettingRow
        data-setting="speech-test"
        :label="$t('readAloud.test.label')"
      >
        <template #description
          ><span
            role="status"
            aria-atomic="true"
            >{{ testDescription }}</span
          ></template
        >
        <UiButton
          v-if="needsReload && !dirty"
          variant="secondary"
          size="sm"
          :disabled="!canReload"
          @click="reload"
        >
          {{ $t('readAloud.reload') }}
        </UiButton>
        <UiButton
          v-else
          variant="secondary"
          size="sm"
          :disabled="!configured || dirty || speaking"
          :aria-busy="speaking || undefined"
          @click="test"
        >
          {{ speaking ? $t('readAloud.reading') : $t('readAloud.test.label') }}
        </UiButton>
      </SettingRow>
      <SettingRow
        data-setting="speech-remove"
        :label="$t('common.remove')"
        :description="$t('readAloud.removeHelp')"
      >
        <UiButton
          variant="secondary-destructive"
          size="sm"
          :disabled="!configured && !speechKey"
          @click="remove"
        >
          {{ $t('common.remove') }}
        </UiButton>
      </SettingRow>
    </SettingGroup>
  </div>
</template>
