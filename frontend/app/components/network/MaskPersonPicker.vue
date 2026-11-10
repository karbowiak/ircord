<script setup vapor lang="ts">
import type { Member } from '../../../shared/types/chat'
import { personSuggestions } from '../../../shared/utils/mask-builder'

const props = defineProps<{
  here: readonly Member[]
  recent: readonly Member[]
  channel: string
  caseMapping?: string
  networkId: string
  conversationId: string
}>()
const emit = defineEmits<{ choose: [person: Member] }>()
const { t } = useI18n()
const chat = useChatStore()
const lookup = usePeopleLookup(() => ({
  networkId: props.networkId,
  conversationId: props.conversationId,
  purpose: 'mask',
  limit: 20,
}))
const query = ref('')
const picker = ref<{ focus: () => void } | null>(null)
const matches = computed(
  () =>
    personSuggestions(
      props.here,
      [
        ...props.recent,
        ...(lookup.state.value.page?.items ?? []).flatMap((person) => {
          const member =
            person.memberId && person.memberId !== 'you' ? chat.member(person.memberId) : undefined
          return member ? [member] : []
        }),
      ],
      query.value,
      8,
      props.caseMapping,
    ) as {
      person: Member
      recent: boolean
    }[],
)
const options = computed(() =>
  matches.value.map(({ person }) => ({ key: person.id, label: person.nick })),
)
const match = (key: string) => matches.value.find((item) => item.person.id === key)
function choose(key: string) {
  const item = match(key)
  if (!item) return
  emit('choose', item.person)
  query.value = ''
}
const address = (m: Member) => (m.username && m.host ? `${m.username}@${m.host}` : '')
defineExpose({ focus: () => picker.value?.focus() })
</script>

<template>
  <PeopleCombobox
    ref="picker"
    v-model="query"
    :options="options"
    :label="t('collectionsUi.channel.maskPerson.find')"
    :placeholder="t('collectionsUi.channel.maskPerson.findIn', { channel })"
    popup
    auto-highlight
    prevent-submit
    @select="choose"
    @search="lookup.search"
  >
    <template #option="{ option }">
      <span
        v-if="match(option.key)"
        class="flex items-center gap-2 min-h-10"
      >
        <UserAvatar
          :member="match(option.key)!.person"
          size="small"
        />
        <span class="grid min-w-0 gap-[1px]">
          <span class="font-[550] text-foreground wrap-anywhere">{{ option.label }}</span>
          <span class="text-[0.82em] text-muted-foreground wrap-anywhere"
            ><template v-if="match(option.key)!.recent">{{
              t('collectionsUi.channel.maskPerson.recentIn', { channel })
            }}</template
            ><template v-if="match(option.key)!.recent && address(match(option.key)!.person)">
              · </template
            >{{ address(match(option.key)!.person) }}</span
          >
        </span>
      </span>
    </template>
    <template #default="{ open }">
      <FieldHint
        v-if="
          open && (lookup.state.value.error || lookup.state.value.page?.status === 'unavailable')
        "
        >{{ t('collectionsUi.channel.maskPerson.lookupUnavailable') }}</FieldHint
      >
      <FieldHint v-else-if="open && lookup.state.value.page?.status === 'partial'">{{
        t('collectionsUi.channel.maskPerson.lookupPartial')
      }}</FieldHint>
    </template>
  </PeopleCombobox>
</template>
