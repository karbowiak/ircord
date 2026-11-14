<script setup vapor lang="ts">
import { isDay } from '../../../shared/utils/search-filter'

/** A filter the field understands as `key:`, and how it is offered before anything is typed. */
export interface TagKey {
  key: string
  icon: string
  /** The suggestion panel's heading while this key is typed, and its row in the idle menu. */
  label: string
  hint?: string
  /** Choices are days: recent ones are offered and a date picker is shown. */
  date?: boolean
  /** Listed in the idle menu; other keys work when typed. */
  menu?: boolean
}
export interface TagChip {
  key: string
  value: string
}
export interface TagSuggestion {
  id: string
  label: string
  detail?: string
  icon?: string
  /** A member id; the row shows their avatar instead of an icon. */
  avatar?: string
  value: string
}

/**
 * A search box with filters as removable chips before the text, Discord-style. Typing a key such
 * as `in:` opens a panel of matching choices, and choosing one turns the token into a chip.
 * Focused and empty, the panel can offer the keys themselves and whatever the `idle` slot adds
 * (recent searches). Arrow keys and Enter work in the panel while it is open; otherwise keydown
 * passes through, so the page can move through its results.
 */
const props = withDefaults(
  defineProps<{
    keys: TagKey[]
    chips: TagChip[]
    /** Choices for a key, narrowed by what was typed after the colon. */
    suggest: (key: string, query: string) => TagSuggestion[]
    chipLabel: (chip: TagChip) => string
    placeholder?: string
    label?: string
    /** Offer the keys when focused and empty. Off where the page shows them as pills. */
    menu?: boolean
    /** Label of the idle menu's last row, which emits `more`; no row without it. */
    moreLabel?: string
    moreHint?: string
  }>(),
  {
    placeholder: undefined,
    label: undefined,
    menu: true,
    moreLabel: undefined,
    moreHint: undefined,
  },
)
const text = defineModel<string>('text', { required: true })
const emit = defineEmits<{
  add: [chip: TagChip]
  remove: [chip: TagChip]
  clear: []
  keydown: [event: KeyboardEvent]
  more: []
}>()
const slots = defineSlots<{ idle?: () => unknown }>()
const { t } = useI18n()
const chat = useChatStore()
const input = ref<HTMLInputElement | null>(null)
const focused = ref(false)
const dismissed = ref(false)
const listId = useId()
watch(text, () => (dismissed.value = false))

