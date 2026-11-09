<script setup lang="ts">
import type { Member } from '../../shared/types/chat'
import { personSuggestions } from '../../shared/utils/mask-builder'

/**
 * Finds someone in a channel, or seen there recently, to build a mask from (the WAI-ARIA
 * combobox pattern with list autocomplete). Choosing someone emits them; the text is only a
 * search and is never sent.
 */
const props = defineProps<{
  here: readonly Member[]
  recent: readonly Member[]
  /** The channel, for labels. */
  channel: string
}>()
const emit = defineEmits<{ choose: [person: Member] }>()
const query = ref('')

const listId = useId()
const optionId = (index: number) => `${listId}-${index}`
const input = ref<HTMLInputElement | null>(null)
const list = ref<HTMLElement | null>(null)
const open = ref(false)
const position = ref<Record<string, string>>({})

const options = computed(
  () =>
    personSuggestions(props.here, props.recent, query.value) as {
      person: Member
      recent: boolean
    }[],
)
const visible = computed(() => open.value && options.value.length > 0)
// The highlight follows a person, not a position: when people join, leave or change nick under
// an open list, it stays on the same person or clears, and never slides onto someone else.
const activeId = ref<string>()
const active = computed({
  get: () =>
    activeId.value === undefined
      ? -1
      : options.value.findIndex((o) => o.person.id === activeId.value),
  set: (index: number) => (activeId.value = options.value[index]?.person.id),
})
watch(options, () => {
  if (active.value < 0) activeId.value = undefined
  if (visible.value) nextTick(place)
})

function place() {
  if (!input.value) return
  const rect = input.value.getBoundingClientRect()
  const p = placePopup(
    rect,
    { width: window.innerWidth, height: window.innerHeight },
    Math.min(320, options.value.length * 44 + 10),
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
  open.value = true
  await nextTick()
  if (!visible.value) return
  place()
  if (list.value && !list.value.matches?.(':popover-open')) list.value.showPopover?.()
}
function close() {
  open.value = false
  active.value = -1
}
watch(visible, (value) => {
  if (value) void show()
})

function choose(person: Member) {
  emit('choose', person)
  query.value = ''
  close()
}
function scrollActive() {
  nextTick(() =>
    list.value
      ?.querySelector(`#${CSS.escape(optionId(active.value))}`)
      ?.scrollIntoView({ block: 'nearest' }),
  )
}
function onInput() {
  // Typing highlights the best match, so Enter picks it.
  active.value = query.value.trim() ? 0 : -1
  void show()
}
function onKeydown(event: KeyboardEvent) {
  if (event.isComposing) return
  const count = options.value.length
  switch (event.key) {
    case 'ArrowDown':
    case 'ArrowUp': {
      event.preventDefault()
      if (!open.value) {
        void show()
        if (count) active.value = event.key === 'ArrowDown' ? 0 : count - 1
      } else if (count) {
        const step = event.key === 'ArrowDown' ? 1 : -1
        active.value =
          active.value < 0 && step < 0 ? count - 1 : (active.value + step + count) % count
      }
      scrollActive()
      break
    }
    case 'Enter':
      // Enter only ever picks a person; it never submits the mask form from the search.
      event.preventDefault()
      if (visible.value && active.value >= 0 && options.value[active.value])
        choose(options.value[active.value]!.person)
      break
    case 'Escape':
      // Close the list without closing the dialog around it.
      if (visible.value) {
        event.preventDefault()
        event.stopPropagation()
        close()
      }
      break
    case 'Tab':
      close()
      break
  }
}
function onOutside(event: Event) {
  const target = event.target as Node | null
  if (target && (input.value?.contains(target) || list.value?.contains(target))) return
  close()
}
function onViewportChange(event: Event) {
  if (event.type === 'scroll' && list.value?.contains(event.target as Node)) return
  place()
}
watch(visible, (value) => {
  const method = value ? 'addEventListener' : 'removeEventListener'
  document[method]('pointerdown', onOutside, true)
  window[method]('resize', onViewportChange)
  window[method]('scroll', onViewportChange, true)
})
onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onOutside, true)
  window.removeEventListener('resize', onViewportChange)
  window.removeEventListener('scroll', onViewportChange, true)
})
defineExpose({ focus: () => input.value?.focus() })

const address = (m: Member) => (m.username && m.host ? `${m.username}@${m.host}` : '')
</script>

<template>
  <div class="mask-person-picker min-w-0 grid">
    <UiInput
      :ref="(el) => (input = (el as { $el: HTMLInputElement } | null)?.$el ?? null)"
      v-model="query"
      type="text"
      role="combobox"
      :class="{ open: visible }"
      :placeholder="`Find someone in ${channel}`"
      aria-label="Find someone to build the mask from"
      aria-autocomplete="list"
      :aria-expanded="visible"
      :aria-controls="visible ? listId : undefined"
      :aria-activedescendant="visible && active >= 0 ? optionId(active) : undefined"
      autocomplete="off"
      autocapitalize="none"
      spellcheck="false"
      @input="onInput"
      @click="show()"
      @keydown="onKeydown"
      @blur="close"
    />
    <ul
      v-if="visible"
      :id="listId"
      ref="list"
      popover="manual"
      role="listbox"
      class="app-select-list mask-person-list fixed inset-auto m-0 p-1 overflow-y-auto overscroll-contain list-none bg-secondary text-foreground border border-solid border-border rounded-md shadow-popover text-[13px]"
      :aria-label="`People in ${channel}`"
      :style="position"
      tabindex="-1"
      @pointerdown.prevent
    >
      <li
        v-for="(item, index) in options"
        :id="optionId(index)"
        :key="item.person.id"
        role="option"
        class="app-select-option mask-person-option flex items-center gap-2 py-1.5 px-2 rounded-sm cursor-pointer min-h-10 [&.active]:bg-accent"
        :class="{ active: index === active }"
        :aria-selected="index === active"
        @pointermove="active = index"
        @click="choose(item.person)"
      >
        <UserAvatar
          :member="item.person"
          size="small"
        />
        <span class="grid min-w-0 gap-[1px]">
          <span class="font-[550] text-foreground wrap-anywhere">{{ item.person.nick }}</span>
          <span class="text-[0.82em] text-muted-foreground wrap-anywhere"
            ><template v-if="item.recent">Recently in {{ channel }}</template
            ><template v-if="item.recent && address(item.person)"> · </template
            >{{ address(item.person) }}</span
          >
        </span>
      </li>
    </ul>
  </div>
</template>
