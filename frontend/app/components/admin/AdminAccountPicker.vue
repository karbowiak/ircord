<script setup vapor lang="ts">
import type { components } from '../../../shared/generated/api'
import { createPeopleCache, createPeopleSearch } from '../../../shared/utils/people-lookup'

type Match = components['schemas']['IdentityMatch']
type User = NonNullable<Match['user']>
const props = defineProps<{ label: string; action: string; busy?: boolean }>()
const emit = defineEmits<{ choose: [user: User] }>()
const { t } = useI18n()
const errorMessage = (value: unknown) => errorText(value, (key) => t(key))
const query = ref('')
const matches = ref<Match[]>([])
const highlighted = ref<string>()
const error = ref<unknown>()
const loading = ref(false)
const auth = useAuthStore()
const refused = useAdministrationRefusal()
const keyOf = (match: Match) =>
  `${match.user?.id ?? ''}:${match.account ?? ''}:${match.nickname ?? ''}`
const describe = (m: Match) =>
  m.nickname && m.account && m.nickname.toLowerCase() !== m.account.toLowerCase()
    ? `${m.nickname} → ${m.account}`
    : (m.account ?? m.nickname ?? '')
const options = computed(() =>
  matches.value.map((item) => ({
    key: keyOf(item),
    label: describe(item),
    disabled: !item.eligible || !item.user,
  })),
)
const chosen = computed(
  () => matches.value.find((item) => keyOf(item) === highlighted.value && item.eligible)?.user,
)
// Owner-only results stay in a private cache and never enter the public Person DTO.
const cache = createPeopleCache<Match>(30)
const searcher = createPeopleSearch(
  cache,
  async (scope, signal) => ({
    items: (await apiClient().administrationIdentities(scope.query, signal)).items,
    source: 'known',
    status: 'ready',
    more: false,
  }),
  (state) => {
    matches.value = state.page?.items ?? []
    loading.value = state.loading
    error.value = state.error
    const first = matches.value.find((item) => item.eligible && item.user)
    highlighted.value = first ? keyOf(first) : undefined
    if (state.error) refused(state.error)
  },
)
function search(value: string) {
  searcher.search(
    value.trim()
      ? {
          accountId: auth.account?.id ?? '',
          networkId: 'administration',
          purpose: 'services',
          query: value.trim(),
          limit: 50,
        }
      : null,
  )
}
watch(query, search)
watch(
  () => auth.account?.id,
  () => {
    searcher.cancel()
    cache.clear()
    search(query.value)
  },
  { flush: 'sync' },
)
onBeforeUnmount(() => {
  searcher.cancel()
  cache.clear()
})
function choose(key = highlighted.value) {
  const match = matches.value.find((item) => keyOf(item) === key)
  if (!match?.eligible || !match.user || props.busy) return
  emit('choose', match.user)
  query.value = ''
}
function highlight(key?: string) {
  if (key) highlighted.value = key
}
</script>

<template>
  <div class="admin-picker">
    <div class="admin-picker-row flex gap-2 items-start">
      <PeopleCombobox
        v-model="query"
        class="flex-1"
        :options="options"
        :label="label"
        auto-highlight
        prevent-submit
        @select="choose"
        @highlight="highlight"
      >
        <template #option="{ option }">
          <span class="grid gap-px"
            ><strong>{{ option.label }}</strong
            ><small
              v-if="matches.find((item) => keyOf(item) === option.key)?.note"
              class="text-muted-foreground"
              >{{ matches.find((item) => keyOf(item) === option.key)?.note }}</small
            ><small
              v-else-if="matches.find((item) => keyOf(item) === option.key)?.user?.owner"
              class="text-muted-foreground"
              >{{ t('administration.alreadyOwner') }}</small
            ></span
          >
        </template>
      </PeopleCombobox>
      <UiButton
        :disabled="!chosen || busy"
        @click="choose()"
        >{{ action }}</UiButton
      >
    </div>
    <UiAlert
      v-if="error"
      as="p"
      variant="destructive"
      presentation="inline"
      class="form-error mt-3"
      >{{ errorMessage(error) }}</UiAlert
    >
    <FieldHint v-else-if="query.trim() && !matches.length && !loading">{{
      t('administration.noAccountMatch', { query: query.trim() })
    }}</FieldHint>
  </div>
</template>
