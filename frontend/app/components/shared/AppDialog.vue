<script setup vapor lang="ts">
const { t: $t } = useI18n()
const props = withDefaults(
  defineProps<{
    title: string
    wide?: boolean
    panel?: boolean
    beforeClose?: () => boolean
  }>(),
  {
    wide: false,
    panel: false,
  },
)
const emit = defineEmits<{ close: [] }>()
const open = ref(true)
function close() {
  if (!open.value) return
  if (props.beforeClose?.() === false) return
  open.value = false
  emit('close')
}
const unregister = useDialogStack().register(close)
// Focus returns to whatever opened the dialog (the composer keeps its caret), unless the dialog
// moved it somewhere on purpose or that control is gone.
const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null
const field =
  opener instanceof HTMLTextAreaElement || opener instanceof HTMLInputElement ? opener : null
const caret = field && { start: field.selectionStart, end: field.selectionEnd, value: field.value }
/** Like a native dialog: an `autofocus` control inside takes focus, else the first control. */
function focusInitial(event: Event) {
  const target = (event.target as HTMLElement | null)?.querySelector<HTMLElement>('[autofocus]')
  if (!target) return
  event.preventDefault()
  target.focus({ preventScroll: true })
}
// After unmount, once the focus trap has gone; while it is mounted it would pull focus back in.
onUnmounted(() => {
  unregister()
  const focus = document.activeElement
  const stranded = !focus || focus === document.body || focus === opener || !focus.isConnected
  if (stranded && opener?.isConnected && !opener.closest('[role="dialog"][data-state="closed"]')) {
    opener.focus({ preventScroll: true })
    // Only the same text gets its caret back (search may have switched conversations).
    if (caret?.start != null && field?.value === caret.value)
      field.setSelectionRange(caret.start, caret.end)
  }
})
</script>
<template>
  <UiDialog
    :open="open"
    @update:open="(value) => value || close()"
  >
    <UiDialogContent
      class="app-dialog max-[700px]:h-dvh max-[700px]:max-h-dvh max-[700px]:w-screen max-[700px]:max-w-screen max-[700px]:rounded-none"
      :class="{
        'dialog-wide w-[min(calc(680px_*_var(--interface-scale,1)),calc(100vw_-_24px))]': wide,
        // A fixed frame, so moving between settings pages never resizes the dialog. Frames grow
        // with the interface scale, up to the viewport.
        'dialog-panel flex h-[min(calc(760px_*_var(--interface-scale,1)),calc(100dvh_-_48px))] max-h-none w-[min(calc(1000px_*_var(--interface-scale,1)),calc(100vw_-_24px))] flex-col overflow-hidden':
          panel,
      }"
      :aria-describedby="undefined"
      @open-auto-focus="focusInitial"
      @close-auto-focus.prevent
    >
      <header
        class="dialog-heading flex items-center justify-between pt-4 pr-3 pb-3 pl-5 gap-3"
        :class="{ 'border-b border-b-border [border-bottom-style:solid]': panel }"
      >
        <UiDialogTitle
          as="h2"
          class="text-[length:calc(17px_*_var(--interface-scale,1))] font-[650] tracking-[-0.01em]"
          >{{ title }}</UiDialogTitle
        >
        <UiButton
          variant="ghost"
          size="icon-sm"
          :aria-label="$t('common.closeDialog')"
          @click="close"
        >
          <AppIcon
            name="close"
            :size="20"
          />
        </UiButton>
      </header>
      <slot :close="close" />
    </UiDialogContent>
  </UiDialog>
</template>
