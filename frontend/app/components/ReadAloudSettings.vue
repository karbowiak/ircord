<script setup lang="ts">
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

const baseUrl = ref('')
const model = ref('')
const voice = ref('')
const apiKey = ref('')
const error = ref('')
const status = ref('')
const testError = ref('')
const testStatus = ref('')
const testDescription = computed(
  () =>
    testError.value ||
    testStatus.value ||
    (dirty.value
      ? 'Save your changes first.'
      : needsReload.value
        ? 'Reload IRCord to connect to the new address first.'
        : 'Reads a short sample with the saved service.'),
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
  trimmedUrl.value && !speechOrigin(trimmedUrl.value)
    ? 'Use https://. Plain http:// works only for a server on this computer (localhost). No login, query or spaces.'
    : '',
)

function save() {
  error.value = status.value = ''
  submitted.value = false
  const next = { baseUrl: trimmedUrl.value, model: model.value.trim(), voice: voice.value.trim() }
  if (!speechOrigin(next.baseUrl) || !next.model || !next.voice) {
    error.value = 'Enter a valid service URL, a model and a voice.'
    return
  }
  if (apiKey.value.trim()) {
    if (!sanitizeSpeechKey(apiKey.value)) {
      error.value = 'The API key can’t contain spaces and is at most 500 characters.'
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
  if (!submitted.value) return status.value
  if (preferences.saveState === 'saving') return 'Saving…'
  if (preferences.saveState === 'failed') return ''
  return needsReload.value ? 'Saved. Reload IRCord to connect to the new address.' : 'Saved.'
})
const saveError = computed(() =>
  submitted.value && preferences.saveState === 'failed'
    ? 'Couldn’t save the speech service to your account. Try again.'
    : error.value,
)
/** A reload reads the service from the server, so it waits for the save to land. */
const canReload = computed(
  () => preferences.saveState !== 'saving' && preferences.saveState !== 'failed',
)
async function test() {
  testError.value = testStatus.value = ''
  try {
    await reader.test('This is how IRCord reads your highlights aloud.')
    testStatus.value = 'Played a sample.'
  } catch (e) {
    testError.value =
      (e as Error)?.name === 'NotAllowedError'
        ? 'Your browser blocked the audio. Try again.'
        : `Couldn’t read the sample: ${(e as Error)?.message ?? 'unknown error'}`
  }
}
async function remove() {
  if (
    !(await confirm({
      title: 'Remove the speech service?',
      body: 'Its address is removed from your account and this device forgets its API key. Highlights are no longer read aloud.',
      confirm: 'Remove',
      danger: true,
    }))
  )
    return
  reader.setEnabled(false)
  reader.setSpeechKey('')
  preferences.update({ speech: { ...emptySpeechService } })
  submitted.value = false
  status.value = 'Removed.'
}
const reload = () => location.reload()
const forgetKey = () => reader.setSpeechKey('')
const { baseUrl: urlMax, name: nameMax, key: keyMax } = speechLimits
</script>
<template>
  <div class="appearance-settings grid gap-6">
    <SettingGroup title="This device">
      <SettingRow
        data-setting="read-aloud-device"
        label="Read highlights aloud"
        :description="
          !configured
            ? 'Set up a speech service below first.'
            : 'Messages with @ and your nick, or a highlight word, are spoken, one at a time. Not your own, ignored people’s, muted conversations’ or during quiet hours.'
        "
      >
        <AppSwitch
          :model-value="readAloudOn"
          aria-label="Read highlights aloud"
          :disabled="!readAloudOn && !configured"
          @update:model-value="reader.setEnabled($event)"
        />
      </SettingRow>
      <SettingNote as="p"
        ><AppIcon
          name="volume"
          :size="15"
        /><span
          >Also switched with the speaker button beside Settings, under the conversation list.</span
        ></SettingNote
      >
    </SettingGroup>

    <SettingGroup
      data-setting="speech-service"
      title="Speech service"
      :scope="scopeLabels.account"
    >
      <SettingNote as="p"
        ><AppIcon
          name="lock"
          :size="15"
        /><span
          >This browser calls the service directly. The address, model and voice follow your
          account; the API key stays on this device and is sent only to the service. The service
          must allow requests from browsers (CORS).</span
        ></SettingNote
      >
      <form
        class="speech-service-form grid gap-3.5"
        @submit.prevent="save"
      >
        <AppInput
          v-model="baseUrl"
          label="Service URL"
          id="speech-base-url"
          type="url"
          inputmode="url"
          autocomplete="off"
          spellcheck="false"
          :maxlength="urlMax"
          placeholder="https://api.openai.com/v1"
          required
          help="Any OpenAI-compatible text-to-speech API; IRCord sends the text to its /audio/speech. A server on your network needs https. For one on this computer (http://localhost), the browser may ask to allow access to local network devices."
          :error="urlError || undefined"
        />
        <AppInput
          v-model="apiKey"
          label="API key"
          id="speech-api-key"
          type="password"
          autocomplete="off"
          spellcheck="false"
          :maxlength="keyMax"
          :placeholder="speechKey ? 'Saved on this device. Leave empty to keep it.' : 'Optional'"
          help="Kept in this browser only; each device needs its own. Removed when you log out."
        />
        <AppInput
          v-model="model"
          label="Model"
          id="speech-model"
          autocomplete="off"
          spellcheck="false"
          :maxlength="nameMax"
          placeholder="tts-1"
          required
        />
        <AppInput
          v-model="voice"
          label="Voice"
          id="speech-voice"
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
            Forget key
          </UiButton>
          <UiButton
            size="sm"
            :disabled="!dirty || !!urlError || !baseUrl.trim() || !model.trim() || !voice.trim()"
          >
            Save
          </UiButton>
        </div>
      </form>
      <SettingRow
        data-setting="speech-test"
        label="Test"
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
          Reload
        </UiButton>
        <UiButton
          v-else
          variant="secondary"
          size="sm"
          :disabled="!configured || dirty || speaking"
          :aria-busy="speaking || undefined"
          @click="test"
        >
          {{ speaking ? 'Reading…' : 'Test' }}
        </UiButton>
      </SettingRow>
      <SettingRow
        data-setting="speech-remove"
        label="Remove"
        description="Removes the service from your account and forgets this device’s key."
      >
        <UiButton
          variant="secondary-destructive"
          size="sm"
          :disabled="!configured && !speechKey"
          @click="remove"
        >
          Remove
        </UiButton>
      </SettingRow>
    </SettingGroup>
  </div>
</template>
