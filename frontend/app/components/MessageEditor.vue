<script setup vapor lang="ts">
import type { Message } from '../../shared/types/chat'
import { ircToMarkdown } from '../../shared/utils/irc'

/** Discord-style inline editing of one of your messages: Enter saves, Escape cancels. */
const props = defineProps<{ message: Message }>()
const chat = useChatStore()
const { t } = useI18n()
const $t = t
const editing = useMessageEditing()
const draft = ref(ircToMarkdown(props.message.text))
const input = ref<HTMLTextAreaElement | null>(null)
const saving = ref(false)

function resize() {
  const el = input.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${Math.min(el.scrollHeight, 320)}px`
}
onMounted(() => {
  const el = input.value
  if (!el) return
  el.focus()
  el.setSelectionRange(el.value.length, el.value.length)
  resize()
})
function cancel() {
  editing.stop()
  // Back to the composer, as after sending.
  nextTick(() => document.querySelector<HTMLTextAreaElement>('.composer textarea')?.focus())
}
async function save() {
  if (saving.value) return
  if (!draft.value.trim()) {
    chat.notify(t('messageEdit.empty'))
    return
  }
  saving.value = true
  const ok = await chat.edit(props.message, draft.value)
  saving.value = false
  if (ok) cancel()
}
function keydown(event: KeyboardEvent) {
  if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
    event.preventDefault()
    void save()
  } else if (event.key === 'Escape') {
    event.preventDefault()
    event.stopPropagation()
    cancel()
  }
}
</script>
<template>
  <div class="message-editor mt-0.5 mr-0 mb-1 ml-0">
    <textarea
      ref="input"
      v-model="draft"
      class="block w-full min-h-[38px] py-2 px-3 border border-solid border-[var(--line)] rounded-md bg-secondary text-body font-chat text-[length:var(--message-size)] leading-[1.5] resize-none focus:outline-2 focus:outline-primary focus:outline-offset-[-1px]"
      :aria-label="$t('messageEdit.label')"
      rows="1"
      maxlength="4000"
      :disabled="saving"
      @input="resize"
      @keydown="keydown"
    />
    <p class="message-editor-hint mt-1 mr-0 mb-0 ml-0 text-[0.75rem] text-muted-foreground">
      <i18n-t
        keypath="messageEdit.shortcuts"
        tag="span"
      >
        <template #cancel
          ><UiButton
            variant="inline"
            size="inline"
            type="button"
            @click="cancel"
            >{{ $t('messageEdit.cancel') }}</UiButton
          ></template
        >
        <template #save
          ><UiButton
            variant="inline"
            size="inline"
            type="button"
            :disabled="saving"
            @click="save"
            >{{ $t('messageEdit.save') }}</UiButton
          ></template
        >
      </i18n-t>
    </p>
  </div>
</template>
