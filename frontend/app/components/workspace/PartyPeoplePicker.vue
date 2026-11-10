<script setup vapor lang="ts">
/** Party selection stays separate from the shared input and list behavior. */
const props = defineProps<{
  results: readonly { value: string; label: string; detail: string; disabled?: boolean }[]
  selected: readonly { value: string; label: string }[]
  loading: boolean
  error: string
  locked?: string
}>()
const emit = defineEmits<{
  search: [query: string | null]
  add: [value: string]
  remove: [value: string]
}>()
const { t } = useI18n()
const query = ref('')
const options = computed(() =>
  props.results.map((person) => ({
    key: person.value,
    label: person.label,
    detail: person.detail,
    disabled: person.disabled,
  })),
)
function choose(value: string) {
  emit('add', value)
  query.value = ''
}
</script>

<template>
  <PeopleCombobox
    v-model="query"
    :options="options"
    :label="t('workspaceUi.party.findPeople')"
    prevent-submit
    @search="emit('search', $event)"
    @select="choose"
  >
    <template #after-input>
      <ul
        v-if="selected.length"
        class="m-0 flex list-none flex-wrap gap-1.5 p-0"
        :aria-label="t('workspaceUi.party.selectedPeople')"
      >
        <li
          v-for="person in selected"
          :key="person.value"
          class="flex items-center gap-1 rounded-md bg-secondary pl-2 text-[length:var(--ui-small)]"
        >
          <span>{{ person.label }}</span>
          <UiButton
            variant="ghost"
            size="icon-sm"
            :aria-label="t('workspaceUi.party.removeSelection', { name: person.label })"
            :disabled="person.value === locked"
            @click="emit('remove', person.value)"
            ><AppIcon
              name="close"
              :size="14"
          /></UiButton>
        </li>
      </ul>
    </template>
    <template #default="{ open, visible }">
      <p
        v-if="open && loading"
        class="text-[length:var(--ui-small)] text-muted-foreground"
        role="status"
      >
        {{ t('workspaceUi.party.searching') }}
      </p>
      <UiAlert
        v-else-if="open && error"
        as="p"
        variant="destructive"
        presentation="inline"
        >{{ error }}</UiAlert
      >
      <p
        v-else-if="open && !visible"
        class="text-[length:var(--ui-small)] text-muted-foreground"
      >
        {{ t('workspaceUi.party.noMatches') }}
      </p>
    </template>
  </PeopleCombobox>
</template>
