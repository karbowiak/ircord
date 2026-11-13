<script setup vapor lang="ts">
import type { Conversation } from '../../../shared/types/chat'
import { modeInfo, parseChanmodes } from '../../../shared/utils/channel-modes'

const props = defineProps<{ networkId: string }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const chat = useChatStore()
const network = computed(() => chat.data?.networks.find((item) => item.id === props.networkId))
const canCreate = computed(() => !!network.value?.capabilities.includes('labeled-response'))
const groups = computed(() => parseChanmodes(network.value?.isupport))
const information = computed(() => modeInfo((key, values) => t(key, values ?? {})))
const common = computed(() => [...groups.value.flags].filter((mode) => 'intmsp'.includes(mode)))
const advanced = computed(() =>
  [...groups.value.flags].filter((mode) => !common.value.includes(mode)),
)
const parameters = computed(() => [...new Set(groups.value.always + groups.value.whenSet)])
const selected = ref('')
const removed = ref('')
const template = ref('private')
const name = ref('')
const topic = ref('')
const values = reactive<Record<string, string>>({})
const availability = ref<'checking' | 'available' | 'unavailable' | 'unknown' | ''>('')
const busy = ref(false)
const error = ref('')
const result = ref<{ conversation: Conversation; warnings: string[] }>()
const discardGuard = useDiscardGuard(
  () =>
    !result.value &&
    (!!name.value.trim() ||
      !!topic.value.trim() ||
      template.value !== 'private' ||
      Object.values(values).some(Boolean)),
  () => emit('close'),
)
const channelName = computed(() => `#${name.value.trim().replace(/^#/, '')}`)
const redactKey = (message: string) => {
  const key = String(values.k ?? '')
  return key ? message.replaceAll(key, '••••') : message
}
const missingModes = computed(() => {
  const desired =
    template.value === 'private' ? 'isnt' : template.value === 'announcements' ? 'mnt' : 'nt'
  return [...desired]
    .filter((mode) => !groups.value.flags.includes(mode))
    .map((mode) => `+${mode}`)
    .join(', ')
})
let timer: ReturnType<typeof setTimeout> | undefined
let controller: AbortController | undefined
let generation = 0
function applyTemplate(value: string) {
  template.value = value
  const flags = value === 'private' ? 'isnt' : value === 'announcements' ? 'mnt' : 'nt'
  selected.value = [...flags].filter((mode) => groups.value.flags.includes(mode)).join('')
  removed.value = [...'imnpst']
    .filter((mode) => groups.value.flags.includes(mode) && !flags.includes(mode))
    .join('')
}
applyTemplate('private')
function toggle(mode: string, enabled: boolean) {
  selected.value = enabled
    ? [...new Set(selected.value + mode)].join('')
    : selected.value.replace(mode, '')
  removed.value = enabled
    ? removed.value.replace(mode, '')
    : [...new Set(removed.value + mode)].join('')
}
watch(name, () => {
  clearTimeout(timer)
  controller?.abort()
  const current = ++generation
  availability.value = ''
  if (channelName.value === '#') return
  availability.value = 'checking'
  timer = setTimeout(async () => {
    controller = new AbortController()
    try {
      const response = await apiClient().channelAvailability(
        props.networkId,
        channelName.value,
        controller.signal,
      )
      if (current === generation) availability.value = response.status
    } catch {
      if (current === generation) availability.value = 'unknown'
    }
  }, 300)
})
onBeforeUnmount(() => {
  clearTimeout(timer)
  controller?.abort()
  generation++
})
async function create() {
  if (
    busy.value ||
    result.value ||
    !canCreate.value ||
    !name.value.trim() ||
    availability.value === 'unavailable'
  )
    return
  busy.value = true
  error.value = ''
  const modeParams: Record<string, string> = {}
  for (const mode of parameters.value) {
    const value = String(values[mode] ?? '').trim()
    if (value) modeParams[mode] = value
  }
  try {
    const response = await apiClient().createChannel(props.networkId, {
      name: channelName.value,
      topic: topic.value.trim() || undefined,
      flags: `${selected.value ? `+${selected.value}` : ''}${removed.value ? `-${removed.value}` : ''}`,
      modeParams,
    })
    chat.rememberCreatedChannel(response.conversation)
    if (!response.registered || !response.configured || response.warnings.length) {
      result.value = {
        conversation: response.conversation,
        warnings: response.warnings.map(redactKey),
      }
    } else {
      chat.openCreatedChannel(response.conversation)
      emit('close')
    }
  } catch (e) {
    error.value = redactKey(errorText(e, t))
  } finally {
    busy.value = false
  }
}
function openResult() {
  if (!result.value) return
  chat.openCreatedChannel(result.value.conversation)
  emit('close')
}
const label = (mode: string) =>
  information.value[mode]?.label ?? t('collectionsUi.channel.modes.serverSpecific')
const description = (mode: string) =>
  information.value[mode]?.description ?? t('collectionsUi.channel.modes.serverDescription')
</script>

