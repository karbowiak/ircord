<script setup lang="ts" generic="T extends string | number">
/**
 * A select-only combobox: a button showing the current choice and a listbox popup. The popup
 * sits in the top layer (popover), so dialogs and scroll containers don't clip it. On touch
 * screens the native select is used instead, which the platform presents better.
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
const listId = useId()
const optionId = (index: number) => `${listId}-${index}`

const coarse = import.meta.client && window.matchMedia?.('(pointer: coarse)').matches
const useNative = computed(() => props.native ?? !!coarse)

// A field's error (aria-invalid from AppField) counts as invalid too.
const isInvalid = computed(
  () => props.invalid || attrs['aria-invalid'] === true || attrs['aria-invalid'] === 'true',
)
const selectedIndex = computed(() => props.options.findIndex((o) => o.value === model.value))
const selectedLabel = computed(() => props.options[selectedIndex.value]?.label)

const trigger = ref<HTMLButtonElement | null>(null)
const list = ref<HTMLElement | null>(null)
const open = ref(false)
const active = ref(-1)
const position = ref<Record<string, string>>({})

function choose(index: number) {
  const option = props.options[index]
  if (!option || option.disabled) return
  // Picking the current value again is not a change.
  if (option.value === model.value) return
  model.value = option.value
  emit('change', option.value)
}

function place() {
  if (!trigger.value) return
  const rect = trigger.value.getBoundingClientRect()
  const wanted = Math.min(320, props.options.length * 34 + 10)
  const p = placePopup(rect, { width: window.innerWidth, height: window.innerHeight }, wanted)
  position.value = {
    left: `${p.left}px`,
    minWidth: `${p.width}px`,
    maxWidth: `${Math.max(p.width, Math.min(window.innerWidth - 16, 420))}px`,
    maxHeight: `${p.maxHeight}px`,
    ...(p.up ? { bottom: `${p.bottom}px`, top: 'auto' } : { top: `${p.top}px`, bottom: 'auto' }),
  }
}

async function show(index: number) {
  if (props.disabled || open.value) return
  active.value = index
  open.value = true
  place()
  await nextTick()
  list.value?.showPopover?.()
  scrollActive()
}
function hide(focus = true) {
  if (!open.value) return
  open.value = false
  if (list.value?.matches?.(':popover-open')) list.value.hidePopover()
  if (focus) trigger.value?.focus({ preventScroll: true })
}
function scrollActive() {
  if (active.value < 0) return
  nextTick(() =>
    list.value?.querySelector(`#${CSS.escape(optionId(active.value))}`)?.scrollIntoView({
      block: 'nearest',
    }),
  )
}

let typed = ''
let typedTimer: ReturnType<typeof setTimeout> | undefined
function onKeydown(event: KeyboardEvent) {
  if (props.disabled) return
  if (isTypeaheadKey(event) && !(event.key === ' ' && typed === '')) {
    clearTimeout(typedTimer)
    typed += event.key
    typedTimer = setTimeout(() => (typed = ''), 500)
    const from = open.value ? active.value : selectedIndex.value
    const found = typeahead(props.options, typed, from)
    event.preventDefault()
    if (found < 0) return
    if (open.value) {
      active.value = found
      scrollActive()
    } else choose(found)
    return
  }
  const action = listboxKey(
    event.key,
    { open: open.value, active: active.value, selected: selectedIndex.value, altKey: event.altKey },
    props.options,
  )
  switch (action.type) {
    case 'open':
      event.preventDefault()
      show(action.active)
      break
    case 'move':
      event.preventDefault()
      active.value = action.active
      scrollActive()
      break
    case 'select':
      if (action.keepFocus) event.preventDefault()
      choose(action.index)
      hide(action.keepFocus)
      break
    case 'close':
      if (event.key !== 'Tab') event.preventDefault()
      hide(event.key !== 'Tab')
      break
  }
}

function onOutside(event: Event) {
  const target = event.target as Node | null
  if (target && (trigger.value?.contains(target) || list.value?.contains(target))) return
  hide(false)
}
function onViewportChange(event: Event) {
  // Scrolling inside the list itself is fine; anything else moving the trigger re-places it.
  if (event.type === 'scroll' && list.value?.contains(event.target as Node)) return
  place()
}
watch(open, (value) => {
  const method = value ? 'addEventListener' : 'removeEventListener'
  document[method]('pointerdown', onOutside, true)
  window[method]('resize', onViewportChange)
  window[method]('scroll', onViewportChange, true)
})
onBeforeUnmount(() => {
  clearTimeout(typedTimer)
  if (open.value) {
    open.value = false
    document.removeEventListener('pointerdown', onOutside, true)
    window.removeEventListener('resize', onViewportChange)
    window.removeEventListener('scroll', onViewportChange, true)
  }
})
function onNativeChange(event: Event) {
  const value = (event.target as HTMLSelectElement).value
  choose(props.options.findIndex((o) => String(o.value) === value))
}
</script>

<template>
  <select
    v-if="useNative"
    v-bind="attrs"
    class="app-select-native"
    :class="{ invalid: isInvalid }"
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
  <template v-else>
    <button
      ref="trigger"
      v-bind="attrs"
      type="button"
      role="combobox"
      class="app-select"
      :class="{ open, invalid: isInvalid, empty: selectedIndex < 0 }"
      aria-haspopup="listbox"
      :aria-expanded="open"
      :aria-controls="open ? listId : undefined"
      :aria-activedescendant="open && active >= 0 ? optionId(active) : undefined"
      :aria-invalid="isInvalid || undefined"
      :disabled="disabled"
      @click="open ? hide() : show(selectedIndex >= 0 ? selectedIndex : enabledFrom(options, 0, 1))"
      @keydown="onKeydown"
    >
      <span class="app-select-value">{{ selectedLabel ?? placeholder }}</span>
      <AppIcon
        name="down"
        :size="15"
        class="app-select-chevron"
      />
    </button>
    <ul
      v-if="open"
      :id="listId"
      ref="list"
      popover="manual"
      role="listbox"
      class="app-select-list"
      :aria-labelledby="attrs['aria-labelledby'] as string | undefined"
      :aria-label="
        attrs['aria-labelledby'] ? undefined : (attrs['aria-label'] as string | undefined)
      "
      :style="position"
      tabindex="-1"
      @pointerdown.prevent
    >
      <li
        v-for="(option, index) in options"
        :id="optionId(index)"
        :key="index"
        role="option"
        class="app-select-option"
        :class="{ active: index === active }"
        :aria-selected="index === selectedIndex"
        :aria-disabled="option.disabled || undefined"
        @pointermove="option.disabled || (active = index)"
        @click="option.disabled || (choose(index), hide())"
      >
        <span>{{ option.label }}</span>
        <AppIcon
          v-if="index === selectedIndex"
          name="check"
          :size="14"
        />
      </li>
    </ul>
  </template>
</template>
