<script setup lang="ts">
/** An on/off switch. Name it with `aria-label` or a `<label for>`; attributes go to the button. */
withDefaults(defineProps<{ disabled?: boolean }>(), { disabled: false })
const model = defineModel<boolean>({ required: true })
const emit = defineEmits<{ change: [value: boolean] }>()
const describedBy = useSettingDescription()
function toggle() {
  // Controlled use only emits; the prop updates on the parent's next render, so don't read it back.
  const next = !model.value
  model.value = next
  emit('change', next)
}
</script>
<template>
  <button
    type="button"
    role="switch"
    class="toggle relative h-5 w-9 shrink-0 rounded-[10px] bg-border transition-[background] duration-150 aria-checked:bg-primary"
    :class="{ on: model }"
    :aria-checked="model"
    :aria-describedby="describedBy"
    :disabled="disabled"
    @click="toggle"
  >
    <span
      class="absolute top-0.5 left-0.5 size-4 rounded-full bg-foreground transition-transform duration-150 in-aria-checked:translate-x-4 in-aria-checked:bg-primary-foreground"
    />
  </button>
</template>
