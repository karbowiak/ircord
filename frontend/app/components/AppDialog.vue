<script setup lang="ts">
withDefaults(defineProps<{ title: string; wide?: boolean; panel?: boolean }>(), {
  wide: false,
  panel: false,
})
const emit = defineEmits<{ close: [] }>()
const dialog = ref<HTMLDialogElement | null>(null)
const titleId = useId()
// Focus returns to whatever opened the dialog (the composer keeps its caret), unless the dialog
// moved it somewhere on purpose or that control is gone.
const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null
const field =
  opener instanceof HTMLTextAreaElement || opener instanceof HTMLInputElement ? opener : null
const caret = field && { start: field.selectionStart, end: field.selectionEnd, value: field.value }
onMounted(() => dialog.value?.showModal())
onBeforeUnmount(() => {
  const focus = document.activeElement
  // The browser may already have returned focus on close, but not always the caret.
  const stranded =
    !focus || focus === document.body || focus === opener || !!dialog.value?.contains(focus)
  if (stranded && opener?.isConnected && !opener.closest('dialog:not([open])')) {
    opener.focus({ preventScroll: true })
    // Only the same text gets its caret back (search may have switched conversations).
    if (caret?.start != null && field?.value === caret.value)
      field.setSelectionRange(caret.start, caret.end)
  }
})
function backdrop(event: MouseEvent) {
  if (event.target !== dialog.value) return
  const box = dialog.value!.getBoundingClientRect()
  if (
    event.clientX < box.left ||
    event.clientX > box.right ||
    event.clientY < box.top ||
    event.clientY > box.bottom
  )
    dialog.value?.close()
}
</script>
<template>
  <Teleport to="body"
    ><dialog
      ref="dialog"
      class="app-dialog"
      :class="{ 'dialog-wide': wide, 'dialog-panel': panel }"
      :aria-labelledby="titleId"
      @close="emit('close')"
      @click="backdrop"
    >
      <header class="dialog-heading">
        <h2 :id="titleId">{{ title }}</h2>
        <button
          class="icon-button"
          aria-label="Close dialog"
          @click="dialog?.close()"
        >
          <AppIcon
            name="close"
            :size="20"
          />
        </button>
      </header>
      <slot /></dialog
  ></Teleport>
</template>