<template>
  <AppDialog
    :title="t('channelCreate.title')"
    :before-close="() => !busy && discardGuard()"
    @close="emit('close')"
  >
    <form
      class="grid min-h-0 gap-4 overflow-y-auto px-5 pb-5"
      @submit.prevent="create"
    >
      <SettingsDescription>{{
        t('channelCreate.description', { network: network?.name ?? '' })
      }}</SettingsDescription>
      <FieldHint v-if="!canCreate">{{ t('channelCreate.unsupported') }}</FieldHint>
      <template v-if="!result">
        <AppField
          id="create-channel-name"
          :label="t('channelCreate.name')"
          :help="availability ? t(`channelCreate.${availability}`) : undefined"
          help-live
          required
        >
          <template #default="control"
            ><UiInput
              v-bind="control"
              v-model="name"
              autofocus
              maxlength="50"
              :disabled="busy"
          /></template>
        </AppField>
        <AppField
          id="create-channel-topic"
          :label="t('channelCreate.topic')"
        >
          <template #default="control"
            ><UiInput
              v-bind="control"
              v-model="topic"
              :disabled="busy"
          /></template>
        </AppField>
        <fieldset
          class="grid min-w-0 gap-2 border-0 p-0"
          :disabled="busy"
        >
          <legend class="mb-2 text-[13px] font-semibold">{{ t('channelCreate.template') }}</legend>
          <label
            v-for="choice in ['private', 'open', 'announcements']"
            :key="choice"
            class="flex items-center gap-2 text-[13px]"
          >
            <input
              type="radio"
              class="accent-primary"
              name="channel-template"
              :value="choice"
              :checked="template === choice"
              @change="applyTemplate(choice)"
            />
            {{ t(`channelCreate.${choice}`) }}
          </label>
        </fieldset>
        <FieldHint v-if="missingModes">{{
          t('channelCreate.missingModes', { modes: missingModes })
        }}</FieldHint>
        <fieldset
          class="grid min-w-0 gap-3 border-0 p-0"
          :disabled="busy"
        >
          <legend class="mb-2 text-[13px] font-semibold">{{ t('channelCreate.settings') }}</legend>
          <FieldHint>{{ t('channelCreate.supported') }}</FieldHint>
          <label
            v-for="mode in common"
            :key="mode"
            class="flex items-start gap-2 text-[13px]"
          >
            <input
              type="checkbox"
              class="accent-primary"
              :checked="selected.includes(mode)"
              @change="toggle(mode, ($event.target as HTMLInputElement).checked)"
            />
            <span
              ><strong>{{ label(mode) }} (+{{ mode }})</strong><br /><small
                class="text-muted-foreground"
                >{{ description(mode) }}</small
              ></span
            >
          </label>
          <AppField
            v-if="parameters.includes('k')"
            id="create-channel-key"
            :label="t('channelCreate.key')"
          >
            <template #default="control"
              ><UiInput
                v-bind="control"
                v-model="values.k"
                type="password"
                autocomplete="new-password"
            /></template>
          </AppField>
          <AppField
            v-if="parameters.includes('l')"
            id="create-channel-limit"
            :label="t('channelCreate.limit')"
          >
            <template #default="control"
              ><UiInput
                v-bind="control"
                v-model="values.l"
                type="number"
                min="1"
            /></template>
          </AppField>
          <details v-if="advanced.length || parameters.some((mode) => !'kl'.includes(mode))">
            <summary class="cursor-pointer text-[13px] font-semibold">
              {{ t('channelCreate.advanced') }}
            </summary>
            <div class="mt-3 grid gap-3">
              <label
                v-for="mode in advanced"
                :key="mode"
                class="flex items-start gap-2 text-[13px]"
              >
                <input
                  type="checkbox"
                  class="accent-primary"
                  :checked="selected.includes(mode)"
                  @change="toggle(mode, ($event.target as HTMLInputElement).checked)"
                />
                <span
                  ><strong>{{ label(mode) }} (+{{ mode }})</strong><br /><small
                    class="text-muted-foreground"
                    >{{ description(mode) }}</small
                  ></span
                >
              </label>
              <AppField
                v-for="mode in parameters.filter((item) => !'kl'.includes(item))"
                :id="`create-mode-${mode}`"
                :key="mode"
                :label="`${label(mode)} (+${mode})`"
                :help="description(mode)"
              >
                <template #default="control"
                  ><UiInput
                    v-bind="control"
                    v-model="values[mode]"
                /></template>
              </AppField>
            </div>
          </details>
        </fieldset>
        <UiAlert
          v-if="error"
          as="p"
          variant="destructive"
          presentation="inline"
          role="alert"
          >{{ error }}</UiAlert
        >
        <div class="flex justify-end gap-2">
          <UiButton
            type="button"
            variant="secondary"
            :disabled="busy"
            @click="emit('close')"
            >{{ t('common.cancel') }}</UiButton
          >
          <UiButton
            type="submit"
            :disabled="
              busy ||
              !canCreate ||
              !name.trim() ||
              availability === 'unavailable' ||
              availability === 'checking'
            "
            >{{ busy ? t('channelCreate.creating') : t('channelCreate.create') }}</UiButton
          >
        </div>
      </template>
      <template v-else>
        <p
          role="status"
          class="text-[13px]"
        >
          {{ t('channelCreate.partial') }}
        </p>
        <ul
          v-if="result.warnings.length"
          class="list-disc pl-5 text-[13px] text-muted-foreground"
        >
          <li
            v-for="warning in result.warnings"
            :key="warning"
          >
            {{ warning }}
          </li>
        </ul>
        <div class="flex justify-end">
          <UiButton
            type="button"
            @click="openResult"
            >{{ t('channelCreate.openChannel') }}</UiButton
          >
        </div>
      </template>
    </form>
  </AppDialog>
</template>
