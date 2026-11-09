<script setup lang="ts">
import {
  activeToken,
  filterChips,
  filterKeys,
  isDay,
  searchHasKinds,
  withChip,
  withoutChip,
  type FilterChip,
  type RecentSearch,
  type FilterKey,
  type SearchFilters,
} from '../../shared/utils/search-filter'

/**
 * The Search page's field: filters as removable chips before the text, and a suggestion panel.
 * Focused and empty, the panel offers the filters and recent searches; typing `from:`, `in:`,
 * `has:`, `mentions:`, a date key or `author:` offers matching choices, and choosing one turns
 * the token into a chip. Arrow keys and Enter work in the panel while it is open; otherwise
 * keydown passes through (the page moves through results with it).
 */
const props = withDefaults(
  defineProps<{
    recent?: RecentSearch[]
    placeholder?: string
    label?: string
    /** Filter keys this search doesn't offer (a conversation's own search has no `in:`). */
    exclude?: FilterKey[]
  }>(),
  { recent: () => [], placeholder: undefined, label: 'Search workspace', exclude: () => [] },
)
const text = defineModel<string>('text', { required: true })
const filters = defineModel<SearchFilters>('filters', { required: true })
const emit = defineEmits<{
  keydown: [event: KeyboardEvent]
  more: []
  recall: [entry: RecentSearch]
  forget: []
}>()
const labels = useSearchFilters()
const chat = useChatStore()
const input = ref<HTMLInputElement | null>(null)
const focused = ref(false)
const dismissed = ref(false)
const listId = useId()
watch(text, () => (dismissed.value = false))

const chips = computed(() => filterChips(filters.value))
const token = computed(() => {
  const t = activeToken(text.value)
  return t && !props.exclude.includes(t.key) ? t : null
})

