import type { FilterOption } from '../../components/shared/FilterMenu.vue'
import type { Conversation, Member } from '../../../shared/types/chat'
import { searchHasKinds, type FilterChip } from '../../../shared/utils/search-filter'

/** Names and choices for message search filters: people, conversations, kinds and dates. */
export function useSearchFilters() {
  const { t } = useI18n()
  const chat = useChatStore()
  const { day } = useTimeFormat()
  const networks = computed(() => chat.data?.networks ?? [])
  const networkName = (id: string) => networks.value.find((n) => n.id === id)?.name ?? ''
  const conversationLabel = (c: Conversation) =>
    c.kind === 'channel' ? `#${c.name}` : `@${chat.directPartner(c)?.nick ?? c.name}`
  const people = computed(() => (chat.data?.members ?? []).filter((m) => m.id !== 'you'))
  const memberName = (id: string) =>
    id === 'you'
      ? (chat.self?.displayName ?? t('collectionsUi.search.yourself'))
      : (chat.member(id)?.displayName ?? t('collectionsUi.search.someone'))
  const dayLabel = (value: string) => day(new Date(`${value}T12:00:00`).toISOString())

  function chipLabel(chip: FilterChip) {
    switch (chip.key) {
      case 'from':
      case 'mentions':
        return memberName(chip.value)
      case 'in': {
        const c = chat.data?.conversations.find((x) => x.id === chip.value)
        return c ? conversationLabel(c) : t('collectionsUi.search.closedConversation')
      }
      case 'has':
        return kindLabel(chip.value)
      case 'author':
        return t(chip.value === 'bot' ? 'collectionsUi.search.bot' : 'collectionsUi.search.person')
      default:
        return dayLabel(chip.value)
    }
  }

  const kindLabel = (kind: string) =>
    t(`collectionsUi.search.kind${kind[0]!.toUpperCase()}${kind.slice(1)}`)
  const personOptions = computed<FilterOption[]>(() =>
    people.value.map((m: Member) => ({
      value: m.id,
      label: t('collectionsUi.search.personOption', { name: m.displayName, nick: `@${m.nick}` }),
    })),
  )
  const conversationOptions = computed<FilterOption[]>(() =>
    (chat.data?.conversations ?? []).map((c) => ({
      value: c.id,
      label: conversationLabel(c),
      group: networks.value.length > 1 ? networkName(c.networkId) : undefined,
    })),
  )
  const hasOptions = computed<FilterOption[]>(() =>
    searchHasKinds.map((kind) => ({ ...kind, label: kindLabel(kind.value) })),
  )
  return {
    people,
    memberName,
    conversationLabel,
    networkName,
    chipLabel,
    dayLabel,
    personOptions,
    conversationOptions,
    get hasOptions() {
      return hasOptions.value
    },
  }
}
