<script setup vapor lang="ts">
/**
 * Settings → Read aloud: whether this device reads highlights aloud, with the voice model on
 * this device (WebGPU) or with the OpenAI-compatible speech service this browser calls. The
 * service's address, model and voice follow the account; the API key is kept on this device only
 * and is sent to nothing but the service itself. The voice model's switch, voice and download
 * are per device.
 */
import {
  emptySpeechService,
  sanitizeSpeechKey,
  speechConfigured,
  speechLimits,
  speechOrigin,
} from '../../../shared/speech-service'
import {
  formatDownloadSize,
  localSpeechModel,
  localSpeechPace,
  localSwitchFor,
  localVoiceOptions,
  speechEngine,
  type SpeechEngine,
} from '../../../shared/local-speech'

const reader = useReadAloud()
const { enabled: readAloudOn, speaking, speechKey, ready } = reader
const local = useLocalSpeech()
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
    await reader.test(t('readAloud.sample'), 'service')
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

// ---- Voice model on this device (#204) ---------------------------------------------------------
// Checking support and the browser's copy loads nothing; only Download or Test does.
onMounted(() => local.refresh())
const localOn = local.on
const modelSize = formatDownloadSize(localSpeechModel.downloadBytes)
const infoOpen = ref(false)
const slow = computed(() => localSpeechPace(local.speed.value) === 'slow')
/** Why WebGPU can't be chosen (or the model is unusable), in place of the selector's help. */
const localReason = computed(() => {
  const support = local.support.value
  if (support === 'checking') return t('readAloud.local.checking')
  if (support === 'no-webgpu') return t('readAloud.local.unavailable.noWebgpu')
  if (support === 'mobile') return t('readAloud.local.unavailable.mobile')
  if (localOn.value && slow.value) return t('readAloud.local.unavailable.slow')
  return ''
})
/**
 * Which voice read-aloud uses. Only this choice changes: the speech service's fields and the
 * voice model's voice and download stay stored while the other one is chosen.
 */
const engine = computed(() => speechEngine(local.active.value))
/** API can always be chosen; WebGPU only where the model can run. */
const engineOptions = computed(() => [
  {
    value: 'webgpu' as const,
    label: t('readAloud.engine.webgpu'),
    disabled: !localOn.value && local.support.value !== 'available',
  },
  { value: 'api' as const, label: t('readAloud.engine.api') },
])
function setEngine(value: SpeechEngine) {
  setLocal(localSwitchFor(value))
}
function setLocal(value: boolean) {
  local.setOn(value)
  // The downsides open by themselves the first time, before anything is downloaded.
  if (value && !local.infoSeen.value) {
    infoOpen.value = true
    local.markInfoSeen()
  }
}
const highlightsDescription = computed(() => {
  if (ready.value) return t('readAloud.highlightsHelp')
  if (!local.active.value) return t('readAloud.setUpFirst')
  if (local.support.value === 'no-webgpu' || local.support.value === 'mobile')
    return t('readAloud.local.notReady.unavailable')
  if (slow.value) return t('readAloud.local.notReady.slow')
  return t('readAloud.local.notReady.download')
})
const voiceOptions = computed(() => localVoiceOptions((key, values) => t(key, values ?? {})))

