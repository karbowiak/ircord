<script setup lang="ts">
import { emptyFilters, type SearchFilters, type SearchHas } from '../../shared/utils/search-filter'

/**
 * Every message search filter in one form, for choosing several people, channels or types at
 * once. Changes apply together; Cancel leaves the search as it was.
 */
/** `scoped`: searching one conversation, so there is no choice of where. */
const props = defineProps<{ filters: SearchFilters; scoped?: boolean }>()
const emit = defineEmits<{ close: []; apply: [filters: SearchFilters] }>()
const labels = useSearchFilters()
const draft = reactive<SearchFilters>(structuredClone(toRaw(props.filters)))
const when = ref<'any' | 'on' | 'between'>(
  draft.on ? 'on' : draft.before || draft.after ? 'between' : 'any',
)
const author = computed({
  get: () => draft.author ?? 'any',
  set: (value: 'any' | 'user' | 'bot') => (draft.author = value === 'any' ? undefined : value),
})
const has = computed({
  get: () => draft.has as string[],
  set: (value: string[]) => (draft.has = value as SearchHas[]),
})
const many = (ids: string[], name: (id: string) => string, none: string) =>
  !ids.length ? none : ids.length === 1 ? name(ids[0]!) : `${ids.length} chosen`
function apply(close: () => void) {
  const result: SearchFilters = structuredClone(toRaw(draft))
  if (when.value !== 'on') delete result.on
  if (when.value !== 'between') (delete result.before, delete result.after)
  emit('apply', result)
  close()
}
function clear() {
  Object.assign(draft, emptyFilters(), {
    before: undefined,
    after: undefined,
    on: undefined,
    author: undefined,
  })
  when.value = 'any'
}
const today = new Date().toISOString().slice(0, 10)
</script>
<template>
  <AppDialog
    title="Filters"
    @close="emit('close')"
  >
    <template #default="{ close }">
      <div
        class="search-filters grid max-h-[min(70dvh,calc(560px*var(--interface-scale,1)))] gap-4 overflow-y-auto px-5 pb-4"
      >
        <AppField
          label="From"
          help="Sent by any of these people."
        >
          <FilterMenu
            v-model="draft.from"
            block
            label="From"
            :display="many(draft.from, labels.memberName, 'Anyone')"
            :options="labels.personOptions.value"
            searchable
            search-placeholder="Find a person"
            clear-label="Anyone"
          />
        </AppField>
        <AppField
          label="Has"
          help="Includes any of these kinds of content."
        >
          <FilterMenu
            v-model="has"
            block
            label="Has"
            :display="
              !draft.has.length
                ? 'Any content'
                : draft.has
                    .map((v) => labels.hasOptions.find((o) => o.value === v)?.label)
                    .join(', ')
            "
            :options="labels.hasOptions"
            clear-label="Any content"
          />
        </AppField>
        <AppField
          label="Mentions"
          help="Names any of these people."
        >
          <FilterMenu
            v-model="draft.mentions"
            block
            label="Mentions"
            :display="many(draft.mentions, labels.memberName, 'Anyone or no one')"
            :options="labels.personOptions.value"
            searchable
            search-placeholder="Find a person"
            clear-label="Clear"
          />
        </AppField>
        <AppField
          v-if="!scoped"
          label="In"
          help="Posted in any of these channels or direct messages."
        >
          <FilterMenu
            v-model="draft.in"
            block
            label="In"
            :display="
              many(
                draft.in,
                (id) => labels.conversationOptions.value.find((o) => o.value === id)?.label ?? '',
                'Everywhere',
              )
            "
            :options="labels.conversationOptions.value"
            searchable
            search-placeholder="Find a channel or person"
            clear-label="Everywhere"
          />
        </AppField>
        <fieldset class="m-0 grid min-w-0 gap-2 border-0 p-0">
          <legend class="mb-2 p-0 text-[length:var(--ui-small)] font-medium text-foreground">
            Date
          </legend>
          <AppSegmented
            v-model="when"
            label="When it was sent"
            :options="[
              { value: 'any', label: 'Any time' },
              { value: 'on', label: 'On a day' },
              { value: 'between', label: 'Between' },
            ]"
          />
          <UiInput
            v-if="when === 'on'"
            v-model="draft.on"
            type="date"
            :max="today"
            aria-label="Sent on"
          />
          <div
            v-else-if="when === 'between'"
            class="grid grid-cols-2 gap-2"
          >
            <UiInput
              v-model="draft.after"
              type="date"
              :max="draft.before || today"
              aria-label="Sent after"
            />
            <UiInput
              v-model="draft.before"
              type="date"
              :min="draft.after || undefined"
              :max="today"
              aria-label="Sent before"
            />
          </div>
        </fieldset>
        <fieldset class="m-0 grid min-w-0 gap-2 border-0 p-0">
          <legend class="mb-2 p-0 text-[length:var(--ui-small)] font-medium text-foreground">
            Author type
          </legend>
          <AppSegmented
            v-model="author"
            label="Author type"
            :options="[
              { value: 'any', label: 'Anyone' },
              { value: 'user', label: 'People' },
              { value: 'bot', label: 'Bots' },
            ]"
          />
        </fieldset>
      </div>
      <div
        class="flex items-center gap-2 border-t [border-top-style:solid] border-border px-5 py-3"
      >
        <UiButton
          variant="link"
          size="text"
          class="-ml-2"
          @click="clear"
          >Clear filters</UiButton
        >
        <UiButton
          variant="secondary"
          class="ml-auto"
          @click="close"
          >Cancel</UiButton
        >
        <UiButton @click="apply(close)">Apply filters</UiButton>
      </div>
    </template>
  </AppDialog>
</template>