type Suggestion = {
  id: string
  label: string
  detail?: string
  icon?: string
  avatar?: string
  run: () => void
}
const heading = computed(() =>
  token.value ? filterKeys.find((k) => k.key === token.value!.key)?.label : 'Filters',
)
/** Replaces the token being typed with a chip. */
function choose(chip: FilterChip) {
  const t = token.value
  // One assignment: a model reads its old value until the parent updates it.
  const rest = t ? text.value.slice(0, t.start).trim() : text.value.trim()
  text.value = rest ? `${rest} ` : ''
  filters.value = withChip(filters.value, chip)
  nextTick(() => input.value?.focus())
}
function startToken(key: string) {
  const base = text.value.trimEnd()
  text.value = `${base ? `${base} ` : ''}${key}:`
  nextTick(() => input.value?.focus())
}
const today = () => {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}
const shift = (day: string, days: number) => {
  const d = new Date(`${day}T12:00:00`)
  d.setDate(d.getDate() + days)
  return d.toISOString().slice(0, 10)
}
const shortlist: string[] = ['from', 'mentions', 'in', 'has', 'on']
const suggestions = computed<Suggestion[]>(() => {
  const t = token.value
  if (!t) {
    // The main filters; before:, after: and author: work when typed and are in More filters.
    return [
      ...filterKeys
        .filter((k) => shortlist.includes(k.key) && !props.exclude.includes(k.key))
        .map((k) => ({
          id: `key-${k.key}`,
          label: k.label,
          detail: k.hint,
          icon: k.icon,
          run: () => startToken(k.key),
        })),
      {
        id: 'more',
        label: 'More filters',
        detail: 'Several people, channels and types at once',
        icon: 'filters',
        run: () => emit('more'),
      },
    ]
  }
  const q = t.value.toLowerCase().replace(/^[#@]/, '')
  switch (t.key) {
    case 'from':
    case 'mentions': {
      const key = t.key
      return [...(key === 'from' && chat.self ? [chat.self] : []), ...labels.people.value]
        .filter(
          (m) =>
            !filters.value[key].includes(m.id) &&
            (!q || `${m.displayName} ${m.nick}`.toLowerCase().includes(q)),
        )
        .slice(0, 8)
        .map((m) => ({
          id: `${key}-${m.id}`,
          label: m.id === 'you' ? `${m.displayName} (you)` : m.displayName,
          detail: `@${m.nick}`,
          avatar: m.id,
          run: () => choose({ key, value: m.id }),
        }))
    }
    case 'in':
      return (chat.data?.conversations ?? [])
        .filter(
          (c) =>
            !filters.value.in.includes(c.id) &&
            (!q || labels.conversationLabel(c).toLowerCase().includes(q)),
        )
        .slice(0, 8)
        .map((c) => ({
          id: `in-${c.id}`,
          label: labels.conversationLabel(c),
          detail: labels.networkName(c.networkId),
          icon: c.kind === 'channel' ? 'hash' : 'message',
          run: () => choose({ key: 'in', value: c.id }),
        }))
    case 'has':
      return searchHasKinds
        .filter((k) => !filters.value.has.includes(k.value) && k.label.toLowerCase().includes(q))
        .map((k) => ({
          id: `has-${k.value}`,
          label: k.label,
          icon: k.icon,
          run: () => choose({ key: 'has', value: k.value }),
        }))
    case 'author':
      return (['user', 'bot'] as const)
        .filter((v) => !q || (v === 'bot' ? 'bot' : 'person people user').includes(q))
        .map((v) => ({
          id: `author-${v}`,
          label: v === 'bot' ? 'Bots' : 'People',
          icon: v === 'bot' ? 'brand' : 'user',
          run: () => choose({ key: 'author', value: v }),
        }))
    default: {
      const key = t.key
      const days = isDay(t.value)
        ? [t.value]
        : [today(), shift(today(), -1), shift(today(), -7), shift(today(), -30)]
      return days.map((value, i) => ({
        id: `${key}-${value}`,
        label: isDay(t.value)
          ? labels.dayLabel(value)
          : ['Today', 'Yesterday', 'A week ago', 'A month ago'][i]!,
        detail: value,
        icon: 'calendar',
        run: () => choose({ key, value } as FilterChip),
      }))
    }
  }
})
const dateKey = computed(() =>
  token.value && ['before', 'after', 'on'].includes(token.value.key) ? token.value.key : null,
)
const showRecent = computed(() => !token.value && !text.value.trim() && props.recent.length > 0)
const open = computed(
  () => focused.value && !dismissed.value && (!text.value.trim() || !!token.value),
)
const highlighted = ref(0)
watch([suggestions, open], () => (highlighted.value = 0))
const activeId = computed(() =>
  open.value && suggestions.value[highlighted.value]
    ? `${listId}-${suggestions.value[highlighted.value]!.id}`
    : undefined,
)

function keydown(event: KeyboardEvent) {
  if (open.value && suggestions.value.length) {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      const n = suggestions.value.length
      highlighted.value = (highlighted.value + (event.key === 'ArrowDown' ? 1 : -1) + n) % n
      nextTick(() =>
        document.getElementById(activeId.value ?? '')?.scrollIntoView({ block: 'nearest' }),
      )
      return
    }
    // Enter picks a choice while a token is being typed; with plain text it searches.
    if ((event.key === 'Enter' || event.key === 'Tab') && token.value) {
      event.preventDefault()
      suggestions.value[highlighted.value]?.run()
      return
    }
    if (event.key === 'Escape') {
      event.preventDefault()
      event.stopPropagation()
      dismissed.value = true
      return
    }
  }
  if (event.key === 'Backspace' && !text.value && chips.value.length) {
    event.preventDefault()
    filters.value = withoutChip(filters.value, chips.value.at(-1)!)
    return
  }
  emit('keydown', event)
}
function blur(event: FocusEvent) {
  const next = event.relatedTarget as Node | null
  if (!next || !root.value?.contains(next)) focused.value = false
}
const root = ref<HTMLElement | null>(null)
defineExpose({ focus: () => input.value?.focus() })
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
        ><span class="truncate">{{ labels.chipLabel(chip) }}</span
        ><button
          type="button"
          class="grid size-5 place-items-center rounded-sm text-muted-foreground hover:bg-accent hover:text-foreground"
          :aria-label="`Remove filter ${chip.key}: ${labels.chipLabel(chip)}`"
          @click="filters = withoutChip(filters, chip)"
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
        :placeholder="chips.length ? 'Add words or filters' : placeholder"
        role="combobox"
        aria-autocomplete="list"
        :aria-expanded="open"
        :aria-controls="listId"
        :aria-activedescendant="activeId"
        :aria-label="label"
        autocomplete="off"
        spellcheck="false"
        autofocus
        @keydown="keydown"
      />
      <button
        v-if="text || chips.length"
        type="button"
        class="flex size-6 shrink-0 items-center justify-center rounded-sm text-muted-foreground hover:bg-accent hover:text-foreground"
        aria-label="Clear search"
        @click="
          ((text = ''), (filters = { from: [], mentions: [], in: [], has: [] }), input?.focus())
        "
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
          v-for="(s, index) in suggestions"
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
          v-if="!suggestions.length"
          class="px-2 py-2 text-[length:var(--ui-small)] text-muted-foreground"
        >
          Nothing matches “{{ token?.value }}”.
        </li>
      </ul>
      <label
        v-if="dateKey"
        class="mt-1 flex items-center gap-2 border-t [border-top-style:solid] border-border/60 px-2 pt-2 pb-1 text-[length:var(--ui-caption)] text-muted-foreground"
        @pointerdown.stop
        >Pick a day
        <UiInput
          type="date"
          class="h-8 flex-1"
          :max="today()"
          @change="
            (e: Event) =>
              isDay((e.target as HTMLInputElement).value) &&
              choose({ key: dateKey as 'on', value: (e.target as HTMLInputElement).value })
          "
      /></label>
      <template v-if="showRecent">
        <div
          class="mt-1 flex items-center justify-between border-t [border-top-style:solid] border-border/60 px-2 pt-2 pb-1"
        >
          <p class="m-0 text-[length:var(--ui-caption)] font-semibold text-muted-foreground">
            Recent
          </p>
          <AppTooltip text="Clear recent searches"
            ><UiButton
              variant="ghost"
              size="icon-xs"
              aria-label="Clear recent searches"
              @pointerdown.prevent
              @click="emit('forget')"
              ><AppIcon
                name="trash"
                :size="13" /></UiButton
          ></AppTooltip>
        </div>
        <button
          v-for="(entry, index) in recent"
          :key="index"
          type="button"
          class="flex w-full min-w-0 flex-wrap items-center gap-1.5 rounded-sm px-2 py-1.5 text-left text-[length:var(--ui-small)] text-body hover:bg-accent"
          @pointerdown.prevent
          @click="emit('recall', entry)"
        >
          <AppIcon
            name="clock"
            :size="14"
            class="mr-0.5 text-muted-foreground"
          /><span
            v-for="chip in filterChips(entry.filters)"
            :key="`${chip.key}:${chip.value}`"
            class="inline-flex items-center gap-1 rounded-sm bg-secondary px-1.5 py-px text-[length:var(--ui-caption)]"
            ><span class="text-primary">{{ chip.key }}:</span>{{ labels.chipLabel(chip) }}</span
          ><span
            v-if="entry.text"
            class="min-w-0 truncate"
            >{{ entry.text }}</span
          >
        </button>
      </template>
    </div>
  </div>
</template>