/** The token being typed at the end of the text, such as `in:gen` → key in, value gen. */
const token = computed(() => {
  const names = props.keys.map((k) => k.key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|')
  const match = names ? new RegExp(`(^|\\s)(${names}):(\\S*)$`, 'i').exec(text.value) : null
  if (!match) return null
  const key = props.keys.find((k) => k.key === match[2]!.toLowerCase())
  return key ? { key, value: match[3]!, start: match.index + match[1]!.length } : null
})

const today = () => {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}
const shift = (day: string, days: number) => {
  const d = new Date(`${day}T12:00:00`)
  d.setDate(d.getDate() + days)
  return d.toISOString().slice(0, 10)
}
const { day } = useTimeFormat()
const dayLabel = (value: string) => day(new Date(`${value}T12:00:00`).toISOString())
function dateSuggestions(key: string, typed: string): TagSuggestion[] {
  if (isDay(typed)) return [{ id: `${key}-${typed}`, label: dayLabel(typed), value: typed }]
  const labels = [
    t('collectionsUi.search.today'),
    t('collectionsUi.search.yesterday'),
    t('collectionsUi.search.weekAgo'),
    t('collectionsUi.search.monthAgo'),
  ]
  return [0, -1, -7, -30].map((days, i) => {
    const value = shift(today(), days)
    return { id: `${key}-${value}`, label: labels[i]!, detail: value, icon: 'calendar', value }
  })
}

type Row = TagSuggestion & { run: () => void }
/** Replaces the token being typed with a chip. */
function choose(key: string, value: string) {
  const active = token.value
  // One assignment: a model reads its old value until the parent updates it.
  const rest = active ? text.value.slice(0, active.start).trim() : text.value.trim()
  text.value = rest ? `${rest} ` : ''
  emit('add', { key, value })
  nextTick(() => input.value?.focus())
}
function startToken(key: string) {
  const base = text.value.trimEnd()
  text.value = `${base ? `${base} ` : ''}${key}:`
  nextTick(() => input.value?.focus())
}
const rows = computed<Row[]>(() => {
  const active = token.value
  if (!active)
    return [
      ...props.keys
        .filter((k) => k.menu)
        .map((k) => ({
          id: `key-${k.key}`,
          label: k.label,
          detail: k.hint,
          icon: k.icon,
          value: k.key,
          run: () => startToken(k.key),
        })),
      ...(props.moreLabel
        ? [
            {
              id: 'more',
              label: props.moreLabel,
              detail: props.moreHint,
              icon: 'filters',
              value: '',
              run: () => emit('more'),
            },
          ]
        : []),
    ]
  const choices = active.key.date
    ? dateSuggestions(active.key.key, active.value)
    : props.suggest(active.key.key, active.value.toLowerCase().replace(/^[#@]/, ''))
  return choices.map((s) => ({ ...s, run: () => choose(active.key.key, s.value) }))
})
const heading = computed(() => token.value?.key.label ?? t('collectionsUi.search.filters'))
const dateKey = computed(() => (token.value?.key.date ? token.value.key.key : null))
const open = computed(
  () => focused.value && !dismissed.value && (!!token.value || (props.menu && !text.value.trim())),
)
const showIdle = computed(() => !token.value && !text.value.trim() && !!slots.idle)
const highlighted = ref(0)
watch([rows, open], () => (highlighted.value = 0))
const activeId = computed(() =>
  open.value && rows.value[highlighted.value]
    ? `${listId}-${rows.value[highlighted.value]!.id}`
    : undefined,
)

function keydown(event: KeyboardEvent) {
  if (open.value && rows.value.length) {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      const n = rows.value.length
      highlighted.value = (highlighted.value + (event.key === 'ArrowDown' ? 1 : -1) + n) % n
      nextTick(() =>
        document.getElementById(activeId.value ?? '')?.scrollIntoView({ block: 'nearest' }),
      )
      return
    }
    // Enter picks a choice while a token is being typed; with plain text it searches.
    if ((event.key === 'Enter' || event.key === 'Tab') && token.value) {
      event.preventDefault()
      rows.value[highlighted.value]?.run()
      return
    }
    if (event.key === 'Escape') {
      event.preventDefault()
      event.stopPropagation()
      dismissed.value = true
      return
    }
  }
  if (event.key === 'Backspace' && !text.value && props.chips.length) {
    event.preventDefault()
    emit('remove', props.chips.at(-1)!)
    return
  }
  emit('keydown', event)
}
const root = ref<HTMLElement | null>(null)
function blur(event: FocusEvent) {
  const next = event.relatedTarget as Node | null
  if (!next || !root.value?.contains(next)) focused.value = false
}
defineExpose({ focus: () => input.value?.focus(), start: startToken })
</script>
<template>
  <div
    ref="root"
    class="relative min-w-[min(calc(240px*var(--interface-scale,1)),100%)] flex-1"
    @focusin="focused = true"
    @focusout="blur"
  >
    <div
      class="search-field-control flex min-h-9 min-w-0 flex-wrap items-center gap-1.5 rounded-md border border-solid border-border bg-field px-3 py-1 text-muted-foreground transition-colors focus-within:border-primary focus-within:ring-2 focus-within:ring-primary/15"
      @click.self="input?.focus()"
    >
      <AppIcon
        name="search"
        :size="16"
        class="mr-1"
      />
      <span
        v-for="chip in chips"
        :key="`${chip.key}:${chip.value}`"
        class="search-chip inline-flex h-6.5 max-w-full items-center gap-1 rounded-sm bg-selection pr-0.5 pl-2 text-[length:var(--ui-small)] text-foreground"
        ><span class="text-primary">{{ chip.key }}:</span
        ><span class="truncate">{{ chipLabel(chip) }}</span
        ><button
          type="button"
          class="grid size-5 place-items-center rounded-sm text-muted-foreground hover:bg-accent hover:text-foreground"
          :aria-label="
            t('collectionsUi.search.removeFilter', { filter: chip.key, value: chipLabel(chip) })
          "
          @click="emit('remove', chip)"
        >
          <AppIcon
            name="close"
            :size="12"
          /></button
      ></span>
      <input
        ref="input"
        v-model="text"
        class="h-7 min-w-32 flex-1 border-0 bg-transparent text-[length:var(--ui-text)] text-foreground outline-none placeholder:text-faint max-[700px]:text-[16px] [@media(pointer:coarse)]:text-[length:max(16px,var(--ui-text))]"
        :placeholder="chips.length ? t('collectionsUi.search.addWords') : placeholder"
        role="combobox"
        aria-autocomplete="list"
        :aria-expanded="open"
        :aria-controls="listId"
        :aria-activedescendant="activeId"
        :aria-label="label ?? t('collectionsUi.search.workspaceLabel')"
        autocomplete="off"
        spellcheck="false"
        autofocus
        @keydown="keydown"
      />
      <button
        v-if="text || chips.length"
        type="button"
        class="flex size-6 shrink-0 items-center justify-center rounded-sm text-muted-foreground hover:bg-accent hover:text-foreground"
        :aria-label="t('settings.clearSearch')"
        @click="((text = ''), emit('clear'), input?.focus())"
      >
        <AppIcon
          name="close"
          :size="14"
        />
      </button>
    </div>
    <!-- Inline suggestions, not a modal popup: workspace shortcuts still work while it shows. -->
    <div
      v-if="open"
      class="search-suggestions absolute top-full left-0 z-50 mt-1.5 w-[min(100%,calc(420px*var(--interface-scale,1)))] rounded-md bg-rail p-1.5 shadow-popover"
    >
      <p
        class="m-0 px-2 pt-1 pb-1.5 text-[length:var(--ui-caption)] font-semibold text-muted-foreground"
      >
        {{ heading }}
      </p>
      <ul
        :id="listId"
        role="listbox"
        data-inline-suggestions
        :aria-label="heading"
        class="m-0 max-h-80 list-none overflow-y-auto p-0"
      >
        <li
          v-for="(s, index) in rows"
          :id="`${listId}-${s.id}`"
          :key="s.id"
          role="option"
          :aria-selected="index === highlighted"
          class="flex min-h-9 cursor-default items-center gap-2.5 rounded-sm px-2 py-1.5 text-[length:var(--ui-small)] text-body aria-selected:bg-accent aria-selected:text-foreground"
          @pointerdown.prevent
          @pointermove="highlighted = index"
          @click="s.run()"
        >
          <UserAvatar
            v-if="s.avatar"
            :member="s.avatar === 'you' ? chat.self : chat.member(s.avatar)"
            size="tiny"
          /><AppIcon
            v-else-if="s.icon"
            :name="s.icon"
            :size="16"
            class="shrink-0 text-muted-foreground"
          /><span class="flex min-w-0 flex-1 flex-col"
            ><span class="truncate font-medium">{{ s.label }}</span
            ><span
              v-if="s.detail"
              class="truncate text-[length:var(--ui-caption)] text-muted-foreground"
              >{{ s.detail }}</span
            ></span
          >
        </li>
        <li
          v-if="!rows.length"
          class="px-2 py-2 text-[length:var(--ui-small)] text-muted-foreground"
        >
          {{ t('collectionsUi.search.noSuggestion', { query: token?.value ?? '' }) }}
        </li>
      </ul>
      <label
        v-if="dateKey"
        class="mt-1 flex items-center gap-2 border-t [border-top-style:solid] border-border/60 px-2 pt-2 pb-1 text-[length:var(--ui-caption)] text-muted-foreground"
        @pointerdown.stop
        >{{ t('collectionsUi.search.pickDay') }}
        <UiInput
          type="date"
          class="h-8 flex-1"
          :max="today()"
          @change="
            (e: Event) =>
              isDay((e.target as HTMLInputElement).value) &&
              choose(dateKey!, (e.target as HTMLInputElement).value)
          "
      /></label>
      <slot
        v-if="showIdle"
        name="idle"
      />
    </div>
  </div>
</template>
