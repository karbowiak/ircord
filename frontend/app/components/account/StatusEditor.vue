<script setup lang="ts">
import { statusDeadline } from '../../../shared/utils/profile'
import { relativeTime } from '../../../shared/utils/devices'
import type { CommandArgumentSpec } from '../../../shared/utils/commands'

const props = defineProps<{ networkId: string; compact?: boolean }>()
const emit = defineEmits<{ close: [saved: boolean] }>()
const chat = useChatStore()
const auth = useAuthStore()
const { t, locale } = useI18n()
const accountId = auth.account?.id
const member = chat.selfFor(props.networkId)
const initialEmoji = member?.statusEmoji ?? ''
const initialText = member?.statusText ?? ''
const initialDeadline = member?.statusExpiresAt ?? ''
const error = ref('')
const saving = ref(false)
const editor = ref<{ focus: (index?: number) => void; suggestionsOpen: boolean } | null>(null)
const validTarget = computed(
  () =>
    auth.account?.id === accountId &&
    chat.networkId === props.networkId &&
    !!chat.selfFor(props.networkId),
)
const deadlineLabel = computed(() =>
  initialDeadline
    ? t('profileDetails.keepDeadline', {
        when: relativeTime(initialDeadline, Date.now(), locale.value, (key, values) =>
          t(`mediaUi.devices.${key}`, values ?? {}),
        ),
      })
    : t('profileDetails.clear.never'),
)
const fields = computed<CommandArgumentSpec[]>(() => [
  {
    name: 'emoji',
    type: 'emoji',
    label: 'profileDetails.statusEmoji',
    hint: 'selfMenu.hint.emoji',
    placeholder: 'profileDetails.statusEmojiPlaceholder',
    autocomplete: 'emoji',
  },
  {
    name: 'message',
    type: 'message',
    label: 'profileDetails.statusText',
    hint: 'selfMenu.hint.message',
    placeholder: 'profileDetails.statusPlaceholder',
  },
  {
    name: 'duration',
    type: 'duration',
    label: 'profileDetails.clearAfter',
    hint: 'selfMenu.hint.expiry',
    choices: [
      { value: 'keep', label: 'workspaceUi.commandEditor.keepDeadline' },
      ...['never', '30m', '1h', '4h', 'today', 'week'].map((value) => ({
        value,
        label: `profileDetails.clear.${value}`,
      })),
    ],
  },
])
const initial = { emoji: initialEmoji, message: initialText, duration: 'keep' }
defineExpose({
  focus: () => editor.value?.focus(),
  get suggestionsOpen() {
    return editor.value?.suggestionsOpen ?? false
  },
})
async function save(values: Record<string, string>, clear = false) {
  if (saving.value || !validTarget.value) return
  const emoji = values.emoji ?? ''
  const message = values.message ?? ''
  const bytes = (value: string) => new TextEncoder().encode(value).length
  error.value = /[\p{Cc}\p{Cs}]/u.test(message + emoji)
    ? t('selfMenu.invalidCharacters')
    : bytes(message) > 200
      ? t('selfMenu.textTooLong')
      : bytes(emoji) > 64
        ? t('selfMenu.emojiTooLong')
        : ''
  if (!clear && error.value) return
  const input: { statusEmoji?: string; statusText?: string; statusExpiresAt?: string } = {}
  if (clear) Object.assign(input, { statusEmoji: '', statusText: '', statusExpiresAt: '' })
  else {
    if (emoji !== initialEmoji) input.statusEmoji = emoji
    if (message !== initialText) input.statusText = message
    if (!emoji && !message) input.statusExpiresAt = ''
    else if (values.duration !== 'keep')
      input.statusExpiresAt = statusDeadline(values.duration ?? '') ?? ''
    else if (emoji !== initialEmoji || message !== initialText) input.statusExpiresAt = ''
  }
  if (!Object.keys(input).length) return emit('close', true)
  saving.value = true
  try {
    await chat.networkProfile.mutateAsync({ id: props.networkId, input })
    emit('close', true)
  } catch (e) {
    error.value = errorText(e, (key) => t(key))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div
    data-status-editor
    :data-status-compact="compact || undefined"
    :aria-label="t('selfMenu.setStatus')"
  >
    <CommandEditor
      ref="editor"
      name="status"
      :fields="fields"
      :initial="initial"
      :compact="compact"
      :busy="saving"
      :error="error || (!validTarget ? t('selfMenu.networkChanged') : '')"
      :submit-label="t('common.save')"
      :busy-label="t('common.saving')"
      :title="t('selfMenu.setStatus')"
      :keep-label="deadlineLabel"
      :changed-keep-label="t('selfMenu.changedDeadline')"
      @submit="save($event)"
      @cancel="emit('close', false)"
    >
      <template #actions
        ><UiButton
          v-if="!compact && (initialEmoji || initialText)"
          type="button"
          variant="ghost"
          size="sm"
          class="mr-auto"
          :disabled="saving || !validTarget"
          @click="save(initial, true)"
          >{{ t('selfMenu.clearStatus') }}</UiButton
        ></template
      >
    </CommandEditor>
  </div>
</template>
