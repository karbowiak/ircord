<script setup lang="ts">
import type { Message } from '../../shared/types/chat'
import { ircToMarkdown } from '../../shared/utils/irc'

/** Discord-style inline editing of one of your messages: Enter saves, Escape cancels. */
const props = defineProps<{ message: Message }>()
const chat = useChatStore()
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
    chat.notify('A message cannot be empty. Delete it instead.')
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
  <div class="message-editor">
    <textarea
      ref="input"
      v-model="draft"
      aria-label="Edit message"
      rows="1"
      maxlength="4000"
      :disabled="saving"
      @input="resize"
      @keydown="keydown"
    />
    <p class="message-editor-hint">
      escape to <button
        type="button"
        @click="cancel"
      >
        cancel</button
      > · enter to <button
        type="button"
        :disabled="saving"
        @click="save"
      >
        save
      </button>
    </p>
  </div>
</template>
