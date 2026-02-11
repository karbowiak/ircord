<template>
  <div v-if="open" class="modal-layer" @click.self="onBackdropClick">
    <div
      ref="panelRef"
      class="modal-shell"
      :class="[`modal-shell--${size}`]"
      role="dialog"
      aria-modal="true"
      :aria-label="ariaLabel"
      tabindex="-1"
      @keydown.esc.prevent="emit('close')"
    >
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
const props = withDefaults(defineProps<{
  open: boolean
  ariaLabel: string
  size?: 'sm' | 'md' | 'lg' | 'xl'
  closeOnBackdrop?: boolean
}>(), {
  size: 'md',
  closeOnBackdrop: true,
})

const emit = defineEmits<{
  close: []
}>()

const panelRef = ref<HTMLElement | null>(null)

watch(() => props.open, async (open) => {
  if (!open) return
  await nextTick()
  panelRef.value?.focus()
})

function onBackdropClick() {
  if (!props.closeOnBackdrop) return
  emit('close')
}
</script>

<style scoped>
.modal-layer {
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background: rgba(0, 0, 0, 0.58);
}

.modal-shell {
  width: min(640px, 100%);
  max-height: min(88vh, 100%);
  overflow-y: auto;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
  background: linear-gradient(180deg, rgba(43, 46, 53, 0.98), rgba(37, 40, 47, 0.98));
  box-shadow: 0 26px 50px rgba(0, 0, 0, 0.44);
  padding: 16px;
  outline: none;
}

.modal-shell--sm {
  width: min(520px, 100%);
}

.modal-shell--md {
  width: min(640px, 100%);
}

.modal-shell--lg {
  width: min(860px, 100%);
}

.modal-shell--xl {
  width: min(1160px, 100%);
}

@media (max-width: 680px) {
  .modal-layer {
    padding: 12px;
  }

  .modal-shell {
    padding: 12px;
    max-height: min(92vh, 100%);
  }
}
</style>
