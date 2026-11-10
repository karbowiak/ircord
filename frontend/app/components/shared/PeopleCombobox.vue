<script setup vapor lang="ts">
/** Presentation-neutral people input. Adapters own lookup, identity and selection. */
const props = withDefaults(
  defineProps<{
    options: readonly { key: string; label: string; detail?: string; disabled?: boolean }[]
    label: string
    placeholder?: string
    disabled?: boolean
    autoHighlight?: boolean
    preventSubmit?: boolean
    popup?: boolean
    describedBy?: string
  }>(),
  {
    placeholder: undefined,
    disabled: false,
    autoHighlight: false,
    preventSubmit: false,
    popup: false,
    describedBy: undefined,
  },
)
const query = defineModel<string>({ required: true })
const emit = defineEmits<{
  select: [key: string]
  search: [query: string | null]
  highlight: [key: string | undefined]
}>()
const input = ref<HTMLInputElement | null>(null)
const root = ref<HTMLElement | null>(null)
const field = () =>
  root.value?.querySelector<HTMLInputElement>('input[role="combobox"]') ?? input.value
function setInput(value: unknown) {
  const element =
    value instanceof HTMLInputElement ? value : (value as { $el?: unknown } | null)?.$el
  input.value = element instanceof HTMLInputElement ? element : null
}
const list = ref<HTMLElement | null>(null)
const listId = useId()
const open = ref(false)
const activeKey = ref<string>()
watch(activeKey, (value) => emit('highlight', value))
const active = computed(() =>
  props.options.findIndex((option) => option.key === activeKey.value && !option.disabled),
)
const visible = computed(() => open.value && props.options.length > 0)
const position = ref<Record<string, string>>({})
let restoringFocus = false
const optionId = (index: number) => `${listId}-${index}`
function place() {
  const element = field()
  if (!props.popup || !element) return
  const p = placePopup(
    element.getBoundingClientRect(),
    { width: window.innerWidth, height: window.innerHeight },
    Math.min(320, props.options.length * 44 + 10),
  )
  position.value = {
    left: `${p.left}px`,
    minWidth: `${p.width}px`,
    maxWidth: `${Math.max(p.width, Math.min(window.innerWidth - 16, 420))}px`,
    maxHeight: `${p.maxHeight}px`,
    ...(p.up ? { bottom: `${p.bottom}px`, top: 'auto' } : { top: `${p.top}px`, bottom: 'auto' }),
  }
}
async function show() {
  if (props.disabled || restoringFocus) return
  if (!open.value) {
    open.value = true
    emit('search', query.value)
  }
  await nextTick()
  place()
  if (props.popup && visible.value && !list.value?.matches?.(':popover-open'))
    list.value?.showPopover?.()
}
function close() {
  if (!open.value) return
  open.value = false
  activeKey.value = undefined
  emit('search', null)
}
function listPointerDown(event: PointerEvent) {
  event.preventDefault()
}
function highlightOption(event: PointerEvent, key: string, disabled?: boolean) {
  if (event.pointerType === 'mouse' && !disabled) activeKey.value = key
}
function choose(key: string) {
  const option = props.options.find((item) => item.key === key)
  if (!option || option.disabled || props.disabled) return
  emit('select', key)
  close()
  void nextTick(() => {
    restoringFocus = true
    field()?.focus({ preventScroll: true })
    restoringFocus = false
  })
}
function onInput(event: Event) {
  input.value = event.currentTarget as HTMLInputElement
  activeKey.value =
    props.autoHighlight && query.value.trim()
      ? props.options.find((item) => !item.disabled)?.key
      : undefined
  if (!open.value) void show()
  else emit('search', query.value)
}
function onFocus(event: FocusEvent) {
  input.value = event.currentTarget as HTMLInputElement
  void show()
}
function keydown(event: KeyboardEvent) {
  if (event.isComposing) return
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    if (!open.value) void show()
    const enabled = props.options.filter((item) => !item.disabled)
    if (!enabled.length) return
    const index = enabled.findIndex((item) => item.key === activeKey.value)
    const next =
      index < 0
        ? event.key === 'ArrowDown'
          ? 0
          : enabled.length - 1
        : (index + (event.key === 'ArrowDown' ? 1 : -1) + enabled.length) % enabled.length
    activeKey.value = enabled[next]?.key
    nextTick(() =>
      list.value
        ?.querySelector(`#${CSS.escape(optionId(active.value))}`)
        ?.scrollIntoView({ block: 'nearest' }),
    )
  } else if (event.key === 'Enter') {
    if (props.preventSubmit || (visible.value && active.value >= 0)) event.preventDefault()
    if (visible.value && active.value >= 0) choose(props.options[active.value]!.key)
    else if (!props.preventSubmit) close()
  } else if (event.key === 'Escape' && open.value) {
    event.preventDefault()
    event.stopPropagation()
    close()
  } else if (event.key === 'Tab') close()
}
watch(
  () => props.options,
  () => {
    if (active.value < 0)
      activeKey.value =
        props.autoHighlight && query.value.trim()
          ? props.options.find((item) => !item.disabled)?.key
          : undefined
    if (visible.value) nextTick(place)
  },
)
watch(
  () => props.disabled,
  (value) => {
    if (value) close()
  },
)
watch(visible, (value) => {
  if (value && props.popup) void show()
})
function outside(event: Event) {
  const target = event.target as Node | null
  if (target && (field()?.contains(target) || list.value?.contains(target))) return
  close()
}
function viewport(event: Event) {
  if (event.type !== 'scroll' || !list.value?.contains(event.target as Node)) place()
}
watch(visible, (value) => {
  if (!props.popup) return
  const method = value ? 'addEventListener' : 'removeEventListener'
  document[method]('pointerdown', outside, true)
  window[method]('resize', viewport)
  window[method]('scroll', viewport, true)
})
onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', outside, true)
  window.removeEventListener('resize', viewport)
  window.removeEventListener('scroll', viewport, true)
})
defineExpose({ focus: () => field()?.focus() })
</script>

