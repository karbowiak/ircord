<script setup vapor lang="ts">
import {
  filterChips,
  filterKeys,
  searchHasKinds,
  withChip,
  withoutChip,
  type FilterChip,
  type RecentSearch,
  type FilterKey,
  type SearchFilters,
} from '../../../shared/utils/search-filter'
import type { TagChip, TagKey, TagSuggestion } from '../shared/TagQueryField.vue'

/**
 * Message search's field: `from:`, `mentions:`, `in:`, `has:`, the date keys and `author:` as
 * chips in a TagQueryField, with people, conversations and content types to choose from.
 * Focused and empty, it can also offer the filters and recent searches.
 */
const props = withDefaults(
  defineProps<{
    recent?: RecentSearch[]
    placeholder?: string
    label?: string
    /** Filter keys this search doesn't offer (a conversation's own search has no `in:`). */
    exclude?: FilterKey[]
    /** Offer filters and recent searches when focused and empty. Off where the page shows them. */
    menu?: boolean
  }>(),
  { recent: () => [], placeholder: undefined, label: undefined, exclude: () => [], menu: true },
)
const { t } = useI18n()
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
const field = ref<{ focus: () => void; start: (key: string) => void } | null>(null)

// The main filters are in the idle menu; before:, after: and author: work when typed.
const shortlist: string[] = ['from', 'mentions', 'in', 'has', 'on']
const keys = computed<TagKey[]>(() =>
  filterKeys
    .filter((k) => !props.exclude.includes(k.key))
    .map((k) => ({
      key: k.key,
      icon: k.icon,
      label: t(`collectionsUi.search.${k.key}Label`),
      hint: t(`collectionsUi.search.${k.key}Hint`),
      date: k.key === 'before' || k.key === 'after' || k.key === 'on',
      menu: shortlist.includes(k.key),
    })),
)
const chips = computed(() => filterChips(filters.value))
const kindLabel = (kind: string) =>
  t(`collectionsUi.search.kind${kind[0]!.toUpperCase()}${kind.slice(1)}`)

function suggest(key: string, q: string): TagSuggestion[] {
  switch (key) {
    case 'from':
    case 'mentions': {
      const chosen = filters.value[key]
      return [...(key === 'from' && chat.self ? [chat.self] : []), ...labels.people.value]
        .filter(
          (m) =>
            !chosen.includes(m.id) &&
            (!q || `${m.displayName} ${m.nick}`.toLowerCase().includes(q)),
        )
        .slice(0, 8)
        .map((m) => ({
          id: `${key}-${m.id}`,
          label:
            m.id === 'you'
              ? t('collectionsUi.search.yourselfWithName', { name: m.displayName })
              : m.displayName,
          detail: `@${m.nick}`,
          avatar: m.id,
          value: m.id,
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
          icon: c.party ? 'users' : c.kind === 'channel' ? 'hash' : 'message',
          value: c.id,
        }))
    case 'has':
      return searchHasKinds
        .filter(
          (k) =>
            !filters.value.has.includes(k.value) && kindLabel(k.value).toLowerCase().includes(q),
        )
        .map((k) => ({
          id: `has-${k.value}`,
          label: kindLabel(k.value),
          icon: k.icon,
          value: k.value,
        }))
    case 'author':
      return (['user', 'bot'] as const)
        .map((v) => ({
          id: `author-${v}`,
          label: t(v === 'bot' ? 'collectionsUi.search.bots' : 'collectionsUi.search.people'),
          icon: v === 'bot' ? 'brand' : 'user',
          value: v,
        }))
        .filter((s) => !q || s.label.toLowerCase().includes(q))
    default:
      return []
  }
}
const add = (chip: TagChip) => (filters.value = withChip(filters.value, chip as FilterChip))
const remove = (chip: TagChip) => (filters.value = withoutChip(filters.value, chip as FilterChip))
defineExpose({
  focus: () => field.value?.focus(),
  start: (key: string) => field.value?.start(key),
})
</script>
<template>
  <TagQueryField
    ref="field"
    v-model:text="text"
    :keys="keys"
    :chips="chips"
    :suggest="suggest"
    :chip-label="(chip) => labels.chipLabel(chip as FilterChip)"
    :placeholder="placeholder"
    :label="label"
    :menu="menu"
    :more-label="t('collectionsUi.search.moreFilters')"
    :more-hint="t('collectionsUi.search.moreFiltersHint')"
    @add="add"
    @remove="remove"
    @clear="filters = { from: [], mentions: [], in: [], has: [] }"
    @keydown="emit('keydown', $event)"
    @more="emit('more')"
  >
    <template
      v-if="recent.length"
      #idle
    >
      <div
        class="mt-1 flex items-center justify-between border-t [border-top-style:solid] border-border/60 px-2 pt-2 pb-1"
      >
        <p class="m-0 text-[length:var(--ui-caption)] font-semibold text-muted-foreground">
          {{ t('collectionsUi.search.recent') }}
        </p>
        <UiButton
          variant="ghost"
          size="icon-xs"
          :aria-label="t('collectionsUi.search.clearRecent')"
          :title="t('collectionsUi.search.clearRecent')"
          @pointerdown.prevent
          @click="emit('forget')"
          ><AppIcon
            name="trash"
            :size="13"
        /></UiButton>
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
  </TagQueryField>
</template>
