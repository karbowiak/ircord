<script setup vapor lang="ts" generic="T extends string | number">
import { SelectGroup, SelectLabel, type AcceptableValue } from 'reka-ui'
import type { ClassValue } from 'clsx'
import { cn } from '../../utils/cn'
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
  /** A heading the option is listed under; consecutive options with one group share it. */
  group?: string
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
  { placeholder: undefined, disabled: false, invalid: false, native: null },
)
const { t } = useI18n()
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
const placeholderText = computed(() => props.placeholder ?? t('common.choose'))

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
/** Runs of options under one heading (one unnamed run when no option has a group). */
const sections = computed(() => {
  const out: { name: string; options: { option: SelectOption<T>; index: number }[] }[] = []
  props.options.forEach((option, index) => {
    const name = option.group ?? ''
    const last = out.at(-1)
    if (last && last.name === name) last.options.push({ option, index })
    else out.push({ name, options: [{ option, index }] })
  })
  return out
})
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
      cn('app-select-native control-input max-[700px]:text-[16px]', attrs.class as ClassValue),
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
      {{ placeholderText }}
    </option>
    <template
      v-for="(section, s) in sections"
      :key="s"
    >
      <optgroup
        v-if="section.name"
        :label="section.name"
      >
        <option
          v-for="{ option, index } in section.options"
          :key="index"
          :value="String(option.value)"
          :selected="option.value === model"
          :disabled="option.disabled"
        >
          {{ option.label }}
        </option>
      </optgroup>
      <template v-else>
        <option
          v-for="{ option, index } in section.options"
          :key="index"
          :value="String(option.value)"
          :selected="option.value === model"
          :disabled="option.disabled"
        >
          {{ option.label }}
        </option>
      </template>
    </template>
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
        >{{ options[selectedIndex]?.label ?? placeholderText }}</span
      >
      <AppIcon
        name="down"
        :size="15"
        class="app-select-chevron shrink-0 text-muted-foreground [transition:transform_0.15s] group-data-[state=open]:rotate-180"
      />
    </UiSelectTrigger>
    <UiSelectContent class="app-select-list">
      <template
        v-for="(section, s) in sections"
        :key="s"
      >
        <SelectGroup v-if="section.name">
          <SelectLabel
            class="app-select-group px-2 pt-2 pb-1 text-[length:var(--ui-caption)] font-[550] text-muted-foreground"
            >{{ section.name }}</SelectLabel
          >
          <UiSelectItem
            v-for="{ option, index } in section.options"
            :key="index"
            :value="String(index)"
            :disabled="option.disabled"
            class="app-select-option"
          >
            {{ option.label }}
          </UiSelectItem>
        </SelectGroup>
        <template v-else>
          <UiSelectItem
            v-for="{ option, index } in section.options"
            :key="index"
            :value="String(index)"
            :disabled="option.disabled"
            class="app-select-option"
          >
            {{ option.label }}
          </UiSelectItem>
        </template>
      </template>
    </UiSelectContent>
  </UiSelect>
</template>
