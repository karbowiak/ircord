<script setup lang="ts">
/** An on/off switch. Name it with `aria-label` or a `<label for>`; attributes go to the button. */
defineOptions({ inheritAttrs: false })
const props = withDefaults(defineProps<{ disabled?: boolean }>(), { disabled: false })
const attrs = useAttrs()
const model = defineModel<boolean>({ required: true })
const emit = defineEmits<{ change: [value: boolean] }>()
const describedBy = useSettingDescription()
function set(next: boolean) {
  if (props.disabled || next === model.value) return
  model.value = next
  emit('change', next)
}
</script>
<template>
  <UiSwitch
    v-bind="attrs"
    :model-value="model"
    :aria-describedby="
      [attrs['aria-describedby'], describedBy].filter(Boolean).join(' ') || undefined
    "
    :disabled="disabled"
    @update:model-value="set"
  />
</template>