/** "18 MB/s · about 30 s left", once the rate is known. */
function seconds(value: number) {
  return value >= 90
    ? t('readAloud.local.minutes', { count: String(Math.round(value / 60)) })
    : t('readAloud.local.seconds', { count: String(Math.max(1, Math.round(value))) })
}
const modelError = ref('')
const modelStatus = computed(() => {
  if (local.support.value === 'no-webgpu' || local.support.value === 'mobile')
    return t('readAloud.local.state.unavailable')
  if (local.phase.value === 'loading') {
    const p = local.progress.value
    if (!p || local.stored.value === 'stored' || p.fetched === 0)
      return local.stored.value === 'stored' || (p && p.loaded >= p.total)
        ? t('readAloud.local.state.loading')
        : t('readAloud.local.state.starting')
    const values = {
      loaded: formatDownloadSize(p.loaded),
      total: formatDownloadSize(p.total),
    }
    if (!p.rate) return t('readAloud.local.state.downloading', values)
    return t('readAloud.local.state.downloadingRate', {
      ...values,
      rate: formatDownloadSize(p.rate),
      left: seconds((p.total - p.loaded) / p.rate),
    })
  }
  if (modelError.value) return modelError.value
  if (local.failure.value) return t('readAloud.local.startFailed', { message: local.failure.value })
  if (local.stored.value === 'stored')
    return slow.value
      ? t('readAloud.local.state.slow')
      : t('readAloud.local.state.stored', { size: modelSize })
  if (local.stored.value === 'partial')
    return t('readAloud.local.state.partial', {
      loaded: formatDownloadSize(local.storedBytes.value),
      total: modelSize,
    })
  if (local.stored.value === 'absent') return t('readAloud.local.state.absent', { size: modelSize })
  return t('readAloud.local.checking')
})
const downloadPercent = computed(() => {
  const p = local.progress.value
  return p && p.total ? Math.min(100, Math.round((p.loaded / p.total) * 100)) : 0
})
const canRun = computed(() => local.support.value === 'available')
async function download() {
  modelError.value = ''
  try {
    await local.download(readAloudOn.value)
  } catch (e) {
    if ((e as Error)?.name !== 'AbortError') modelError.value = (e as Error)?.message ?? ''
  }
}
async function removeModel() {
  if (
    !(await confirm({
      title: t('readAloud.local.removeConfirm.title'),
      body: t('readAloud.local.removeConfirm.body', { size: modelSize }),
      confirm: t('common.remove'),
      danger: true,
    }))
  )
    return
  // Nothing is left to read with: switch off rather than fail on every highlight.
  if (local.active.value) reader.setEnabled(false)
  modelError.value = ''
  await local.remove()
}
const localTestError = ref('')
const localTestStatus = ref('')
const localTestDescription = computed(
  () =>
    localTestError.value ||
    localTestStatus.value ||
    (local.stored.value === 'stored'
      ? t('readAloud.local.test.help')
      : t('readAloud.local.test.downloads', { size: modelSize })),
)
async function localTest() {
  localTestError.value = localTestStatus.value = modelError.value = ''
  try {
    await reader.test(t('readAloud.sample'))
    localTestStatus.value = t('readAloud.played')
  } catch (e) {
    const error = e as Error
    if (error?.name === 'AbortError') return
    localTestError.value =
      error?.name === 'NotAllowedError'
        ? t('readAloud.audioBlocked')
        : error?.name === 'LocalSpeechError'
          ? error.message
          : t('readAloud.sampleFailed', { message: error?.message ?? t('readAloud.unknownError') })
  }
}
</script>
<template>
  <div class="appearance-settings grid gap-6">
    <SettingGroup :title="$t('readAloud.groups.device')">
      <SettingRow
        data-setting="read-aloud-device"
        :label="$t('readAloud.highlights')"
        :description="highlightsDescription"
      >
        <AppSwitch
          :model-value="readAloudOn"
          :aria-label="$t('readAloud.highlights')"
          :disabled="!readAloudOn && !ready"
          @update:model-value="reader.setEnabledByUser($event)"
        />
      </SettingRow>
      <SettingRow
        v-if="local.operatorOn"
        data-setting="local-speech"
        :label="$t('readAloud.engine.label')"
        :description="localReason || $t('readAloud.engine.help')"
      >
        <div class="flex shrink-0 items-center gap-1.5">
          <LocalSpeechInfo v-model:open="infoOpen" />
          <AppSegmented
            :model-value="engine"
            :options="engineOptions"
            :label="$t('readAloud.engine.label')"
            @change="setEngine"
          />
        </div>
      </SettingRow>
      <SettingNote as="p"
        ><AppIcon
          name="read-aloud"
          :size="15"
        /><span>{{ $t('readAloud.topBarNote') }}</span></SettingNote
      >
    </SettingGroup>

    <SettingGroup
      v-if="local.active.value"
      data-setting="local-voice-group"
      :title="$t('readAloud.local.group')"
    >
      <SettingNote as="p"
        ><AppIcon
          name="lock"
          :size="15"
        /><span>{{ $t('readAloud.local.note') }}</span></SettingNote
      >
      <SettingRow
        data-setting="local-voice"
        :label="$t('readAloud.voice')"
      >
        <AppSelect
          class="w-auto min-w-56 max-w-72"
          :model-value="local.voice.value"
          :options="voiceOptions"
          :aria-label="$t('readAloud.voice')"
          @change="local.setVoice($event)"
        />
      </SettingRow>
      <SettingRow
        data-setting="local-model"
        :label="$t('readAloud.local.model')"
      >
        <template #description
          ><span
            role="status"
            aria-atomic="true"
            >{{ modelStatus }}</span
          ><span
            v-if="local.phase.value === 'loading' && local.progress.value?.fetched"
            class="local-model-progress mt-1.5 block h-1.5 w-full max-w-72 overflow-hidden rounded-full bg-secondary"
            role="progressbar"
            :aria-label="$t('readAloud.local.model')"
            aria-valuemin="0"
            aria-valuemax="100"
            :aria-valuenow="downloadPercent"
            ><span
              class="block h-full rounded-full bg-primary [transition:width_0.3s]"
              :style="{ width: `${downloadPercent}%` }" /></span
        ></template>
        <UiButton
          v-if="local.phase.value === 'loading'"
          variant="secondary"
          size="sm"
          @click="local.cancel()"
        >
          {{ $t('common.cancel') }}
        </UiButton>
        <UiButton
          v-else-if="local.stored.value === 'stored'"
          variant="secondary-destructive"
          size="sm"
          @click="removeModel"
        >
          {{ $t('common.remove') }}
        </UiButton>
        <UiButton
          v-else
          variant="secondary"
          size="sm"
          :disabled="!canRun || local.stored.value === 'unknown'"
          @click="download"
        >
          {{
            modelError || local.failure.value
              ? $t('readAloud.local.retry')
              : $t('readAloud.local.download')
          }}
        </UiButton>
      </SettingRow>
      <SettingRow
        data-setting="local-test"
        :label="$t('readAloud.test.label')"
      >
        <template #description
          ><span
            role="status"
            aria-atomic="true"
            >{{ localTestDescription }}</span
          ></template
        >
        <UiButton
          variant="secondary"
          size="sm"
          :disabled="!canRun || speaking || local.phase.value === 'loading'"
          :aria-busy="speaking || undefined"
          @click="localTest"
        >
          {{ speaking ? $t('readAloud.reading') : $t('readAloud.test.label') }}
        </UiButton>
      </SettingRow>
    </SettingGroup>

    <SettingGroup
      v-if="!local.active.value"
      data-setting="speech-service"
      :title="$t('readAloud.speechService')"
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