<template>
  <div
    ref="root"
    class="min-w-0 grid gap-1"
  >
    <UiInput
      :ref="setInput"
      v-model="query"
      type="text"
      role="combobox"
      :aria-label="label"
      :aria-describedby="describedBy"
      :placeholder="placeholder ?? label"
      :disabled="disabled"
      :aria-expanded="visible"
      :aria-controls="visible ? listId : undefined"
      :aria-activedescendant="visible && active >= 0 ? optionId(active) : undefined"
      aria-autocomplete="list"
      autocomplete="off"
      autocapitalize="none"
      spellcheck="false"
      @focus="onFocus"
      @click="show"
      @input="onInput"
      @keydown="keydown"
      @blur="close"
    />
    <slot name="after-input" />
    <ul
      v-if="visible"
      :id="listId"
      ref="list"
      :popover="popup ? 'manual' : undefined"
      role="listbox"
      :aria-label="label"
      :style="popup ? position : undefined"
      :class="
        popup
          ? 'app-select-list fixed inset-auto m-0 p-1 overflow-y-auto overscroll-contain list-none bg-secondary text-foreground border border-solid border-border rounded-md shadow-popover text-[13px]'
          : 'max-h-48 overflow-y-auto rounded-md bg-field p-1'
      "
      @pointerdown="listPointerDown"
    >
      <li
        v-for="(option, index) in options"
        :id="optionId(index)"
        :key="option.key"
        :data-person-key="option.key"
        role="option"
        :aria-selected="index === active"
        :aria-disabled="option.disabled || undefined"
        class="rounded-sm px-2 py-1.5"
        :class="{
          'bg-accent': index === active,
          'cursor-pointer': !option.disabled,
          'opacity-70': option.disabled,
        }"
        @pointermove="highlightOption($event, option.key, option.disabled)"
        @click="choose(option.key)"
      >
        <slot
          name="option"
          :option="option"
          ><span class="block font-medium">{{ option.label }}</span
          ><span
            v-if="option.detail"
            class="block text-[length:var(--ui-caption)] text-muted-foreground"
            >{{ option.detail }}</span
          ></slot
        >
      </li>
    </ul>
    <slot
      :open="open"
      :visible="visible"
    />
  </div>
</template>
