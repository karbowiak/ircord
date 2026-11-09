<script setup lang="ts">
import { emptyFilters, type SearchFilters, type SearchHas } from '../../shared/utils/search-filter'

/**
 * Every message search filter in one form, for choosing several people, channels or types at
 * once. Changes apply together; Cancel leaves the search as it was.
 */
/** `scoped`: searching one conversation, so there is no choice of where. */
const props = defineProps<{ filters: SearchFilters; scoped?: boolean }>()
const emit = defineEmits<{ close: []; apply: [filters: SearchFilters] }>()
const { t, n } = useI18n()
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
  !ids.length
    ? none
    : ids.length === 1
      ? name(ids[0]!)
      : t('collectionsUi.searchFilters.chosenCount', { count: n(ids.length) }, ids.length)
function apply(close: () => void) {
  const result: SearchFilters = structuredClone(toRaw(draft))
  if (when.value !== 'on') delete result.on
  if (when.value !== 'between') {
    delete result.before
    delete result.after
  }
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
const today = (() => {
  const date = new Date()
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
})()
</script>
<template>
  <AppDialog
    :title="t('collectionsUi.searchFilters.title')"
    @close="emit('close')"
  >
    <template #default="{ close }">
      <div
        class="search-filters grid max-h-[min(70dvh,calc(560px*var(--interface-scale,1)))] gap-4 overflow-y-auto px-5 pb-4"
      >
        <AppField
          :label="t('collectionsUi.searchFilters.from')"
          :help="t('collectionsUi.searchFilters.fromHelp')"
        >
          <FilterMenu
            v-model="draft.from"
            block
            :label="t('collectionsUi.searchFilters.from')"
            :display="many(draft.from, labels.memberName, t('collectionsUi.searchFilters.anyone'))"
            :options="labels.personOptions.value"
            searchable
            :search-placeholder="t('collectionsUi.searchFilters.findPerson')"
            :clear-label="t('collectionsUi.searchFilters.anyone')"
          />
        </AppField>
        <AppField
          :label="t('collectionsUi.searchFilters.has')"
          :help="t('collectionsUi.searchFilters.hasHelp')"
        >
          <FilterMenu
            v-model="has"
            block
            :label="t('collectionsUi.searchFilters.has')"
            :display="
              !draft.has.length
                ? t('collectionsUi.searchFilters.anyContent')
                : draft.has
                    .map((v) => labels.hasOptions.find((o) => o.value === v)?.label)
                    .join(', ')
            "
            :options="labels.hasOptions"
            :clear-label="t('collectionsUi.searchFilters.anyContent')"
          />
        </AppField>
        <AppField
          :label="t('collectionsUi.searchFilters.mentions')"
          :help="t('collectionsUi.searchFilters.mentionsHelp')"
        >
          <FilterMenu
            v-model="draft.mentions"
            block
            :label="t('collectionsUi.searchFilters.mentions')"
            :display="
              many(
                draft.mentions,
                labels.memberName,
                t('collectionsUi.searchFilters.anyoneOrNoOne'),
              )
            "
            :options="labels.personOptions.value"
            searchable
            :search-placeholder="t('collectionsUi.searchFilters.findPerson')"
            :clear-label="t('collectionsUi.searchFilters.clear')"
          />
        </AppField>
        <AppField
          v-if="!scoped"
          :label="t('collectionsUi.searchFilters.in')"
          :help="t('collectionsUi.searchFilters.inHelp')"
        >
          <FilterMenu
            v-model="draft.in"
            block
            :label="t('collectionsUi.searchFilters.in')"
            :display="
              many(
                draft.in,
                (id) => labels.conversationOptions.value.find((o) => o.value === id)?.label ?? '',
                t('collectionsUi.searchFilters.everywhere'),
              )
            "
            :options="labels.conversationOptions.value"
            searchable
            :search-placeholder="t('collectionsUi.searchFilters.findChannelOrPerson')"
            :clear-label="t('collectionsUi.searchFilters.everywhere')"
          />
        </AppField>
        <fieldset class="m-0 grid min-w-0 gap-2 border-0 p-0">
          <legend class="mb-2 p-0 text-[length:var(--ui-small)] font-medium text-foreground">
            {{ t('collectionsUi.searchFilters.date') }}
          </legend>
          <AppSegmented
            v-model="when"
            :label="t('collectionsUi.searchFilters.whenSent')"
            :options="[
              { value: 'any', label: t('collectionsUi.searchFilters.anyTime') },
              { value: 'on', label: t('collectionsUi.searchFilters.onDay') },
              { value: 'between', label: t('collectionsUi.searchFilters.between') },
            ]"
          />
          <UiInput
            v-if="when === 'on'"
            v-model="draft.on"
            type="date"
            :max="today"
            :aria-label="t('collectionsUi.searchFilters.sentOn')"
          />
          <div
            v-else-if="when === 'between'"
            class="grid grid-cols-2 gap-2"
          >
            <UiInput
              v-model="draft.after"
              type="date"
              :max="draft.before || today"
              :aria-label="t('collectionsUi.searchFilters.sentAfter')"
            />
            <UiInput
              v-model="draft.before"
              type="date"
              :min="draft.after || undefined"
              :max="today"
              :aria-label="t('collectionsUi.searchFilters.sentBefore')"
            />
          </div>
        </fieldset>
        <fieldset class="m-0 grid min-w-0 gap-2 border-0 p-0">
          <legend class="mb-2 p-0 text-[length:var(--ui-small)] font-medium text-foreground">
            {{ t('collectionsUi.searchFilters.authorType') }}
          </legend>
          <AppSegmented
            v-model="author"
            :label="t('collectionsUi.searchFilters.authorType')"
            :options="[
              { value: 'any', label: t('collectionsUi.searchFilters.anyone') },
              { value: 'user', label: t('collectionsUi.search.people') },
              { value: 'bot', label: t('collectionsUi.search.bots') },
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
          >{{ t('collectionsUi.searchFilters.clearFilters') }}</UiButton
        >
        <UiButton
          variant="secondary"
          class="ml-auto"
          @click="close"
          >{{ t('common.cancel') }}</UiButton
        >
        <UiButton @click="apply(close)">{{
          t('collectionsUi.searchFilters.applyFilters')
        }}</UiButton>
      </div>
    </template>
  </AppDialog>
</template>
