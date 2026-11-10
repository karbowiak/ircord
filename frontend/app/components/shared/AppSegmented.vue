<script setup vapor lang="ts" generic="T extends string | number">
import type { AcceptableValue } from 'reka-ui'
/**
 * A small set of mutually exclusive choices shown side by side: a radio group. Arrow keys move
 * between choices and select them; only the chosen one is in the tab order.
 */
export interface SegmentedOption<V> {
  value: V
  label: string
  icon?: string
  title?: string
  disabled?: boolean
}
const props = withDefaults(
  defineProps<{
    options: readonly SegmentedOption<T>[]
    /** Accessible name of the group. */
    label: string
    disabled?: boolean
    /** Choosing the current option again still emits `change` (to retry what it does). */
    reselect?: boolean
  }>(),
  { disabled: false, reselect: false },
)
const model = defineModel<T>({ required: true })
const emit = defineEmits<{ change: [value: T] }>()
const describedBy = useSettingDescription()
function choose(value: AcceptableValue) {
  const option = props.options.find((option) => option.value === value)
  if (!option || option.disabled || props.disabled || option.value === model.value) return
  model.value = option.value
  emit('change', option.value)
}
function reselectOption(option: SegmentedOption<T>) {
  if (props.reselect && !props.disabled && !option.disabled && option.value === model.value)
    emit('change', option.value)
}
</script>
<template>
  <UiRadioGroup
    class="inline-flex gap-0.5 rounded-md bg-card p-0.5 shadow-[inset_0_0_0_1px_var(--border)]"
    :model-value="model"
    :disabled="disabled"
    :aria-label="label"
    :aria-describedby="describedBy"
    :aria-disabled="disabled || undefined"
    @update:model-value="choose"
  >
    <UiRadioGroupItem
      v-for="option in options"
      :key="String(option.value)"
      variant="segment"
      :value="option.value"
      :title="option.title"
      :disabled="disabled || option.disabled"
      @select="reselectOption(option)"
    >
      <slot
        name="option"
        :option="option"
        ><AppIcon
          v-if="option.icon"
          :name="option.icon"
          :size="14"
        />{{ option.label }}</slot
      >
    </UiRadioGroupItem>
  </UiRadioGroup>
</template>
