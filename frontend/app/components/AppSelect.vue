<script setup lang="ts" generic="T extends string | number">
import type { AcceptableValue } from 'reka-ui'
import type { ClassValue } from 'clsx'
/**
 * A select-only combobox: a button showing the current choice and a listbox popup (Reka Select,
 * portalled above dialogs and scroll containers). On touch screens the native select is used
 * instead, which the platform presents better.
 *
 * Attributes such as `id`, `aria-label` and `aria-describedby` go to the button (or the
 * native select), so `<label for>` and AppField work as with any form control.
 */
export interface SelectOption<V> {
  value: V
  label: string
  disabled?: boolean
}
defineOptions({ inheritAttrs: false })
const props = withDefaults(
  defineProps<{
    options: readonly SelectOption<T>[]
    placeholder?: string
    disabled?: boolean
    invalid?: boolean
    /** Forces the native select (or the custom popup), instead of choosing by pointer type. */
    native?: boolean | null
  }>(),
  { placeholder: 'Choose…', disabled: false, invalid: false, native: null },
)
const model = defineModel<T | null>({ default: null })
const emit = defineEmits<{ change: [value: T] }>()
const attrs = useAttrs()
// Utility overrides from callers must win over the primitive's defaults. Forward all other
// native attributes unchanged; Vue's normal class concatenation cannot resolve utility clashes.
const controlAttrs = () => {
  const { class: _class, ...native } = attrs
  return native
}

const coarse = import.meta.client && window.matchMedia?.('(pointer: coarse)').matches
const useNative = computed(() => props.native ?? !!coarse)

// A field's error (aria-invalid from AppField) counts as invalid too.
const isInvalid = computed(
  () => props.invalid || attrs['aria-invalid'] === true || attrs['aria-invalid'] === 'true',
)
const selectedIndex = computed(() => props.options.findIndex((o) => o.value === model.value))

function choose(index: number) {
  const option = props.options[index]
  if (!option || option.disabled) return
  // Picking the current value again is not a change.
  if (option.value === model.value) return
  model.value = option.value
  emit('change', option.value)
}
// Reka keys items by string, and an empty string can't be one; options go by their index.
const selectedKey = computed(() =>
  selectedIndex.value < 0 ? undefined : String(selectedIndex.value),
)
const chooseKey = (key: AcceptableValue) => choose(Number(key))
function onNativeChange(event: Event) {
  const value = (event.target as HTMLSelectElement).value
  choose(props.options.findIndex((o) => String(o.value) === value))
}
</script>

<template>
  <select
    v-if="useNative"
    v-bind="controlAttrs()"
    :class="[
      cn('app-select-native control-input', attrs.class as ClassValue),
      { invalid: isInvalid },
    ]"
    :disabled="disabled"
    :aria-invalid="isInvalid || undefined"
    @change="onNativeChange"
  >
    <option
      v-if="model == null"
      value=""
      disabled
      selected
    >
      {{ placeholder }}
    </option>
    <option
      v-for="(option, index) in options"
      :key="index"
      :value="String(option.value)"
      :selected="option.value === model"
      :disabled="option.disabled"
    >
      {{ option.label }}
    </option>
  </select>
  <UiSelect
    v-else
    :model-value="selectedKey"
    :disabled="disabled"
    @update:model-value="chooseKey"
  >
    <UiSelectTrigger
      v-bind="controlAttrs()"
      :class="cn('app-select', attrs.class as ClassValue)"
      :aria-invalid="isInvalid || undefined"
    >
      <span
        class="app-select-value flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap"
        :class="{ 'text-faint': selectedIndex < 0 }"
        >{{ options[selectedIndex]?.label ?? placeholder }}</span
      >
      <AppIcon
        name="down"
        :size="15"
        class="app-select-chevron shrink-0 text-muted-foreground [transition:transform_0.15s] group-data-[state=open]:rotate-180"
      />
    </UiSelectTrigger>
    <UiSelectContent class="app-select-list">
      <UiSelectItem
        v-for="(option, index) in options"
        :key="index"
        :value="String(index)"
        :disabled="option.disabled"
        class="app-select-option"
      >
        {{ option.label }}
      </UiSelectItem>
    </UiSelectContent>
  </UiSelect>
</template>
