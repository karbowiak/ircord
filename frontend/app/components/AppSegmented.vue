<script setup lang="ts" generic="T extends string | number">
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
const buttons = ref<HTMLButtonElement[]>([])
const describedBy = useSettingDescription()
const focusable = computed(() => {
  const chosen = props.options.findIndex((o) => o.value === model.value && !o.disabled)
  return chosen >= 0 ? chosen : props.options.findIndex((o) => !o.disabled)
})
function choose(option: SegmentedOption<T>) {
  if (option.disabled || props.disabled) return
  if (option.value === model.value) {
    if (props.reselect) emit('change', option.value)
    return
  }
  model.value = option.value
  emit('change', option.value)
}
function onKeydown(event: KeyboardEvent, index: number) {
  const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[event.key]
  const edge = event.key === 'Home' ? 0 : event.key === 'End' ? props.options.length - 1 : null
  if (!step && edge === null) return
  event.preventDefault()
  const n = props.options.length
  const direction = step ?? (edge === 0 ? 1 : -1)
  let next = edge ?? index + direction
  for (let tries = 0; tries < n; tries++, next += direction) {
    next = (next + n) % n
    if (!props.options[next]!.disabled) break
  }
  const option = props.options[next]!
  if (option.disabled) return
  choose(option)
  buttons.value[next]?.focus()
}
</script>
<template>
  <div
    class="segmented"
    role="radiogroup"
    :aria-label="label"
    :aria-describedby="describedBy"
    :aria-disabled="disabled || undefined"
  >
    <button
      v-for="(option, index) in options"
      :key="String(option.value)"
      ref="buttons"
      type="button"
      role="radio"
      :class="{ active: option.value === model }"
      :aria-checked="option.value === model"
      :tabindex="index === focusable ? 0 : -1"
      :title="option.title"
      :disabled="disabled || option.disabled"
      @click="choose(option)"
      @keydown="onKeydown($event, index)"
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
    </button>
  </div>
</template>
