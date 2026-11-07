<script setup lang="ts">
withDefaults(defineProps<{ title: string; wide?: boolean; panel?: boolean }>(), {
  wide: false,
  panel: false,
})
const emit = defineEmits<{ close: [] }>()
const dialog = ref<HTMLDialogElement | null>(null)
const titleId = useId()
onMounted(() => dialog.value?.showModal())
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
